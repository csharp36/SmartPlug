package core

import (
	"encoding/json"
	"testing"
	"time"
)

func TestSensorData_MarshalUnmarshal(t *testing.T) {
	original := SensorData{
		NodeID:         "sensor-1",
		HotOutlet:      120.5,
		ReturnLine:     95.2,
		HotValid:       true,
		ReturnValid:    true,
		HotSensorID:    "28-hot-001",
		ReturnSensorID: "28-ret-001",
		Timestamp:      time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal SensorData: %v", err)
	}

	// Verify JSON contains expected fields
	jsonStr := string(data)
	if !contains(jsonStr, `"node_id":"sensor-1"`) {
		t.Error("JSON missing node_id")
	}
	if !contains(jsonStr, `"hot_outlet":120.5`) {
		t.Error("JSON missing hot_outlet")
	}
	if !contains(jsonStr, `"timestamp":"2024-01-15T10:30:00Z"`) {
		t.Error("JSON missing or incorrect timestamp")
	}

	// Unmarshal back
	var decoded SensorData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal SensorData: %v", err)
	}

	// Verify fields
	if decoded.NodeID != original.NodeID {
		t.Errorf("NodeID mismatch: got %s, want %s", decoded.NodeID, original.NodeID)
	}
	if decoded.HotOutlet != original.HotOutlet {
		t.Errorf("HotOutlet mismatch: got %f, want %f", decoded.HotOutlet, original.HotOutlet)
	}
	if decoded.ReturnLine != original.ReturnLine {
		t.Errorf("ReturnLine mismatch: got %f, want %f", decoded.ReturnLine, original.ReturnLine)
	}
	if !decoded.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp mismatch: got %v, want %v", decoded.Timestamp, original.Timestamp)
	}
}

func TestFlowData_MarshalUnmarshal(t *testing.T) {
	original := FlowData{
		NodeID:     "sensor-1",
		Active:     true,
		PulseCount: 150,
		FlowRate:   2.5,
		Timestamp:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal FlowData: %v", err)
	}

	var decoded FlowData
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal FlowData: %v", err)
	}

	if decoded.NodeID != original.NodeID {
		t.Errorf("NodeID mismatch: got %s, want %s", decoded.NodeID, original.NodeID)
	}
	if decoded.Active != original.Active {
		t.Errorf("Active mismatch: got %v, want %v", decoded.Active, original.Active)
	}
	if decoded.PulseCount != original.PulseCount {
		t.Errorf("PulseCount mismatch: got %d, want %d", decoded.PulseCount, original.PulseCount)
	}
	if decoded.FlowRate != original.FlowRate {
		t.Errorf("FlowRate mismatch: got %f, want %f", decoded.FlowRate, original.FlowRate)
	}
}

func TestFlowEvent_MarshalUnmarshal(t *testing.T) {
	original := FlowEvent{
		NodeID:     "sensor-1",
		Timestamp:  time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		PulseCount: 450,
		FlowRate:   3.2,
		Duration:   45 * time.Second,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal FlowEvent: %v", err)
	}

	// Verify duration is serialized as seconds
	jsonStr := string(data)
	if !contains(jsonStr, `"duration_seconds":45`) {
		t.Errorf("Expected duration_seconds:45 in JSON, got: %s", jsonStr)
	}

	var decoded FlowEvent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal FlowEvent: %v", err)
	}

	if decoded.Duration != original.Duration {
		t.Errorf("Duration mismatch: got %v, want %v", decoded.Duration, original.Duration)
	}
	if decoded.PulseCount != original.PulseCount {
		t.Errorf("PulseCount mismatch: got %d, want %d", decoded.PulseCount, original.PulseCount)
	}
}

func TestPumpCommand_MarshalUnmarshal(t *testing.T) {
	original := PumpCommand{
		Command:   "on",
		Source:    "schedule",
		RequestID: "req-123",
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal PumpCommand: %v", err)
	}

	var decoded PumpCommand
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal PumpCommand: %v", err)
	}

	if decoded.Command != original.Command {
		t.Errorf("Command mismatch: got %s, want %s", decoded.Command, original.Command)
	}
	if decoded.Source != original.Source {
		t.Errorf("Source mismatch: got %s, want %s", decoded.Source, original.Source)
	}
	if decoded.RequestID != original.RequestID {
		t.Errorf("RequestID mismatch: got %s, want %s", decoded.RequestID, original.RequestID)
	}
}

