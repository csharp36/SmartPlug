package core

import (
	"encoding/json"
	"time"
)

// SensorData represents temperature sensor readings published by a sensor node.
// This message is published to smartplug/{node_id}/sensors/data
type SensorData struct {
	NodeID           string    `json:"node_id"`
	HotOutlet        float64   `json:"hot_outlet"`        // Temperature in Fahrenheit
	ReturnLine       float64   `json:"return_line"`       // Temperature in Fahrenheit
	HotValid         bool      `json:"hot_valid"`
	ReturnValid      bool      `json:"return_valid"`
	HotSensorID      string    `json:"hot_sensor_id,omitempty"`
	ReturnSensorID   string    `json:"return_sensor_id,omitempty"`
	Timestamp        time.Time `json:"timestamp"`
}

// MarshalJSON implements json.Marshaler for SensorData.
func (s SensorData) MarshalJSON() ([]byte, error) {
	type Alias SensorData
	return json.Marshal(&struct {
		Timestamp string `json:"timestamp"`
		*Alias
	}{
		Timestamp: s.Timestamp.Format(time.RFC3339),
		Alias:     (*Alias)(&s),
	})
}

// UnmarshalJSON implements json.Unmarshaler for SensorData.
func (s *SensorData) UnmarshalJSON(data []byte) error {
	type Alias SensorData
	aux := &struct {
		Timestamp string `json:"timestamp"`
		*Alias
	}{
		Alias: (*Alias)(s),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Timestamp != "" {
		t, err := time.Parse(time.RFC3339, aux.Timestamp)
		if err != nil {
			return err
		}
		s.Timestamp = t
	}
	return nil
}

// FlowData represents flow meter data published by a sensor node.
// This message is published to smartplug/{node_id}/flow/data
type FlowData struct {
	NodeID     string    `json:"node_id"`
	Active     bool      `json:"active"`      // Is flow currently detected
	PulseCount int64     `json:"pulse_count"` // Current pulse count in session
	FlowRate   float64   `json:"flow_rate"`   // Liters per minute (calculated)
	Timestamp  time.Time `json:"timestamp"`
}

// MarshalJSON implements json.Marshaler for FlowData.
func (f FlowData) MarshalJSON() ([]byte, error) {
	type Alias FlowData
	return json.Marshal(&struct {
		Timestamp string `json:"timestamp"`
		*Alias
	}{
		Timestamp: f.Timestamp.Format(time.RFC3339),
		Alias:     (*Alias)(&f),
	})
}

// UnmarshalJSON implements json.Unmarshaler for FlowData.
func (f *FlowData) UnmarshalJSON(data []byte) error {
	type Alias FlowData
	aux := &struct {
		Timestamp string `json:"timestamp"`
		*Alias
	}{
		Alias: (*Alias)(f),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Timestamp != "" {
		t, err := time.Parse(time.RFC3339, aux.Timestamp)
		if err != nil {
			return err
		}
		f.Timestamp = t
	}
	return nil
}

// FlowEvent represents a completed flow session.
// This message is published to smartplug/{node_id}/flow/event
type FlowEvent struct {
	NodeID     string        `json:"node_id"`
	Timestamp  time.Time     `json:"timestamp"`  // When the flow session started
	PulseCount int64         `json:"pulse_count"`
	FlowRate   float64       `json:"flow_rate"` // Average liters per minute
	Duration   time.Duration `json:"duration"`
}

// MarshalJSON implements json.Marshaler for FlowEvent.
func (f FlowEvent) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		NodeID     string  `json:"node_id"`
		Timestamp  string  `json:"timestamp"`
		PulseCount int64   `json:"pulse_count"`
		FlowRate   float64 `json:"flow_rate"`
		Duration   float64 `json:"duration_seconds"`
	}{
		NodeID:     f.NodeID,
		Timestamp:  f.Timestamp.Format(time.RFC3339),
		PulseCount: f.PulseCount,
		FlowRate:   f.FlowRate,
		Duration:   f.Duration.Seconds(),
	})
}

