package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
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

func gstdSetProperty(baseURL, element, property, value string) error {
	u := fmt.Sprintf("%s/pipelines/cam/elements/%s/properties/%s?name=%s",
		baseURL, element, property, url.QueryEscape(value))
	req, err := http.NewRequest(http.MethodPut, u, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gstd %s.%s: status %d", element, property, resp.StatusCode)
	}
	return nil
}

func main() {
	device := flag.String("device", "/dev/ttyUSB0", "serial device path")
	baud := flag.Int("baud", 115200, "baud rate")
	addr := flag.String("addr", ":9000", "listen address")
	gstdURL := flag.String("gstd-url", "http://gstreamer:5000", "gstd HTTP API base URL")
	rtspURL := flag.String("rtsp-url", "rtsp://mediamtx:8554/cam", "RTSP stream URL")
	flag.Parse()

	ctrl, err := nxmc.Open(*device, *baud)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("serial: %s @ %d", *device, *baud)

	var mu sync.Mutex

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
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Width   int `json:"width"`
			Height  int `json:"height"`
			Bitrate int `json:"bitrate"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		log.Printf("stream settings: %dx%d @ %d kbps", req.Width, req.Height, req.Bitrate)

		var errs []string

		caps := fmt.Sprintf("video/x-raw,width=%d,height=%d", req.Width, req.Height)
		if err := gstdSetProperty(*gstdURL, "scaler", "caps", caps); err != nil {
			errs = append(errs, fmt.Sprintf("resolution: %v", err))
		}
		if err := gstdSetProperty(*gstdURL, "encoder", "bitrate", fmt.Sprintf("%d", req.Bitrate)); err != nil {
			errs = append(errs, fmt.Sprintf("bitrate: %v", err))
		}

		w.Header().Set("Content-Type", "application/json")
		if len(errs) > 0 {
			w.WriteHeader(http.StatusBadGateway)
			json.NewEncoder(w).Encode(map[string]any{"status": "error", "errors": errs})
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
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

		args := []string{
			"-rtsp_transport", "tcp",
			"-i", *rtspURL,
			"-frames:v", "1",
		}

		qw := r.URL.Query().Get("width")
		qh := r.URL.Query().Get("height")
		if qw != "" || qh != "" {
			sw, sh := "-1", "-1"
			if qw != "" {
				sw = qw
			}
			if qh != "" {
				sh = qh
			}
			args = append(args, "-vf", fmt.Sprintf("scale=%s:%s", sw, sh))
		}

		args = append(args, "-f", "image2", "-c:v", "mjpeg", "-q:v", "2", "pipe:1")

		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, "ffmpeg", args...)
		out, err := cmd.Output()
		if err != nil {
			log.Printf("screenshot error: %v", err)
			http.Error(w, "screenshot failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "image/jpeg")
		w.Write(out)
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
