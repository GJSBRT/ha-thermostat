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
	"gijs.eu/thermostat/internal/waveshare"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	cfg := config.Load()

	events := make(chan input.Event, 16)

	ha := homeassistant.New(homeassistant.Config{
		BaseURL:          cfg.HAURL,
		Token:            cfg.HAToken,
		ThermostatEntity: cfg.HAThermostat,
		OutsideEntity:    cfg.HAOutside,
		Timeout:          cfg.HATimeout,
	})

	var disp fb.Display
	var window *display.Window
	width, height := cfg.DisplayWidth, cfg.DisplayHeight

	// Resolve the accent color for the development window preview and the
	// Waveshare driver. The framebuffer backend is monochrome, so it keeps
	// accent at zero.
	accent := byte(0)
	if cfg.Dev || cfg.DisplayDriver == "waveshare" {
		switch cfg.WSAccent {
		case "yellow":
			accent = fb.AccentYellow
		case "red", "":
			accent = fb.AccentRed
		default:
			log.Fatalf("display: unknown accent color %q", cfg.WSAccent)
		}
	}

	if cfg.Dev {
		window = display.NewWindow(width, height, events)
		disp = window
	} else {
		switch cfg.DisplayDriver {
		case "waveshare":
			wsd, err := waveshare.New(waveshare.Config{
				SPIDevice:     cfg.WSSPIDevice,
				SpeedHz:       cfg.WSSpeedHz,
				DCPin:         cfg.WSDCPin,
				ResetPin:      cfg.WSResetPin,
				BusyPin:       cfg.WSBusyPin,
				BusyActiveLow: cfg.WSBusyActiveLow,
				Panel:         cfg.WSPanel,
				Width:         cfg.DisplayWidth,
				Height:        cfg.DisplayHeight,
			})
			if err != nil {
				log.Fatalf("display: %v", err)
			}
			defer wsd.Close()
			disp = wsd
			width, height = wsd.Width(), wsd.Height()
		default:
			fbdisp, err := fb.NewFramebuffer(fb.FramebufferConfig{
				Path:   cfg.DisplayDevice,
				Width:  width,
				Height: height,
				Format: cfg.DisplayFormat,
			})
			if err != nil {
				log.Fatalf("display: %v", err)
			}
			defer fbdisp.Close()
			disp = fbdisp
		}
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
		Width:        width,
		Height:       height,
		Step:         cfg.Step,
		PollInterval: cfg.PollInterval,
		EditTimeout:  cfg.EditTimeout,
		HoldDelay:    cfg.HoldDelay,
		HoldRepeat:   cfg.HoldRepeat,
		Accent:       accent,
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
