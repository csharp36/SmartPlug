//go:build !linux

package hardware

// NewLinuxGPIO returns a mock GPIO on non-Linux systems
func NewLinuxGPIO() *MockGPIO {
	return NewMockGPIO()
}
