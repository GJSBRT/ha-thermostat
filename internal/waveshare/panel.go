package waveshare

// command is a single controller command: a command byte plus zero or more
// parameter bytes. A nil data slice means "no parameters".
type command struct {
	cmd  byte
	data []byte
}

// panel describes a supported e-paper panel.
type panel struct {
	name       string
	width      int
	height     int
	initHeight int // controller init resolution height; 0 means same as height
}

// panelPresets maps the -display.panel flag value to a known panel. The SSD2677
// four-color panel initializes its controller to 800x680 but exposes a visible
// 800x480 area, hence the separate init height.
var panelPresets = []panel{
	{name: "4in26g", width: 800, height: 480, initHeight: 680},
}

// lookupPanel returns the named preset. An empty or unknown name returns false
// so the caller can fall back to explicit width/height flags.
func lookupPanel(name string) (panel, bool) {
	for _, p := range panelPresets {
		if p.name == name {
			return p, true
		}
	}
	return panel{}, false
}

// initCommands builds the SSD2677 four-color initialization sequence, following
// the Waveshare reference init for the 4.26" (G) panel.
func initCommands(width, initHeight int) []command {
	return []command{
		{cmd: 0x00, data: []byte{0x2B, 0x29}}, // panel setting
		{cmd: 0x06, data: []byte{0x0F, 0x8B, 0x93, 0xA1}}, // booster soft start
		{cmd: 0x61, data: []byte{ // resolution (controller init height)
			byte(width >> 8), byte(width & 0xFF),
			byte(initHeight >> 8), byte(initHeight & 0xFF),
		}},
		{cmd: 0x50, data: []byte{0x37}}, // VCOM and data interval
		{cmd: 0x30, data: []byte{0x08}}, // PLL control
		{cmd: 0x62, data: []byte{0x76, 0x76, 0x76, 0x5A, 0x9D, 0x8A, 0x76, 0x62}},
		{cmd: 0xE3, data: []byte{0x22}}, // power saving
		{cmd: 0xE0, data: []byte{0x10}}, // active temperature
		{cmd: 0xE7, data: []byte{0xA1}}, // temperature sensor
		{cmd: 0xE9, data: []byte{0x01}},
		{cmd: 0x04}, // power on
	}
}
