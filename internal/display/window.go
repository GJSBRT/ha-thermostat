// Package display provides the development GUI window that emulates the 5"
// e-paper screen and captures keyboard input.
//
// It is implemented with Ebiten (github.com/hajimehoshi/ebiten/v2), which uses
// github.com/ebitengine/purego to call native libraries via dlopen/dlsym rather
// than cgo. This keeps the whole project buildable with CGO_ENABLED=0.
package display

import (
	"image/color"
	"sync"
	"sync/atomic"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"

	"gijs.eu/thermostat/internal/fb"
	"gijs.eu/thermostat/internal/input"
)

// Window renders the thermostat framebuffer in a desktop window and translates
// key presses into input events. It implements both fb.Display and ebiten.Game.
type Window struct {
	width  int
	height int
	events chan<- input.Event

	mu    sync.Mutex
	rgba  []byte // latest frame in RGBA, written by Show
	dirty bool
	img   *ebiten.Image

	closeRequested atomic.Bool
}

// NewWindow configures the window. It must be called before Run.
func NewWindow(width, height int, events chan<- input.Event) *Window {
	ebiten.SetWindowSize(width, height)
	ebiten.SetWindowTitle("Thermostat")
	return &Window{width: width, height: height, events: events}
}

// Update implements ebiten.Game. It runs on the game loop goroutine and turns
// key transitions into press/release input events.
func (w *Window) Update() error {
	if ebiten.IsWindowBeingClosed() || w.closeRequested.Load() {
		return ebiten.Termination
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyUp) {
		w.events <- input.Event{Button: input.ButtonUp, Phase: input.PhasePressed}
	}
	if inpututil.IsKeyJustReleased(ebiten.KeyUp) {
		w.events <- input.Event{Button: input.ButtonUp, Phase: input.PhaseReleased}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyDown) {
		w.events <- input.Event{Button: input.ButtonDown, Phase: input.PhasePressed}
	}
	if inpututil.IsKeyJustReleased(ebiten.KeyDown) {
		w.events <- input.Event{Button: input.ButtonDown, Phase: input.PhaseReleased}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) || inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		w.events <- input.Event{Button: input.ButtonQuit, Phase: input.PhasePressed}
	}
	return nil
}

// Draw implements ebiten.Game. It uploads the latest frame (if any) and draws
// it to the window.
func (w *Window) Draw(screen *ebiten.Image) {
	w.mu.Lock()
	if w.dirty {
		if w.img == nil {
			w.img = ebiten.NewImage(w.width, w.height)
		}
		w.img.ReplacePixels(w.rgba)
		w.dirty = false
	}
	img := w.img
	w.mu.Unlock()

	if img == nil {
		screen.Fill(color.White)
		return
	}
	screen.DrawImage(img, &ebiten.DrawImageOptions{})
}

// Layout implements ebiten.Game. It fixes the logical screen size to the panel
// resolution.
func (w *Window) Layout(_, _ int) (int, int) {
	return w.width, w.height
}

// Show stores the latest frame for the next repaint. It is safe to call from
// any goroutine.
func (w *Window) Show(frame *fb.Frame) error {
	w.mu.Lock()
	w.rgba = frameRGBA(frame)
	w.dirty = true
	w.mu.Unlock()
	return nil
}

// Close requests termination. The game loop observes the flag and calls
// ebiten.Terminate on its own goroutine, avoiding any cross-goroutine teardown.
func (w *Window) Close() error {
	w.closeRequested.Store(true)
	return nil
}

// Run starts the Ebiten game loop. It blocks until the window is closed and
// must be called from the main goroutine.
func (w *Window) Run() {
	_ = ebiten.RunGame(w)
}

// frameRGBA converts an ink-coverage frame (0 = white, 255 = black) into RGBA
// pixels for Ebiten.
func frameRGBA(f *fb.Frame) []byte {
	rgba := make([]byte, f.Width*f.Height*4)
	for i, v := range f.Pix {
		c := 255 - v // 0 black .. 255 white
		rgba[i*4] = c
		rgba[i*4+1] = c
		rgba[i*4+2] = c
		rgba[i*4+3] = 0xFF
	}
	return rgba
}
