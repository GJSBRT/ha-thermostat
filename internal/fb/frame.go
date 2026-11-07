// Package fb provides the pixel framebuffer model shared by every display
// backend (the e-paper panel, the device framebuffer, and the development GUI
// window), plus the bitmap font and screen layout used to render the UI. It is
// pure standard library and has no GUI or hardware dependencies.
package fb

import (
	"image"
	"unicode"
)

// Frame is a grayscale image with an optional accent-color plane.
//
// Pix holds the black ink coverage of each pixel: 0 means paper/white and 255
// means ink/black. The buffer is row-major with no padding.
//
// Accent, when non-nil, holds the accent color of each pixel for three-color
// e-paper panels (Waveshare black/white/red and black/white/yellow). A value of
// AccentNone leaves the pixel rendered from Pix, while AccentRed or AccentYellow
// colors the pixel. Monochrome backends simply ignore Accent.
type Frame struct {
	Width  int
	Height int
	Pix    []byte
	Accent []byte
}

const (
	White = 0x00 // paper, no ink
	Black = 0xFF // ink
)

// Accent colors for Waveshare three-color panels. The panel's second ink plane
// is physically either red or yellow depending on the model; both constants are
// stored in the frame so the development window can show red and yellow, while
// a three-color driver maps any non-none accent to its single accent plane.
const (
	AccentNone   byte = 0
	AccentRed    byte = 1
	AccentYellow byte = 2
)

// NewFrame returns a blank (white, monochrome) frame of the given dimensions.
func NewFrame(width, height int) *Frame {
	return &Frame{Width: width, Height: height, Pix: make([]byte, width*height)}
}

// NewColorFrame returns a blank (white) frame with an accent-color plane.
func NewColorFrame(width, height int) *Frame {
	f := NewFrame(width, height)
	f.Accent = make([]byte, width*height)
	return f
}

// Clone returns a deep copy of the frame, including any accent plane.
func (f *Frame) Clone() *Frame {
	n := NewFrame(f.Width, f.Height)
	copy(n.Pix, f.Pix)
	if f.Accent != nil {
		n.Accent = make([]byte, len(f.Accent))
		copy(n.Accent, f.Accent)
	}
	return n
}

// Clear resets every pixel to white (and no accent).
func (f *Frame) Clear() {
	for i := range f.Pix {
		f.Pix[i] = White
	}
	if f.Accent != nil {
		for i := range f.Accent {
			f.Accent[i] = AccentNone
		}
	}
}

// ensureAccent lazily allocates the accent plane when it is first needed.
func (f *Frame) ensureAccent() {
	if f.Accent == nil || len(f.Accent) != f.Width*f.Height {
		f.Accent = make([]byte, f.Width*f.Height)
	}
}

// SetPixel sets a single grayscale pixel, ignoring out-of-bounds coordinates.
// It also clears any accent color at that pixel.
func (f *Frame) SetPixel(x, y int, v byte) {
	if x < 0 || y < 0 || x >= f.Width || y >= f.Height {
		return
	}
	i := y*f.Width + x
	f.Pix[i] = v
	if f.Accent != nil {
		f.Accent[i] = AccentNone
	}
}

// SetInk sets a single pixel to an ink color: Black, AccentRed, AccentYellow, or
// AccentNone (white). Out-of-bounds coordinates are ignored.
func (f *Frame) SetInk(x, y int, ink byte) {
	if x < 0 || y < 0 || x >= f.Width || y >= f.Height {
		return
	}
	f.setInk(x, y, ink)
}

func (f *Frame) setInk(x, y int, ink byte) {
	i := y*f.Width + x
	switch ink {
	case Black:
		f.Pix[i] = Black
		if f.Accent != nil {
			f.Accent[i] = AccentNone
		}
	case AccentRed, AccentYellow:
		f.ensureAccent()
		f.Pix[i] = White
		f.Accent[i] = ink
	default: // AccentNone (and any unknown value) renders as paper/white.
		f.Pix[i] = White
		if f.Accent != nil {
			f.Accent[i] = AccentNone
		}
	}
}

