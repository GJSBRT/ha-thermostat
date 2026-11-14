// Package waveshare implements fb.Display for Waveshare three-color e-paper
// panels (black/white/red and black/white/yellow) driven over SPI with
// Data/Command, reset, and busy GPIO lines.
//
// The driver targets the SSD1680-class command set shared by Waveshare's
// three-color "B" panels (4.2", 5.65", 5.83", and 7.5"). It is Linux-only: it
// talks to /dev/spidev and the GPIO character device (/dev/gpiochipN), matching
// the button input package.
package waveshare

import (
	"fmt"
	"time"

	"gijs.eu/thermostat/internal/fb"
	"gijs.eu/thermostat/internal/gpio"
)

// Config configures a Waveshare three-color e-paper panel.
type Config struct {
	SPIDevice string // e.g. /dev/spidev0.0
	SpeedHz   int    // SPI clock; 0 means 4 MHz
	GPIOchip  string // gpiochip device, e.g. /dev/gpiochip0

	DCPin         int // Data/Command GPIO (BCM numbering)
	ResetPin      int // reset GPIO
	BusyPin       int // busy input GPIO
	BusyActiveLow bool // busy reads low while the panel is refreshing

	Panel string // named preset, e.g. "7in5b_v2"

	Width  int // used when Panel is empty/unknown
	Height int
}

// Display drives a single Waveshare panel.
type Display struct {
	spi     *spiDevice
	dc      *gpioPin
	reset   *gpioPin
	busy    *gpioPin
	busyLow bool

	width  int
	height int
}

// New opens the SPI device, configures the GPIO pins, and initializes the
// panel.
func New(cfg Config) (*Display, error) {
	if cfg.SPIDevice == "" {
		cfg.SPIDevice = "/dev/spidev0.0"
	}

	width, height := cfg.Width, cfg.Height
	if p, ok := lookupPanel(cfg.Panel); ok {
		width, height = p.width, p.height
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
		spi:     spi,
		dc:      &gpioPin{num: cfg.DCPin},
		reset:   &gpioPin{num: cfg.ResetPin},
		busy:    &gpioPin{num: cfg.BusyPin},
		busyLow: cfg.BusyActiveLow,
		width:   width,
		height:  height,
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

// Width returns the panel width resolved from the preset or explicit flags.
func (d *Display) Width() int { return d.width }

// Height returns the panel height resolved from the preset or explicit flags.
func (d *Display) Height() int { return d.height }

// init resets the panel and runs the controller initialization sequence.
func (d *Display) init() error {
	if err := d.reset.set(false); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)
	if err := d.reset.set(true); err != nil {
		return err
	}
	time.Sleep(20 * time.Millisecond)

	for _, c := range initCommands(d.width, d.height) {
		if err := d.sendCommand(c.cmd); err != nil {
			return err
		}
		if len(c.data) > 0 {
			if err := d.sendData(c.data); err != nil {
				return err
			}
		}
		if c.cmd == 0x04 { // power on must settle before the next command
			if err := d.waitBusy(); err != nil {
				return err
			}
		}
	}
	return nil
}

// Show renders a frame by sending the black plane (command 0x10), the color
// plane (command 0x13), and then refreshing the panel (command 0x12).
//
// The panel is four-color (black/white/red/yellow): the two planes encode two
// bits per pixel, where the fourth combination (both bits set) selects yellow.
// So the black plane is set for black and yellow pixels, and the color plane is
// set for red and yellow pixels.
func (d *Display) Show(frame *fb.Frame) error {
	if frame.Width != d.width || frame.Height != d.height {
		return fmt.Errorf("frame %dx%d does not match panel %dx%d", frame.Width, frame.Height, d.width, d.height)
	}

	black := packPlane(d.width, d.height, func(x, y int) bool {
		i := y*d.width + x
		return frame.Pix[i] >= 128 || (frame.Accent != nil && frame.Accent[i] == fb.AccentYellow)
	})
	color := packPlane(d.width, d.height, func(x, y int) bool {
		i := y*d.width + x
		return frame.Accent != nil && frame.Accent[i] != fb.AccentNone
	})

	if err := d.sendCommand(0x10); err != nil { // DTM1: black plane
		return err
	}
	if err := d.sendData(black); err != nil {
		return err
	}
	if err := d.sendCommand(0x13); err != nil { // DTM2: color (red/yellow) plane
		return err
	}
	if err := d.sendData(color); err != nil {
		return err
	}
	if err := d.sendCommand(0x12); err != nil { // DRF: refresh
		return err
	}
	return d.waitBusy()
}

// Close puts the panel into deep sleep (best effort) and releases the SPI
// device and GPIO pins.
func (d *Display) Close() error {
	// 0x07 (deep sleep) with 0xA5 (mode 1) reduces standby current; ignored if
	// the panel revision does not support it.
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
// busy pin is unavailable).
func (d *Display) waitBusy() error {
	if d.busy == nil {
		time.Sleep(50 * time.Millisecond)
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
			return nil
		}
		time.Sleep(time.Millisecond)
	}
	return fmt.Errorf("e-paper busy timeout")
}
