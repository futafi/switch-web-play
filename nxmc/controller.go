package nxmc

import (
	"fmt"

	"go.bug.st/serial"
)

type Controller struct {
	port serial.Port
}

func Open(device string, baud int) (*Controller, error) {
	mode := &serial.Mode{
		BaudRate: baud,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	port, err := serial.Open(device, mode)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", device, err)
	}
	return &Controller{port: port}, nil
}

func (c *Controller) Close() error {
	return c.port.Close()
}

func (c *Controller) SendReport(r Report) error {
	_, err := c.port.Write(r.Bytes())
	return err
}

func (c *Controller) Reset() error {
	return c.SendReport(NewReport())
}
