// Package waveshare implements fb.Display for the Waveshare 4.26" four-color
// e-paper panel (black/white/red/yellow) driven over SPI with Data/Command,
// reset, and busy GPIO lines.
//
// The driver targets the SSD2677 controller used by the 4.26" (G) panel. It is
// Linux-only: it talks to /dev/spidev and the GPIO character device
// (/dev/gpiochipN), matching the button input package.
package waveshare

import (
	"fmt"
	"time"

	"gijs.eu/thermostat/internal/fb"
	"gijs.eu/thermostat/internal/gpio"
)

// Config configures the four-color e-paper panel.
type Config struct {
	SPIDevice string // e.g. /dev/spidev0.0
	SpeedHz   int    // SPI clock; 0 means 4 MHz
	GPIOchip  string // gpiochip device, e.g. /dev/gpiochip0

	DCPin         int  // Data/Command GPIO (BCM numbering)
	ResetPin      int  // reset GPIO
	BusyPin       int  // busy input GPIO
	BusyActiveLow bool // busy reads low while the panel is refreshing

	Panel string // named preset, e.g. "4in26g"

	Width  int // used when Panel is empty/unknown
	Height int
}

// Display drives the four-color panel.
type Display struct {
	spi     *spiDevice
	dc      *gpioPin
	reset   *gpioPin
	busy    *gpioPin
	busyLow bool

	width      int
	height     int
	initHeight int
}

// New opens the SPI device, configures the GPIO pins, and initializes the
// panel.
func New(cfg Config) (*Display, error) {
	if cfg.SPIDevice == "" {
		cfg.SPIDevice = "/dev/spidev0.0"
	}

	width, height := cfg.Width, cfg.Height
	initHeight := height
	if p, ok := lookupPanel(cfg.Panel); ok {
		width, height = p.width, p.height
		if p.initHeight != 0 {
			initHeight = p.initHeight
		}
	}
	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid panel resolution %dx%d", width, height)
	}
	if cfg.DCPin == 0 || cfg.ResetPin == 0 || cfg.BusyPin == 0 {
		return nil, fmt.Errorf("dc, reset, and busy GPIO pins are all required")
	}

	spi, err := openSPI(cfg.SPIDevice, cfg.SpeedHz)
	if err != nil {
		return nil, err
	}

	d := &Display{
		spi:        spi,
		dc:         &gpioPin{num: cfg.DCPin},
		reset:      &gpioPin{num: cfg.ResetPin},
		busy:       &gpioPin{num: cfg.BusyPin},
		busyLow:    cfg.BusyActiveLow,
		width:      width,
		height:     height,
		initHeight: initHeight,
	}

	if err := d.dc.open(cfg.GPIOchip, gpio.Output, "thermostat-dc"); err != nil {
		spi.Close()
		return nil, err
	}
	if err := d.reset.open(cfg.GPIOchip, gpio.Output, "thermostat-reset"); err != nil {
		d.Close()
		return nil, err
	}
	if err := d.busy.open(cfg.GPIOchip, gpio.Input, "thermostat-busy"); err != nil {
		d.Close()
		return nil, err
	}

	if err := d.init(); err != nil {
		d.Close()
		return nil, err
	}
	return d, nil
}

// Width returns the visible panel width.
func (d *Display) Width() int { return d.width }

// Height returns the visible panel height.
func (d *Display) Height() int { return d.height }

