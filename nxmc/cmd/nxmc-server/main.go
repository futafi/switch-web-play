package main

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
	xdraw "golang.org/x/image/draw"
	"web-switch/nxmc"
)

//go:embed web
var webFS embed.FS

type InputMessage struct {
	Buttons uint16 `json:"buttons"`
	Hat     byte   `json:"hat"`
	LX      byte   `json:"lx"`
	LY      byte   `json:"ly"`
	RX      byte   `json:"rx"`
	RY      byte   `json:"ry"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type streamSettings struct {
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Bitrate int    `json:"bitrate"`
	Codec   string `json:"codec"`
}

type screenshotCache struct {
	mu        sync.RWMutex
	jpeg      []byte
	updatedAt time.Time
}

var buttonMap = map[string]uint16{
	"A": nxmc.ButtonA, "B": nxmc.ButtonB, "X": nxmc.ButtonX, "Y": nxmc.ButtonY,
	"L": nxmc.ButtonL, "R": nxmc.ButtonR, "ZL": nxmc.ButtonZL, "ZR": nxmc.ButtonZR,
	"Plus": nxmc.ButtonPlus, "Minus": nxmc.ButtonMinus,
	"LClick": nxmc.ButtonLClick, "RClick": nxmc.ButtonRClick,
	"Home": nxmc.ButtonHome, "Capture": nxmc.ButtonCapture,
}

var hatMap = map[string]byte{
	"up": nxmc.HatUp, "upright": nxmc.HatUpRight, "right": nxmc.HatRight,
	"downright": nxmc.HatDownRight, "down": nxmc.HatDown, "downleft": nxmc.HatDownLeft,
	"left": nxmc.HatLeft, "upleft": nxmc.HatUpLeft, "center": nxmc.HatCenter,
}

func gstdDo(ctx context.Context, method, baseURL, path string, params url.Values) error {
	u := strings.TrimRight(baseURL, "/") + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, method, u, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("gstd %s %s: status %d", method, path, resp.StatusCode)
	}
	return nil
}

func gstdSetProperty(ctx context.Context, baseURL, element, property, value string) error {
	return gstdDo(ctx, http.MethodPut, baseURL,
		fmt.Sprintf("/pipelines/cam/elements/%s/properties/%s", element, property),
		url.Values{"name": {value}},
	)
}

func gstdSetState(ctx context.Context, baseURL, state string) error {
	return gstdDo(ctx, http.MethodPut, baseURL, "/pipelines/cam/state", url.Values{"name": {state}})
}

func gstdDeletePipeline(ctx context.Context, baseURL string) error {
	return gstdDo(ctx, http.MethodDelete, baseURL, "/pipelines", url.Values{"name": {"cam"}})
}

func gstdCreatePipeline(ctx context.Context, baseURL, description string) error {
	return gstdDo(ctx, http.MethodPost, baseURL, "/pipelines", url.Values{
		"name":        {"cam"},
		"description": {description},
	})
}

func captureCapsForHeight(height int) string {
	if height >= 1080 {
		return "image/jpeg,width=1920,height=1080,framerate=30/1"
	}
	return "image/jpeg,width=1280,height=720,framerate=30/1"
}

func encoderElements(codec string) (string, string) {
	if codec == "h264" {
		return "vaapih264enc name=encoder tune=low-power rate-control=cbr bitrate=%d keyframe-period=30",
			"h264parse config-interval=1"
	}
	return "vaapih265enc name=encoder tune=low-power rate-control=cbr bitrate=%d keyframe-period=30",
		"h265parse config-interval=1"
}

func buildPipelineDescription(settings streamSettings, audioDevice, rtspHost string, screenshotFPS, screenshotPort int) string {
	if rtspHost == "" {
		rtspHost = "mediamtx"
	}
	if screenshotFPS <= 0 {
		screenshotFPS = 10
	}
	if screenshotPort <= 0 {
		screenshotPort = 9001
	}

	audioSrc := "alsasrc"
	if audioDevice != "" {
		audioSrc += " device=" + audioDevice
	}

	encTmpl, parser := encoderElements(settings.Codec)
	enc := fmt.Sprintf(encTmpl, settings.Bitrate)

	return fmt.Sprintf(
		"v4l2src device=/dev/video0 "+
			"! capsfilter name=capture caps=%s "+
			"! jpegdec "+
			"! videoconvert "+
			"! videoscale "+
			"! capsfilter name=scaler caps=video/x-raw,width=%d,height=%d "+
			"! tee name=video_split "+
			"video_split. "+
			"! queue "+
			"! %s "+
			"! %s "+
			"! rtspclientsink location=rtsp://%s:8554/cam name=sink "+
			"%s "+
			"! audioconvert "+
			"! audioresample "+
			"! opusenc bitrate=128000 frame-size=10 "+
			"! sink. "+
			"video_split. "+
			"! queue leaky=downstream max-size-buffers=1 "+
			"! videorate "+
			"! video/x-raw,framerate=%d/1 "+
			"! jpegenc quality=95 "+
			"! multipartmux boundary=frame "+
			"! tcpserversink host=0.0.0.0 port=%d sync=false",
		captureCapsForHeight(settings.Height),
		settings.Width,
		settings.Height,
		enc,
		parser,
		rtspHost,
		audioSrc,
		screenshotFPS,
		screenshotPort,
	)
}

func recreateGstdPipeline(ctx context.Context, baseURL string, settings streamSettings, audioDevice, rtspHost string, screenshotFPS, screenshotPort int) error {
	if err := gstdSetState(ctx, baseURL, "null"); err != nil {
		return err
	}
	if err := gstdDeletePipeline(ctx, baseURL); err != nil {
		return err
	}
	time.Sleep(2 * time.Second)
	if err := gstdCreatePipeline(ctx, baseURL, buildPipelineDescription(settings, audioDevice, rtspHost, screenshotFPS, screenshotPort)); err != nil {
		return err
	}
	if err := gstdSetState(ctx, baseURL, "playing"); err != nil {
		return err
	}
	return nil
}

func envInt(name string, fallback int) int {
	value := os.Getenv(name)
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}

func defaultRTSPURL() string {
	host := os.Getenv("RTSP_HOST")
	if host == "" {
		host = "mediamtx"
	}
	return fmt.Sprintf("rtsp://%s:8554/cam", host)
}

func defaultScreenshotStreamAddr() string {
	if addr := os.Getenv("SCREENSHOT_STREAM_ADDR"); addr != "" {
		return addr
	}
	rtspHost := os.Getenv("RTSP_HOST")
	if rtspHost == "127.0.0.1" || rtspHost == "localhost" {
		return "127.0.0.1:9001"
	}
	return "gstreamer:9001"
}

func readJPEGFrame(r *bufio.Reader) ([]byte, error) {
	for {
		b, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if b != 0xff {
			continue
		}
		next, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		if next == 0xd8 {
			frame := []byte{0xff, 0xd8}
			prev := byte(0xd8)
			for {
				b, err := r.ReadByte()
				if err != nil {
					return nil, err
				}
				frame = append(frame, b)
				if prev == 0xff && b == 0xd9 {
					return frame, nil
				}
				prev = b
			}
		}
	}
}

func startScreenshotCache(ctx context.Context, streamAddr string) *screenshotCache {
	cache := &screenshotCache{}

	go func() {
		for ctx.Err() == nil {
			conn, err := net.DialTimeout("tcp", streamAddr, 3*time.Second)
			if err != nil {
				log.Printf("screenshot stream connect %s: %v", streamAddr, err)
				time.Sleep(time.Second)
				continue
			}
			log.Printf("screenshot stream connected: %s", streamAddr)

			reader := bufio.NewReaderSize(conn, 1<<20)
			for ctx.Err() == nil {
				frame, err := readJPEGFrame(reader)
				if err != nil {
					log.Printf("screenshot stream read: %v", err)
					break
				}
				cache.mu.Lock()
				cache.jpeg = frame
				cache.updatedAt = time.Now()
				cache.mu.Unlock()
			}

			conn.Close()
			if ctx.Err() == nil {
				time.Sleep(time.Second)
			}
		}
	}()

	return cache
}

func (c *screenshotCache) latest(maxAge time.Duration) ([]byte, time.Time, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.jpeg) == 0 || time.Since(c.updatedAt) > maxAge {
		return nil, time.Time{}, false
	}
	frame := append([]byte(nil), c.jpeg...)
	return frame, c.updatedAt, true
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func parseResizeDimensions(width, height string, src image.Rectangle) (int, int, error) {
	srcW := src.Dx()
	srcH := src.Dy()
	if srcW <= 0 || srcH <= 0 {
		return 0, 0, fmt.Errorf("invalid source size")
	}

	dstW, dstH := 0, 0
	var err error
	if width != "" {
		dstW, err = strconv.Atoi(width)
		if err != nil || dstW <= 0 {
			return 0, 0, fmt.Errorf("invalid width")
		}
	}
	if height != "" {
		dstH, err = strconv.Atoi(height)
		if err != nil || dstH <= 0 {
			return 0, 0, fmt.Errorf("invalid height")
		}
	}

	switch {
	case dstW > 0 && dstH > 0:
		return dstW, dstH, nil
	case dstW > 0:
		return dstW, maxInt(1, dstW*srcH/srcW), nil
	case dstH > 0:
		return maxInt(1, dstH*srcW/srcH), dstH, nil
	default:
		return srcW, srcH, nil
	}
}

func resizeJPEG(jpegData []byte, width, height string) ([]byte, error) {
	src, err := jpeg.Decode(bytes.NewReader(jpegData))
	if err != nil {
		return nil, err
	}

	dstW, dstH, err := parseResizeDimensions(width, height, src.Bounds())
	if err != nil {
		return nil, err
	}
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	xdraw.ApproxBiLinear.Scale(dst, dst.Bounds(), src, src.Bounds(), xdraw.Over, nil)

	var out bytes.Buffer
	if err := jpeg.Encode(&out, dst, &jpeg.Options{Quality: 95}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func captureScreenshotOneShot(ctx context.Context, rtspURL, width, height string) ([]byte, error) {
	args := []string{
		"-loglevel", "error",
		"-threads", "1",
		"-probesize", "32",
		"-analyzeduration", "0",
		"-fflags", "nobuffer",
		"-rtsp_transport", "tcp",
		"-skip_frame", "nokey",
		"-i", rtspURL,
		"-frames:v", "1",
	}

	if width != "" || height != "" {
		sw, sh := "-1", "-1"
		if width != "" {
			sw = width
		}
		if height != "" {
			sh = height
		}
		args = append(args, "-vf", fmt.Sprintf("scale=%s:%s", sw, sh))
	}

	args = append(args, "-f", "image2", "-c:v", "mjpeg", "-q:v", "2", "pipe:1")
	return exec.CommandContext(ctx, "ffmpeg", args...).Output()
}

func main() {
	device := flag.String("device", "/dev/ttyUSB0", "serial device path")
	baud := flag.Int("baud", 115200, "baud rate")
	addr := flag.String("addr", ":9000", "listen address")
	gstdURL := flag.String("gstd-url", "http://gstreamer:5000", "gstd HTTP API base URL")
	rtspURL := flag.String("rtsp-url", defaultRTSPURL(), "RTSP stream URL")
	screenshotStreamAddr := flag.String("screenshot-stream-addr", defaultScreenshotStreamAddr(), "JPEG screenshot stream address")
	flag.Parse()
	audioDevice := os.Getenv("AUDIO_DEVICE")
	rtspHost := os.Getenv("RTSP_HOST")
	screenshotFPS := envInt("SCREENSHOT_CACHE_FPS", 10)
	screenshotPort := envInt("SCREENSHOT_STREAM_PORT", 9001)
	appCtx, stopApp := context.WithCancel(context.Background())
	defer stopApp()

	ctrl, err := nxmc.Open(*device, *baud)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("serial: %s @ %d", *device, *baud)

	var mu sync.Mutex
	var streamMu sync.Mutex
	currentStream := streamSettings{
		Width:   envInt("WIDTH", 1920),
		Height:  envInt("HEIGHT", 1080),
		Bitrate: envInt("BITRATE", 12000),
		Codec:   "h265",
	}
	screenshotCache := startScreenshotCache(appCtx, *screenshotStreamAddr)

	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade: %v", err)
			return
		}
		defer conn.Close()
		log.Printf("client connected: %s", r.RemoteAddr)

		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Printf("client disconnected: %s", r.RemoteAddr)
				mu.Lock()
				ctrl.Reset()
				mu.Unlock()
				return
			}

			var input InputMessage
			if err := json.Unmarshal(msg, &input); err != nil {
				continue
			}

			report := nxmc.Report{
				Buttons: input.Buttons,
				Hat:     input.Hat,
				LX:      input.LX,
				LY:      input.LY,
				RX:      input.RX,
				RY:      input.RY,
			}

			mu.Lock()
			ctrl.SendReport(report)
			mu.Unlock()
		}
	})

	http.HandleFunc("/api/stream", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			streamMu.Lock()
			s := currentStream
			streamMu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(s)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req streamSettings
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if req.Width <= 0 || req.Height <= 0 || req.Bitrate <= 0 {
			http.Error(w, "width, height, and bitrate must be positive", http.StatusBadRequest)
			return
		}
		if req.Codec == "" {
			req.Codec = "h265"
		}
		log.Printf("stream settings: %dx%d @ %d kbps codec=%s", req.Width, req.Height, req.Bitrate, req.Codec)

		var errs []string
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		streamMu.Lock()
		pipelineChanged := req.Width != currentStream.Width || req.Height != currentStream.Height || req.Codec != currentStream.Codec
		if pipelineChanged {
			if err := recreateGstdPipeline(ctx, *gstdURL, req, audioDevice, rtspHost, screenshotFPS, screenshotPort); err != nil {
				errs = append(errs, fmt.Sprintf("pipeline recreate: %v", err))
			}
		} else if req.Bitrate != currentStream.Bitrate {
			if err := gstdSetProperty(ctx, *gstdURL, "encoder", "bitrate", fmt.Sprintf("%d", req.Bitrate)); err != nil {
				errs = append(errs, fmt.Sprintf("bitrate: %v", err))
			}
		}
		if len(errs) == 0 {
			currentStream = req
		}
		streamMu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if len(errs) > 0 {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]any{"status": "error", "errors": errs})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "restarted": pipelineChanged})
	})

	http.HandleFunc("/api/input", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Buttons    []string `json:"buttons"`
			Hat        string   `json:"hat"`
			LX         *int     `json:"lx"`
			LY         *int     `json:"ly"`
			RX         *int     `json:"rx"`
			RY         *int     `json:"ry"`
			DurationMs int      `json:"duration_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		report := nxmc.NewReport()

		for _, name := range req.Buttons {
			b, ok := buttonMap[name]
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "unknown button: " + name})
				return
			}
			report.Buttons |= b
		}

		if req.Hat != "" {
			h, ok := hatMap[strings.ToLower(req.Hat)]
			if !ok {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				json.NewEncoder(w).Encode(map[string]string{"error": "unknown hat: " + req.Hat})
				return
			}
			report.Hat = h
		}

		if req.LX != nil {
			report.LX = byte(*req.LX)
		}
		if req.LY != nil {
			report.LY = byte(*req.LY)
		}
		if req.RX != nil {
			report.RX = byte(*req.RX)
		}
		if req.RY != nil {
			report.RY = byte(*req.RY)
		}

		mu.Lock()
		ctrl.SendReport(report)
		mu.Unlock()

		if req.DurationMs > 0 {
			time.Sleep(time.Duration(req.DurationMs) * time.Millisecond)
			mu.Lock()
			ctrl.Reset()
			mu.Unlock()
		}

		log.Printf("input: buttons=%v hat=%s duration=%dms", req.Buttons, req.Hat, req.DurationMs)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	http.HandleFunc("/api/screenshot", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		qw := r.URL.Query().Get("width")
		qh := r.URL.Query().Get("height")
		frame, updatedAt, ok := screenshotCache.latest(3 * time.Second)
		if !ok {
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			out, err := captureScreenshotOneShot(ctx, *rtspURL, qw, qh)
			if err != nil {
				log.Printf("screenshot fallback error: %v", err)
				http.Error(w, "screenshot failed", http.StatusInternalServerError)
				return
			}
			frame = out
		} else if qw != "" || qh != "" {
			out, err := resizeJPEG(frame, qw, qh)
			if err != nil {
				log.Printf("screenshot resize error: %v", err)
				http.Error(w, "screenshot failed", http.StatusInternalServerError)
				return
			}
			frame = out
		}

		w.Header().Set("Content-Type", "image/jpeg")
		if !updatedAt.IsZero() {
			w.Header().Set("X-Screenshot-Age-Ms", strconv.FormatInt(time.Since(updatedAt).Milliseconds(), 10))
		}
		w.Write(frame)
	})

	http.HandleFunc("/api/audio", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		duration := 3
		if d := r.URL.Query().Get("duration"); d != "" {
			v, err := strconv.Atoi(d)
			if err != nil || v < 1 || v > 30 {
				http.Error(w, "duration must be 1-30", http.StatusBadRequest)
				return
			}
			duration = v
		}

		ctx, cancel := context.WithTimeout(r.Context(), time.Duration(duration+10)*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "ffmpeg",
			"-rtsp_transport", "tcp",
			"-i", *rtspURL,
			"-vn",
			"-t", strconv.Itoa(duration),
			"-c:a", "pcm_s16le",
			"-f", "wav",
			"pipe:1",
		)
		out, err := cmd.Output()
		if err != nil {
			log.Printf("audio error: %v", err)
			http.Error(w, "audio capture failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "audio/wav")
		w.Write(out)
	})

	webContent, _ := fs.Sub(webFS, "web")
	http.Handle("/", http.FileServer(http.FS(webContent)))

	srv := &http.Server{Addr: *addr}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sig
		log.Println("shutting down...")
		stopApp()
		mu.Lock()
		ctrl.Reset()
		ctrl.Close()
		mu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("listening on %s", *addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
