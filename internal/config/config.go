// Package config handles YAML configuration loading and management
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// Config represents the complete application configuration
type Config struct {
	Deployment DeploymentConfig `yaml:"deployment"`
	Hardware   HardwareConfig   `yaml:"hardware"`
	Sensors    SensorsConfig    `yaml:"sensors"`
	Pump       PumpConfig       `yaml:"pump"`
	FlowMeter  FlowMeterConfig  `yaml:"flowmeter"`
	Schedule   ScheduleConfig   `yaml:"schedule"`
	Web        WebConfig        `yaml:"web"`
	MQTT       MQTTConfig       `yaml:"mqtt"`
	Logging    LoggingConfig    `yaml:"logging"`
	System     SystemConfig     `yaml:"system"`
}

// DeploymentConfig defines the deployment mode and distributed settings
type DeploymentConfig struct {
	// Mode specifies how SmartPlug operates:
	// - "all-in-one": Single device with local GPIO sensors, flow meter, and relay (default)
	// - "sensor": Sensor node that publishes readings over MQTT (no pump control)
	// - "controller": Controller that receives MQTT sensor data and controls pump via MQTT
	Mode string `yaml:"mode"`

	// NodeID is the unique identifier for this node (required for sensor/controller modes)
	NodeID string `yaml:"node_id"`

	// SensorNodeIDs lists the sensor nodes to subscribe to (controller mode only)
	SensorNodeIDs []string `yaml:"sensor_node_ids"`

	// ActuatorType specifies how the pump is controlled in controller mode:
	// - "local": Direct GPIO control (local relay)
	// - "mqtt": Send commands over MQTT to a smart plug or actuator node
	ActuatorType string `yaml:"actuator_type"`

	// DataTimeout is how long (in seconds) before sensor data is considered stale
	// Default: 30 seconds
	DataTimeout int `yaml:"data_timeout"`
}

// HardwareConfig defines GPIO pin assignments
type HardwareConfig struct {
	OneWireGPIO   int `yaml:"onewire_gpio"`
	RelayGPIO     int `yaml:"relay_gpio"`
	FlowMeterGPIO int `yaml:"flowmeter_gpio"`
}

// SensorsConfig defines temperature sensor settings
type SensorsConfig struct {
	HotOutletID  string `yaml:"hot_outlet_id"`
	ReturnLineID string `yaml:"return_line_id"`
	PollInterval int    `yaml:"poll_interval"`
}

// PumpConfig defines pump control thresholds
type PumpConfig struct {
	StartThreshold    float64 `yaml:"start_threshold"`
	StopThreshold     float64 `yaml:"stop_threshold"`
	MaxRuntimeMinutes int     `yaml:"max_runtime_minutes"`
	CooldownMinutes   int     `yaml:"cooldown_minutes"`
	MinTemperature    float64 `yaml:"min_temperature"`
}

// FlowMeterConfig defines flow meter settings
type FlowMeterConfig struct {
	Enabled          bool `yaml:"enabled"`
	PulsesPerLiter   int  `yaml:"pulses_per_liter"`
	TriggerThreshold int  `yaml:"trigger_threshold"`
	DebounceMs       int  `yaml:"debounce_ms"`
	DemandTimeout    int  `yaml:"demand_timeout"`
}

// ScheduleSlot represents a scheduled time window
type ScheduleSlot struct {
	Start   string `yaml:"start"`
	End     string `yaml:"end"`
	Days    []int  `yaml:"days"`
	Enabled bool   `yaml:"enabled"`
}

// LearningConfig defines adaptive learning settings
type LearningConfig struct {
	Enabled              bool `yaml:"enabled"`
	MinDays              int  `yaml:"min_days"`
	VacationTimeoutHours int  `yaml:"vacation_timeout_hours"`
}

// ScheduleConfig defines scheduling settings
type ScheduleConfig struct {
	Enabled  bool           `yaml:"enabled"`
	Slots    []ScheduleSlot `yaml:"slots"`
	Learning LearningConfig `yaml:"learning"`
}

// WebConfig defines web server settings
type WebConfig struct {
	Address      string `yaml:"address"`
	HTTPSEnabled bool   `yaml:"https_enabled"`
	CertFile     string `yaml:"cert_file"`
	KeyFile      string `yaml:"key_file"`
}

// MQTTConfig defines MQTT settings for Home Assistant
type MQTTConfig struct {
	Enabled           bool   `yaml:"enabled"`
	Broker            string `yaml:"broker"`
	Username          string `yaml:"username"`
	Password          string `yaml:"password"`
	TopicPrefix       string `yaml:"topic_prefix"`
	HADiscovery       bool   `yaml:"ha_discovery"`
	HADiscoveryPrefix string `yaml:"ha_discovery_prefix"`
	ClientID          string `yaml:"client_id"`
}

