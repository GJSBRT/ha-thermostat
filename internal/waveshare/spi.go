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

// write sends tx bytes full-duplex over SPI, discarding the received data. A
// single logical write may exceed the controller's per-transfer limit, so it is
// split into multiple transfers that are submitted together as one SPI message;
// this keeps chip select asserted across the whole write.
func (d *spiDevice) write(tx []byte) error {
	if len(tx) == 0 {
		return nil
	}

	chunks := (len(tx) + spiMaxChunk - 1) / spiMaxChunk
	transfers := make([]spiIOCTransfer, chunks)
	rx := make([]byte, len(tx))

	for i := 0; i < chunks; i++ {
		off := i * spiMaxChunk
		n := len(tx) - off
		if n > spiMaxChunk {
			n = spiMaxChunk
		}
		transfers[i] = spiIOCTransfer{
			txBuf:  uint64(uintptr(unsafe.Pointer(&tx[off]))),
			rxBuf:  uint64(uintptr(unsafe.Pointer(&rx[off]))),
			length: uint32(n),
		}
	}

	err := d.ioctl(spiIOCMessage(chunks), unsafe.Pointer(&transfers[0]))
	// Keep the buffers alive until the synchronous ioctl completes; the syscall
	// captures only raw pointers.
	runtime.KeepAlive(transfers)
	runtime.KeepAlive(tx)
	runtime.KeepAlive(rx)
	return err
}

// spiMaxChunk is the largest single transfer issued within a message. The
// Raspberry Pi SPI controller limits individual transfers (its DMA length field
// is 16-bit, and some kernels impose a smaller cap), so frame data is chunked.
const spiMaxChunk = 4096

// spiIOCMessage returns the SPI_IOC_MESSAGE(n) request code, carrying n
// spi_ioc_transfer structs (32 bytes each) in a single ioctl.
func spiIOCMessage(n int) uintptr {
	// _IOW(0x6b, 0, char[n*32]) = write(1<<30) | magic(0x6b<<8) | size((n*32)<<16)
	return uintptr(0x40006b00 | uint32(n)*32<<16)
}
