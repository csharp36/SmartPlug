// Package core defines interfaces that abstract hardware access for the dual-mode architecture.
// These interfaces allow SmartPlug to run in either all-in-one mode (local GPIO) or
// distributed mode (MQTT-based communication between sensor nodes and controllers).
package core

import (
	"time"
)

// TemperatureReading represents a single temperature reading from a sensor.
type TemperatureReading struct {
	SensorID    string    `json:"sensor_id"`
	Temperature float64   `json:"temperature"` // Fahrenheit
	Timestamp   time.Time `json:"timestamp"`
	Valid       bool      `json:"valid"`
	Error       error     `json:"-"`
}

// TemperatureProvider abstracts access to temperature sensors.
// Implementations may read from local GPIO sensors or receive data via MQTT.
type TemperatureProvider interface {
	// GetCurrentReadings returns the most recent hot outlet and return line readings.
	GetCurrentReadings() (hot, ret TemperatureReading)

	// GetTemperatureDifferential returns the current temperature differential (hot - return).
	GetTemperatureDifferential() (float64, error)

	// OnReading registers a callback for temperature reading updates.
	// The callback is invoked each time new readings are available.
	OnReading(callback func(hot, ret TemperatureReading))

	// Start begins temperature monitoring.
	Start() error

	// Stop halts temperature monitoring and releases resources.
	Stop()
}

// DemandDetector abstracts flow-based demand detection.
// Implementations may read from a local flow meter or receive flow events via MQTT.
type DemandDetector interface {
	// IsFlowActive returns true if water flow is currently detected.
	IsFlowActive() bool

	// OnDemand registers a callback for demand state changes.
	// The callback is invoked when flow detection starts (active=true) or stops (active=false).
	OnDemand(callback func(active bool))

	// Start begins flow monitoring.
	Start() error

	// Stop halts flow monitoring and releases resources.
	Stop()
}

// PumpActuator abstracts pump control.
// Implementations may control a local relay, send MQTT commands to a smart plug,
// or integrate with other home automation systems.
type PumpActuator interface {
	// TurnOn activates the pump.
	TurnOn() error

	// TurnOff deactivates the pump.
	TurnOff() error

	// IsOn returns true if the pump is currently running.
	IsOn() bool

	// OnStateChange registers a callback for pump state changes.
	OnStateChange(callback func(on bool))

	// Initialize prepares the actuator for use.
	Initialize() error

	// Close releases resources and ensures the pump is off.
	Close() error
}

// SensorDiscoverer provides sensor discovery capabilities.
// This is optional and only available for local hardware implementations.
type SensorDiscoverer interface {
	// DiscoverSensors finds all connected temperature sensors.
	DiscoverSensors() ([]string, error)

	// GetSensorIDs returns the configured sensor IDs.
	GetSensorIDs() (hotOutlet, returnLine string)

	// SetSensorIDs configures specific sensor IDs.
	SetSensorIDs(hotOutlet, returnLine string)
}

// FlowMeterStats provides flow meter statistics.
// This is optional and only available when demand detection includes detailed flow data.
type FlowMeterStats struct {
	IsFlowActive         bool          `json:"is_flow_active"`
	PulseCount           int64         `json:"pulse_count"`
	LastPulseTime        time.Time     `json:"last_pulse_time"`
	CurrentFlowDuration  time.Duration `json:"current_flow_duration"`
	CurrentSessionPulses int64         `json:"current_session_pulses"`
}

// FlowStatsProvider provides detailed flow meter statistics.
// This is optional and only available for implementations that track detailed flow data.
type FlowStatsProvider interface {
	GetStats() FlowMeterStats
}

// NodeStatus represents the health status of a remote sensor or actuator node.
type NodeStatus struct {
	NodeID       string    `json:"node_id"`
	Online       bool      `json:"online"`
	LastSeen     time.Time `json:"last_seen"`
	Version      string    `json:"version,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// NodeMonitor provides monitoring of remote nodes in distributed mode.
type NodeMonitor interface {
	// GetNodeStatus returns the status of a specific node.
	GetNodeStatus(nodeID string) NodeStatus

	// GetAllNodeStatuses returns status of all known nodes.
	GetAllNodeStatuses() []NodeStatus

	// OnNodeStatusChange registers a callback for node status changes.
	OnNodeStatusChange(callback func(status NodeStatus))
}
