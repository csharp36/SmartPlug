package remote

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/smartplug/smartplug/internal/core"
	"github.com/smartplug/smartplug/internal/hardware"
	mqttutil "github.com/smartplug/smartplug/internal/mqtt"
)

// SensorPublisher publishes local sensor readings to MQTT topics.
// This is used by sensor nodes in distributed mode.
type SensorPublisher struct {
	mu sync.RWMutex

	client       mqtt.Client
	topics       *mqttutil.Topics
	nodeID       string
	sensors      *hardware.SensorManager
	flowMeter    *hardware.FlowMeter
	version      string
	publishRate  time.Duration

	startTime    time.Time
	stopChan     chan struct{}
}

// NewSensorPublisher creates a new SensorPublisher.
func NewSensorPublisher(
	client mqtt.Client,
	topics *mqttutil.Topics,
	nodeID string,
	sensors *hardware.SensorManager,
	flowMeter *hardware.FlowMeter,
	version string,
) *SensorPublisher {
	return &SensorPublisher{
		client:      client,
		topics:      topics,
		nodeID:      nodeID,
		sensors:     sensors,
		flowMeter:   flowMeter,
		version:     version,
		publishRate: 2 * time.Second,
		stopChan:    make(chan struct{}),
	}
}

// SetPublishRate sets how often sensor data is published.
func (p *SensorPublisher) SetPublishRate(rate time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publishRate = rate
}

// Start begins publishing sensor data.
func (p *SensorPublisher) Start() error {
	p.startTime = time.Now()

	// Register for sensor callbacks to publish immediately on change
	p.sensors.OnReading(func(hot, ret hardware.TemperatureReading) {
		p.publishSensorData()
	})

	// Register for flow meter callbacks
	if p.flowMeter != nil {
		p.flowMeter.OnDemand(func(active bool) {
			p.publishFlowData()
		})
		p.flowMeter.OnFlowEvent(func(event hardware.FlowEvent) {
			p.publishFlowEvent(event)
		})
	}

	// Publish availability
	p.publishAvailability(true)

	// Start periodic publishing
	go p.publishLoop()

	// Start heartbeat publishing
	go p.heartbeatLoop()

	log.Printf("Sensor publisher started for node: %s", p.nodeID)
	return nil
}

// Stop halts publishing and sends offline status.
func (p *SensorPublisher) Stop() {
	close(p.stopChan)
	p.publishAvailability(false)
	log.Printf("Sensor publisher stopped for node: %s", p.nodeID)
}

// publishLoop periodically publishes sensor and flow data.
func (p *SensorPublisher) publishLoop() {
	p.mu.RLock()
	rate := p.publishRate
	p.mu.RUnlock()

	ticker := time.NewTicker(rate)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			p.publishSensorData()
			if p.flowMeter != nil {
				p.publishFlowData()
			}

		case <-p.stopChan:
			return
		}
	}
}

// heartbeatLoop periodically publishes heartbeat messages.
func (p *SensorPublisher) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Publish initial heartbeat
	p.publishHeartbeat()

	for {
		select {
		case <-ticker.C:
			p.publishHeartbeat()

		case <-p.stopChan:
			return
		}
	}
}

// publishSensorData publishes current temperature readings.
func (p *SensorPublisher) publishSensorData() {
	hot, ret := p.sensors.GetCurrentReadings()
	hotID, retID := p.sensors.GetSensorIDs()

	data := core.SensorData{
		NodeID:         p.nodeID,
		HotOutlet:      hot.Temperature,
		ReturnLine:     ret.Temperature,
		HotValid:       hot.Valid,
		ReturnValid:    ret.Valid,
		HotSensorID:    hotID,
		ReturnSensorID: retID,
		Timestamp:      time.Now(),
	}

	payload, err := json.Marshal(data)
	if err != nil {
		log.Printf("Failed to marshal sensor data: %v", err)
		return
	}

	topic := p.topics.SensorData(p.nodeID)
	token := p.client.Publish(topic, 0, false, payload)
	token.Wait()
}

