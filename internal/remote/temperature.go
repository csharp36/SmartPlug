// Package remote provides MQTT-based implementations of core interfaces for distributed mode.
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

// MQTTTemperatureProvider subscribes to temperature data from remote sensor nodes.
// It implements core.TemperatureProvider for controller mode.
type MQTTTemperatureProvider struct {
	mu sync.RWMutex

	client     mqtt.Client
	topics     *mqttutil.Topics
	nodeIDs    []string
	timeout    time.Duration

	lastHot    core.TemperatureReading
	lastReturn core.TemperatureReading
	lastSeen   map[string]time.Time

	callbacks []func(hot, ret core.TemperatureReading)
	stopChan  chan struct{}
}

// NewMQTTTemperatureProvider creates a new MQTTTemperatureProvider.
// nodeIDs specifies which sensor node(s) to listen to. If multiple nodes are specified,
// the first one that sends data will be used (for redundancy).
// timeout specifies how long before sensor data is considered stale.
func NewMQTTTemperatureProvider(client mqtt.Client, topics *mqttutil.Topics, nodeIDs []string, timeout time.Duration) *MQTTTemperatureProvider {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &MQTTTemperatureProvider{
		client:   client,
		topics:   topics,
		nodeIDs:  nodeIDs,
		timeout:  timeout,
		lastSeen: make(map[string]time.Time),
		stopChan: make(chan struct{}),
	}
}

// GetCurrentReadings returns the most recent temperature readings.
func (m *MQTTTemperatureProvider) GetCurrentReadings() (hot, ret core.TemperatureReading) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hot = m.lastHot
	ret = m.lastReturn

	// Mark as invalid if data is stale
	if time.Since(hot.Timestamp) > m.timeout {
		hot.Valid = false
	}
	if time.Since(ret.Timestamp) > m.timeout {
		ret.Valid = false
	}

	return hot, ret
}

// GetTemperatureDifferential returns the current temperature differential.
func (m *MQTTTemperatureProvider) GetTemperatureDifferential() (float64, error) {
	hot, ret := m.GetCurrentReadings()
	if !hot.Valid || !ret.Valid {
		return 0, fmt.Errorf("sensor readings not available or stale")
	}
	return hot.Temperature - ret.Temperature, nil
}

// OnReading registers a callback for temperature reading updates.
func (m *MQTTTemperatureProvider) OnReading(callback func(hot, ret core.TemperatureReading)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// Start begins subscribing to sensor data topics.
func (m *MQTTTemperatureProvider) Start() error {
	if !m.client.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	// Subscribe to each node's sensor data topic
	for _, nodeID := range m.nodeIDs {
		topic := m.topics.SensorData(nodeID)
		token := m.client.Subscribe(topic, 1, m.handleSensorData)
		if token.Wait() && token.Error() != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", topic, token.Error())
		}
		log.Printf("Subscribed to sensor data: %s", topic)
	}

	// Also subscribe to wildcard to catch all sensors (for discovery)
	wildcard := m.topics.SensorDataWildcard()
	token := m.client.Subscribe(wildcard, 1, m.handleSensorData)
	if token.Wait() && token.Error() != nil {
		log.Printf("Warning: failed to subscribe to wildcard %s: %v", wildcard, token.Error())
	}

	// Start stale data checker
	go m.checkStaleData()

	return nil
}

// Stop halts subscription and releases resources.
func (m *MQTTTemperatureProvider) Stop() {
	close(m.stopChan)

	// Unsubscribe from topics
	for _, nodeID := range m.nodeIDs {
		topic := m.topics.SensorData(nodeID)
		m.client.Unsubscribe(topic)
	}
	m.client.Unsubscribe(m.topics.SensorDataWildcard())
}

// handleSensorData processes incoming sensor data messages.
func (m *MQTTTemperatureProvider) handleSensorData(client mqtt.Client, msg mqtt.Message) {
	var data core.SensorData
	if err := json.Unmarshal(msg.Payload(), &data); err != nil {
		log.Printf("Failed to parse sensor data: %v", err)
		return
	}

	// Check if this is from a configured node
	if !m.isConfiguredNode(data.NodeID) && len(m.nodeIDs) > 0 {
		// Only accept data from configured nodes if any are specified
		return
	}

	m.mu.Lock()
	m.lastSeen[data.NodeID] = time.Now()

	// Update readings
	m.lastHot = core.TemperatureReading{
		SensorID:    data.HotSensorID,
		Temperature: data.HotOutlet,
		Timestamp:   data.Timestamp,
		Valid:       data.HotValid,
	}
	m.lastReturn = core.TemperatureReading{
		SensorID:    data.ReturnSensorID,
		Temperature: data.ReturnLine,
		Timestamp:   data.Timestamp,
		Valid:       data.ReturnValid,
	}

	callbacks := m.callbacks
	hot := m.lastHot
	ret := m.lastReturn
	m.mu.Unlock()

	// Notify callbacks
	for _, cb := range callbacks {
		go cb(hot, ret)
	}
}

// isConfiguredNode checks if the given node ID is in our configured list.
func (m *MQTTTemperatureProvider) isConfiguredNode(nodeID string) bool {
	for _, id := range m.nodeIDs {
		if id == nodeID {
			return true
		}
	}
	return false
}

// checkStaleData periodically checks for stale sensor data.
func (m *MQTTTemperatureProvider) checkStaleData() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.mu.RLock()
			hotStale := time.Since(m.lastHot.Timestamp) > m.timeout
			retStale := time.Since(m.lastReturn.Timestamp) > m.timeout
			m.mu.RUnlock()

			if hotStale || retStale {
				log.Printf("Warning: Sensor data is stale (hot: %v, return: %v)", hotStale, retStale)
			}

		case <-m.stopChan:
			return
		}
	}
}

// GetLastSeen returns when data was last received from each node.
func (m *MQTTTemperatureProvider) GetLastSeen() map[string]time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]time.Time)
	for k, v := range m.lastSeen {
		result[k] = v
	}
	return result
}

// Ensure interface is satisfied at compile time.
var _ core.TemperatureProvider = (*MQTTTemperatureProvider)(nil)
