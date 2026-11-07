// Package state defines the shared data model exchanged between the Home
// Assistant client, input devices, display, and controller. It is deliberately
// a plain data structure so it can cross package boundaries without depending
// on any specific backend.
package state

// State is a point-in-time snapshot of everything the thermostat knows and
// displays. All values are owned by Home Assistant.
type State struct {
	// Thermostat values owned by Home Assistant.
	CurrentTemp float64 // current room temperature reported by HA
	TargetTemp  float64 // desired temperature
	Mode        string  // heat, cool, heat_cool, auto, off
	Action      string  // idle, heating, cooling, off

	// Outside temperature, read from a separate Home Assistant entity. It is
	// displayed only when OutsideTempSet is true (an entity is configured and
	// returns a numeric value).
	OutsideTemp    float64
	OutsideTempSet bool

	// Connectivity.
	HAConnected bool
	HAError     string
}