// LoggingConfig defines logging settings
type LoggingConfig struct {
	Level            string `yaml:"level"`
	File             string `yaml:"file"`
	UsageHistoryFile string `yaml:"usage_history_file"`
}

// SystemConfig defines system settings
type SystemConfig struct {
	DataDir string `yaml:"data_dir"`
}

// Manager handles configuration loading and live updates
type Manager struct {
	mu       sync.RWMutex
	config   *Config
	filePath string
}

// NewManager creates a new configuration manager
func NewManager(filePath string) *Manager {
	return &Manager{
		filePath: filePath,
	}
}

// Load reads configuration from the YAML file
func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply defaults
	applyDefaults(&cfg)

	m.config = &cfg
	return nil
}

// Get returns a copy of the current configuration
func (m *Manager) Get() Config {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config == nil {
		return Config{}
	}
	return *m.config
}

// Save writes the current configuration to the YAML file
func (m *Manager) Save() error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	data, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(m.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(m.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Update applies changes to the configuration
func (m *Manager) Update(fn func(*Config)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.config == nil {
		return fmt.Errorf("no configuration loaded")
	}

	fn(m.config)
	return nil
}

// applyDefaults sets default values for unspecified configuration
func applyDefaults(cfg *Config) {
	// Deployment defaults
	if cfg.Deployment.Mode == "" {
		cfg.Deployment.Mode = "all-in-one"
	}
	if cfg.Deployment.DataTimeout == 0 {
		cfg.Deployment.DataTimeout = 30
	}
	if cfg.Deployment.ActuatorType == "" {
		cfg.Deployment.ActuatorType = "local"
	}

	// Hardware defaults
	if cfg.Hardware.OneWireGPIO == 0 {
		cfg.Hardware.OneWireGPIO = 4
	}
	if cfg.Hardware.RelayGPIO == 0 {
		cfg.Hardware.RelayGPIO = 17
	}
	if cfg.Hardware.FlowMeterGPIO == 0 {
		cfg.Hardware.FlowMeterGPIO = 27
	}

	// Sensor defaults
	if cfg.Sensors.PollInterval == 0 {
		cfg.Sensors.PollInterval = 2
	}

	// Pump defaults
	if cfg.Pump.StartThreshold == 0 {
		cfg.Pump.StartThreshold = 12.0
	}
	if cfg.Pump.StopThreshold == 0 {
		cfg.Pump.StopThreshold = 8.0
	}
	if cfg.Pump.MaxRuntimeMinutes == 0 {
		cfg.Pump.MaxRuntimeMinutes = 15
	}
	if cfg.Pump.CooldownMinutes == 0 {
		cfg.Pump.CooldownMinutes = 5
	}
	if cfg.Pump.MinTemperature == 0 {
		cfg.Pump.MinTemperature = 40.0
	}

	// Flow meter defaults
	if cfg.FlowMeter.PulsesPerLiter == 0 {
		cfg.FlowMeter.PulsesPerLiter = 450
	}
	if cfg.FlowMeter.TriggerThreshold == 0 {
		cfg.FlowMeter.TriggerThreshold = 3
	}
	if cfg.FlowMeter.DebounceMs == 0 {
		cfg.FlowMeter.DebounceMs = 50
	}
	if cfg.FlowMeter.DemandTimeout == 0 {
		cfg.FlowMeter.DemandTimeout = 30
	}

	// Schedule defaults
	if cfg.Schedule.Learning.MinDays == 0 {
		cfg.Schedule.Learning.MinDays = 7
	}
	if cfg.Schedule.Learning.VacationTimeoutHours == 0 {
		cfg.Schedule.Learning.VacationTimeoutHours = 24
	}

	// Web defaults
	if cfg.Web.Address == "" {
		cfg.Web.Address = ":8080"
	}

	// MQTT defaults
	if cfg.MQTT.TopicPrefix == "" {
		cfg.MQTT.TopicPrefix = "smartplug"
	}
	if cfg.MQTT.HADiscoveryPrefix == "" {
		cfg.MQTT.HADiscoveryPrefix = "homeassistant"
	}

	// Logging defaults
	if cfg.Logging.Level == "" {
		cfg.Logging.Level = "info"
	}
	if cfg.Logging.UsageHistoryFile == "" {
		cfg.Logging.UsageHistoryFile = "/var/lib/smartplug/usage.json"
	}

	// System defaults
	if cfg.System.DataDir == "" {
		cfg.System.DataDir = "/var/lib/smartplug"
	}
}

// DefaultConfig returns a configuration with all defaults applied
func DefaultConfig() *Config {
	cfg := &Config{}
	applyDefaults(cfg)
	return cfg
}
