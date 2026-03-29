//go:build linux

package hardware

import (
	"fmt"
	"os"
	"strconv"
)

// LinuxGPIO implements GPIOController using sysfs interface
// This is a fallback when periph.io is not available
type LinuxGPIO struct {
	exportedPins map[int]bool
}

// NewLinuxGPIO creates a new Linux GPIO controller
func NewLinuxGPIO() *LinuxGPIO {
	return &LinuxGPIO{
		exportedPins: make(map[int]bool),
	}
}

func (g *LinuxGPIO) Setup(pin int, output bool) error {
	// Export the pin if not already exported
	if !g.exportedPins[pin] {
		if err := g.export(pin); err != nil {
			return err
		}
		g.exportedPins[pin] = true
	}

	// Set direction
	direction := "in"
	if output {
		direction = "out"
	}

	dirPath := fmt.Sprintf("/sys/class/gpio/gpio%d/direction", pin)
	return os.WriteFile(dirPath, []byte(direction), 0644)
}

func (g *LinuxGPIO) Write(pin int, high bool) error {
	value := "0"
	if high {
		value = "1"
	}

	valuePath := fmt.Sprintf("/sys/class/gpio/gpio%d/value", pin)
	return os.WriteFile(valuePath, []byte(value), 0644)
}

func (g *LinuxGPIO) Read(pin int) (bool, error) {
	valuePath := fmt.Sprintf("/sys/class/gpio/gpio%d/value", pin)
	data, err := os.ReadFile(valuePath)
	if err != nil {
		return false, err
	}

	value, err := strconv.Atoi(string(data[0]))
	if err != nil {
		return false, err
	}

	return value == 1, nil
}

func (g *LinuxGPIO) Close() error {
	// Unexport all pins
	for pin := range g.exportedPins {
		g.unexport(pin)
	}
	return nil
}

func (g *LinuxGPIO) export(pin int) error {
	exportPath := "/sys/class/gpio/export"
	return os.WriteFile(exportPath, []byte(strconv.Itoa(pin)), 0644)
}

func (g *LinuxGPIO) unexport(pin int) error {
	unexportPath := "/sys/class/gpio/unexport"
	return os.WriteFile(unexportPath, []byte(strconv.Itoa(pin)), 0644)
}
