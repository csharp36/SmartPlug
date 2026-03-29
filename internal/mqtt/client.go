// Package mqtt implements MQTT client for Home Assistant integration
package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/smartplug/smartplug/internal/config"
	"github.com/smartplug/smartplug/internal/controller"
	"github.com/smartplug/smartplug/internal/hardware"
)

// Client manages MQTT connections and messaging
type Client struct {
	mu sync.RWMutex

	client      mqtt.Client
	cfg         *config.MQTTConfig
	connected   bool

	// References for publishing state
	pump     *controller.PumpController
	sensors  *hardware.SensorManager
	flowMeter *hardware.FlowMeter

	// Command handlers
	commandHandler func(command string) error

	stopChan chan struct{}
}

// NewClient creates a new MQTT client
func NewClient(cfg *config.MQTTConfig) *Client {
	return &Client{
		cfg:      cfg,
		stopChan: make(chan struct{}),
	}
}

// SetReferences sets references to components for state publishing
func (c *Client) SetReferences(pump *controller.PumpController, sensors *hardware.SensorManager, flowMeter *hardware.FlowMeter) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pump = pump
	c.sensors = sensors
	c.flowMeter = flowMeter
}

// SetCommandHandler sets the handler for pump commands
func (c *Client) SetCommandHandler(handler func(command string) error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.commandHandler = handler
}

// Connect establishes connection to the MQTT broker
func (c *Client) Connect() error {
	if !c.cfg.Enabled {
		log.Println("MQTT disabled")
		return nil
	}

	opts := mqtt.NewClientOptions()
	opts.AddBroker(c.cfg.Broker)

	if c.cfg.ClientID != "" {
		opts.SetClientID(c.cfg.ClientID)
	} else {
		opts.SetClientID(fmt.Sprintf("smartplug-%d", time.Now().UnixNano()))
	}

	if c.cfg.Username != "" {
		opts.SetUsername(c.cfg.Username)
		opts.SetPassword(c.cfg.Password)
	}

	opts.SetAutoReconnect(true)
	opts.SetConnectTimeout(5 * time.Second)

	opts.SetOnConnectHandler(c.onConnect)
	opts.SetConnectionLostHandler(c.onConnectionLost)

	c.client = mqtt.NewClient(opts)

	// Connect asynchronously - don't block startup
	go func() {
		token := c.client.Connect()
		if token.WaitTimeout(10 * time.Second) {
			if token.Error() != nil {
				log.Printf("MQTT initial connection failed: %v (will retry)", token.Error())
			}
		} else {
			log.Printf("MQTT connection timeout (will retry in background)")
		}
	}()

	return nil
}

// Disconnect closes the MQTT connection
func (c *Client) Disconnect() {
	close(c.stopChan)

	if c.client != nil && c.client.IsConnected() {
		// Publish offline availability
		c.publishAvailability(false)
		c.client.Disconnect(1000)
	}
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// Start begins MQTT operations
func (c *Client) Start() error {
	if err := c.Connect(); err != nil {
		return err
	}

	// Start state publishing loop
	go c.publishLoop()

	return nil
}

// onConnect handles successful connection
func (c *Client) onConnect(client mqtt.Client) {
	c.mu.Lock()
	c.connected = true
	c.mu.Unlock()

	log.Println("MQTT connected")

	// Publish Home Assistant discovery
	if c.cfg.HADiscovery {
		c.publishDiscovery()
	}

	// Subscribe to command topics
	c.subscribeToCommands()

	// Publish initial state
	c.publishAvailability(true)
	c.publishState()
}

// onConnectionLost handles connection loss
func (c *Client) onConnectionLost(client mqtt.Client, err error) {
	c.mu.Lock()
	c.connected = false
	c.mu.Unlock()

	log.Printf("MQTT connection lost: %v", err)
}

// subscribeToCommands subscribes to command topics
func (c *Client) subscribeToCommands() {
	commandTopic := fmt.Sprintf("%s/pump/set", c.cfg.TopicPrefix)

	token := c.client.Subscribe(commandTopic, 1, c.handleCommand)
	if token.Wait() && token.Error() != nil {
		log.Printf("MQTT subscribe error: %v", token.Error())
	}
}

// handleCommand processes incoming commands
func (c *Client) handleCommand(client mqtt.Client, msg mqtt.Message) {
	payload := string(msg.Payload())
	log.Printf("MQTT command received: %s", payload)

	c.mu.RLock()
	handler := c.commandHandler
	c.mu.RUnlock()

	if handler != nil {
		if err := handler(payload); err != nil {
			log.Printf("Command handler error: %v", err)
		}
	}
}

// publishLoop periodically publishes state
func (c *Client) publishLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if c.IsConnected() {
				c.publishState()
			}

		case <-c.stopChan:
			return
		}
	}
}

