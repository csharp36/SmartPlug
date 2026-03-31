package mqtt

import (
	"fmt"
	"strings"
)

// Topic patterns for distributed SmartPlug communication.
// The {node_id} placeholder is replaced with the actual node identifier.
const (
	// TopicPatternSensorData is the topic for temperature sensor data from a sensor node.
	// Published by: sensor node
	// Subscribed by: controller
	// Payload: core.SensorData JSON
	TopicPatternSensorData = "smartplug/%s/sensors/data"

	// TopicPatternFlowData is the topic for flow meter data from a sensor node.
	// Published by: sensor node
	// Subscribed by: controller
	// Payload: core.FlowData JSON
	TopicPatternFlowData = "smartplug/%s/flow/data"

	// TopicPatternFlowEvent is the topic for completed flow events from a sensor node.
	// Published by: sensor node
	// Subscribed by: controller (for logging/analytics)
	// Payload: core.FlowEvent JSON
	TopicPatternFlowEvent = "smartplug/%s/flow/event"

	// TopicPatternHeartbeat is the topic for node heartbeat messages.
	// Published by: any node (sensor, controller, actuator)
	// Subscribed by: controller (for monitoring)
	// Payload: core.NodeHeartbeat JSON
	TopicPatternHeartbeat = "smartplug/%s/heartbeat"

	// TopicPumpCommand is the topic for pump control commands.
	// Published by: controller
	// Subscribed by: actuator node (or smart plug bridge)
	// Payload: core.PumpCommand JSON
	TopicPumpCommand = "smartplug/pump/command"

	// TopicPumpCommandSimple is a simplified command topic for basic on/off commands.
	// Published by: controller, Home Assistant, etc.
	// Subscribed by: actuator node
	// Payload: "ON" or "OFF" string
	TopicPumpCommandSimple = "smartplug/pump/set"

	// TopicPumpStatus is the topic for pump actuator status updates.
	// Published by: actuator node
	// Subscribed by: controller
	// Payload: core.PumpStatus JSON
	TopicPumpStatus = "smartplug/pump/status"

	// TopicPumpState is the topic for simple pump state (for Home Assistant compatibility).
	// Published by: actuator node or controller
	// Payload: "ON" or "OFF" string
	TopicPumpState = "smartplug/pump/state"

	// TopicControllerState is the topic for the overall controller state.
	// Published by: controller
	// Payload: JSON with state, enabled, temperatures, etc.
	TopicControllerState = "smartplug/controller/state"

	// TopicAvailability is the topic for node availability (online/offline).
	// Published by: any node
	// Payload: "online" or "offline"
	TopicAvailability = "smartplug/availability"

	// TopicPatternNodeAvailability is the topic for per-node availability.
	// Published by: specific node
	// Payload: "online" or "offline"
	TopicPatternNodeAvailability = "smartplug/%s/availability"
)

// Topics provides helper methods for generating MQTT topic strings.
type Topics struct {
	// Prefix is the base topic prefix (default: "smartplug")
	Prefix string
}

// NewTopics creates a new Topics helper with the given prefix.
func NewTopics(prefix string) *Topics {
	if prefix == "" {
		prefix = "smartplug"
	}
	return &Topics{Prefix: prefix}
}

// SensorData returns the topic for sensor data from a specific node.
func (t *Topics) SensorData(nodeID string) string {
	return t.format(TopicPatternSensorData, nodeID)
}

// SensorDataWildcard returns a wildcard topic to subscribe to all sensor data.
func (t *Topics) SensorDataWildcard() string {
	return t.format(TopicPatternSensorData, "+")
}

// FlowData returns the topic for flow data from a specific node.
func (t *Topics) FlowData(nodeID string) string {
	return t.format(TopicPatternFlowData, nodeID)
}

// FlowDataWildcard returns a wildcard topic to subscribe to all flow data.
func (t *Topics) FlowDataWildcard() string {
	return t.format(TopicPatternFlowData, "+")
}

// FlowEvent returns the topic for flow events from a specific node.
func (t *Topics) FlowEvent(nodeID string) string {
	return t.format(TopicPatternFlowEvent, nodeID)
}

// FlowEventWildcard returns a wildcard topic to subscribe to all flow events.
func (t *Topics) FlowEventWildcard() string {
	return t.format(TopicPatternFlowEvent, "+")
}

// Heartbeat returns the topic for heartbeat from a specific node.
func (t *Topics) Heartbeat(nodeID string) string {
	return t.format(TopicPatternHeartbeat, nodeID)
}

// HeartbeatWildcard returns a wildcard topic to subscribe to all heartbeats.
func (t *Topics) HeartbeatWildcard() string {
	return t.format(TopicPatternHeartbeat, "+")
}

// PumpCommand returns the topic for pump commands.
func (t *Topics) PumpCommand() string {
	return t.applyPrefix(TopicPumpCommand)
}

// PumpCommandSimple returns the simplified command topic.
func (t *Topics) PumpCommandSimple() string {
	return t.applyPrefix(TopicPumpCommandSimple)
}

// PumpStatus returns the topic for pump status updates.
func (t *Topics) PumpStatus() string {
	return t.applyPrefix(TopicPumpStatus)
}

// PumpState returns the topic for simple pump state.
func (t *Topics) PumpState() string {
	return t.applyPrefix(TopicPumpState)
}

// ControllerState returns the topic for controller state.
func (t *Topics) ControllerState() string {
	return t.applyPrefix(TopicControllerState)
}

// Availability returns the general availability topic.
func (t *Topics) Availability() string {
	return t.applyPrefix(TopicAvailability)
}

// NodeAvailability returns the availability topic for a specific node.
func (t *Topics) NodeAvailability(nodeID string) string {
	return t.format(TopicPatternNodeAvailability, nodeID)
}

// NodeAvailabilityWildcard returns a wildcard topic for all node availability.
func (t *Topics) NodeAvailabilityWildcard() string {
	return t.format(TopicPatternNodeAvailability, "+")
}

// format applies the prefix and formats the topic pattern.
func (t *Topics) format(pattern, nodeID string) string {
	topic := fmt.Sprintf(pattern, nodeID)
	return t.applyPrefix(topic)
}

// applyPrefix replaces the default "smartplug" prefix with the configured prefix.
func (t *Topics) applyPrefix(topic string) string {
	if t.Prefix == "smartplug" {
		return topic
	}
	return strings.Replace(topic, "smartplug", t.Prefix, 1)
}

// ExtractNodeID extracts the node ID from a topic string.
// Returns empty string if the topic doesn't match the expected pattern.
func (t *Topics) ExtractNodeID(topic string) string {
	// Remove the prefix
	topic = strings.TrimPrefix(topic, t.Prefix+"/")

	// Split by /
	parts := strings.Split(topic, "/")
	if len(parts) < 2 {
		return ""
	}

	// The node ID is the first part after the prefix
	return parts[0]
}

// DefaultTopics returns a Topics helper with the default "smartplug" prefix.
func DefaultTopics() *Topics {
	return NewTopics("smartplug")
}
