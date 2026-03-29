package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Check hardware defaults
	if cfg.Hardware.OneWireGPIO != 4 {
		t.Errorf("Expected OneWireGPIO 4, got %d", cfg.Hardware.OneWireGPIO)
	}
	if cfg.Hardware.RelayGPIO != 17 {
		t.Errorf("Expected RelayGPIO 17, got %d", cfg.Hardware.RelayGPIO)
	}
	if cfg.Hardware.FlowMeterGPIO != 27 {
		t.Errorf("Expected FlowMeterGPIO 27, got %d", cfg.Hardware.FlowMeterGPIO)
	}

	// Check pump defaults
	if cfg.Pump.StartThreshold != 12.0 {
		t.Errorf("Expected StartThreshold 12.0, got %f", cfg.Pump.StartThreshold)
	}
	if cfg.Pump.StopThreshold != 8.0 {
		t.Errorf("Expected StopThreshold 8.0, got %f", cfg.Pump.StopThreshold)
	}

	// Check web defaults
	if cfg.Web.Address != ":8080" {
		t.Errorf("Expected Web.Address ':8080', got %s", cfg.Web.Address)
	}
}

func TestManager_LoadSave(t *testing.T) {
	// Create temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test_config.yaml")

	// Create test config file
	content := `
hardware:
  onewire_gpio: 4
  relay_gpio: 17
pump:
  start_threshold: 15.0
  stop_threshold: 10.0
web:
  address: ":9090"
`
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load config
	mgr := NewManager(configPath)
	if err := mgr.Load(); err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	cfg := mgr.Get()

	// Verify loaded values
	if cfg.Pump.StartThreshold != 15.0 {
		t.Errorf("Expected StartThreshold 15.0, got %f", cfg.Pump.StartThreshold)
	}
	if cfg.Pump.StopThreshold != 10.0 {
		t.Errorf("Expected StopThreshold 10.0, got %f", cfg.Pump.StopThreshold)
	}
	if cfg.Web.Address != ":9090" {
		t.Errorf("Expected Web.Address ':9090', got %s", cfg.Web.Address)
	}

	// Test save
	err := mgr.Update(func(c *Config) {
		c.Pump.StartThreshold = 20.0
	})
	if err != nil {
		t.Fatalf("Failed to update config: %v", err)
	}

	if err := mgr.Save(); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Reload and verify
	mgr2 := NewManager(configPath)
	if err := mgr2.Load(); err != nil {
		t.Fatalf("Failed to reload config: %v", err)
	}

	cfg2 := mgr2.Get()
	if cfg2.Pump.StartThreshold != 20.0 {
		t.Errorf("Expected StartThreshold 20.0 after save, got %f", cfg2.Pump.StartThreshold)
	}
}

func TestManager_LoadMissingFile(t *testing.T) {
	mgr := NewManager("/nonexistent/path/config.yaml")
	err := mgr.Load()
	if err == nil {
		t.Error("Expected error loading nonexistent file")
	}
}

func TestManager_GetWithoutLoad(t *testing.T) {
	mgr := NewManager("/some/path.yaml")
	cfg := mgr.Get()

	// Should return empty config
	if cfg.Hardware.OneWireGPIO != 0 {
		t.Error("Expected empty config when Get() called without Load()")
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &Config{}
	applyDefaults(cfg)

	// Verify all defaults are applied
	if cfg.Sensors.PollInterval != 2 {
		t.Errorf("Expected PollInterval 2, got %d", cfg.Sensors.PollInterval)
	}
	if cfg.FlowMeter.PulsesPerLiter != 450 {
		t.Errorf("Expected PulsesPerLiter 450, got %d", cfg.FlowMeter.PulsesPerLiter)
	}
	if cfg.Schedule.Learning.MinDays != 7 {
		t.Errorf("Expected Learning.MinDays 7, got %d", cfg.Schedule.Learning.MinDays)
	}
	if cfg.MQTT.TopicPrefix != "smartplug" {
		t.Errorf("Expected MQTT.TopicPrefix 'smartplug', got %s", cfg.MQTT.TopicPrefix)
	}
}
