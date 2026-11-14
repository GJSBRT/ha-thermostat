package waveshare

import (
	"fmt"
	"runtime"
	"syscall"
	"unsafe"
)

// The Linux spidev interface is driven through ioctl. These request values are
// the standard SPI ioctls (magic 'k'); the numbers are stable across the x86,
// ARM, and AArch64 targets this project runs on, because spi_ioc_transfer is 32
// bytes on both 32-bit and 64-bit kernels.
const (
	spiIOCWrMode        = 0x40016b01 // _IOW('k', 1, uint8)
	spiIOCWrBitsPerWord = 0x40016b03 // _IOW('k', 3, uint8)
	spiIOCWrMaxSpeedHz  = 0x40046b04 // _IOW('k', 4, uint32)
)

// spiIOCTransfer mirrors the kernel's struct spi_ioc_transfer. The field order
// and sizes must match the kernel ABI exactly (32 bytes on 32-bit and 64-bit).
type spiIOCTransfer struct {
	txBuf       uint64
	rxBuf       uint64
	length      uint32
	speedHz     uint32
	delayUsecs  uint16
	bitsPerWord uint8
	csChange    uint8
	txNBits     uint8
	rxNBits     uint8
	pad         uint16
}

// Compile-time guard: the ioctl request codes encode this struct's size, so it
// must stay 32 bytes (matching the kernel's struct spi_ioc_transfer).
var _ [32]byte = [unsafe.Sizeof(spiIOCTransfer{})]byte{}

// spiDevice is a Linux spidev file descriptor configured for the e-paper panel.
type spiDevice struct {
	fd int
}

// openSPI opens the SPI device and configures SPI mode 0, 8-bit words, and the
// requested clock speed.
func openSPI(path string, speedHz int) (*spiDevice, error) {
	if speedHz <= 0 {
		speedHz = 4000000
	}
	fd, err := syscall.Open(path, syscall.O_RDWR, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	d := &spiDevice{fd: fd}

	mode := uint8(0) // CPOL=0, CPHA=0
	if err := d.ioctl(spiIOCWrMode, unsafe.Pointer(&mode)); err != nil {
		d.Close()
		return nil, fmt.Errorf("spi mode: %w", err)
	}
	bits := uint8(8)
	if err := d.ioctl(spiIOCWrBitsPerWord, unsafe.Pointer(&bits)); err != nil {
		d.Close()
		return nil, fmt.Errorf("spi bits per word: %w", err)
	}
	speed := uint32(speedHz)
	if err := d.ioctl(spiIOCWrMaxSpeedHz, unsafe.Pointer(&speed)); err != nil {
		d.Close()
		return nil, fmt.Errorf("spi speed: %w", err)
	}
	return d, nil
}

// Close releases the SPI device.
func (d *spiDevice) Close() error {
	if d.fd < 0 {
		return nil
	}
	err := syscall.Close(d.fd)
	d.fd = -1
	return err
}

// ioctl issues a single ioctl request.
func (d *spiDevice) ioctl(req uintptr, arg unsafe.Pointer) error {
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(d.fd), req, uintptr(arg))
	if errno != 0 {
		return errno
	}
	return nil
}

// write sends tx bytes full-duplex over SPI, discarding the received data. The
// Raspberry Pi SPI controller rejects some transfers, so large writes are split
// into a sequence of smaller single-transfer ioctls (one SPI_IOC_MESSAGE(1)
// each). The panel tolerates chip-select toggling between transfers, as it
// already does between the command and data phases of every command.
func (d *spiDevice) write(tx []byte) error {
	for len(tx) > 0 {
		n := len(tx)
		if n > spiMaxChunk {
			n = spiMaxChunk
		}
		// Keep chip select asserted until the final chunk, so the panel sees one
		// continuous data stream across the separate ioctls.
		keepCS := len(tx) > n
		if err := d.writeChunk(tx[:n], keepCS); err != nil {
			return fmt.Errorf("spi write %d-byte chunk: %w", n, err)
		}
		tx = tx[n:]
	}
	return nil
}

// writeChunk sends a single SPI transfer full-duplex, discarding the received data.
func (d *spiDevice) writeChunk(tx []byte, keepCS bool) error {
	if len(tx) == 0 {
		return nil
	}
	rx := make([]byte, len(tx))
	msg := spiIOCTransfer{
		txBuf:  uint64(uintptr(unsafe.Pointer(&tx[0]))),
		rxBuf:  uint64(uintptr(unsafe.Pointer(&rx[0]))),
		length: uint32(len(tx)),
	}
	if keepCS {
		msg.csChange = 1
	}
	err := d.ioctl(spiIOCMessage1, unsafe.Pointer(&msg))
	// Keep the buffers alive until the synchronous ioctl completes; the syscall
	// captures only raw pointers.
	runtime.KeepAlive(msg)
	runtime.KeepAlive(tx)
	runtime.KeepAlive(rx)
	return err
}

// spiMaxChunk is the largest single transfer issued at once. 4096 bytes is
// comfortably under the Raspberry Pi SPI controller's per-transfer limit.
const spiMaxChunk = 4096

// spiIOCMessage1 is _IOW('k', 0, char[32]) — one spi_ioc_transfer.
const spiIOCMessage1 = 0x40206b00
