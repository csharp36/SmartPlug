package hardware

import (
	"sync"
	"sync/atomic"
	"time"
)

// FlowEvent represents a detected flow event
type FlowEvent struct {
	Timestamp   time.Time
	PulseCount  int64
	FlowRate    float64 // liters per minute
	Duration    time.Duration
}

// FlowMeter manages hall-effect flow meter operations
type FlowMeter struct {
	mu               sync.RWMutex
	gpio             GPIOController
	pin              int
	pulsesPerLiter   int
	triggerThreshold int
	debounceMs       int
	demandTimeout    time.Duration

	pulseCount       atomic.Int64
	lastPulseTime    time.Time
	flowActive       atomic.Bool
	flowStartTime    time.Time
	sessionPulses    int64

	callbacks        []func(event FlowEvent)
	demandCallbacks  []func(active bool)

	stopChan         chan struct{}
	mockMode         bool
}

// NewFlowMeter creates a new flow meter controller
func NewFlowMeter(gpio GPIOController, pin, pulsesPerLiter, triggerThreshold, debounceMs, demandTimeoutSec int) *FlowMeter {
	return &FlowMeter{
		gpio:             gpio,
		pin:              pin,
		pulsesPerLiter:   pulsesPerLiter,
		triggerThreshold: triggerThreshold,
		debounceMs:       debounceMs,
		demandTimeout:    time.Duration(demandTimeoutSec) * time.Second,
		stopChan:         make(chan struct{}),
	}
}

// EnableMockMode enables mock mode for testing
func (fm *FlowMeter) EnableMockMode() {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.mockMode = true
}

// Initialize sets up the GPIO pin for flow meter input
func (fm *FlowMeter) Initialize() error {
	if fm.mockMode {
		return nil
	}

	if fm.gpio == nil {
		return nil // No GPIO available, skip initialization
	}

	// Set pin as input
	if err := fm.gpio.Setup(fm.pin, false); err != nil {
		return err
	}

	return nil
}

// Start begins monitoring the flow meter
func (fm *FlowMeter) Start() error {
	if err := fm.Initialize(); err != nil {
		return err
	}

	go fm.monitorLoop()
	go fm.demandTimeoutLoop()

	return nil
}

// Stop halts flow meter monitoring
func (fm *FlowMeter) Stop() {
	close(fm.stopChan)
}

// SimulatePulse simulates a flow meter pulse (for testing)
func (fm *FlowMeter) SimulatePulse() {
	fm.recordPulse()
}

// SimulateFlow simulates a flow event with multiple pulses
func (fm *FlowMeter) SimulateFlow(pulseCount int) {
	for i := 0; i < pulseCount; i++ {
		fm.recordPulse()
		time.Sleep(time.Duration(fm.debounceMs) * time.Millisecond * 2)
	}
}

// recordPulse records a single pulse from the flow meter
func (fm *FlowMeter) recordPulse() {
	now := time.Now()

	fm.mu.Lock()
	// Debounce check
	if !fm.lastPulseTime.IsZero() && now.Sub(fm.lastPulseTime) < time.Duration(fm.debounceMs)*time.Millisecond {
		fm.mu.Unlock()
		return
	}
	fm.lastPulseTime = now
	fm.mu.Unlock()

	count := fm.pulseCount.Add(1)

	// Check if we've crossed the trigger threshold
	if !fm.flowActive.Load() && count >= int64(fm.triggerThreshold) {
		fm.startFlowSession()
	}
}

// startFlowSession marks the start of a flow event
func (fm *FlowMeter) startFlowSession() {
	fm.mu.Lock()
	if fm.flowActive.Load() {
		fm.mu.Unlock()
		return
	}

	fm.flowActive.Store(true)
	fm.flowStartTime = time.Now()
	fm.sessionPulses = fm.pulseCount.Load()
	callbacks := fm.demandCallbacks
	fm.mu.Unlock()

	// Notify demand callbacks
	for _, cb := range callbacks {
		cb(true)
	}
}

