package fb

import (
	"sync"

	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"

	_ "embed"
)

// uiFont is parsed once from the embedded Go Regular font. The Go typeface is a
// proportional, humanist sans-serif that reads much more smoothly than the
// hand-drawn 5x7 pixel font it replaces. The font bytes are embedded in the
// golang.org/x/image module, so no separate font file has to be vendored here.
var (
	uiFontOnce sync.Once
	uiFont     *sfnt.Font
)

//go:embed font.ttf
var fontFile []byte

// faceCache reuses one font.Face per pixel size. Faces are cheap to create but
// caching them avoids re-parsing glyph data on every render.
var faceCache = struct {
	sync.Mutex
	faces map[int]font.Face
}{faces: make(map[int]font.Face)}

// parsedFont returns the parsed embedded font, loading it on first use. The
// font is a compile-time constant, so a parse failure indicates a broken build
// and is treated as a programmer error.
func parsedFont() *sfnt.Font {
	uiFontOnce.Do(func() {
		f, err := opentype.Parse(fontFile)
		if err != nil {
			panic("fb: parse embedded font: " + err.Error())
		}
		uiFont = f
	})
	return uiFont
}

// faceForScale returns the font face for the given scale. The face is sized at
// 7*scale pixels, matching TextHeight so the existing scale-based layout keeps
// its proportions.
func faceForScale(scale int) font.Face {
	size := 7 * scale
	faceCache.Lock()
	defer faceCache.Unlock()
	if f, ok := faceCache.faces[size]; ok {
		return f
	}
	f, err := opentype.NewFace(parsedFont(), &opentype.FaceOptions{
		Size:    float64(size),
		DPI:     72,
		Hinting: font.HintingNone,
	})
	if err != nil {
		panic("fb: create font face: " + err.Error())
	}
	faceCache.faces[size] = f
	return f
}