// publishFlowData publishes current flow meter status.
func (p *SensorPublisher) publishFlowData() {
	if p.flowMeter == nil {
		return
	}

	stats := p.flowMeter.GetStats()

	data := core.FlowData{
		NodeID:     p.nodeID,
		Active:     stats.IsFlowActive,
		PulseCount: stats.PulseCount,
		Timestamp:  time.Now(),
	}

	payload, err := json.Marshal(data)
	if err != nil {
		log.Printf("Failed to marshal flow data: %v", err)
		return
	}

	topic := p.topics.FlowData(p.nodeID)
	token := p.client.Publish(topic, 0, false, payload)
	token.Wait()
}

// publishFlowEvent publishes a completed flow event.
func (p *SensorPublisher) publishFlowEvent(hwEvent hardware.FlowEvent) {
	event := core.FlowEvent{
		NodeID:     p.nodeID,
		Timestamp:  hwEvent.Timestamp,
		PulseCount: hwEvent.PulseCount,
		FlowRate:   hwEvent.FlowRate,
		Duration:   hwEvent.Duration,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("Failed to marshal flow event: %v", err)
		return
	}

	topic := p.topics.FlowEvent(p.nodeID)
	token := p.client.Publish(topic, 0, false, payload)
	token.Wait()
}

// publishHeartbeat publishes a heartbeat message.
func (p *SensorPublisher) publishHeartbeat() {
	heartbeat := core.NodeHeartbeat{
		NodeID:    p.nodeID,
		Mode:      "sensor",
		Version:   p.version,
		Uptime:    int64(time.Since(p.startTime).Seconds()),
		Timestamp: time.Now(),
	}

	payload, err := json.Marshal(heartbeat)
	if err != nil {
		log.Printf("Failed to marshal heartbeat: %v", err)
		return
	}

	topic := p.topics.Heartbeat(p.nodeID)
	token := p.client.Publish(topic, 0, false, payload)
	token.Wait()
}

// publishAvailability publishes online/offline status.
func (p *SensorPublisher) publishAvailability(online bool) {
	status := "offline"
	if online {
		status = "online"
	}

	topic := p.topics.NodeAvailability(p.nodeID)
	token := p.client.Publish(topic, 0, true, status) // Retained message
	token.Wait()
}

// ActuatorNode receives pump commands and controls a local relay.
// This is used when a smart plug or relay is controlled by a separate device from the controller.
type ActuatorNode struct {
	mu sync.RWMutex

	client    mqtt.Client
	topics    *mqttutil.Topics
	nodeID    string
	relay     *hardware.RelayController
	version   string

	startTime time.Time
	stopChan  chan struct{}
}

// NewActuatorNode creates a new ActuatorNode.
func NewActuatorNode(
	client mqtt.Client,
	topics *mqttutil.Topics,
	nodeID string,
	relay *hardware.RelayController,
	version string,
) *ActuatorNode {
	return &ActuatorNode{
		client:   client,
		topics:   topics,
		nodeID:   nodeID,
		relay:    relay,
		version:  version,
		stopChan: make(chan struct{}),
	}
}

// Start begins listening for pump commands.
func (a *ActuatorNode) Start() error {
	a.startTime = time.Now()

	// Subscribe to pump commands (JSON)
	cmdTopic := a.topics.PumpCommand()
	token := a.client.Subscribe(cmdTopic, 1, a.handlePumpCommand)
	if token.Wait() && token.Error() != nil {
		return token.Error()
	}
	log.Printf("Subscribed to pump commands: %s", cmdTopic)

	// Subscribe to simple commands
	simpleTopic := a.topics.PumpCommandSimple()
	token = a.client.Subscribe(simpleTopic, 1, a.handleSimpleCommand)
	if token.Wait() && token.Error() != nil {
		log.Printf("Warning: failed to subscribe to %s: %v", simpleTopic, token.Error())
	}

	// Register for relay state changes to publish status
	a.relay.OnStateChange(func(state hardware.RelayState) {
		a.publishStatus()
	})

	// Publish availability
	a.publishAvailability(true)

	// Publish initial status
	a.publishStatus()

	// Start heartbeat publishing
	go a.heartbeatLoop()

	log.Printf("Actuator node started: %s", a.nodeID)
	return nil
}

