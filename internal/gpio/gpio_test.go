package gpio

import (
	"testing"
	"unsafe"
)

// Guard against struct layout drift: these sizes are baked into the ioctl
// request codes, so they must match the kernel's gpiohandle structs exactly.
func TestStructSizes(t *testing.T) {
	if got := unsafe.Sizeof(gpioHandleRequest{}); got != 364 {
		t.Fatalf("gpioHandleRequest size = %d, want 364", got)
	}
	if got := unsafe.Sizeof(gpioHandleData{}); got != 64 {
		t.Fatalf("gpioHandleData size = %d, want 64", got)
	}
}
