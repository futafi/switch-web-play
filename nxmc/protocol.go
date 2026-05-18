package nxmc

const (
	HeaderNX2 byte = 0xAB

	ButtonY       uint16 = 0x0001
	ButtonB       uint16 = 0x0002
	ButtonA       uint16 = 0x0004
	ButtonX       uint16 = 0x0008
	ButtonL       uint16 = 0x0010
	ButtonR       uint16 = 0x0020
	ButtonZL      uint16 = 0x0040
	ButtonZR      uint16 = 0x0080
	ButtonMinus   uint16 = 0x0100
	ButtonPlus    uint16 = 0x0200
	ButtonLClick  uint16 = 0x0400
	ButtonRClick  uint16 = 0x0800
	ButtonHome    uint16 = 0x1000
	ButtonCapture uint16 = 0x2000

	HatUp        byte = 0
	HatUpRight   byte = 1
	HatRight     byte = 2
	HatDownRight byte = 3
	HatDown      byte = 4
	HatDownLeft  byte = 5
	HatLeft      byte = 6
	HatUpLeft    byte = 7
	HatCenter    byte = 8

	StickCenter byte = 128
	StickMin    byte = 0
	StickMax    byte = 255
)

type Report struct {
	Buttons uint16
	Hat     byte
	LX      byte
	LY      byte
	RX      byte
	RY      byte
}

func NewReport() Report {
	return Report{
		Hat: HatCenter,
		LX:  StickCenter,
		LY:  StickCenter,
		RX:  StickCenter,
		RY:  StickCenter,
	}
}

func (r Report) Bytes() []byte {
	return []byte{
		HeaderNX2,
		byte(r.Buttons & 0xFF),
		byte(r.Buttons >> 8),
		r.Hat,
		r.LX,
		r.LY,
		r.RX,
		r.RY,
		0, // keyboard mode (unused)
		0, // keyboard key (unused)
		0, // padding (firmware expects 11 bytes total)
	}
}
