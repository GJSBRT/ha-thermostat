// Command thermostat is a Home Assistant thermostat interface for a 5"
// e-paper display. It is a pure interface: it reads a generic thermostat
// entity from Home Assistant and pushes user temperature adjustments back, but
// never decides on its own when to heat or cool.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gijs.eu/thermostat/internal/config"
	"gijs.eu/thermostat/internal/controller"
	"gijs.eu/thermostat/internal/display"
	"gijs.eu/thermostat/internal/fb"
	"gijs.eu/thermostat/internal/homeassistant"
	"gijs.eu/thermostat/internal/input"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	cfg := config.Load()

	events := make(chan input.Event, 16)

	ha := homeassistant.New(homeassistant.Config{
		BaseURL:          cfg.HAURL,
		Token:            cfg.HAToken,
		ThermostatEntity: cfg.HAThermostat,
		Timeout:          cfg.HATimeout,
	})

	var disp fb.Display
	var window *display.Window
	if cfg.Dev {
		window = display.NewWindow(cfg.DisplayWidth, cfg.DisplayHeight, events)
		disp = window
	} else {
		fbdisp, err := fb.NewFramebuffer(fb.FramebufferConfig{
			Path:   cfg.DisplayDevice,
			Width:  cfg.DisplayWidth,
			Height: cfg.DisplayHeight,
			Format: cfg.DisplayFormat,
		})
		if err != nil {
			log.Fatalf("display: %v", err)
		}
		defer fbdisp.Close()
		disp = fbdisp
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.ButtonsEnabled {
		btns, err := input.NewButtons(input.ButtonsConfig{
			Up:        cfg.ButtonUp,
			Down:      cfg.ButtonDown,
			ActiveLow: cfg.ButtonActiveLow,
			Debounce:  cfg.ButtonDebounce,
		}, events)
		if err != nil {
			log.Printf("buttons disabled: %v", err)
		} else {
			go btns.Run(ctx)
		}
	}

	ctrl := controller.New(controller.Config{
		Width:        cfg.DisplayWidth,
		Height:       cfg.DisplayHeight,
		Step:         cfg.Step,
		PollInterval: cfg.PollInterval,
		EditTimeout:  cfg.EditTimeout,
		HoldDelay:    cfg.HoldDelay,
		HoldRepeat:   cfg.HoldRepeat,
	}, ha, disp, events)

	go func() {
		ctrl.Run(ctx)
		cancel()
	}()

	// Handle SIGINT/SIGTERM by closing the display and stopping the loop.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sig
		_ = disp.Close()
		cancel()
	}()

	if window != nil {
		window.Run() // blocks until the window is closed
		cancel()
	} else {
		<-ctx.Done()
	}
}