// endFlowSession marks the end of a flow event
func (fm *FlowMeter) endFlowSession() {
	fm.mu.Lock()
	if !fm.flowActive.Load() {
		fm.mu.Unlock()
		return
	}

	fm.flowActive.Store(false)
	duration := time.Since(fm.flowStartTime)
	totalPulses := fm.pulseCount.Load() - fm.sessionPulses

	// Calculate flow rate
	liters := float64(totalPulses) / float64(fm.pulsesPerLiter)
	flowRate := liters / duration.Minutes()

	event := FlowEvent{
		Timestamp:  fm.flowStartTime,
		PulseCount: totalPulses,
		FlowRate:   flowRate,
		Duration:   duration,
	}

	// Reset pulse counter
	fm.pulseCount.Store(0)

	callbacks := fm.callbacks
	demandCallbacks := fm.demandCallbacks
	fm.mu.Unlock()

	// Notify flow event callbacks
	for _, cb := range callbacks {
		cb(event)
	}

	// Notify demand callbacks
	for _, cb := range demandCallbacks {
		cb(false)
	}
}

// monitorLoop polls the GPIO pin for pulses
// Note: In production, this should use edge-triggered interrupts
func (fm *FlowMeter) monitorLoop() {
	if fm.mockMode || fm.gpio == nil {
		// In mock mode, wait for simulated pulses
		<-fm.stopChan
		return
	}

	// Poll every 10ms - not ideal but works for lower flow rates
	// For production, use periph.io's edge detection
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	var lastState bool

	for {
		select {
		case <-ticker.C:
			state, err := fm.gpio.Read(fm.pin)
			if err != nil {
				continue
			}

			// Detect rising edge
			if state && !lastState {
				fm.recordPulse()
			}
			lastState = state

		case <-fm.stopChan:
			return
		}
	}
}

// demandTimeoutLoop monitors for demand timeout
func (fm *FlowMeter) demandTimeoutLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !fm.flowActive.Load() {
				continue
			}

			fm.mu.RLock()
			lastPulse := fm.lastPulseTime
			fm.mu.RUnlock()

			// Check if we've exceeded the demand timeout
			if time.Since(lastPulse) > fm.demandTimeout {
				fm.endFlowSession()
			}

		case <-fm.stopChan:
			return
		}
	}
}

// IsFlowActive returns true if flow is currently detected
func (fm *FlowMeter) IsFlowActive() bool {
	return fm.flowActive.Load()
}

// GetPulseCount returns the current pulse count
func (fm *FlowMeter) GetPulseCount() int64 {
	return fm.pulseCount.Load()
}

// GetStats returns flow meter statistics
func (fm *FlowMeter) GetStats() FlowMeterStats {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	stats := FlowMeterStats{
		IsFlowActive:  fm.flowActive.Load(),
		PulseCount:    fm.pulseCount.Load(),
		LastPulseTime: fm.lastPulseTime,
	}

	if fm.flowActive.Load() {
		stats.CurrentFlowDuration = time.Since(fm.flowStartTime)
		stats.CurrentSessionPulses = fm.pulseCount.Load() - fm.sessionPulses
	}

	return stats
}

// FlowMeterStats contains flow meter statistics
type FlowMeterStats struct {
	IsFlowActive         bool
	PulseCount           int64
	LastPulseTime        time.Time
	CurrentFlowDuration  time.Duration
	CurrentSessionPulses int64
}

// OnFlowEvent registers a callback for flow events
func (fm *FlowMeter) OnFlowEvent(callback func(event FlowEvent)) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.callbacks = append(fm.callbacks, callback)
}

// OnDemand registers a callback for demand detection
func (fm *FlowMeter) OnDemand(callback func(active bool)) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.demandCallbacks = append(fm.demandCallbacks, callback)
}

// ResetStats resets the flow meter statistics
func (fm *FlowMeter) ResetStats() {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.pulseCount.Store(0)
	fm.sessionPulses = 0
}
