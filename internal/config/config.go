// Package config loads thermostat configuration from command-line flags, with
// environment-variable overrides for secrets.
package config

import (
	"flag"
	"os"
	"time"
)

// Config holds all runtime configuration.
type Config struct {
	Dev bool // run the development GUI window and keyboard controls

	DisplayWidth  int
	DisplayHeight int
	DisplayFormat string // gray8, rgb565, mono (framebuffer only)
	DisplayDevice string // framebuffer path (framebuffer only)
	DisplayDriver string // framebuffer or waveshare (device builds only)

	WSSPIDevice     string
	WSSpeedHz       int
	WSGPIOchip      string
	WSDCPin         int
	WSResetPin      int
	WSBusyPin       int
	WSBusyActiveLow bool
	WSAccent        string // red or yellow
	WSPanel         string // named Waveshare panel preset

	HAURL            string
	HAToken          string
	HAThermostat     string
	HAOutside        string
	HATimeout        time.Duration

	Step         float64
	PollInterval time.Duration
	EditTimeout  time.Duration
	HoldDelay    time.Duration
	HoldRepeat   time.Duration

	ButtonsEnabled  bool
	ButtonUp        int
	ButtonDown      int
	ButtonGPIOchip  string
	ButtonActiveLow bool
	ButtonDebounce  time.Duration
}

// Load parses command-line flags and applies environment overrides.
func Load() Config {
	c := Config{}

	flag.BoolVar(&c.Dev, "dev", true, "run development mode (GUI window + keyboard controls)")
	flag.IntVar(&c.DisplayWidth, "display.width", 800, "display width in pixels")
	flag.IntVar(&c.DisplayHeight, "display.height", 480, "display height in pixels")
	flag.StringVar(&c.DisplayDriver, "display.driver", "framebuffer", "device display backend: framebuffer, waveshare")
	flag.StringVar(&c.DisplayFormat, "display.format", "gray8", "framebuffer pixel format: gray8, rgb565, mono")
	flag.StringVar(&c.DisplayDevice, "display.device", "/dev/fb0", "framebuffer device path (used when -dev=false)")

	flag.StringVar(&c.WSSPIDevice, "display.spi.device", "/dev/spidev0.0", "SPI device for Waveshare e-paper")
	flag.IntVar(&c.WSSpeedHz, "display.spi.speed", 4000000, "SPI clock speed in Hz")
	flag.StringVar(&c.WSGPIOchip, "display.gpiochip", "/dev/gpiochip0", "GPIO chip device for Waveshare e-paper")
	flag.IntVar(&c.WSDCPin, "display.gpio.dc", 25, "Data/Command GPIO pin (BCM)")
	flag.IntVar(&c.WSResetPin, "display.gpio.reset", 17, "reset GPIO pin (BCM)")
	flag.IntVar(&c.WSBusyPin, "display.gpio.busy", 24, "busy GPIO pin (BCM)")
	flag.BoolVar(&c.WSBusyActiveLow, "display.gpio.busy_active_low", true, "busy signal is active-low")
	flag.StringVar(&c.WSAccent, "display.accent", "red", "accent color for three-color panels: red, yellow")
	flag.StringVar(&c.WSPanel, "display.panel", "", "Waveshare panel preset (e.g. 7in5b_v2)")

	flag.StringVar(&c.HAURL, "ha.url", "http://homeassistant.local:8123", "Home Assistant base URL")
	flag.StringVar(&c.HAToken, "ha.token", "", "Home Assistant long-lived access token")
	flag.StringVar(&c.HAThermostat, "ha.thermostat", "climate.thermostat", "thermostat (climate) entity id")
	flag.StringVar(&c.HAOutside, "ha.outside", "", "outside temperature (sensor) entity id; empty hides the readout")
	flag.DurationVar(&c.HATimeout, "ha.timeout", 5*time.Second, "Home Assistant request timeout")

	flag.Float64Var(&c.Step, "step", 0.5, "temperature change per button/key press (degrees C)")
	flag.DurationVar(&c.PollInterval, "poll", 5*time.Second, "how often to poll Home Assistant state")
	flag.DurationVar(&c.EditTimeout, "edit.timeout", 3*time.Second, "how long the full-screen temperature is shown after a change")
	flag.DurationVar(&c.HoldDelay, "hold.delay", 400*time.Millisecond, "how long a button must be held before auto-repeat starts")
	flag.DurationVar(&c.HoldRepeat, "hold.repeat", 200*time.Millisecond, "auto-repeat interval while a button is held")

	flag.BoolVar(&c.ButtonsEnabled, "buttons.enabled", false, "enable GPIO increase/decrease buttons")
	flag.IntVar(&c.ButtonUp, "buttons.up", 17, "GPIO number for the increase button")
	flag.IntVar(&c.ButtonDown, "buttons.down", 27, "GPIO number for the decrease button")
	flag.StringVar(&c.ButtonGPIOchip, "buttons.gpiochip", "/dev/gpiochip0", "GPIO chip device for buttons")
	flag.BoolVar(&c.ButtonActiveLow, "buttons.active_low", true, "buttons are active-low (pressed reads 0)")
	flag.DurationVar(&c.ButtonDebounce, "buttons.debounce", 25*time.Millisecond, "button sampling/debounce interval")

	flag.Parse()

	// Secrets are usually supplied via the environment rather than argv.
	if v := os.Getenv("THERMOSTAT_HA_TOKEN"); v != "" {
		c.HAToken = v
	}
	if v := os.Getenv("THERMOSTAT_HA_URL"); v != "" {
		c.HAURL = v
	}

	return c
}