// Stop halts the actuator node and turns off the pump.
func (a *ActuatorNode) Stop() {
	close(a.stopChan)
	a.publishAvailability(false)
	a.relay.TurnOff()
	a.client.Unsubscribe(a.topics.PumpCommand())
	a.client.Unsubscribe(a.topics.PumpCommandSimple())
	log.Printf("Actuator node stopped: %s", a.nodeID)
}

// handlePumpCommand processes JSON pump commands.
func (a *ActuatorNode) handlePumpCommand(client mqtt.Client, msg mqtt.Message) {
	var cmd core.PumpCommand
	if err := json.Unmarshal(msg.Payload(), &cmd); err != nil {
		log.Printf("Failed to parse pump command: %v", err)
		return
	}

	log.Printf("Received pump command: %s (source: %s)", cmd.Command, cmd.Source)

	switch cmd.Command {
	case "on":
		if err := a.relay.TurnOn(); err != nil {
			log.Printf("Failed to turn on pump: %v", err)
		}
	case "off":
		if err := a.relay.TurnOff(); err != nil {
			log.Printf("Failed to turn off pump: %v", err)
		}
	default:
		log.Printf("Unknown pump command: %s", cmd.Command)
	}
}

// handleSimpleCommand processes simple ON/OFF commands.
func (a *ActuatorNode) handleSimpleCommand(client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())
	log.Printf("Received simple pump command: %s", payload)

	switch payload {
	case "ON":
		if err := a.relay.TurnOn(); err != nil {
			log.Printf("Failed to turn on pump: %v", err)
		}
	case "OFF":
		if err := a.relay.TurnOff(); err != nil {
			log.Printf("Failed to turn off pump: %v", err)
		}
	default:
		log.Printf("Unknown simple command: %s", payload)
	}
}

// publishStatus publishes current pump status.
func (a *ActuatorNode) publishStatus() {
	status := core.PumpStatus{
		NodeID:    a.nodeID,
		IsOn:      a.relay.IsOn(),
		LastOn:    a.relay.GetLastOnTime(),
		LastOff:   a.relay.GetLastOffTime(),
		Timestamp: time.Now(),
	}

	payload, err := json.Marshal(status)
	if err != nil {
		log.Printf("Failed to marshal pump status: %v", err)
		return
	}

	// Publish JSON status
	topic := a.topics.PumpStatus()
	token := a.client.Publish(topic, 0, false, payload)
	token.Wait()

	// Also publish simple state for HA compatibility
	state := "OFF"
	if status.IsOn {
		state = "ON"
	}
	stateTopic := a.topics.PumpState()
	token = a.client.Publish(stateTopic, 0, false, state)
	token.Wait()
}

// heartbeatLoop periodically publishes heartbeat messages.
func (a *ActuatorNode) heartbeatLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Publish initial heartbeat
	a.publishHeartbeat()

	for {
		select {
		case <-ticker.C:
			a.publishHeartbeat()
			a.publishStatus()

		case <-a.stopChan:
			return
		}
	}
}

// publishHeartbeat publishes a heartbeat message.
func (a *ActuatorNode) publishHeartbeat() {
	heartbeat := core.NodeHeartbeat{
		NodeID:    a.nodeID,
		Mode:      "actuator",
		Version:   a.version,
		Uptime:    int64(time.Since(a.startTime).Seconds()),
		Timestamp: time.Now(),
	}

	payload, err := json.Marshal(heartbeat)
	if err != nil {
		log.Printf("Failed to marshal heartbeat: %v", err)
		return
	}

	topic := a.topics.Heartbeat(a.nodeID)
	token := a.client.Publish(topic, 0, false, payload)
	token.Wait()
}

// publishAvailability publishes online/offline status.
func (a *ActuatorNode) publishAvailability(online bool) {
	status := "offline"
	if online {
		status = "online"
	}

	topic := a.topics.NodeAvailability(a.nodeID)
	token := a.client.Publish(topic, 0, true, status) // Retained message
	token.Wait()
}
