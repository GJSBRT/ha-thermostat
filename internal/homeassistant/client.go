// Package homeassistant is a minimal client for the Home Assistant REST API,
// implemented entirely with the standard library.
package homeassistant

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Config holds the connection and entity configuration for Home Assistant.
type Config struct {
	BaseURL          string
	Token            string // long-lived access token
	ThermostatEntity string // e.g. climate.thermostat
	OutsideEntity    string // e.g. sensor.outside_temperature (empty disables)
	Timeout          time.Duration
}

// Client talks to a single Home Assistant instance.
type Client struct {
	cfg Config
	hc  *http.Client
}

// New returns a client using the given configuration.
func New(cfg Config) *Client {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 5 * time.Second
	}
	return &Client{cfg: cfg, hc: &http.Client{Timeout: cfg.Timeout}}
}

// ThermostatState is the subset of a Home Assistant climate entity that the
// thermostat cares about.
type ThermostatState struct {
	Mode         string
	TargetTemp   float64
	CurrentTemp  float64
	Action       string
	FriendlyName string
}

// ThermostatState fetches the current state of the climate entity.
func (c *Client) ThermostatState() (ThermostatState, error) {
	var raw struct {
		State      string `json:"state"`
		Attributes struct {
			Temperature        float64 `json:"temperature"`
			CurrentTemperature float64 `json:"current_temperature"`
			HvacAction         string  `json:"hvac_action"`
			FriendlyName       string  `json:"friendly_name"`
		} `json:"attributes"`
	}
	if err := c.get("/api/states/"+c.cfg.ThermostatEntity, &raw); err != nil {
		return ThermostatState{}, err
	}
	return ThermostatState{
		Mode:         raw.State,
		TargetTemp:   raw.Attributes.Temperature,
		CurrentTemp:  raw.Attributes.CurrentTemperature,
		Action:       raw.Attributes.HvacAction,
		FriendlyName: raw.Attributes.FriendlyName,
	}, nil
}

// SetTargetTemperature asks Home Assistant to change the thermostat target.
func (c *Client) SetTargetTemperature(temp float64) error {
	return c.post("/api/services/climate/set_temperature", map[string]any{
		"entity_id":   c.cfg.ThermostatEntity,
		"temperature": temp,
	})
}

// OutsideTemperature fetches the numeric state of the optional outside
// temperature entity. It returns ok=false when no entity is configured or the
// entity's state is not a usable number; the caller should then hide the
// outside readout.
func (c *Client) OutsideTemperature() (temp float64, ok bool, err error) {
	if c.cfg.OutsideEntity == "" {
		return 0, false, nil
	}
	var raw struct {
		State string `json:"state"`
	}
	if err := c.get("/api/states/"+c.cfg.OutsideEntity, &raw); err != nil {
		return 0, false, err
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(raw.State), 64)
	if err != nil {
		return 0, false, nil
	}
	return v, true, nil
}

func (c *Client) get(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.base()+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) post(path string, body any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.base()+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, nil)
}

func (c *Client) do(req *http.Request, out any) error {
	req.Header.Set("Authorization", "Bearer "+c.cfg.Token)
	resp, err := c.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s: %s", resp.Status, strings.TrimSpace(string(b)))
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return nil
}

func (c *Client) base() string {
	return strings.TrimRight(c.cfg.BaseURL, "/")
}
