package waveshare

import (
	"testing"
	"unsafe"

	"gijs.eu/thermostat/internal/fb"
)

func TestPack4Color(t *testing.T) {
	// Four pixels across -> one byte, with the leftmost pixel in the two most
	// significant bits.
	pix := []byte{255, 0, 0, 0}                          // black, white, white, white
	accent := []byte{0, 0, fb.AccentYellow, fb.AccentRed} // none, none, yellow, red
	got := pack4Color(4, 1, pix, accent)
	if len(got) != 1 {
		t.Fatalf("len = %d, want 1", len(got))
	}
	// black=00, white=01, yellow=10, red=11 -> 0b00_01_10_11 = 0x1B
	if got[0] != 0x1B {
		t.Fatalf("byte = %#02x, want 0x1B", got[0])
	}
}

func TestPixelCode(t *testing.T) {
	cases := []struct {
		pix    byte
		accent []byte
		want   byte
	}{
		{255, nil, 0},              // black
		{0, nil, 1},                // white
		{0, []byte{fb.AccentRed}, 3},    // red
		{0, []byte{fb.AccentYellow}, 2}, // yellow
	}
	for _, c := range cases {
		if got := pixelCode(c.pix, c.accent, 0); got != c.want {
			t.Fatalf("pixelCode(%d, %v) = %d, want %d", c.pix, c.accent, got, c.want)
		}
	}
}

func TestLookupPanel(t *testing.T) {
	p, ok := lookupPanel("4in26g")
	if !ok {
		t.Fatal("4in26g preset not found")
	}
	if p.width != 800 || p.height != 480 || p.initHeight != 680 {
		t.Fatalf("4in26g = %dx%d (init %d), want 800x480 (init 680)", p.width, p.height, p.initHeight)
	}

	if _, ok := lookupPanel("not-a-panel"); ok {
		t.Fatal("unknown panel name unexpectedly matched")
	}
}

func TestSPIStructSize(t *testing.T) {
	if got := unsafe.Sizeof(spiIOCTransfer{}); got != 32 {
		t.Fatalf("spiIOCTransfer size = %d, want 32", got)
	}
	if spiIOCMessage1 != 0x40206b00 {
		t.Fatalf("spiIOCMessage1 = %#x, want 0x40206b00", spiIOCMessage1)
	}
}
