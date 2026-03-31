package hardware

import (
	"github.com/smartplug/smartplug/internal/core"
)

// LocalTemperatureProvider wraps SensorManager to implement core.TemperatureProvider.
// This adapter allows the existing SensorManager to be used through the interface.
type LocalTemperatureProvider struct {
	sensors *SensorManager
}

// NewLocalTemperatureProvider creates a new LocalTemperatureProvider wrapping the given SensorManager.
func NewLocalTemperatureProvider(sensors *SensorManager) *LocalTemperatureProvider {
	return &LocalTemperatureProvider{sensors: sensors}
}

// GetCurrentReadings returns the most recent temperature readings.
func (l *LocalTemperatureProvider) GetCurrentReadings() (hot, ret core.TemperatureReading) {
	hotHW, retHW := l.sensors.GetCurrentReadings()
	return convertReading(hotHW), convertReading(retHW)
}

// GetTemperatureDifferential returns the current temperature differential.
func (l *LocalTemperatureProvider) GetTemperatureDifferential() (float64, error) {
	return l.sensors.GetTemperatureDifferential()
}

// OnReading registers a callback for temperature reading updates.
func (l *LocalTemperatureProvider) OnReading(callback func(hot, ret core.TemperatureReading)) {
	l.sensors.OnReading(func(hotHW, retHW TemperatureReading) {
		callback(convertReading(hotHW), convertReading(retHW))
	})
}

// Start begins temperature monitoring.
func (l *LocalTemperatureProvider) Start() error {
	return l.sensors.Start()
}

// Stop halts temperature monitoring.
func (l *LocalTemperatureProvider) Stop() {
	l.sensors.Stop()
}

// Underlying returns the underlying SensorManager.
// This is useful when sensor discovery or other hardware-specific features are needed.
func (l *LocalTemperatureProvider) Underlying() *SensorManager {
	return l.sensors
}

// DiscoverSensors implements core.SensorDiscoverer.
func (l *LocalTemperatureProvider) DiscoverSensors() ([]string, error) {
	return l.sensors.DiscoverSensors()
}

// GetSensorIDs implements core.SensorDiscoverer.
func (l *LocalTemperatureProvider) GetSensorIDs() (hotOutlet, returnLine string) {
	return l.sensors.GetSensorIDs()
}

// SetSensorIDs implements core.SensorDiscoverer.
func (l *LocalTemperatureProvider) SetSensorIDs(hotOutlet, returnLine string) {
	l.sensors.SetSensorIDs(hotOutlet, returnLine)
}

// convertReading converts a hardware.TemperatureReading to a core.TemperatureReading.
func convertReading(hw TemperatureReading) core.TemperatureReading {
	return core.TemperatureReading{
		SensorID:    hw.SensorID,
		Temperature: hw.Temperature,
		Timestamp:   hw.Timestamp,
		Valid:       hw.Valid,
		Error:       hw.Error,
	}
}

// LocalDemandDetector wraps FlowMeter to implement core.DemandDetector.
type LocalDemandDetector struct {
	flowMeter *FlowMeter
}

// NewLocalDemandDetector creates a new LocalDemandDetector wrapping the given FlowMeter.
func NewLocalDemandDetector(flowMeter *FlowMeter) *LocalDemandDetector {
	return &LocalDemandDetector{flowMeter: flowMeter}
}

// IsFlowActive returns true if water flow is currently detected.
func (l *LocalDemandDetector) IsFlowActive() bool {
	return l.flowMeter.IsFlowActive()
}

// OnDemand registers a callback for demand state changes.
func (l *LocalDemandDetector) OnDemand(callback func(active bool)) {
	l.flowMeter.OnDemand(callback)
}

// Start begins flow monitoring.
func (l *LocalDemandDetector) Start() error {
	return l.flowMeter.Start()
}

// Stop halts flow monitoring.
func (l *LocalDemandDetector) Stop() {
	l.flowMeter.Stop()
}

