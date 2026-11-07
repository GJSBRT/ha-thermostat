package fb

import "testing"

func TestTextWidth(t *testing.T) {
	if got, want := TextWidth("21.5", 1), 4*6; got != want {
		t.Fatalf("TextWidth(\"21.5\", 1) = %d, want %d", got, want)
	}
	if got, want := TextWidth("°C", 2), 2*6*2; got != want {
		t.Fatalf("TextWidth(\"°C\", 2) = %d, want %d", got, want)
	}
}

func TestFrameClearAndDraw(t *testing.T) {
	f := NewFrame(4, 4)
	for i := range f.Pix {
		if f.Pix[i] != White {
			t.Fatalf("new frame pixel %d = %d, want white", i, f.Pix[i])
		}
	}

	f.SetPixel(0, 0, Black)
	if f.Pix[0] != Black {
		t.Fatal("SetPixel did not set the pixel")
	}

	f.Clear()
	if f.Pix[0] != White {
		t.Fatal("Clear did not reset the pixel")
	}
}

func TestFrameFillRectClamps(t *testing.T) {
	f := NewFrame(2, 2)
	f.FillRect(-5, -5, 100, 100, Black)
	for i, v := range f.Pix {
		if v != Black {
			t.Fatalf("pixel %d = %d, want black after clamped fill", i, v)
		}
	}
}

func TestEncodeMono(t *testing.T) {
	f := NewFrame(8, 1)
	f.SetPixel(0, 0, Black)
	f.SetPixel(7, 0, Black)

	out, err := encode(f, "mono")
	if err != nil {
		t.Fatalf("encode mono: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("mono stride = %d, want 1", len(out))
	}
	// Bit 7 (leftmost) and bit 0 (rightmost) set -> 0x81.
	if out[0] != 0x81 {
		t.Fatalf("mono byte = %#02x, want 0x81", out[0])
	}
}