// FillRect fills the inclusive rectangle (x0,y0)-(x1,y1) with a grayscale value,
// clamping to bounds and clearing any accent color in the region.
func (f *Frame) FillRect(x0, y0, x1, y1 int, v byte) {
	x0, y0, x1, y1 = f.clampRect(x0, y0, x1, y1)
	for y := y0; y <= y1; y++ {
		row := y * f.Width
		for x := x0; x <= x1; x++ {
			f.Pix[row+x] = v
		}
	}
	if f.Accent != nil {
		for y := y0; y <= y1; y++ {
			row := y * f.Width
			for x := x0; x <= x1; x++ {
				f.Accent[row+x] = AccentNone
			}
		}
	}
}

// FillInk fills the inclusive rectangle (x0,y0)-(x1,y1) with an ink color
// (Black, AccentRed, AccentYellow, or AccentNone), clamping to bounds.
func (f *Frame) FillInk(x0, y0, x1, y1 int, ink byte) {
	x0, y0, x1, y1 = f.clampRect(x0, y0, x1, y1)
	for y := y0; y <= y1; y++ {
		for x := x0; x <= x1; x++ {
			f.setInk(x, y, ink)
		}
	}
}

// clampRect clamps an inclusive rectangle to the frame bounds and reports
// whether any area remains to draw.
func (f *Frame) clampRect(x0, y0, x1, y1 int) (int, int, int, int) {
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 >= f.Width {
		x1 = f.Width - 1
	}
	if y1 >= f.Height {
		y1 = f.Height - 1
	}
	return x0, y0, x1, y1
}

// DrawGlyph draws a single 5x7 glyph with its top-left corner at (x, y),
// scaled by scale (each font pixel becomes a scale-by-scale block). The glyph is
// drawn in black ink.
func (f *Frame) DrawGlyph(g [7]byte, x, y, scale int) {
	f.drawGlyph(g, x, y, scale, Black)
}

// DrawGlyphColor draws a single glyph in the given ink color.
func (f *Frame) DrawGlyphColor(g [7]byte, x, y, scale int, ink byte) {
	f.drawGlyph(g, x, y, scale, ink)
}

func (f *Frame) drawGlyph(g [7]byte, x, y, scale int, ink byte) {
	for row := 0; row < 7; row++ {
		for col := 0; col < 5; col++ {
			if g[row]>>uint(col)&1 != 0 {
				f.FillInk(x+col*scale, y+row*scale, x+(col+1)*scale-1, y+(row+1)*scale-1, ink)
			}
		}
	}
}

// Text draws s with its top-left corner at (x, y) in black and returns the
// advance width in pixels. Unknown runes are rendered as spaces.
func (f *Frame) Text(s string, x, y, scale int) int {
	return f.TextColor(s, x, y, scale, Black)
}

// TextColor draws s in the given ink color and returns the advance width.
func (f *Frame) TextColor(s string, x, y, scale int, ink byte) int {
	cx := x
	for _, r := range s {
		g, ok := font5x7[unicode.ToUpper(r)]
		if !ok {
			g = font5x7[' ']
		}
		f.drawGlyph(g, cx, y, scale, ink)
		cx += 6 * scale
	}
	return cx - x
}

// TextWidth returns the pixel width of s rendered at the given scale.
func TextWidth(s string, scale int) int {
	n := 0
	for range s {
		n++
	}
	return n * 6 * scale
}

// TextHeight returns the pixel height of text rendered at the given scale.
func TextHeight(scale int) int {
	return 7 * scale
}

// TextCentered draws s centered horizontally at the given y coordinate in black.
func (f *Frame) TextCentered(s string, y, scale int) {
	f.TextCenteredColor(s, y, scale, Black)
}

// TextCenteredColor draws s centered horizontally in the given ink color.
func (f *Frame) TextCenteredColor(s string, y, scale int, ink byte) {
	x := (f.Width - TextWidth(s, scale)) / 2
	f.TextColor(s, x, y, scale, ink)
}

// Image converts the frame to a standard image.Gray for display or export. The
// accent plane is ignored, matching the monochrome backends.
func (f *Frame) Image() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, f.Width, f.Height))
	for i, v := range f.Pix {
		img.Pix[i] = 255 - v
	}
	return img
}
