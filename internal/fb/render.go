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
}

// NewRenderer returns a renderer for the given panel resolution.
func NewRenderer(width, height int) *Renderer {
	return &Renderer{Width: width, Height: height}
}

// Render produces a frame for the given state. When editing is true the screen
// is cleared and the target temperature is shown full screen.
func (r *Renderer) Render(st state.State) *Frame {
	f := NewFrame(r.Width, r.Height)
	r.renderNormal(f, st)
	return f
}

// renderNormal draws the two-column overview.
func (r *Renderer) renderNormal(f *Frame, st state.State) {
	m := 24

	f.Text(strings.ToUpper(st.Action), m, m, 3)

	if !st.HAConnected {
		status := "HA OFFLINE"
		f.Text(status, f.Width-m-TextWidth(status, 2), m, 2)
	}

	f.Text(fmt.Sprintf("CURRENT %.1f°C", st.CurrentTemp), m, r.Height - m - TextHeight(4), 4)

	drawTemp(f, formatTemp(st.TargetTemp), m, (r.Height / 2) - TextHeight(25) + m * 2, 25)

	if st.HAError != "" {
		f.Text("ERROR: "+truncate(st.HAError, 64), m, 230, 1)
	}
}

// drawTemp draws a temperature value followed by a smaller "°C" unit.
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
