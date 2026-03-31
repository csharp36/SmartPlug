// Package controller implements pump control logic
package controller

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/smartplug/smartplug/internal/config"
	"github.com/smartplug/smartplug/internal/core"
)

// PumpState represents the pump controller state
type PumpState int

const (
	StateIdle PumpState = iota
	StateHeating
	StateSatisfied
	StateCooldown
	StateFault
)

func (s PumpState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateHeating:
		return "heating"
	case StateSatisfied:
		return "satisfied"
	case StateCooldown:
		return "cooldown"
	case StateFault:
		return "fault"
	default:
		return "unknown"
	}
}

// TriggerSource indicates what triggered a pump activation
type TriggerSource int

const (
	TriggerManual TriggerSource = iota
	TriggerDemand
	TriggerSchedule
	TriggerTemperature
)

func (t TriggerSource) String() string {
	switch t {
	case TriggerManual:
		return "manual"
	case TriggerDemand:
		return "demand"
	case TriggerSchedule:
		return "schedule"
	case TriggerTemperature:
		return "temperature"
	default:
		return "unknown"
	}
}

// PumpEvent represents a pump activation event
type PumpEvent struct {
	Timestamp    time.Time
	Trigger      TriggerSource
	Duration     time.Duration
	HotTemp      float64
	ReturnTemp   float64
	Differential float64
}

// PumpController manages pump operation based on temperature and demand
type PumpController struct {
	mu sync.RWMutex

	actuator core.PumpActuator
	temps    core.TemperatureProvider
	demand   core.DemandDetector
	cfg      *config.PumpConfig

	state            PumpState
	lastTrigger      TriggerSource
	heatingStartTime time.Time
	cooldownEndTime  time.Time

	// Callbacks
	stateCallbacks []func(state PumpState, trigger TriggerSource)
	eventCallbacks []func(event PumpEvent)

	// Control
	stopChan   chan struct{}
	enabled    bool
	manualMode bool
}

// NewPumpController creates a new pump controller using interface-based dependencies.
// This constructor supports both local hardware and remote MQTT implementations.
func NewPumpController(
	actuator core.PumpActuator,
	temps core.TemperatureProvider,
	demand core.DemandDetector,
	cfg *config.PumpConfig,
) *PumpController {
	return &PumpController{
		actuator: actuator,
		temps:    temps,
		demand:   demand,
		cfg:      cfg,
		state:    StateIdle,
		stopChan: make(chan struct{}),
		enabled:  true,
	}
}

// Start begins pump control operations
func (pc *PumpController) Start() {
	// Register for temperature readings
	pc.temps.OnReading(pc.onTemperatureReading)

	// Register for demand detection if available
	if pc.demand != nil {
		pc.demand.OnDemand(pc.onDemandChange)
	}

	// Start control loop
	go pc.controlLoop()
}

// Stop halts pump control operations
func (pc *PumpController) Stop() {
	close(pc.stopChan)

	// Ensure pump is off
	if err := pc.actuator.TurnOff(); err != nil {
		log.Printf("Error turning off pump during shutdown: %v", err)
	}
}

// Enable enables automatic pump control
func (pc *PumpController) Enable() {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.enabled = true
	log.Println("Pump controller enabled")
}

// Disable disables automatic pump control
func (pc *PumpController) Disable() {
	pc.mu.Lock()
	pc.enabled = false
	pc.mu.Unlock()

	// Turn off pump if running
	if err := pc.actuator.TurnOff(); err != nil {
		log.Printf("Error turning off pump: %v", err)
	}

	pc.setState(StateIdle, TriggerManual)
	log.Println("Pump controller disabled")
}

// IsEnabled returns whether automatic control is enabled
func (pc *PumpController) IsEnabled() bool {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.enabled
}

// HeatNow manually triggers the pump
func (pc *PumpController) HeatNow() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.state == StateCooldown {
		return fmt.Errorf("pump in cooldown, please wait")
	}

	pc.manualMode = true
	return pc.startPump(TriggerManual)
}

// StopManual stops manual pump operation
func (pc *PumpController) StopManual() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	pc.manualMode = false
	return pc.stopPump()
}

// TriggerSchedule activates pump from schedule
func (pc *PumpController) TriggerSchedule() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if !pc.enabled {
		return fmt.Errorf("pump controller disabled")
	}

	if pc.state == StateCooldown {
		return fmt.Errorf("pump in cooldown")
	}

	return pc.startPump(TriggerSchedule)
}

// GetState returns the current pump state
func (pc *PumpController) GetState() PumpState {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.state
}

// GetLastTrigger returns what triggered the current/last pump activation
func (pc *PumpController) GetLastTrigger() TriggerSource {
	pc.mu.RLock()
	defer pc.mu.RUnlock()
	return pc.lastTrigger
}

// GetStatus returns comprehensive pump status
func (pc *PumpController) GetStatus() PumpStatus {
	pc.mu.RLock()
	defer pc.mu.RUnlock()

	hot, ret := pc.temps.GetCurrentReadings()

	status := PumpStatus{
		State:             pc.state,
		Enabled:           pc.enabled,
		IsRunning:         pc.actuator.IsOn(),
		LastTrigger:       pc.lastTrigger,
		HotTemperature:    hot.Temperature,
		ReturnTemperature: ret.Temperature,
	}

	if hot.Valid && ret.Valid {
		status.Differential = hot.Temperature - ret.Temperature
	}

	if pc.state == StateHeating {
		status.Runtime = time.Since(pc.heatingStartTime)
	}

	if pc.state == StateCooldown {
		status.CooldownRemaining = time.Until(pc.cooldownEndTime)
	}

	return status
}

