package input

import (
	"context"
	"fmt"
	"time"

	"gijs.eu/thermostat/internal/gpio"
)

// ButtonsConfig configures the two GPIO increase/decrease buttons.
type ButtonsConfig struct {
	Up        int
	Down      int
	ActiveLow bool
	Debounce  time.Duration
	GPIOchip  string
}

// Buttons reads two GPIO buttons through the GPIO character device and emits
// press/release events.
type Buttons struct {
	up       *button
	down     *button
	events   chan<- Event
	debounce time.Duration
}

type button struct {
	num       int
	activeLow bool
	line      *gpio.Line
}

// NewButtons requests and configures the two GPIO pins, returning an error if
// the GPIO chip is unavailable.
func NewButtons(cfg ButtonsConfig, events chan<- Event) (*Buttons, error) {
	if cfg.Up == cfg.Down {
		return nil, fmt.Errorf("increase and decrease buttons must use different GPIO pins")
	}
	if cfg.Debounce <= 0 {
		cfg.Debounce = 25 * time.Millisecond
	}

	b := &Buttons{
		up:       &button{num: cfg.Up, activeLow: cfg.ActiveLow},
		down:     &button{num: cfg.Down, activeLow: cfg.ActiveLow},
		events:   events,
		debounce: cfg.Debounce,
	}
	for _, btn := range []*button{b.up, b.down} {
		if err := btn.init(cfg.GPIOchip); err != nil {
			b.close()
			return nil, fmt.Errorf("gpio %d: %w", btn.num, err)
		}
	}
	return b, nil
}

// Run polls the buttons and emits press/release events until the context is
// cancelled.
func (b *Buttons) Run(ctx context.Context) {
	defer b.close()

	tick := time.NewTicker(b.debounce)
	defer tick.Stop()

	upPrev, downPrev := false, false
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			up := b.up.read()
			down := b.down.read()
			if up != upPrev {
				b.events <- Event{Button: ButtonUp, Phase: phase(up)}
			}
			if down != downPrev {
				b.events <- Event{Button: ButtonDown, Phase: phase(down)}
			}
			upPrev, downPrev = up, down
		}
	}
}

// phase converts a debounced "pressed" boolean into a transition phase.
func phase(pressed bool) Phase {
	if pressed {
		return PhasePressed
	}
	return PhaseReleased
}

func (b *Buttons) close() {
	for _, btn := range []*button{b.up, b.down} {
		btn.close()
	}
}

func (b *button) init(chip string) error {
	line, err := gpio.Open(gpio.Config{
		Chip:      chip,
		Offset:    b.num,
		Direction: gpio.Input,
		Consumer:  "thermostat-button",
		PullUp:    b.activeLow,
	})
	if err != nil {
		return err
	}
	b.line = line
	return nil
}

// read returns the debounced "pressed" state of the button, applying polarity.
func (b *button) read() bool {
	high, err := b.line.Get()
	if err != nil {
		return false
	}
	if b.activeLow {
		return !high
	}
	return high
}

func (b *button) close() {
	if b.line != nil {
		_ = b.line.Close()
		b.line = nil
	}
}
