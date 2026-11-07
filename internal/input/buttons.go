package input

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// ButtonsConfig configures the two GPIO increase/decrease buttons.
type ButtonsConfig struct {
	Up        int
	Down      int
	ActiveLow bool
	Debounce  time.Duration
}

// Buttons reads two GPIO buttons through the Linux sysfs interface and emits
// press/release events.
//
// NOTE: the sysfs GPIO interface (/sys/class/gpio) is deprecated on newer
// kernels but remains available on most Raspberry Pi images. If it is not
// present, NewButtons returns an error and the thermostat simply runs without
// buttons.
type Buttons struct {
	up       *button
	down     *button
	events   chan<- Event
	debounce time.Duration
}

type button struct {
	num       int
	activeLow bool
}

// NewButtons exports and configures the two GPIO pins, returning an error if
// the sysfs interface is unavailable.
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
		if err := btn.init(); err != nil {
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
		btn.unexport()
	}
}

func (b *button) init() error {
	if err := export(b.num); err != nil {
		return err
	}
	return writeSys(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", b.num), "in")
}

// read returns the debounced "pressed" state of the button, applying polarity.
func (b *button) read() bool {
	data, err := os.ReadFile(fmt.Sprintf("/sys/class/gpio/gpio%d/value", b.num))
	if err != nil {
		return false
	}
	high := strings.TrimSpace(string(data)) == "1"
	if b.activeLow {
		return !high
	}
	return high
}

func (b *button) unexport() {
	_ = writeSys("/sys/class/gpio/unexport", fmt.Sprintf("%d", b.num))
}

// export makes a GPIO pin available through sysfs. It is idempotent.
func export(num int) error {
	path := fmt.Sprintf("/sys/class/gpio/gpio%d", num)
	if _, err := os.Stat(path); err == nil {
		return nil // already exported
	}

	if err := writeSys("/sys/class/gpio/export", fmt.Sprintf("%d", num)); err != nil {
		// "Device or resource busy" means another exporter already owns it.
		if strings.Contains(err.Error(), "busy") {
			return nil
		}
		return err
	}

	// Wait briefly for udev to create the sysfs node.
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("sysfs node %s did not appear", path)
}

// writeSys writes a value to a sysfs attribute without truncation semantics.
func writeSys(path, value string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.WriteString(f, value)
	return err
}
