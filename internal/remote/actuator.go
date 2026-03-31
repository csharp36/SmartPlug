package remote

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/smartplug/smartplug/internal/core"
	mqttutil "github.com/smartplug/smartplug/internal/mqtt"
)

// MQTTPumpActuator controls a pump via MQTT commands.
// It implements core.PumpActuator for controller mode, sending commands to a smart plug
// or actuator node that controls the physical pump.
type MQTTPumpActuator struct {
	mu sync.RWMutex

	client  mqtt.Client
	topics  *mqttutil.Topics
	timeout time.Duration

	isOn         bool
	lastStatus   core.PumpStatus
	lastCommand  time.Time
	pendingOn    bool

	callbacks []func(on bool)
	stopChan  chan struct{}
}

// NewMQTTPumpActuator creates a new MQTTPumpActuator.
// timeout specifies how long to wait for status confirmation after sending a command.
func NewMQTTPumpActuator(client mqtt.Client, topics *mqttutil.Topics, timeout time.Duration) *MQTTPumpActuator {
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	return &MQTTPumpActuator{
		client:   client,
		topics:   topics,
		timeout:  timeout,
		stopChan: make(chan struct{}),
	}
}

// TurnOn sends a command to turn on the pump.
func (m *MQTTPumpActuator) TurnOn() error {
	m.mu.Lock()
	m.pendingOn = true
	m.lastCommand = time.Now()
	m.mu.Unlock()

	// Send structured command
	cmd := core.PumpCommand{
		Command:   "on",
		Source:    "controller",
		Timestamp: time.Now(),
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	topic := m.topics.PumpCommand()
	token := m.client.Publish(topic, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish command: %w", token.Error())
	}

	// Also publish simple ON for compatibility
	simpleTopic := m.topics.PumpCommandSimple()
	token = m.client.Publish(simpleTopic, 1, false, "ON")
	token.Wait()

	log.Printf("Sent pump ON command to %s", topic)
	return nil
}

// TurnOff sends a command to turn off the pump.
func (m *MQTTPumpActuator) TurnOff() error {
	m.mu.Lock()
	m.pendingOn = false
	m.lastCommand = time.Now()
	m.mu.Unlock()

	// Send structured command
	cmd := core.PumpCommand{
		Command:   "off",
		Source:    "controller",
		Timestamp: time.Now(),
	}
	payload, err := json.Marshal(cmd)
	if err != nil {
		return fmt.Errorf("failed to marshal command: %w", err)
	}

	topic := m.topics.PumpCommand()
	token := m.client.Publish(topic, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish command: %w", token.Error())
	}

	// Also publish simple OFF for compatibility
	simpleTopic := m.topics.PumpCommandSimple()
	token = m.client.Publish(simpleTopic, 1, false, "OFF")
	token.Wait()

	log.Printf("Sent pump OFF command to %s", topic)
	return nil
}

// IsOn returns true if the pump is currently running (based on last status).
func (m *MQTTPumpActuator) IsOn() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isOn
}

// OnStateChange registers a callback for pump state changes.
func (m *MQTTPumpActuator) OnStateChange(callback func(on bool)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// Initialize subscribes to pump status topics.
func (m *MQTTPumpActuator) Initialize() error {
	if !m.client.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	// Subscribe to pump status (JSON)
	statusTopic := m.topics.PumpStatus()
	token := m.client.Subscribe(statusTopic, 1, m.handlePumpStatus)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", statusTopic, token.Error())
	}
	log.Printf("Subscribed to pump status: %s", statusTopic)

	// Subscribe to pump state (simple ON/OFF)
	stateTopic := m.topics.PumpState()
	token = m.client.Subscribe(stateTopic, 1, m.handlePumpState)
	if token.Wait() && token.Error() != nil {
		log.Printf("Warning: failed to subscribe to %s: %v", stateTopic, token.Error())
	}

	return nil
}

// Close releases resources.
func (m *MQTTPumpActuator) Close() error {
	close(m.stopChan)

	// Send final OFF command to ensure pump is off
	m.TurnOff()

	// Unsubscribe
	m.client.Unsubscribe(m.topics.PumpStatus())
	m.client.Unsubscribe(m.topics.PumpState())

	return nil
}

// handlePumpStatus processes incoming pump status messages (JSON format).
func (m *MQTTPumpActuator) handlePumpStatus(client mqtt.Client, msg mqtt.Message) {
	var status core.PumpStatus
	if err := json.Unmarshal(msg.Payload(), &status); err != nil {
		log.Printf("Failed to parse pump status: %v", err)
		return
	}

	m.mu.Lock()
	previousOn := m.isOn
	m.isOn = status.IsOn
	m.lastStatus = status
	callbacks := m.callbacks
	currentOn := m.isOn
	m.mu.Unlock()

	// Notify callbacks if state changed
	if currentOn != previousOn {
		log.Printf("Pump state changed: %v", currentOn)
		for _, cb := range callbacks {
			go cb(currentOn)
		}
	}
}

// handlePumpState processes incoming simple pump state messages (ON/OFF).
func (m *MQTTPumpActuator) handlePumpState(client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())
	isOn := payload == "ON"

	m.mu.Lock()
	previousOn := m.isOn
	m.isOn = isOn
	callbacks := m.callbacks
	m.mu.Unlock()

	// Notify callbacks if state changed
	if isOn != previousOn {
		log.Printf("Pump state changed (simple): %v", isOn)
		for _, cb := range callbacks {
			go cb(isOn)
		}
	}
}

// GetLastStatus returns the last received pump status.
func (m *MQTTPumpActuator) GetLastStatus() core.PumpStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastStatus
}

// Ensure interface is satisfied at compile time.
var _ core.PumpActuator = (*MQTTPumpActuator)(nil)