// publishState publishes current state to MQTT
func (c *Client) publishState() {
	c.mu.RLock()
	pump := c.pump
	sensors := c.sensors
	flowMeter := c.flowMeter
	c.mu.RUnlock()

	if pump == nil || sensors == nil {
		return
	}

	// Publish pump state
	pumpState := "OFF"
	if pump.GetState() == controller.StateHeating {
		pumpState = "ON"
	}
	c.publish("pump/state", pumpState)

	// Publish temperatures
	hot, ret := sensors.GetCurrentReadings()
	if hot.Valid {
		c.publish("temperature/hot", fmt.Sprintf("%.1f", hot.Temperature))
	}
	if ret.Valid {
		c.publish("temperature/return", fmt.Sprintf("%.1f", ret.Temperature))
	}
	if hot.Valid && ret.Valid {
		c.publish("temperature/differential", fmt.Sprintf("%.1f", hot.Temperature-ret.Temperature))
	}

	// Publish flow meter state
	if flowMeter != nil {
		flowActive := "OFF"
		if flowMeter.IsFlowActive() {
			flowActive = "ON"
		}
		c.publish("flow/active", flowActive)
	}

	// Publish controller state
	status := pump.GetStatus()
	c.publish("controller/state", status.State.String())
	c.publish("controller/enabled", fmt.Sprintf("%t", status.Enabled))

	// Publish JSON state
	stateJSON, _ := json.Marshal(map[string]interface{}{
		"pump":        pumpState,
		"state":       status.State.String(),
		"enabled":     status.Enabled,
		"hot_temp":    hot.Temperature,
		"return_temp": ret.Temperature,
		"flow_active": flowMeter != nil && flowMeter.IsFlowActive(),
	})
	c.publish("state", string(stateJSON))
}

// publishAvailability publishes availability status
func (c *Client) publishAvailability(online bool) {
	status := "offline"
	if online {
		status = "online"
	}
	c.publish("availability", status)
}

// publish sends a message to a topic
func (c *Client) publish(topic, payload string) {
	fullTopic := fmt.Sprintf("%s/%s", c.cfg.TopicPrefix, topic)
	token := c.client.Publish(fullTopic, 0, false, payload)
	token.Wait()
}

// publishRetained sends a retained message
func (c *Client) publishRetained(topic, payload string) {
	fullTopic := fmt.Sprintf("%s/%s", c.cfg.TopicPrefix, topic)
	token := c.client.Publish(fullTopic, 0, true, payload)
	token.Wait()
}

// PublishEvent publishes a pump event
func (c *Client) PublishEvent(event controller.PumpEvent) {
	eventJSON, _ := json.Marshal(map[string]interface{}{
		"timestamp":    event.Timestamp.Format(time.RFC3339),
		"trigger":      event.Trigger.String(),
		"duration":     event.Duration.Seconds(),
		"hot_temp":     event.HotTemp,
		"return_temp":  event.ReturnTemp,
		"differential": event.Differential,
	})
	c.publish("event", string(eventJSON))
}

// PublishFlowEvent publishes a flow event
func (c *Client) PublishFlowEvent(event hardware.FlowEvent) {
	eventJSON, _ := json.Marshal(map[string]interface{}{
		"timestamp":   event.Timestamp.Format(time.RFC3339),
		"pulse_count": event.PulseCount,
		"flow_rate":   event.FlowRate,
		"duration":    event.Duration.Seconds(),
	})
	c.publish("flow/event", string(eventJSON))
}
