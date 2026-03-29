// Package hardware provides interfaces to physical hardware components
package hardware

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// OneWireBasePath is the sysfs path for 1-Wire devices
	OneWireBasePath = "/sys/bus/w1/devices"
)

// TemperatureReading represents a single temperature reading
type TemperatureReading struct {
	SensorID    string
	Temperature float64 // Fahrenheit
	Timestamp   time.Time
	Valid       bool
	Error       error
}

// SensorManager handles DS18B20 temperature sensor operations
type SensorManager struct {
	mu              sync.RWMutex
	hotOutletID     string
	returnLineID    string
	lastHotOutlet   TemperatureReading
	lastReturnLine  TemperatureReading
	pollInterval    time.Duration
	stopChan        chan struct{}
	callbacks       []func(hot, ret TemperatureReading)
	mockMode        bool
	mockHotTemp     float64
	mockReturnTemp  float64
}

// NewSensorManager creates a new sensor manager
func NewSensorManager(hotOutletID, returnLineID string, pollInterval int) *SensorManager {
	return &SensorManager{
		hotOutletID:  hotOutletID,
		returnLineID: returnLineID,
		pollInterval: time.Duration(pollInterval) * time.Second,
		stopChan:     make(chan struct{}),
	}
}

// EnableMockMode enables mock mode for testing without hardware
func (sm *SensorManager) EnableMockMode(hotTemp, returnTemp float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.mockMode = true
	sm.mockHotTemp = hotTemp
	sm.mockReturnTemp = returnTemp
}

// SetMockTemperatures updates mock temperatures
func (sm *SensorManager) SetMockTemperatures(hotTemp, returnTemp float64) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.mockHotTemp = hotTemp
	sm.mockReturnTemp = returnTemp
}

// DiscoverSensors finds all connected DS18B20 sensors
func (sm *SensorManager) DiscoverSensors() ([]string, error) {
	if sm.mockMode {
		return []string{"28-mock-hot-001", "28-mock-ret-002"}, nil
	}

	entries, err := os.ReadDir(OneWireBasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read 1-Wire bus: %w", err)
	}

	var sensors []string
	for _, entry := range entries {
		name := entry.Name()
		// DS18B20 sensors start with "28-"
		if strings.HasPrefix(name, "28-") {
			sensors = append(sensors, name)
		}
	}

	return sensors, nil
}

// AutoAssignSensors automatically assigns discovered sensors
// The first sensor found is assigned as hot outlet, second as return line
func (sm *SensorManager) AutoAssignSensors() error {
	sensors, err := sm.DiscoverSensors()
	if err != nil {
		return err
	}

	if len(sensors) < 2 {
		return fmt.Errorf("need at least 2 sensors, found %d", len(sensors))
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.hotOutletID == "" {
		sm.hotOutletID = sensors[0]
	}
	if sm.returnLineID == "" {
		sm.returnLineID = sensors[1]
	}

	return nil
}

// ReadSensor reads temperature from a specific sensor
func (sm *SensorManager) ReadSensor(sensorID string) (TemperatureReading, error) {
	reading := TemperatureReading{
		SensorID:  sensorID,
		Timestamp: time.Now(),
	}

	if sm.mockMode {
		sm.mu.RLock()
		if sensorID == sm.hotOutletID || strings.Contains(sensorID, "hot") {
			reading.Temperature = sm.mockHotTemp
		} else {
			reading.Temperature = sm.mockReturnTemp
		}
		sm.mu.RUnlock()
		reading.Valid = true
		return reading, nil
	}

	// Read from sysfs
	path := filepath.Join(OneWireBasePath, sensorID, "w1_slave")
	file, err := os.Open(path)
	if err != nil {
		reading.Error = fmt.Errorf("failed to open sensor file: %w", err)
		return reading, reading.Error
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		reading.Error = fmt.Errorf("failed to read sensor file: %w", err)
		return reading, reading.Error
	}

	if len(lines) < 2 {
		reading.Error = fmt.Errorf("unexpected sensor output format")
		return reading, reading.Error
	}

	// First line should end with "YES" for valid CRC
	if !strings.HasSuffix(lines[0], "YES") {
		reading.Error = fmt.Errorf("CRC check failed")
		return reading, reading.Error
	}

	// Second line contains temperature after "t="
	parts := strings.Split(lines[1], "t=")
	if len(parts) != 2 {
		reading.Error = fmt.Errorf("temperature value not found")
		return reading, reading.Error
	}

	// Temperature is in millidegrees Celsius
	milliC, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
	if err != nil {
		reading.Error = fmt.Errorf("failed to parse temperature: %w", err)
		return reading, reading.Error
	}

	// Convert to Fahrenheit
	celsius := float64(milliC) / 1000.0
	reading.Temperature = celsiusToFahrenheit(celsius)
	reading.Valid = true

	return reading, nil
}

// GetCurrentReadings returns the most recent temperature readings
func (sm *SensorManager) GetCurrentReadings() (hot, ret TemperatureReading) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.lastHotOutlet, sm.lastReturnLine
}

// GetTemperatureDifferential returns the current temperature differential
func (sm *SensorManager) GetTemperatureDifferential() (float64, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if !sm.lastHotOutlet.Valid || !sm.lastReturnLine.Valid {
		return 0, fmt.Errorf("sensor readings not available")
	}

	return sm.lastHotOutlet.Temperature - sm.lastReturnLine.Temperature, nil
}

// OnReading registers a callback for temperature readings
func (sm *SensorManager) OnReading(callback func(hot, ret TemperatureReading)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.callbacks = append(sm.callbacks, callback)
}

// Start begins polling sensors
func (sm *SensorManager) Start() error {
	// Do initial sensor discovery if IDs not set
	if sm.hotOutletID == "" || sm.returnLineID == "" {
		if err := sm.AutoAssignSensors(); err != nil {
			return fmt.Errorf("sensor auto-assignment failed: %w", err)
		}
	}

	go sm.pollLoop()
	return nil
}

// Stop halts sensor polling
func (sm *SensorManager) Stop() {
	close(sm.stopChan)
}

func (sm *SensorManager) pollLoop() {
	ticker := time.NewTicker(sm.pollInterval)
	defer ticker.Stop()

	// Initial read
	sm.readAllSensors()

	for {
		select {
		case <-ticker.C:
			sm.readAllSensors()
		case <-sm.stopChan:
			return
		}
	}
}

func (sm *SensorManager) readAllSensors() {
	hotReading, _ := sm.ReadSensor(sm.hotOutletID)
	retReading, _ := sm.ReadSensor(sm.returnLineID)

	sm.mu.Lock()
	sm.lastHotOutlet = hotReading
	sm.lastReturnLine = retReading
	callbacks := sm.callbacks
	sm.mu.Unlock()

	// Notify callbacks
	for _, cb := range callbacks {
		cb(hotReading, retReading)
	}
}

// GetSensorIDs returns the configured sensor IDs
func (sm *SensorManager) GetSensorIDs() (hotOutlet, returnLine string) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.hotOutletID, sm.returnLineID
}

// SetSensorIDs configures specific sensor IDs
func (sm *SensorManager) SetSensorIDs(hotOutlet, returnLine string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.hotOutletID = hotOutlet
	sm.returnLineID = returnLine
}

// celsiusToFahrenheit converts temperature from Celsius to Fahrenheit
func celsiusToFahrenheit(c float64) float64 {
	return c*9.0/5.0 + 32.0
}

// fahrenheitToCelsius converts temperature from Fahrenheit to Celsius
func fahrenheitToCelsius(f float64) float64 {
	return (f - 32.0) * 5.0 / 9.0
}
