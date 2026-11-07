package waveshare

import "testing"

func TestPackPlane(t *testing.T) {
	// 10 pixels wide -> 2 bytes per row, with the last 6 bits padded.
	got := packPlane(10, 1, func(x, y int) bool {
		return x == 0 || x == 7
	})
	if len(got) != 2 {
		t.Fatalf("stride = %d bytes, want 2", len(got))
	}
	// Bit 7 (leftmost pixel 0) and bit 0 (pixel 7) set in the first byte.
	if got[0] != 0x81 {
		t.Fatalf("first byte = %#02x, want 0x81", got[0])
	}
	if got[1] != 0x00 {
		t.Fatalf("second byte = %#02x, want 0x00 (padding)", got[1])
	}
}

func TestLookupPanel(t *testing.T) {
	p, ok := lookupPanel("7in5b_v2")
	if !ok {
		t.Fatal("7in5b_v2 preset not found")
	}
	if p.width != 800 || p.height != 480 {
		t.Fatalf("7in5b_v2 = %dx%d, want 800x480", p.width, p.height)
	}

	if _, ok := lookupPanel("not-a-panel"); ok {
		t.Fatal("unknown panel name unexpectedly matched")
	}
}
