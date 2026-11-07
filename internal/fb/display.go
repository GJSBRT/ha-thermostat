package fb

// Display renders frames to a physical or virtual screen. Implementations are
// the e-paper/framebuffer device and the development GUI window.
type Display interface {
	Show(frame *Frame) error
	Close() error
}
