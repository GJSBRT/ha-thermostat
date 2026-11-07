package fb

import (
	"fmt"
	"os"
)

// FramebufferConfig configures a Linux framebuffer backend.
type FramebufferConfig struct {
	Path   string // e.g. /dev/fb0
	Width  int
	Height int
	Format string // "gray8", "rgb565", or "mono" (1-bit packed)
}

// Framebuffer writes frames directly to a Linux framebuffer device. This is
// the production path for a 5" e-paper panel exposed as a framebuffer (for
// example by the fbtft or epdiy drivers, which present the panel at /dev/fb0).
type Framebuffer struct {
	cfg FramebufferConfig
	f   *os.File
}

// NewFramebuffer opens the device in write-only mode.
func NewFramebuffer(cfg FramebufferConfig) (*Framebuffer, error) {
	if cfg.Path == "" {
		cfg.Path = "/dev/fb0"
	}
	f, err := os.OpenFile(cfg.Path, os.O_WRONLY, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", cfg.Path, err)
	}
	return &Framebuffer{cfg: cfg, f: f}, nil
}

// Show converts the frame to the device's pixel format and writes it.
func (fb *Framebuffer) Show(frame *Frame) error {
	data, err := encode(frame, fb.cfg.Format)
	if err != nil {
		return err
	}
	_, err = fb.f.WriteAt(data, 0)
	return err
}

// Close releases the device.
func (fb *Framebuffer) Close() error {
	return fb.f.Close()
}

// encode converts an ink-coverage frame (0 = white, 255 = black) to the
// device's native pixel format.
func encode(f *Frame, format string) ([]byte, error) {
	switch format {
	case "", "gray8":
		out := make([]byte, len(f.Pix))
		for i, v := range f.Pix {
			out[i] = 255 - v // framebuffer grayscale: 0 black, 255 white
		}
		return out, nil

	case "rgb565":
		out := make([]byte, f.Width*f.Height*2)
		for i, v := range f.Pix {
			g := 255 - int(v) // 0 black .. 255 white
			val := uint16(g>>3)<<11 | uint16(g>>2)<<5 | uint16(g>>3)
			out[i*2] = byte(val & 0xFF)
			out[i*2+1] = byte(val >> 8)
		}
		return out, nil

	case "mono":
		stride := (f.Width + 7) / 8
		out := make([]byte, stride*f.Height)
		for y := 0; y < f.Height; y++ {
			row := y * f.Width
			for x := 0; x < f.Width; x++ {
				if f.Pix[row+x] >= 128 {
					bit := uint(7 - x%8)
					out[y*stride+x/8] |= byte(1 << bit)
				}
			}
		}
		return out, nil

	default:
		return nil, fmt.Errorf("unsupported framebuffer format %q", format)
	}
}
