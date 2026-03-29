package hardware

import (
	"fmt"
	"sync"
	"time"
)

// RelayState represents the current state of the relay
type RelayState int

const (
	RelayOff RelayState = iota
	RelayOn
)

func (s RelayState) String() string {
	if s == RelayOn {
		return "on"
	}
	return "off"
}

// GPIOController interface for GPIO operations (allows mocking)
type GPIOController interface {
	Setup(pin int, output bool) error
	Write(pin int, high bool) error
	Read(pin int) (bool, error)
	Close() error
}

// RelayController manages the pump relay
type RelayController struct {
	mu            sync.RWMutex
	gpio          GPIOController
	pin           int
	state         RelayState
	lastOn        time.Time
	lastOff       time.Time
	totalRuntime  time.Duration
	cycleCount    int
	callbacks     []func(state RelayState)
	mockMode      bool
}

// NewRelayController creates a new relay controller
func NewRelayController(gpio GPIOController, pin int) *RelayController {
	return &RelayController{
		gpio:  gpio,
		pin:   pin,
		state: RelayOff,
	}
}

// EnableMockMode enables mock mode for testing
func (rc *RelayController) EnableMockMode() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.mockMode = true
}

// Initialize sets up the GPIO pin for relay control
func (rc *RelayController) Initialize() error {
	if rc.mockMode {
		return nil
	}

	if rc.gpio == nil {
		return fmt.Errorf("GPIO controller not initialized")
	}

	// Set pin as output
	if err := rc.gpio.Setup(rc.pin, true); err != nil {
		return fmt.Errorf("failed to setup relay GPIO pin: %w", err)
	}

	// Ensure relay starts in off state
	return rc.turnOff()
}

// TurnOn activates the relay (turns pump on)
func (rc *RelayController) TurnOn() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.state == RelayOn {
		return nil // Already on
	}

	if err := rc.turnOn(); err != nil {
		return err
	}

	rc.state = RelayOn
	rc.lastOn = time.Now()
	rc.cycleCount++

	// Notify callbacks
	callbacks := rc.callbacks
	rc.mu.Unlock()
	for _, cb := range callbacks {
		cb(RelayOn)
	}
	rc.mu.Lock()

	return nil
}

// TurnOff deactivates the relay (turns pump off)
func (rc *RelayController) TurnOff() error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.state == RelayOff {
		return nil // Already off
	}

	if err := rc.turnOff(); err != nil {
		return err
	}

	// Update runtime statistics
	if !rc.lastOn.IsZero() {
		rc.totalRuntime += time.Since(rc.lastOn)
	}

	rc.state = RelayOff
	rc.lastOff = time.Now()

	// Notify callbacks
	callbacks := rc.callbacks
	rc.mu.Unlock()
	for _, cb := range callbacks {
		cb(RelayOff)
	}
	rc.mu.Lock()

	return nil
}

// internal turn on (must hold lock)
func (rc *RelayController) turnOn() error {
	if rc.mockMode || rc.gpio == nil {
		return nil
	}
	// Most relay modules are active-low, so we write LOW to turn on
	// Adjust based on your specific relay module
	return rc.gpio.Write(rc.pin, true)
}

// internal turn off (must hold lock)
func (rc *RelayController) turnOff() error {
	if rc.mockMode || rc.gpio == nil {
		return nil
	}
	return rc.gpio.Write(rc.pin, false)
}

// GetState returns the current relay state
func (rc *RelayController) GetState() RelayState {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.state
}

// IsOn returns true if the relay is currently on
func (rc *RelayController) IsOn() bool {
	return rc.GetState() == RelayOn
}

// GetLastOnTime returns when the relay was last turned on
func (rc *RelayController) GetLastOnTime() time.Time {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.lastOn
}

// GetLastOffTime returns when the relay was last turned off
func (rc *RelayController) GetLastOffTime() time.Time {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.lastOff
}

// GetCurrentRuntime returns how long the pump has been running in current cycle
func (rc *RelayController) GetCurrentRuntime() time.Duration {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	if rc.state == RelayOff {
		return 0
	}
	return time.Since(rc.lastOn)
}

// GetTotalRuntime returns total runtime since startup
func (rc *RelayController) GetTotalRuntime() time.Duration {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	total := rc.totalRuntime
	if rc.state == RelayOn && !rc.lastOn.IsZero() {
		total += time.Since(rc.lastOn)
	}
	return total
}

// GetCycleCount returns the number of pump cycles since startup
func (rc *RelayController) GetCycleCount() int {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.cycleCount
}

// OnStateChange registers a callback for state changes
func (rc *RelayController) OnStateChange(callback func(state RelayState)) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.callbacks = append(rc.callbacks, callback)
}

// GetStats returns relay statistics
func (rc *RelayController) GetStats() RelayStats {
	rc.mu.RLock()
	defer rc.mu.RUnlock()

	stats := RelayStats{
		State:        rc.state,
		LastOn:       rc.lastOn,
		LastOff:      rc.lastOff,
		TotalRuntime: rc.totalRuntime,
		CycleCount:   rc.cycleCount,
	}

	if rc.state == RelayOn && !rc.lastOn.IsZero() {
		stats.CurrentRuntime = time.Since(rc.lastOn)
		stats.TotalRuntime += stats.CurrentRuntime
	}

	return stats
}

// RelayStats contains relay statistics
type RelayStats struct {
	State          RelayState
	LastOn         time.Time
	LastOff        time.Time
	CurrentRuntime time.Duration
	TotalRuntime   time.Duration
	CycleCount     int
}

// Close releases GPIO resources
func (rc *RelayController) Close() error {
	// Ensure relay is off
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if err := rc.turnOff(); err != nil {
		return err
	}

	if rc.gpio != nil && !rc.mockMode {
		return rc.gpio.Close()
	}
	return nil
}

// MockGPIO is a mock GPIO controller for testing
type MockGPIO struct {
	mu     sync.Mutex
	pins   map[int]bool
	states map[int]bool
}

// NewMockGPIO creates a new mock GPIO controller
func NewMockGPIO() *MockGPIO {
	return &MockGPIO{
		pins:   make(map[int]bool),
		states: make(map[int]bool),
	}
}

func (m *MockGPIO) Setup(pin int, output bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pins[pin] = output
	return nil
}

func (m *MockGPIO) Write(pin int, high bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[pin] = high
	return nil
}

func (m *MockGPIO) Read(pin int) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.states[pin], nil
}

func (m *MockGPIO) Close() error {
	return nil
}

// GetPinState returns the state of a pin (for testing)
func (m *MockGPIO) GetPinState(pin int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.states[pin]
}
