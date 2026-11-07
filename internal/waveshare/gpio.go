package waveshare

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// gpioPin is a single Linux sysfs GPIO line. The driver uses the same legacy
// sysfs interface (/sys/class/gpio) as the button input so it needs no extra
// kernel support on typical Raspberry Pi images.
type gpioPin struct {
	num int
}

// export makes the pin available through sysfs. It is idempotent.
func (p *gpioPin) export() error {
	path := fmt.Sprintf("/sys/class/gpio/gpio%d", p.num)
	if _, err := os.Stat(path); err == nil {
		return nil // already exported
	}
	if err := writeSysFile("/sys/class/gpio/export", fmt.Sprintf("%d", p.num)); err != nil {
		// "Device or resource busy" means another exporter already owns the pin.
		if strings.Contains(err.Error(), "busy") {
			return nil
		}
		return err
	}
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return fmt.Errorf("sysfs node %s did not appear", path)
}

// direction sets the pin direction: "in" or "out".
func (p *gpioPin) direction(dir string) error {
	return writeSysFile(fmt.Sprintf("/sys/class/gpio/gpio%d/direction", p.num), dir)
}

// set drives an output pin high or low.
func (p *gpioPin) set(high bool) error {
	v := "0"
	if high {
		v = "1"
	}
	return writeSysFile(fmt.Sprintf("/sys/class/gpio/gpio%d/value", p.num), v)
}

// read returns the current state of an input pin (true = high).
func (p *gpioPin) read() (bool, error) {
	data, err := os.ReadFile(fmt.Sprintf("/sys/class/gpio/gpio%d/value", p.num))
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(data)) == "1", nil
}

// unexport releases the pin.
func (p *gpioPin) unexport() {
	_ = writeSysFile("/sys/class/gpio/unexport", fmt.Sprintf("%d", p.num))
}

// writeSysFile writes a value to a sysfs attribute.
func writeSysFile(path, value string) error {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.WriteString(f, value)
	return err
}
