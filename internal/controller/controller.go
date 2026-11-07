// Package controller wires the Home Assistant client, input sources, and
// display together. It is deliberately "dumb": it never decides when to heat
// or cool. It only mirrors Home Assistant state onto the screen and forwards
// user input back to Home Assistant.
package controller

import (
	"context"
	"log"
	"math"
	"time"

	"gijs.eu/thermostat/internal/fb"
	"gijs.eu/thermostat/internal/homeassistant"
	"gijs.eu/thermostat/internal/input"
	"gijs.eu/thermostat/internal/state"
)

// Config holds controller tuning parameters.
type Config struct {
	Width        int
	Height       int
	Step         float64
	PollInterval time.Duration
	EditTimeout  time.Duration
	HoldDelay    time.Duration
	HoldRepeat   time.Duration
}

// Controller owns the shared state and the main event loop.
type Controller struct {
	cfg      Config
	ha       *homeassistant.Client
	disp     fb.Display
	renderer *fb.Renderer
	events   <-chan input.Event

	st        state.State
	editing   bool
	editTimer *time.Timer

	heldButton  input.Button
	heldActive  bool
	repeatTimer *time.Timer
}

// New builds a controller, applying defaults for any zero tuning values.
func New(cfg Config, ha *homeassistant.Client, disp fb.Display, events <-chan input.Event) *Controller {
	if cfg.Step == 0 {
		cfg.Step = 0.5
	}
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 5 * time.Second
	}
	if cfg.EditTimeout == 0 {
		cfg.EditTimeout = 3 * time.Second
	}
	if cfg.HoldDelay == 0 {
		cfg.HoldDelay = 400 * time.Millisecond
	}
	if cfg.HoldRepeat == 0 {
		cfg.HoldRepeat = 200 * time.Millisecond
	}

	c := &Controller{
		cfg:      cfg,
		ha:       ha,
		disp:     disp,
		renderer: fb.NewRenderer(cfg.Width, cfg.Height),
		events:   events,
	}
	c.editTimer = time.NewTimer(time.Hour)
	c.editTimer.Stop()
	c.repeatTimer = time.NewTimer(time.Hour)
	c.repeatTimer.Stop()
	return c
}

// Run starts the event loop and blocks until the context is cancelled or a
// quit event is received.
func (c *Controller) Run(ctx context.Context) {
	c.refreshHA()
	c.render()

	haTick := time.NewTicker(c.cfg.PollInterval)
	defer haTick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case ev := <-c.events:
			if c.handle(ev) {
				return
			}
		case <-haTick.C:
			c.refreshHA()
			c.render()
		case <-c.editTimer.C:
			c.editing = false
			c.render()
		case <-c.repeatTimer.C:
			if c.heldActive {
				if c.heldButton == input.ButtonUp {
					c.adjust(c.cfg.Step)
				} else if c.heldButton == input.ButtonDown {
					c.adjust(-c.cfg.Step)
				}
				c.repeatTimer.Reset(c.cfg.HoldRepeat)
			}
		}
	}
}

// handle applies a single input event, returning true if the loop should stop.
func (c *Controller) handle(ev input.Event) bool {
	if ev.Phase == input.PhasePressed {
		switch ev.Button {
		case input.ButtonUp:
			c.heldButton = input.ButtonUp
			c.heldActive = true
			c.adjust(c.cfg.Step)
			c.repeatTimer.Reset(c.cfg.HoldDelay)
		case input.ButtonDown:
			c.heldButton = input.ButtonDown
			c.heldActive = true
			c.adjust(-c.cfg.Step)
			c.repeatTimer.Reset(c.cfg.HoldDelay)
		case input.ButtonQuit:
			_ = c.disp.Close()
			return true
		}
	} else if ev.Button == c.heldButton {
		c.heldActive = false
		c.repeatTimer.Stop()
	}
	return false
}

// adjust changes the target temperature optimistically, shows the new value
// full screen immediately, and then sends the change to Home Assistant.
func (c *Controller) adjust(delta float64) {
	target := roundStep(c.st.TargetTemp+delta, c.cfg.Step)
	c.st.TargetTemp = target
	c.beginEdit()
	c.render()

	if err := c.ha.SetTargetTemperature(target); err != nil {
		c.st.HAConnected = false
		c.st.HAError = err.Error()
		log.Printf("set target: %v", err)
		c.render()
	} else {
		c.st.HAConnected = true
		c.st.HAError = ""
	}
}

func (c *Controller) beginEdit() {
	c.editing = true
	c.editTimer.Reset(c.cfg.EditTimeout)
}

// refreshHA pulls the latest thermostat state from Home Assistant.
func (c *Controller) refreshHA() {
	ts, err := c.ha.ThermostatState()
	if err != nil {
		c.st.HAConnected = false
		c.st.HAError = err.Error()
		return
	}
	c.st.HAConnected = true
	c.st.HAError = ""
	c.st.CurrentTemp = ts.CurrentTemp
	c.st.TargetTemp = ts.TargetTemp
	c.st.Mode = ts.Mode
	c.st.Action = ts.Action
}

func (c *Controller) render() {
	frame := c.renderer.Render(c.st)
	if err := c.disp.Show(frame); err != nil {
		log.Printf("display: %v", err)
	}
}

// roundStep rounds a temperature to the configured increment and to one
// decimal place, avoiding floating-point artefacts.
func roundStep(v, step float64) float64 {
	if step <= 0 {
		step = 0.5
	}
	return math.Round(math.Round(v/step)*step*10) / 10
}
