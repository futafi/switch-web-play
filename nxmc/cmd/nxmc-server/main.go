package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
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

func main() {
	device := flag.String("device", "/dev/ttyUSB0", "serial device path")
	baud := flag.Int("baud", 115200, "baud rate")
	addr := flag.String("addr", ":9000", "listen address")
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
