package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"web-switch/nxmc"
)

var buttons = map[string]uint16{
	"a":    nxmc.ButtonA,
	"b":    nxmc.ButtonB,
	"x":    nxmc.ButtonX,
	"y":    nxmc.ButtonY,
	"home": nxmc.ButtonHome,
}

func main() {
	device := flag.String("device", "/dev/ttyUSB0", "serial device path")
	baud := flag.Int("baud", 115200, "baud rate")
	flag.Parse()

	name := "a"
	if flag.NArg() > 0 {
		name = flag.Arg(0)
	}

	bit, ok := buttons[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown button: %s\navailable: a, b, x, y, home\n", name)
		os.Exit(1)
	}

	ctrl, err := nxmc.Open(*device, *baud)
	if err != nil {
		log.Fatal(err)
	}
	defer ctrl.Close()

	fmt.Fprintf(os.Stderr, "connected to %s @ %d\n", *device, *baud)

	ctrl.Reset()
	time.Sleep(100 * time.Millisecond)

	fmt.Fprintf(os.Stderr, "pressing %s...\n", name)
	r := nxmc.NewReport()
	r.Buttons = bit
	if err := ctrl.SendReport(r); err != nil {
		log.Fatal(err)
	}
	time.Sleep(100 * time.Millisecond)

	fmt.Fprintf(os.Stderr, "releasing...\n")
	ctrl.Reset()
	time.Sleep(50 * time.Millisecond)

	fmt.Fprintf(os.Stderr, "done\n")
}
