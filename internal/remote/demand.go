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

// MQTTDemandDetector subscribes to flow data from remote sensor nodes.
// It implements core.DemandDetector for controller mode.
type MQTTDemandDetector struct {
	mu sync.RWMutex

	client    mqtt.Client
	topics    *mqttutil.Topics
	nodeIDs   []string
	timeout   time.Duration

	flowActive   bool
	lastFlowData core.FlowData
	lastSeen     map[string]time.Time

	callbacks []func(active bool)
	stopChan  chan struct{}
}

// NewMQTTDemandDetector creates a new MQTTDemandDetector.
// nodeIDs specifies which sensor node(s) to listen to.
// timeout specifies how long before flow data is considered stale.
func NewMQTTDemandDetector(client mqtt.Client, topics *mqttutil.Topics, nodeIDs []string, timeout time.Duration) *MQTTDemandDetector {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &MQTTDemandDetector{
		client:   client,
		topics:   topics,
		nodeIDs:  nodeIDs,
		timeout:  timeout,
		lastSeen: make(map[string]time.Time),
		stopChan: make(chan struct{}),
	}
}

// IsFlowActive returns true if water flow is currently detected.
func (m *MQTTDemandDetector) IsFlowActive() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Consider flow inactive if data is stale
	if time.Since(m.lastFlowData.Timestamp) > m.timeout {
		return false
	}

	return m.flowActive
}

// OnDemand registers a callback for demand state changes.
func (m *MQTTDemandDetector) OnDemand(callback func(active bool)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callbacks = append(m.callbacks, callback)
}

// Start begins subscribing to flow data topics.
func (m *MQTTDemandDetector) Start() error {
	if !m.client.IsConnected() {
		return fmt.Errorf("MQTT client not connected")
	}

	// Subscribe to each node's flow data topic
	for _, nodeID := range m.nodeIDs {
		topic := m.topics.FlowData(nodeID)
		token := m.client.Subscribe(topic, 1, m.handleFlowData)
		if token.Wait() && token.Error() != nil {
			return fmt.Errorf("failed to subscribe to %s: %w", topic, token.Error())
		}
		log.Printf("Subscribed to flow data: %s", topic)
	}

	// Also subscribe to wildcard to catch all flow data
	wildcard := m.topics.FlowDataWildcard()
	token := m.client.Subscribe(wildcard, 1, m.handleFlowData)
	if token.Wait() && token.Error() != nil {
		log.Printf("Warning: failed to subscribe to wildcard %s: %v", wildcard, token.Error())
	}

	// Start stale data checker
	go m.checkStaleData()

	return nil
}

// Stop halts subscription and releases resources.
func (m *MQTTDemandDetector) Stop() {
	close(m.stopChan)

	// Unsubscribe from topics
	for _, nodeID := range m.nodeIDs {
		topic := m.topics.FlowData(nodeID)
		m.client.Unsubscribe(topic)
	}
	m.client.Unsubscribe(m.topics.FlowDataWildcard())
}

// handleFlowData processes incoming flow data messages.
func (m *MQTTDemandDetector) handleFlowData(client mqtt.Client, msg mqtt.Message) {
	var data core.FlowData
	if err := json.Unmarshal(msg.Payload(), &data); err != nil {
		log.Printf("Failed to parse flow data: %v", err)
		return
	}

	// Check if this is from a configured node
	if !m.isConfiguredNode(data.NodeID) && len(m.nodeIDs) > 0 {
		return
	}

	m.mu.Lock()
	m.lastSeen[data.NodeID] = time.Now()

	previousActive := m.flowActive
	m.flowActive = data.Active
	m.lastFlowData = data

	callbacks := m.callbacks
	currentActive := m.flowActive
	m.mu.Unlock()

	// Notify callbacks if state changed
	if currentActive != previousActive {
		log.Printf("Flow state changed: %v (from node: %s)", currentActive, data.NodeID)
		for _, cb := range callbacks {
			go cb(currentActive)
		}
	}
}

// isConfiguredNode checks if the given node ID is in our configured list.
func (m *MQTTDemandDetector) isConfiguredNode(nodeID string) bool {
	for _, id := range m.nodeIDs {
		if id == nodeID {
			return true
		}
	}
	return false
}

// checkStaleData periodically checks for stale flow data and considers flow inactive.
func (m *MQTTDemandDetector) checkStaleData() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.mu.Lock()
			if m.flowActive && time.Since(m.lastFlowData.Timestamp) > m.timeout {
				log.Printf("Flow data stale, considering flow inactive")
				m.flowActive = false
				callbacks := m.callbacks
				m.mu.Unlock()

				// Notify callbacks
				for _, cb := range callbacks {
					go cb(false)
				}
			} else {
				m.mu.Unlock()
			}

		case <-m.stopChan:
			return
		}
	}
}

// GetStats returns flow meter statistics (limited in remote mode).
func (m *MQTTDemandDetector) GetStats() core.FlowMeterStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return core.FlowMeterStats{
		IsFlowActive: m.flowActive,
		PulseCount:   m.lastFlowData.PulseCount,
	}
}

// GetLastSeen returns when data was last received from each node.
func (m *MQTTDemandDetector) GetLastSeen() map[string]time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]time.Time)
	for k, v := range m.lastSeen {
		result[k] = v
	}
	return result
}

// Ensure interfaces are satisfied at compile time.
var (
	_ core.DemandDetector    = (*MQTTDemandDetector)(nil)
	_ core.FlowStatsProvider = (*MQTTDemandDetector)(nil)
)