// UnmarshalJSON implements json.Unmarshaler for FlowEvent.
func (f *FlowEvent) UnmarshalJSON(data []byte) error {
	aux := &struct {
		NodeID     string  `json:"node_id"`
		Timestamp  string  `json:"timestamp"`
		PulseCount int64   `json:"pulse_count"`
		FlowRate   float64 `json:"flow_rate"`
		Duration   float64 `json:"duration_seconds"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	f.NodeID = aux.NodeID
	f.PulseCount = aux.PulseCount
	f.FlowRate = aux.FlowRate
	f.Duration = time.Duration(aux.Duration * float64(time.Second))
	if aux.Timestamp != "" {
		t, err := time.Parse(time.RFC3339, aux.Timestamp)
		if err != nil {
			return err
		}
		f.Timestamp = t
	}
	return nil
}

// PumpCommand represents a command sent to control the pump.
// This message is published to smartplug/pump/command
type PumpCommand struct {
	Command   string    `json:"command"`    // "on", "off"
	Source    string    `json:"source"`     // "manual", "schedule", "demand", "temperature"
	RequestID string    `json:"request_id"` // Optional unique ID for tracking
	Timestamp time.Time `json:"timestamp"`
}

// MarshalJSON implements json.Marshaler for PumpCommand.
func (p PumpCommand) MarshalJSON() ([]byte, error) {
	type Alias PumpCommand
	return json.Marshal(&struct {
		Timestamp string `json:"timestamp"`
		*Alias
	}{
		Timestamp: p.Timestamp.Format(time.RFC3339),
		Alias:     (*Alias)(&p),
	})
}

// UnmarshalJSON implements json.Unmarshaler for PumpCommand.
func (p *PumpCommand) UnmarshalJSON(data []byte) error {
	type Alias PumpCommand
	aux := &struct {
		Timestamp string `json:"timestamp"`
		*Alias
	}{
		Alias: (*Alias)(p),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Timestamp != "" {
		t, err := time.Parse(time.RFC3339, aux.Timestamp)
		if err != nil {
			return err
		}
		p.Timestamp = t
	}
	return nil
}

// PumpStatus represents the current state of the pump actuator.
// This message is published to smartplug/pump/status
type PumpStatus struct {
	NodeID    string    `json:"node_id,omitempty"` // Actuator node ID if applicable
	IsOn      bool      `json:"is_on"`
	LastOn    time.Time `json:"last_on,omitempty"`
	LastOff   time.Time `json:"last_off,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// MarshalJSON implements json.Marshaler for PumpStatus.
func (p PumpStatus) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{
		"is_on":     p.IsOn,
		"timestamp": p.Timestamp.Format(time.RFC3339),
	}
	if p.NodeID != "" {
		m["node_id"] = p.NodeID
	}
	if !p.LastOn.IsZero() {
		m["last_on"] = p.LastOn.Format(time.RFC3339)
	}
	if !p.LastOff.IsZero() {
		m["last_off"] = p.LastOff.Format(time.RFC3339)
	}
	return json.Marshal(m)
}

// UnmarshalJSON implements json.Unmarshaler for PumpStatus.
func (p *PumpStatus) UnmarshalJSON(data []byte) error {
	aux := &struct {
		NodeID    string `json:"node_id"`
		IsOn      bool   `json:"is_on"`
		LastOn    string `json:"last_on"`
		LastOff   string `json:"last_off"`
		Timestamp string `json:"timestamp"`
	}{}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	p.NodeID = aux.NodeID
	p.IsOn = aux.IsOn
	if aux.LastOn != "" {
		if t, err := time.Parse(time.RFC3339, aux.LastOn); err == nil {
			p.LastOn = t
		}
	}
	if aux.LastOff != "" {
		if t, err := time.Parse(time.RFC3339, aux.LastOff); err == nil {
			p.LastOff = t
		}
	}
	if aux.Timestamp != "" {
		if t, err := time.Parse(time.RFC3339, aux.Timestamp); err == nil {
			p.Timestamp = t
		}
	}
	return nil
}

// NodeHeartbeat represents a periodic heartbeat from a sensor or actuator node.
// This message is published to smartplug/{node_id}/heartbeat
type NodeHeartbeat struct {
	NodeID    string    `json:"node_id"`
	Mode      string    `json:"mode"` // "sensor", "actuator", "all-in-one"
	Version   string    `json:"version"`
	Uptime    int64     `json:"uptime_seconds"`
	Timestamp time.Time `json:"timestamp"`
}

// MarshalJSON implements json.Marshaler for NodeHeartbeat.
func (n NodeHeartbeat) MarshalJSON() ([]byte, error) {
	type Alias NodeHeartbeat
	return json.Marshal(&struct {
		Timestamp string `json:"timestamp"`
		*Alias
	}{
		Timestamp: n.Timestamp.Format(time.RFC3339),
		Alias:     (*Alias)(&n),
	})
}

// UnmarshalJSON implements json.Unmarshaler for NodeHeartbeat.
func (n *NodeHeartbeat) UnmarshalJSON(data []byte) error {
	type Alias NodeHeartbeat
	aux := &struct {
		Timestamp string `json:"timestamp"`
		*Alias
	}{
		Alias: (*Alias)(n),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.Timestamp != "" {
		t, err := time.Parse(time.RFC3339, aux.Timestamp)
		if err != nil {
			return err
		}
		n.Timestamp = t
	}
	return nil
}
