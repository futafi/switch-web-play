package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"web-switch/nxmc"
)

func main() {
	device := flag.String("device", "/dev/ttyUSB0", "serial device path")
	baud := flag.Int("baud", 115200, "baud rate")
	flag.Parse()

	ctrl, err := nxmc.Open(*device, *baud)
	if err != nil {
		log.Fatal(err)
	}
	defer ctrl.Close()

	fmt.Fprintf(os.Stderr, "connected to %s @ %d\n", *device, *baud)

	// reset
	ctrl.Reset()
	time.Sleep(100 * time.Millisecond)

	// press A for 100ms
	fmt.Fprintf(os.Stderr, "pressing A...\n")
	r := nxmc.NewReport()
	r.Buttons = nxmc.ButtonA
	if err := ctrl.SendReport(r); err != nil {
		log.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	// release
	fmt.Fprintf(os.Stderr, "releasing...\n")
	ctrl.Reset()
	time.Sleep(50 * time.Millisecond)

	fmt.Fprintf(os.Stderr, "done\n")
}