// Underlying returns the underlying FlowMeter.
// This is useful when flow event callbacks or other hardware-specific features are needed.
func (l *LocalDemandDetector) Underlying() *FlowMeter {
	return l.flowMeter
}

// GetStats implements core.FlowStatsProvider.
func (l *LocalDemandDetector) GetStats() core.FlowMeterStats {
	stats := l.flowMeter.GetStats()
	return core.FlowMeterStats{
		IsFlowActive:         stats.IsFlowActive,
		PulseCount:           stats.PulseCount,
		LastPulseTime:        stats.LastPulseTime,
		CurrentFlowDuration:  stats.CurrentFlowDuration,
		CurrentSessionPulses: stats.CurrentSessionPulses,
	}
}

// OnFlowEvent registers a callback for flow events (proxy to underlying FlowMeter).
func (l *LocalDemandDetector) OnFlowEvent(callback func(event FlowEvent)) {
	l.flowMeter.OnFlowEvent(callback)
}

// LocalPumpActuator wraps RelayController to implement core.PumpActuator.
type LocalPumpActuator struct {
	relay *RelayController
}

// NewLocalPumpActuator creates a new LocalPumpActuator wrapping the given RelayController.
func NewLocalPumpActuator(relay *RelayController) *LocalPumpActuator {
	return &LocalPumpActuator{relay: relay}
}

// TurnOn activates the pump.
func (l *LocalPumpActuator) TurnOn() error {
	return l.relay.TurnOn()
}

// TurnOff deactivates the pump.
func (l *LocalPumpActuator) TurnOff() error {
	return l.relay.TurnOff()
}

// IsOn returns true if the pump is currently running.
func (l *LocalPumpActuator) IsOn() bool {
	return l.relay.IsOn()
}

// OnStateChange registers a callback for pump state changes.
func (l *LocalPumpActuator) OnStateChange(callback func(on bool)) {
	l.relay.OnStateChange(func(state RelayState) {
		callback(state == RelayOn)
	})
}

// Initialize prepares the actuator for use.
func (l *LocalPumpActuator) Initialize() error {
	return l.relay.Initialize()
}

// Close releases resources and ensures the pump is off.
func (l *LocalPumpActuator) Close() error {
	return l.relay.Close()
}

// Underlying returns the underlying RelayController.
// This is useful when relay statistics or other hardware-specific features are needed.
func (l *LocalPumpActuator) Underlying() *RelayController {
	return l.relay
}

// GetStats returns relay statistics (proxy to underlying RelayController).
func (l *LocalPumpActuator) GetStats() RelayStats {
	return l.relay.GetStats()
}

// NullDemandDetector is a no-op implementation of core.DemandDetector.
// It's used when flow meter is disabled or not available.
type NullDemandDetector struct{}

// NewNullDemandDetector creates a new NullDemandDetector.
func NewNullDemandDetector() *NullDemandDetector {
	return &NullDemandDetector{}
}

// IsFlowActive always returns false for null detector.
func (n *NullDemandDetector) IsFlowActive() bool {
	return false
}

// OnDemand is a no-op for null detector.
func (n *NullDemandDetector) OnDemand(callback func(active bool)) {
	// No-op: null detector never fires callbacks
}

// Start is a no-op for null detector.
func (n *NullDemandDetector) Start() error {
	return nil
}

// Stop is a no-op for null detector.
func (n *NullDemandDetector) Stop() {
	// No-op
}

// Ensure interfaces are satisfied at compile time.
var (
	_ core.TemperatureProvider = (*LocalTemperatureProvider)(nil)
	_ core.SensorDiscoverer    = (*LocalTemperatureProvider)(nil)
	_ core.DemandDetector      = (*LocalDemandDetector)(nil)
	_ core.FlowStatsProvider   = (*LocalDemandDetector)(nil)
	_ core.PumpActuator        = (*LocalPumpActuator)(nil)
	_ core.DemandDetector      = (*NullDemandDetector)(nil)
)