// PumpStatus contains comprehensive pump status
type PumpStatus struct {
	State             PumpState
	Enabled           bool
	IsRunning         bool
	LastTrigger       TriggerSource
	HotTemperature    float64
	ReturnTemperature float64
	Differential      float64
	Runtime           time.Duration
	CooldownRemaining time.Duration
}

// OnStateChange registers a callback for state changes
func (pc *PumpController) OnStateChange(callback func(state PumpState, trigger TriggerSource)) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.stateCallbacks = append(pc.stateCallbacks, callback)
}

// OnEvent registers a callback for pump events
func (pc *PumpController) OnEvent(callback func(event PumpEvent)) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.eventCallbacks = append(pc.eventCallbacks, callback)
}

// onTemperatureReading handles temperature updates
func (pc *PumpController) onTemperatureReading(hot, ret core.TemperatureReading) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if !pc.enabled || pc.manualMode {
		return
	}

	// Check for sensor faults
	if !hot.Valid || !ret.Valid {
		if pc.state == StateHeating {
			log.Println("Sensor fault detected, stopping pump")
			pc.stopPump()
			pc.state = StateFault
		}
		return
	}

	// Check minimum temperature safety
	if hot.Temperature < pc.cfg.MinTemperature || ret.Temperature < pc.cfg.MinTemperature {
		if pc.state == StateHeating {
			log.Println("Temperature below minimum, stopping pump")
			pc.stopPump()
		}
		return
	}

	diff := hot.Temperature - ret.Temperature

	switch pc.state {
	case StateIdle, StateSatisfied:
		// Start pump if differential exceeds threshold
		if diff >= pc.cfg.StartThreshold {
			pc.startPump(TriggerTemperature)
		}

	case StateHeating:
		// Stop pump if differential drops below threshold
		if diff <= pc.cfg.StopThreshold {
			pc.stopPump()
			pc.setState(StateSatisfied, pc.lastTrigger)
		}
	}
}

// onDemandChange handles flow meter demand detection
func (pc *PumpController) onDemandChange(active bool) {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if !pc.enabled {
		return
	}

	if active {
		// Start pump on demand
		if pc.state != StateHeating && pc.state != StateCooldown {
			log.Println("Flow detected, starting pump")
			pc.startPump(TriggerDemand)
		}
	} else {
		// Flow stopped - let temperature logic handle shutdown
		log.Println("Flow stopped")
	}
}

// startPump activates the pump
func (pc *PumpController) startPump(trigger TriggerSource) error {
	if pc.state == StateCooldown {
		return fmt.Errorf("pump in cooldown")
	}

	if err := pc.actuator.TurnOn(); err != nil {
		return fmt.Errorf("failed to turn on pump: %w", err)
	}

	pc.heatingStartTime = time.Now()
	pc.setState(StateHeating, trigger)

	log.Printf("Pump started (trigger: %s)", trigger)
	return nil
}

// stopPump deactivates the pump
func (pc *PumpController) stopPump() error {
	if err := pc.actuator.TurnOff(); err != nil {
		return fmt.Errorf("failed to turn off pump: %w", err)
	}

	// Record event
	if pc.state == StateHeating {
		hot, ret := pc.temps.GetCurrentReadings()
		event := PumpEvent{
			Timestamp:    pc.heatingStartTime,
			Trigger:      pc.lastTrigger,
			Duration:     time.Since(pc.heatingStartTime),
			HotTemp:      hot.Temperature,
			ReturnTemp:   ret.Temperature,
			Differential: hot.Temperature - ret.Temperature,
		}

		// Notify event callbacks
		for _, cb := range pc.eventCallbacks {
			go cb(event)
		}
	}

	// Enter cooldown
	pc.cooldownEndTime = time.Now().Add(time.Duration(pc.cfg.CooldownMinutes) * time.Minute)
	pc.setState(StateCooldown, pc.lastTrigger)

	log.Printf("Pump stopped (runtime: %v)", time.Since(pc.heatingStartTime))
	return nil
}

// setState updates the pump state and notifies callbacks
func (pc *PumpController) setState(state PumpState, trigger TriggerSource) {
	pc.state = state
	pc.lastTrigger = trigger

	// Notify callbacks
	for _, cb := range pc.stateCallbacks {
		go cb(state, trigger)
	}
}

// controlLoop runs the main control loop
func (pc *PumpController) controlLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			pc.checkSafetyLimits()

		case <-pc.stopChan:
			return
		}
	}
}

// checkSafetyLimits enforces safety limits
func (pc *PumpController) checkSafetyLimits() {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	switch pc.state {
	case StateHeating:
		// Check max runtime
		if time.Since(pc.heatingStartTime) >= time.Duration(pc.cfg.MaxRuntimeMinutes)*time.Minute {
			log.Println("Max runtime exceeded, stopping pump")
			pc.stopPump()
		}

	case StateCooldown:
		// Check if cooldown is complete
		if time.Now().After(pc.cooldownEndTime) {
			pc.setState(StateIdle, pc.lastTrigger)
			log.Println("Cooldown complete")
		}
	}
}

// UpdateConfig updates the pump configuration
func (pc *PumpController) UpdateConfig(cfg *config.PumpConfig) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.cfg = cfg
}

// GetTemperatureProvider returns the temperature provider.
// This is useful for accessing sensor discovery features.
func (pc *PumpController) GetTemperatureProvider() core.TemperatureProvider {
	return pc.temps
}

// GetDemandDetector returns the demand detector.
// This is useful for accessing flow meter features.
func (pc *PumpController) GetDemandDetector() core.DemandDetector {
	return pc.demand
}

// GetActuator returns the pump actuator.
// This is useful for accessing relay statistics.
func (pc *PumpController) GetActuator() core.PumpActuator {
	return pc.actuator
}
