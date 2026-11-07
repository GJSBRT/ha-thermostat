// Package input defines the events that drive the thermostat and the sources
// that produce them (GPIO buttons on the device, the keyboard in development).
//
// Events model press and release transitions rather than single "taps", so the
// controller can implement press-and-hold auto-repeat in one place.
package input

// Button identifies a user control.
type Button int

const (
	ButtonNone Button = iota
	ButtonUp
	ButtonDown
	ButtonQuit
)

// Phase describes whether a button transitioned to pressed or released.
type Phase int

const (
	PhaseReleased Phase = iota
	PhasePressed
)

// Event is a single button press or release transition.
type Event struct {
	Button Button
	Phase  Phase
}

func (e Event) String() string {
	phase := "released"
	if e.Phase == PhasePressed {
		phase = "pressed"
	}
	switch e.Button {
	case ButtonUp:
		return "up " + phase
	case ButtonDown:
		return "down " + phase
	case ButtonQuit:
		return "quit " + phase
	default:
		return "none"
	}
}
