package fb

import "testing"

func TestTextWidth(t *testing.T) {
	if got := TextWidth("", 1); got != 0 {
		t.Fatalf("TextWidth(\"\", 1) = %d, want 0", got)
	}
	if got := TextWidth("21.5", 1); got <= 0 {
		t.Fatalf("TextWidth(\"21.5\", 1) = %d, want > 0", got)
	}
}

func TestTextRendersInk(t *testing.T) {
	f := NewFrame(64, 32)
	f.Text("A", 0, 0, 2)

	found := false
	for _, v := range f.Pix {
		if v > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("Text did not set any ink")
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

func TestSetInkAccent(t *testing.T) {
	f := NewColorFrame(2, 1)

	f.SetInk(0, 0, AccentRed)
	if f.Pix[0] != White {
		t.Fatalf("red pixel Pix = %d, want white", f.Pix[0])
	}
	if f.Accent[0] != AccentRed {
		t.Fatalf("red pixel accent = %d, want AccentRed", f.Accent[0])
	}

	f.SetInk(1, 0, Black)
	if f.Pix[1] != Black {
		t.Fatalf("black pixel Pix = %d, want black", f.Pix[1])
	}
	if f.Accent[1] != AccentNone {
		t.Fatalf("black pixel accent = %d, want none", f.Accent[1])
	}

	f.SetInk(1, 0, AccentNone)
	if f.Pix[1] != White || f.Accent[1] != AccentNone {
		t.Fatalf("clearing ink did not reset pixel")
	}
}

func TestColorFrameCloneIndependent(t *testing.T) {
	f := NewColorFrame(1, 1)
	f.SetInk(0, 0, AccentYellow)

	c := f.Clone()
	c.SetInk(0, 0, Black)

	if f.Accent[0] != AccentYellow {
		t.Fatalf("clone mutated the original accent plane")
	}
}
