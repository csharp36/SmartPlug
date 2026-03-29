package hardware

import (
	"testing"
	"time"
)

func TestSensorManager_MockMode(t *testing.T) {
	sm := NewSensorManager("", "", 1)
	sm.EnableMockMode(120.0, 95.0)

	// Test initial temperatures
	reading, err := sm.ReadSensor("28-mock-hot-001")
	if err != nil {
		t.Fatalf("Failed to read mock sensor: %v", err)
	}

	if reading.Temperature != 120.0 {
		t.Errorf("Expected temperature 120.0, got %f", reading.Temperature)
	}

	if !reading.Valid {
		t.Error("Expected reading to be valid")
	}
}

func TestSensorManager_SetMockTemperatures(t *testing.T) {
	sm := NewSensorManager("hot-001", "ret-001", 1)
	sm.EnableMockMode(100.0, 80.0)

	// Change temperatures
	sm.SetMockTemperatures(130.0, 90.0)

	reading, _ := sm.ReadSensor("hot-001")
	if reading.Temperature != 130.0 {
		t.Errorf("Expected temperature 130.0, got %f", reading.Temperature)
	}
}

func TestSensorManager_DiscoverSensors(t *testing.T) {
	sm := NewSensorManager("", "", 1)
	sm.EnableMockMode(100.0, 80.0)

	sensors, err := sm.DiscoverSensors()
	if err != nil {
		t.Fatalf("Failed to discover sensors: %v", err)
	}

	if len(sensors) != 2 {
		t.Errorf("Expected 2 mock sensors, got %d", len(sensors))
	}
}

func TestSensorManager_GetTemperatureDifferential(t *testing.T) {
	sm := NewSensorManager("hot", "ret", 1)
	sm.EnableMockMode(120.0, 100.0)

	// Start polling to populate readings
	sm.Start()
	defer sm.Stop()

	// Wait for initial reading
	time.Sleep(100 * time.Millisecond)

	diff, err := sm.GetTemperatureDifferential()
	if err != nil {
		t.Fatalf("Failed to get differential: %v", err)
	}

	expected := 20.0
	if diff != expected {
		t.Errorf("Expected differential %f, got %f", expected, diff)
	}
}

func TestCelsiusToFahrenheit(t *testing.T) {
	tests := []struct {
		celsius    float64
		fahrenheit float64
	}{
		{0.0, 32.0},
		{100.0, 212.0},
		{37.0, 98.6},
		{-40.0, -40.0},
	}

	for _, test := range tests {
		result := celsiusToFahrenheit(test.celsius)
		if result != test.fahrenheit {
			t.Errorf("celsiusToFahrenheit(%f) = %f, expected %f",
				test.celsius, result, test.fahrenheit)
		}
	}
}

func TestFahrenheitToCelsius(t *testing.T) {
	tests := []struct {
		fahrenheit float64
		celsius    float64
	}{
		{32.0, 0.0},
		{212.0, 100.0},
		{98.6, 37.0},
		{-40.0, -40.0},
	}

	for _, test := range tests {
		result := fahrenheitToCelsius(test.fahrenheit)
		// Allow small floating point errors
		if result < test.celsius-0.1 || result > test.celsius+0.1 {
			t.Errorf("fahrenheitToCelsius(%f) = %f, expected %f",
				test.fahrenheit, result, test.celsius)
		}
	}
}
