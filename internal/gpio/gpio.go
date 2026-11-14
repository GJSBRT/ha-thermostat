// Package gpio drives Linux GPIO lines through the GPIO character device
// interface (/dev/gpiochipN). It replaces the deprecated sysfs
// /sys/class/gpio interface, which recent Raspberry Pi kernels no longer
// expose.
//
// The package is stdlib-only and cgo-free: it talks to the kernel with raw
// ioctl calls, matching the style of the SPI driver in internal/waveshare.
//
// It uses the older "v1" linehandle ioctls (GPIO_GET_LINEHANDLE_IOCTL). They
// are deprecated in favour of the v2 uAPI but remain supported by current
// kernels and are far simpler, which keeps this low-level code easy to audit.
package gpio

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// Direction selects whether a line is requested for input or output.
type Direction int

const (
	Input Direction = iota
	Output
)

// Config describes a single GPIO line request.
type Config struct {
	Chip      string    // gpiochip device path, e.g. /dev/gpiochip0
	Offset    int       // line offset within the chip (BCM pin number on Raspberry Pi)
	Direction Direction // Input or Output
	Consumer  string    // consumer label shown by gpioinfo
	PullUp    bool      // enable the internal pull-up (input lines only)
}

// Line is a single requested GPIO line.
type Line struct {
	fd int
}

// These ioctl request values derive from the kernel's uapi/linux/gpio.h. The
// magic number is 0xB4 and the encoded payload sizes are the v1 linehandle
// structs below.
const (
	gpioGetLinehandleIOCTL = 0xC16CB403 // _IOWR(0xB4, 0x03, struct gpiohandle_request) — 364 bytes
	gpioGetLinevaluesIOCTL = 0xC040B408 // _IOWR(0xB4, 0x08, struct gpiohandle_data) — 64 bytes
	gpioSetLinevaluesIOCTL = 0xC040B409 // _IOWR(0xB4, 0x09, struct gpiohandle_data) — 64 bytes

	requestInput      = 1 << 0 // GPIOHANDLE_REQUEST_INPUT
	requestOutput     = 1 << 1 // GPIOHANDLE_REQUEST_OUTPUT
	requestBiasPullUp = 1 << 7 // GPIOHANDLE_REQUEST_BIAS_PULL_UP
)

// gpioHandleRequest mirrors the kernel's struct gpiohandle_request. The field
// order and sizes must match the kernel ABI exactly (364 bytes on 32-bit and
// 64-bit targets).
type gpioHandleRequest struct {
	lineOffsets   [64]uint32
	flags         uint32
	defaultValues [64]uint8
	consumerLabel [32]byte
	lines         uint32
	fd            int32
}

// gpioHandleData mirrors the kernel's struct gpiohandle_data.
type gpioHandleData struct {
	values [64]uint8
}

// Open requests a single GPIO line and returns a handle for reading/writing it.
func Open(cfg Config) (*Line, error) {
	if cfg.Chip == "" {
		cfg.Chip = "/dev/gpiochip0"
	}
	if cfg.Offset < 0 {
		return nil, fmt.Errorf("gpio: negative line offset %d", cfg.Offset)
	}
	if cfg.Consumer == "" {
		cfg.Consumer = "thermostat"
	}

	flags := uint32(0)
	switch cfg.Direction {
	case Input:
		flags |= requestInput
	case Output:
		flags |= requestOutput
	default:
		return nil, fmt.Errorf("gpio: invalid direction %d", cfg.Direction)
	}
	if cfg.PullUp {
		if cfg.Direction != Input {
			return nil, fmt.Errorf("gpio: pull-up only applies to input lines")
		}
		flags |= requestBiasPullUp
	}

	chip, err := syscall.Open(cfg.Chip, syscall.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("gpio: open %s: %w", cfg.Chip, err)
	}
	defer syscall.Close(chip)

	req := gpioHandleRequest{
		lineOffsets: [64]uint32{uint32(cfg.Offset)},
		flags:       flags,
		lines:       1,
		fd:          -1,
	}
	copy(req.consumerLabel[:], cfg.Consumer)

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(chip), gpioGetLinehandleIOCTL, uintptr(unsafe.Pointer(&req))); errno != 0 {
		return nil, fmt.Errorf("gpio: request line %d: %w", cfg.Offset, errno)
	}
	fd := req.fd
	runtime.KeepAlive(req)
	if fd < 0 {
		return nil, fmt.Errorf("gpio: request line %d: no file descriptor returned", cfg.Offset)
	}

	return &Line{fd: int(fd)}, nil
}

// Set drives an output line high or low.
func (l *Line) Set(high bool) error {
	var data gpioHandleData
	if high {
		data.values[0] = 1
	}
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(l.fd), gpioSetLinevaluesIOCTL, uintptr(unsafe.Pointer(&data))); errno != 0 {
		return errno
	}
	runtime.KeepAlive(data)
	return nil
}

// Get returns the current state of an input line (true = high).
func (l *Line) Get() (bool, error) {
	var data gpioHandleData
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(l.fd), gpioGetLinevaluesIOCTL, uintptr(unsafe.Pointer(&data))); errno != 0 {
		return false, errno
	}
	runtime.KeepAlive(data)
	return data.values[0] != 0, nil
}

// Close releases the line.
func (l *Line) Close() error {
	if l.fd < 0 {
		return nil
	}
	err := syscall.Close(l.fd)
	l.fd = -1
	return err
}
