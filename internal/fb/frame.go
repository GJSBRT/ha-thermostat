// Package fb provides the pixel framebuffer model shared by every display
// backend (the e-paper panel, the device framebuffer, and the development GUI
// window), plus the bitmap font and screen layout used to render the UI. It is
// pure standard library and has no GUI or hardware dependencies.
package fb

import (
	"image"
	"unicode"
)

// Frame is a grayscale image. Each byte in Pix is the "ink coverage" of a
// pixel: 0 means paper/white and 255 means ink/black. The buffer is row-major
// with no padding.
type Frame struct {
	Width  int
	Height int
	Pix    []byte
}

const (
	White = 0x00 // paper, no ink
	Black = 0xFF // ink
)

// NewFrame returns a blank (white) frame of the given dimensions.
func NewFrame(width, height int) *Frame {
	return &Frame{Width: width, Height: height, Pix: make([]byte, width*height)}
}

// Clone returns a deep copy of the frame.
func (f *Frame) Clone() *Frame {
	n := NewFrame(f.Width, f.Height)
	copy(n.Pix, f.Pix)
	return n
}

// Clear resets every pixel to white.
func (f *Frame) Clear() {
	for i := range f.Pix {
		f.Pix[i] = White
	}
}

// SetPixel sets a single pixel, ignoring out-of-bounds coordinates.
func (f *Frame) SetPixel(x, y int, v byte) {
	if x < 0 || y < 0 || x >= f.Width || y >= f.Height {
		return
	}
	f.Pix[y*f.Width+x] = v
}

// FillRect fills the inclusive rectangle (x0,y0)-(x1,y1), clamping to bounds.
func (f *Frame) FillRect(x0, y0, x1, y1 int, v byte) {
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
	if x0 > x1 || y0 > y1 {
		return
	}
	for y := y0; y <= y1; y++ {
		row := y * f.Width
		for x := x0; x <= x1; x++ {
			f.Pix[row+x] = v
		}
	}
}

// DrawGlyph draws a single 5x7 glyph with its top-left corner at (x, y),
// scaled by scale (each font pixel becomes a scale-by-scale block).
func (f *Frame) DrawGlyph(g [7]byte, x, y, scale int) {
	for row := 0; row < 7; row++ {
		for col := 0; col < 5; col++ {
			if g[row]>>uint(col)&1 != 0 {
				f.FillRect(x+col*scale, y+row*scale, x+(col+1)*scale-1, y+(row+1)*scale-1, Black)
			}
		}
	}
}

// Text draws s with its top-left corner at (x, y) and returns the advance
// width in pixels. Unknown runes are rendered as spaces.
func (f *Frame) Text(s string, x, y, scale int) int {
	cx := x
	for _, r := range s {
		g, ok := font5x7[unicode.ToUpper(r)]
		if !ok {
			g = font5x7[' ']
		}
		f.DrawGlyph(g, cx, y, scale)
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


// TextCentered draws s centered horizontally at the given y coordinate.
func (f *Frame) TextCentered(s string, y, scale int) {
	x := (f.Width - TextWidth(s, scale)) / 2
	f.Text(s, x, y, scale)
}

// Image converts the frame to a standard image.Gray for display or export.
func (f *Frame) Image() *image.Gray {
	img := image.NewGray(image.Rect(0, 0, f.Width, f.Height))
	for i, v := range f.Pix {
		img.Pix[i] = 255 - v
	}
	return img
}
