package waveshare

import (
	"fmt"

	"gijs.eu/thermostat/internal/gpio"
)

// gpioPin wraps a single requested GPIO line used as a panel control signal
// (Data/Command, reset, or busy).
type gpioPin struct {
	num  int
	line *gpio.Line
}

// open requests the line through the GPIO character device.
func (p *gpioPin) open(chip string, dir gpio.Direction, consumer string) error {
	line, err := gpio.Open(gpio.Config{
		Chip:      chip,
		Offset:    p.num,
		Direction: dir,
		Consumer:  consumer,
	})
	if err != nil {
		return fmt.Errorf("gpio %d: %w", p.num, err)
	}
	p.line = line
	return nil
}

// set drives an output pin high or low.
func (p *gpioPin) set(high bool) error {
	return p.line.Set(high)
}

// read returns the current state of an input pin (true = high).
func (p *gpioPin) read() (bool, error) {
	return p.line.Get()
}

// close releases the pin.
func (p *gpioPin) close() error {
	if p.line == nil {
		return nil
	}
	err := p.line.Close()
	p.line = nil
	return err
}