func TestPumpStatus_MarshalUnmarshal(t *testing.T) {
	lastOn := time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	lastOff := time.Date(2024, 1, 15, 10, 15, 0, 0, time.UTC)

	original := PumpStatus{
		NodeID:    "actuator-1",
		IsOn:      false,
		LastOn:    lastOn,
		LastOff:   lastOff,
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal PumpStatus: %v", err)
	}

	var decoded PumpStatus
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal PumpStatus: %v", err)
	}

	if decoded.NodeID != original.NodeID {
		t.Errorf("NodeID mismatch: got %s, want %s", decoded.NodeID, original.NodeID)
	}
	if decoded.IsOn != original.IsOn {
		t.Errorf("IsOn mismatch: got %v, want %v", decoded.IsOn, original.IsOn)
	}
	if !decoded.LastOn.Equal(original.LastOn) {
		t.Errorf("LastOn mismatch: got %v, want %v", decoded.LastOn, original.LastOn)
	}
	if !decoded.LastOff.Equal(original.LastOff) {
		t.Errorf("LastOff mismatch: got %v, want %v", decoded.LastOff, original.LastOff)
	}
}

func TestPumpStatus_OmitEmptyTimes(t *testing.T) {
	// Status with zero times should omit them from JSON
	original := PumpStatus{
		IsOn:      true,
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal PumpStatus: %v", err)
	}

	jsonStr := string(data)
	if contains(jsonStr, "last_on") {
		t.Error("Expected last_on to be omitted when zero")
	}
	if contains(jsonStr, "last_off") {
		t.Error("Expected last_off to be omitted when zero")
	}
}

func TestNodeHeartbeat_MarshalUnmarshal(t *testing.T) {
	original := NodeHeartbeat{
		NodeID:    "sensor-1",
		Mode:      "sensor",
		Version:   "1.0.0",
		Uptime:    3600,
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Failed to marshal NodeHeartbeat: %v", err)
	}

	var decoded NodeHeartbeat
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal NodeHeartbeat: %v", err)
	}

	if decoded.NodeID != original.NodeID {
		t.Errorf("NodeID mismatch: got %s, want %s", decoded.NodeID, original.NodeID)
	}
	if decoded.Mode != original.Mode {
		t.Errorf("Mode mismatch: got %s, want %s", decoded.Mode, original.Mode)
	}
	if decoded.Version != original.Version {
		t.Errorf("Version mismatch: got %s, want %s", decoded.Version, original.Version)
	}
	if decoded.Uptime != original.Uptime {
		t.Errorf("Uptime mismatch: got %d, want %d", decoded.Uptime, original.Uptime)
	}
}

func TestSensorData_UnmarshalFromExternalJSON(t *testing.T) {
	// Test unmarshaling from JSON that might come from an external source
	jsonStr := `{
		"node_id": "external-sensor",
		"hot_outlet": 115.5,
		"return_line": 92.0,
		"hot_valid": true,
		"return_valid": true,
		"timestamp": "2024-01-15T12:00:00Z"
	}`

	var data SensorData
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("Failed to unmarshal external JSON: %v", err)
	}

	if data.NodeID != "external-sensor" {
		t.Errorf("NodeID mismatch: got %s", data.NodeID)
	}
	if data.HotOutlet != 115.5 {
		t.Errorf("HotOutlet mismatch: got %f", data.HotOutlet)
	}
	if data.Timestamp.Hour() != 12 {
		t.Errorf("Timestamp hour mismatch: got %d", data.Timestamp.Hour())
	}
}

func TestFlowEvent_DurationConversion(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{"zero", 0},
		{"one second", time.Second},
		{"one minute", time.Minute},
		{"mixed", 90 * time.Second},
		{"fractional", 1500 * time.Millisecond},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original := FlowEvent{
				NodeID:    "test",
				Duration:  tt.duration,
				Timestamp: time.Now(),
			}

			data, err := json.Marshal(original)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var decoded FlowEvent
			if err := json.Unmarshal(data, &decoded); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// Allow for floating point precision loss
			diff := original.Duration - decoded.Duration
			if diff < 0 {
				diff = -diff
			}
			if diff > time.Millisecond {
				t.Errorf("Duration mismatch: got %v, want %v", decoded.Duration, original.Duration)
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
