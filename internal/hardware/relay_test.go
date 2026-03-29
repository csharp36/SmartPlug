package hardware

import (
	"testing"
	"time"
)

func TestRelayController_MockMode(t *testing.T) {
	gpio := NewMockGPIO()
	rc := NewRelayController(gpio, 17)
	rc.EnableMockMode()

	if err := rc.Initialize(); err != nil {
		t.Fatalf("Failed to initialize relay: %v", err)
	}

	// Initial state should be off
	if rc.GetState() != RelayOff {
		t.Error("Expected initial state to be RelayOff")
	}

	// Turn on
	if err := rc.TurnOn(); err != nil {
		t.Fatalf("Failed to turn on relay: %v", err)
	}

	if rc.GetState() != RelayOn {
		t.Error("Expected state to be RelayOn after TurnOn")
	}

	if !rc.IsOn() {
		t.Error("Expected IsOn() to return true")
	}

	// Turn off
	if err := rc.TurnOff(); err != nil {
		t.Fatalf("Failed to turn off relay: %v", err)
	}

	if rc.GetState() != RelayOff {
		t.Error("Expected state to be RelayOff after TurnOff")
	}
}

func TestRelayController_StateString(t *testing.T) {
	if RelayOn.String() != "on" {
		t.Errorf("Expected RelayOn.String() to be 'on', got %s", RelayOn.String())
	}

	if RelayOff.String() != "off" {
		t.Errorf("Expected RelayOff.String() to be 'off', got %s", RelayOff.String())
	}
}

func TestRelayController_Statistics(t *testing.T) {
	gpio := NewMockGPIO()
	rc := NewRelayController(gpio, 17)
	rc.EnableMockMode()
	rc.Initialize()

	// Turn on and wait a bit
	rc.TurnOn()
	time.Sleep(50 * time.Millisecond)

	runtime := rc.GetCurrentRuntime()
	if runtime < 50*time.Millisecond {
		t.Errorf("Expected runtime >= 50ms, got %v", runtime)
	}

	// Turn off
	rc.TurnOff()

	// Current runtime should be 0 after turning off
	if rc.GetCurrentRuntime() != 0 {
		t.Error("Expected current runtime to be 0 after TurnOff")
	}

	// Cycle count should be 1
	if rc.GetCycleCount() != 1 {
		t.Errorf("Expected cycle count 1, got %d", rc.GetCycleCount())
	}

	// Total runtime should be preserved
	if rc.GetTotalRuntime() < 50*time.Millisecond {
		t.Error("Expected total runtime to be preserved")
	}
}

func TestRelayController_Callback(t *testing.T) {
	gpio := NewMockGPIO()
	rc := NewRelayController(gpio, 17)
	rc.EnableMockMode()
	rc.Initialize()

	callbackCalled := false
	var lastState RelayState

	rc.OnStateChange(func(state RelayState) {
		callbackCalled = true
		lastState = state
	})

	rc.TurnOn()

	// Give callback time to execute
	time.Sleep(10 * time.Millisecond)

	if !callbackCalled {
		t.Error("Expected callback to be called")
	}

	if lastState != RelayOn {
		t.Errorf("Expected callback state to be RelayOn, got %v", lastState)
	}
}

func TestRelayController_DoubleOn(t *testing.T) {
	gpio := NewMockGPIO()
	rc := NewRelayController(gpio, 17)
	rc.EnableMockMode()
	rc.Initialize()

	// Turn on twice
	rc.TurnOn()
	rc.TurnOn()

	// Should still only have 1 cycle
	if rc.GetCycleCount() != 1 {
		t.Errorf("Expected cycle count 1 after double TurnOn, got %d", rc.GetCycleCount())
	}
}

func TestRelayController_DoubleOff(t *testing.T) {
	gpio := NewMockGPIO()
	rc := NewRelayController(gpio, 17)
	rc.EnableMockMode()
	rc.Initialize()

	// Turn off twice (already off)
	rc.TurnOff()
	rc.TurnOff()

	// Should have 0 cycles
	if rc.GetCycleCount() != 0 {
		t.Errorf("Expected cycle count 0, got %d", rc.GetCycleCount())
	}
}

func TestMockGPIO(t *testing.T) {
	gpio := NewMockGPIO()

	// Setup pin as output
	if err := gpio.Setup(17, true); err != nil {
		t.Fatalf("Failed to setup pin: %v", err)
	}

	// Write high
	if err := gpio.Write(17, true); err != nil {
		t.Fatalf("Failed to write pin: %v", err)
	}

	// Read back
	value, err := gpio.Read(17)
	if err != nil {
		t.Fatalf("Failed to read pin: %v", err)
	}

	if !value {
		t.Error("Expected pin to be high")
	}

	// Use helper
	if !gpio.GetPinState(17) {
		t.Error("Expected GetPinState to return true")
	}
}
