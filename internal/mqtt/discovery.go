package mqtt

import (
	"encoding/json"
	"fmt"
	"log"
)

// DiscoveryConfig contains Home Assistant MQTT discovery configuration
type DiscoveryConfig struct {
	Name              string   `json:"name"`
	UniqueID          string   `json:"unique_id"`
	StateTopic        string   `json:"state_topic,omitempty"`
	CommandTopic      string   `json:"command_topic,omitempty"`
	AvailabilityTopic string   `json:"availability_topic,omitempty"`
	PayloadOn         string   `json:"payload_on,omitempty"`
	PayloadOff        string   `json:"payload_off,omitempty"`
	DeviceClass       string   `json:"device_class,omitempty"`
	UnitOfMeasurement string   `json:"unit_of_measurement,omitempty"`
	ValueTemplate     string   `json:"value_template,omitempty"`
	Icon              string   `json:"icon,omitempty"`
	Device            *Device  `json:"device,omitempty"`
}

// Device contains device information for Home Assistant
type Device struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Model        string   `json:"model"`
	Manufacturer string   `json:"manufacturer"`
	SWVersion    string   `json:"sw_version"`
}

// publishDiscovery publishes Home Assistant MQTT discovery messages
func (c *Client) publishDiscovery() {
	log.Println("Publishing Home Assistant discovery messages")

	device := &Device{
		Identifiers:  []string{"smartplug_hwrc"},
		Name:         "SmartPlug Hot Water Recirculation",
		Model:        "SmartPlug HWRC",
		Manufacturer: "SmartPlug Open Source",
		SWVersion:    "1.0.0",
	}

	prefix := c.cfg.HADiscoveryPrefix
	topicPrefix := c.cfg.TopicPrefix

	// Pump switch
	c.publishDiscoveryConfig(prefix, "switch", "pump", DiscoveryConfig{
		Name:              "Recirculation Pump",
		UniqueID:          "smartplug_pump",
		StateTopic:        fmt.Sprintf("%s/pump/state", topicPrefix),
		CommandTopic:      fmt.Sprintf("%s/pump/set", topicPrefix),
		AvailabilityTopic: fmt.Sprintf("%s/availability", topicPrefix),
		PayloadOn:         "ON",
		PayloadOff:        "OFF",
		Icon:              "mdi:pump",
		Device:            device,
	})

	// Hot water temperature sensor
	c.publishDiscoveryConfig(prefix, "sensor", "temp_hot", DiscoveryConfig{
		Name:              "Hot Water Temperature",
		UniqueID:          "smartplug_temp_hot",
		StateTopic:        fmt.Sprintf("%s/temperature/hot", topicPrefix),
		AvailabilityTopic: fmt.Sprintf("%s/availability", topicPrefix),
		DeviceClass:       "temperature",
		UnitOfMeasurement: "°F",
		Icon:              "mdi:thermometer-high",
		Device:            device,
	})

	// Return line temperature sensor
	c.publishDiscoveryConfig(prefix, "sensor", "temp_return", DiscoveryConfig{
		Name:              "Return Line Temperature",
		UniqueID:          "smartplug_temp_return",
		StateTopic:        fmt.Sprintf("%s/temperature/return", topicPrefix),
		AvailabilityTopic: fmt.Sprintf("%s/availability", topicPrefix),
		DeviceClass:       "temperature",
		UnitOfMeasurement: "°F",
		Icon:              "mdi:thermometer-low",
		Device:            device,
	})

	// Temperature differential sensor
	c.publishDiscoveryConfig(prefix, "sensor", "temp_diff", DiscoveryConfig{
		Name:              "Temperature Differential",
		UniqueID:          "smartplug_temp_diff",
		StateTopic:        fmt.Sprintf("%s/temperature/differential", topicPrefix),
		AvailabilityTopic: fmt.Sprintf("%s/availability", topicPrefix),
		UnitOfMeasurement: "°F",
		Icon:              "mdi:thermometer-lines",
		Device:            device,
	})

	// Flow sensor
	c.publishDiscoveryConfig(prefix, "binary_sensor", "flow", DiscoveryConfig{
		Name:              "Water Flow",
		UniqueID:          "smartplug_flow",
		StateTopic:        fmt.Sprintf("%s/flow/active", topicPrefix),
		AvailabilityTopic: fmt.Sprintf("%s/availability", topicPrefix),
		PayloadOn:         "ON",
		PayloadOff:        "OFF",
		DeviceClass:       "running",
		Icon:              "mdi:water-pump",
		Device:            device,
	})

	// Controller state sensor
	c.publishDiscoveryConfig(prefix, "sensor", "controller_state", DiscoveryConfig{
		Name:              "Controller State",
		UniqueID:          "smartplug_controller_state",
		StateTopic:        fmt.Sprintf("%s/controller/state", topicPrefix),
		AvailabilityTopic: fmt.Sprintf("%s/availability", topicPrefix),
		Icon:              "mdi:state-machine",
		Device:            device,
	})

	// Controller enabled switch
	c.publishDiscoveryConfig(prefix, "binary_sensor", "controller_enabled", DiscoveryConfig{
		Name:              "Controller Enabled",
		UniqueID:          "smartplug_controller_enabled",
		StateTopic:        fmt.Sprintf("%s/controller/enabled", topicPrefix),
		AvailabilityTopic: fmt.Sprintf("%s/availability", topicPrefix),
		PayloadOn:         "true",
		PayloadOff:        "false",
		Icon:              "mdi:power",
		Device:            device,
	})
}

// publishDiscoveryConfig publishes a single discovery config
func (c *Client) publishDiscoveryConfig(prefix, component, objectID string, config DiscoveryConfig) {
	topic := fmt.Sprintf("%s/%s/smartplug_%s/config", prefix, component, objectID)

	payload, err := json.Marshal(config)
	if err != nil {
		log.Printf("Failed to marshal discovery config: %v", err)
		return
	}

	token := c.client.Publish(topic, 0, true, payload)
	if token.Wait() && token.Error() != nil {
		log.Printf("Failed to publish discovery: %v", token.Error())
	}
}

// RemoveDiscovery removes all discovery entries
func (c *Client) RemoveDiscovery() {
	prefix := c.cfg.HADiscoveryPrefix

	// Send empty payload to remove discovery entries
	components := []struct {
		component string
		objectID  string
	}{
		{"switch", "pump"},
		{"sensor", "temp_hot"},
		{"sensor", "temp_return"},
		{"sensor", "temp_diff"},
		{"binary_sensor", "flow"},
		{"sensor", "controller_state"},
		{"binary_sensor", "controller_enabled"},
	}

	for _, comp := range components {
		topic := fmt.Sprintf("%s/%s/smartplug_%s/config", prefix, comp.component, comp.objectID)
		c.client.Publish(topic, 0, true, "")
	}

	log.Println("Removed Home Assistant discovery entries")
}
