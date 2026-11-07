package fb

import (
	"fmt"
	"strings"

	"gijs.eu/thermostat/internal/state"
)

// Renderer draws the thermostat UI into a Frame of a fixed resolution.
type Renderer struct {
	Width  int
	Height int

	// Accent, when non-zero, is the accent ink color (AccentRed or AccentYellow)
	// used for the target temperature on three-color e-paper panels. Zero keeps
	// the output monochrome.
	Accent byte
}

// NewRenderer returns a renderer for the given panel resolution.
func NewRenderer(width, height int) *Renderer {
	return &Renderer{Width: width, Height: height}
}

// Render produces a frame for the given state. When editing is true the screen
// is cleared and the target temperature is shown full screen.
func (r *Renderer) Render(st state.State) *Frame {
	var f *Frame
	if r.Accent != 0 {
		f = NewColorFrame(r.Width, r.Height)
	} else {
		f = NewFrame(r.Width, r.Height)
	}
	r.renderNormal(f, st)
	return f
}

// renderNormal draws the two-column overview.
func (r *Renderer) renderNormal(f *Frame, st state.State) {
	m := 24

	actionColor := r.Accent
	if st.Action == "IDLE" {
		actionColor = Black
	}
	f.TextColor(strings.ToUpper(st.Action), m, m, 3, actionColor)

	if !st.HAConnected {
		status := "HA OFFLINE"
		f.TextColor(status, f.Width-m-TextWidth(status, 2), m, 2, r.Accent)
	}

	f.Text(fmt.Sprintf("CURRENT %.1f°C", st.CurrentTemp), m, r.Height - m - TextHeight(4), 4)

	drawTemp(f, formatTemp(st.TargetTemp), m, (r.Height / 2) - TextHeight(25) + m * 2, 25)

	if st.HAError != "" {
		f.TextColor("ERROR: "+truncate(st.HAError, 64), m, r.Height - TextHeight(4) - TextHeight(2) - m * 2, 2, r.Accent)
	}
}

// drawTemp draws a temperature value followed by a smaller "°C" unit. A non-zero
// ink selects an accent color for the numeric value; the unit stays black.
func drawTemp(f *Frame, s string, x, y, scale int) {
	f.Text(s, x, y, scale)
	unit := scale / 2
	if unit < 1 {
		unit = 1
	}
	f.Text("°C", x+TextWidth(s, scale)+4, y+2, unit)
}

func formatTemp(t float64) string {
	return fmt.Sprintf("%.1f", t)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
