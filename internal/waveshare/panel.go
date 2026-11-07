package waveshare

// command is a single controller command: a command byte plus zero or more
// parameter bytes. A nil data slice means "no parameters".
type command struct {
	cmd  byte
	data []byte
}

// panel describes a Waveshare three-color (black/white/red or
// black/white/yellow) e-paper panel. The driver uses the SSD1680-class command
// set shared by these panels, so only the resolution differs between the
// entries below.
type panel struct {
	name   string
	width  int
	height int
}

// panelPresets maps the -display.panel flag value to a known panel resolution.
// The init sequence below is the common SSD1680 reference sequence; specific
// panel revisions occasionally need a different power-setting or TCON byte, so
// this table is the place to add a custom init for an exotic panel.
var panelPresets = []panel{
	{name: "7in5b_v2", width: 800, height: 480},
	{name: "7in5b_v1", width: 800, height: 480},
	{name: "5in83b_v2", width: 600, height: 448},
	{name: "5in65b", width: 600, height: 448},
	{name: "4in2b_v2", width: 400, height: 300},
	{name: "2in9b_v3", width: 296, height: 128},
	{name: "2in13b_v4", width: 122, height: 250},
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

// initCommands builds the SSD1680-class initialization sequence for the given
// resolution. The power-on commands (0x04) must be followed by a busy wait,
// which the caller handles.
//
// The resolution (0x61) parameters are width-1 and height-1, little-endian,
// following the SSD1680 datasheet. Panel revisions using other controllers
// (for example the UC8179-based 7.5" B) sometimes expect the raw resolution or
// a different byte order, so this is the place to add a custom sequence for an
// exotic panel.
func initCommands(width, height int) []command {
	w := width - 1
	h := height - 1
	return []command{
		{cmd: 0x01, data: []byte{0x07, 0x07, 0x3F, 0x3F}}, // power setting
		{cmd: 0x04}, // power on
		{cmd: 0x00, data: []byte{0x0F}}, // panel setting: BWR mode, LUT from OTP
		{cmd: 0x61, data: []byte{byte(w & 0xFF), byte(w >> 8), byte(h & 0xFF), byte(h >> 8)}}, // resolution
		{cmd: 0x15, data: []byte{0x00}}, // dual SPI: disabled
		{cmd: 0x50, data: []byte{0x11, 0x07}}, // VCOM and data interval
		{cmd: 0x60, data: []byte{0x22}},       // TCON setting
		{cmd: 0x04},                           // power on
	}
}

// packPlane packs a 1-bit plane into the panel's row-major buffer: each row is
// width/8 bytes (rounded up), MSB first, padded with zero bits on the right.
func packPlane(width, height int, set func(x, y int) bool) []byte {
	stride := (width + 7) / 8
	out := make([]byte, stride*height)
	for y := 0; y < height; y++ {
		row := y * stride
		for x := 0; x < width; x++ {
			if set(x, y) {
				out[row+x/8] |= byte(1 << uint(7-x%8))
			}
		}
	}
	return out
}