// init resets the panel and runs the SSD2677 initialization sequence.
func (d *Display) init() error {
	// Hardware reset: high, low pulse, high, matching the reference timings.
	if err := d.reset.set(true); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)
	if err := d.reset.set(false); err != nil {
		return err
	}
	time.Sleep(2 * time.Millisecond)
	if err := d.reset.set(true); err != nil {
		return err
	}
	time.Sleep(200 * time.Millisecond)

	if err := d.waitBusy(); err != nil {
		return err
	}
	time.Sleep(30 * time.Millisecond)

	for _, c := range initCommands(d.width, d.initHeight) {
		if err := d.sendCommand(c.cmd); err != nil {
			return err
		}
		if len(c.data) > 0 {
			if err := d.sendData(c.data); err != nil {
				return err
			}
		}
		// The reference waits for the controller after the resolution and
		// power-on commands.
		if c.cmd == 0x61 || c.cmd == 0x04 {
			if err := d.waitBusy(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Show renders a frame. The SSD2677 takes 2 bits per pixel (four colors) packed
// four pixels per byte, sent as a single data stream (command 0x10) followed by
// a refresh (command 0x12 with a 0x00 parameter).
func (d *Display) Show(frame *fb.Frame) error {
	if frame.Width != d.width || frame.Height != d.height {
		return fmt.Errorf("frame %dx%d does not match panel %dx%d", frame.Width, frame.Height, d.width, d.height)
	}

	data := pack4Color(d.width, d.height, frame.Pix, frame.Accent)

	if err := d.sendCommand(0x10); err != nil { // DTM: 2-bit color data
		return err
	}
	if err := d.sendData(data); err != nil {
		return err
	}
	if err := d.sendCommand(0x12); err != nil { // DRF: refresh
		return err
	}
	if err := d.sendData([]byte{0x00}); err != nil {
		return err
	}
	return d.waitBusy()
}

// pack4Color packs a frame into the panel's 2-bit-per-pixel format: four pixels
// per byte, most-significant pair first. Color codes are 0=black, 1=white,
// 2=yellow, 3=red.
func pack4Color(width, height int, pix, accent []byte) []byte {
	stride := (width + 3) / 4
	out := make([]byte, stride*height)
	for y := 0; y < height; y++ {
		row := y * stride
		base := y * width
		for x := 0; x < width; x += 4 {
			var b byte
			for k := 0; k < 4 && x+k < width; k++ {
				b |= pixelCode(pix[base+x+k], accent, base+x+k) << (6 - 2*k)
			}
			out[row+x/4] = b
		}
	}
	return out
}

// pixelCode returns the 2-bit color code for a pixel: 0=black, 1=white,
// 2=yellow, 3=red.
func pixelCode(pix byte, accent []byte, i int) byte {
	if accent != nil {
		switch accent[i] {
		case fb.AccentRed:
			return 3
		case fb.AccentYellow:
			return 2
		}
	}
	if pix >= 128 {
		return 0 // black
	}
	return 1 // white
}

// Close puts the panel into deep sleep (best effort) and releases the SPI
// device and GPIO pins.
func (d *Display) Close() error {
	// Deep sleep (best effort): 0x02 + 0x00 then 0x07 + 0xA5, matching the
	// reference sleep sequence.
	_ = d.sendCommand(0x02)
	_ = d.sendData([]byte{0x00})
	_ = d.sendCommand(0x07)
	_ = d.sendData([]byte{0xA5})

	if d.spi != nil {
		_ = d.spi.Close()
	}
	for _, pin := range []*gpioPin{d.dc, d.reset, d.busy} {
		if pin != nil {
			pin.close()
		}
	}
	return nil
}

// sendCommand sends a command byte with the Data/Command line low.
func (d *Display) sendCommand(cmd byte) error {
	if d.dc != nil {
		if err := d.dc.set(false); err != nil {
			return err
		}
	}
	return d.spi.write([]byte{cmd})
}

// sendData sends data bytes with the Data/Command line high.
func (d *Display) sendData(data []byte) error {
	if d.dc != nil {
		if err := d.dc.set(true); err != nil {
			return err
		}
	}
	return d.spi.write(data)
}

// waitBusy blocks until the panel's busy line is idle (or a fixed delay if the
// busy pin is unavailable). It matches the reference: poll every 5ms while busy,
// then a 200ms settling delay after the line goes idle.
func (d *Display) waitBusy() error {
	if d.busy == nil {
		time.Sleep(200 * time.Millisecond)
		return nil
	}
	for i := 0; i < 10000; i++ {
		high, err := d.busy.read()
		if err != nil {
			return err
		}
		busy := high
		if d.busyLow {
			busy = !high
		}
		if !busy {
			time.Sleep(200 * time.Millisecond)
			return nil
		}
		time.Sleep(5 * time.Millisecond)
	}
	return fmt.Errorf("e-paper busy timeout")
}
