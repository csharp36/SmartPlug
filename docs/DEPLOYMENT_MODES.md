# SmartPlug Deployment Modes

SmartPlug supports three deployment modes to accommodate different hardware configurations and installation requirements.

## Overview

| Mode | Use Case | Components |
|------|----------|------------|
| **all-in-one** | Single Pi with all hardware connected | Sensors, flow meter, relay on same device |
| **sensor** | Distributed sensor node | Only sensors and flow meter, publishes via MQTT |
| **controller** | Centralized control | Receives sensor data via MQTT, controls pump |

## Architecture Diagrams

### All-in-One Mode (Default)

```
┌─────────────────────────────────────────────────────┐
│                  Raspberry Pi                        │
│                                                      │
│  DS18B20 ──┬──► SensorManager ──► PumpController    │
│  DS18B20 ──┘                            │           │
│                                         │           │
│  FlowMeter ──────────────────────────►──┘           │
│                                         │           │
│                               RelayController ──► Pump
│                                                      │
│  Web UI ◄──► API Server                             │
└─────────────────────────────────────────────────────┘
```

This is the original deployment mode where all components (temperature sensors, flow meter, and relay) are connected to a single Raspberry Pi.

### Distributed Mode

```
┌──────────────────────┐        MQTT        ┌──────────────────────┐
│   Sensor Node (Pi)   │ ────────────────► │      Controller      │
│                      │                    │   (HA Add-on / Pi)   │
│  DS18B20 ──► MQTT    │  sensor data       │                      │
│  DS18B20 ──┘ Publish │                    │  MQTT ──► PumpCtrl   │
│                      │  flow data         │    Sub       │       │
│  FlowMeter ──────►───┤                    │              ▼       │
└──────────────────────┘                    │         MQTT Pub     │
                                            │              │       │
                                            │  Web UI ◄─── API     │
                                            └──────────────┼───────┘
                                                           │
                                                     MQTT  │ cmd
                                                           ▼
                                            ┌──────────────────────┐
                                            │     Smart Plug       │
                                            │   (WiFi-enabled)     │
                                            │                      │
                                            │  e.g., Kasa, Tapo,   │
                                            │  Shelly, Zigbee      │
                                            └──────────────────────┘
```

Distributed mode separates the sensor node from the controller, communicating via MQTT. This is ideal when:

- Sensors are in a different location than the controller (e.g., sensors at water heater, controller in network closet)
- Using a WiFi smart plug instead of a wired relay
- Running the controller on Home Assistant
- Multiple sensor nodes monitoring different zones

## Mode: all-in-one

The default mode that matches the original SmartPlug behavior.

### When to Use

- Single Raspberry Pi with direct GPIO access to all hardware
- Sensors, flow meter, and relay all connected to the same device
- Simplest setup with minimal network dependencies

### Configuration

```yaml
# /etc/smartplug/smartplug.yaml

deployment:
  mode: all-in-one
  # No additional settings needed

hardware:
  onewire_gpio: 4      # DS18B20 sensors
  relay_gpio: 17       # Pump relay
  flowmeter_gpio: 27   # Flow meter

# ... rest of config
```

### Binary

Use the main `smartplug` binary:

```bash
smartplug --config /etc/smartplug/smartplug.yaml
```

## Mode: sensor

A lightweight sensor node that reads hardware and publishes to MQTT.

### When to Use

- Sensors are physically distant from the controller
- You want to run the controller on a more capable device (server, Home Assistant)
- Multiple sensor locations feeding into one controller

### What It Does

- Reads DS18B20 temperature sensors via 1-Wire
- Monitors flow meter for demand detection
- Publishes sensor data and flow events to MQTT
- Sends periodic heartbeats
- **No pump control logic or web UI**

### Configuration

```yaml
# /etc/smartplug/smartplug.yaml (on sensor node)

deployment:
  mode: sensor
  node_id: "sensor-kitchen"    # Unique identifier for this node

hardware:
  onewire_gpio: 4
  flowmeter_gpio: 27

sensors:
  poll_interval: 2

flowmeter:
  enabled: true
  pulses_per_liter: 450
  trigger_threshold: 3
  demand_timeout: 30

mqtt:
  enabled: true
  broker: "tcp://192.168.1.100:1883"
  topic_prefix: "smartplug"
```

### Binary

Use the dedicated sensor binary:

```bash
smartplug-sensor --config /etc/smartplug/smartplug.yaml
```

Or use the main binary which auto-detects mode from config:

```bash
smartplug --config /etc/smartplug/smartplug.yaml
```

### MQTT Topics Published

| Topic | Payload | Frequency |
|-------|---------|-----------|
| `smartplug/{node_id}/sensors/data` | SensorData JSON | Every poll interval |
| `smartplug/{node_id}/flow/data` | FlowData JSON | On flow state change |
| `smartplug/{node_id}/flow/event` | FlowEvent JSON | When flow event completes |
| `smartplug/{node_id}/heartbeat` | NodeHeartbeat JSON | Every 30 seconds |

## Mode: controller

The brain of the system - receives sensor data and controls the pump.

### When to Use

- Running on Home Assistant (as add-on or standalone)
- Controlling a WiFi smart plug via MQTT
- Centralizing control from multiple sensor nodes
- Hardware with no GPIO access

### What It Does

- Subscribes to sensor data from remote nodes via MQTT
- Runs the PumpController logic (thresholds, cooldowns, safety checks)
- Sends pump commands via MQTT (for smart plugs)
- Provides web UI and REST API
- Runs scheduling and learning algorithms

### Configuration

```yaml
# /etc/smartplug/smartplug.yaml (on controller)

deployment:
  mode: controller
  sensor_node_ids:
    - "sensor-kitchen"        # Node IDs to subscribe to
  actuator_type: mqtt         # "mqtt" for smart plug, "local" for GPIO relay
  data_timeout: 30            # Seconds before sensor data is stale

pump:
  start_threshold: 12.0
  stop_threshold: 8.0
  max_runtime_minutes: 15
  cooldown_minutes: 5

schedule:
  enabled: true
  slots:
    - start: "06:00"
      end: "09:00"
      days: [0, 1, 2, 3, 4, 5, 6]
      enabled: true

web:
  address: ":8080"

mqtt:
  enabled: true
  broker: "tcp://localhost:1883"    # Often localhost for HA
  topic_prefix: "smartplug"
  ha_discovery: true
```

### Binary

Use the dedicated controller binary:

```bash
smartplug-controller --config /etc/smartplug/smartplug.yaml
```

Or use the main binary:

```bash
smartplug --config /etc/smartplug/smartplug.yaml
```

### MQTT Topics

| Topic | Direction | Purpose |
|-------|-----------|---------|
| `smartplug/{node_id}/sensors/data` | Subscribe | Receive sensor readings |
| `smartplug/{node_id}/flow/data` | Subscribe | Receive flow state |
| `smartplug/pump/command` | Publish | Send on/off commands |
| `smartplug/pump/status` | Subscribe | Receive pump status |

### Controller with Local Relay

If the controller device has GPIO access (e.g., running on a Pi in a different location), you can use a local relay instead of MQTT:

```yaml
deployment:
  mode: controller
  sensor_node_ids:
    - "sensor-kitchen"
  actuator_type: local         # Use GPIO relay, not MQTT

hardware:
  relay_gpio: 17               # Local relay pin
```

## MQTT Message Formats

### SensorData

```json
{
  "node_id": "sensor-kitchen",
  "hot_outlet": 120.5,
  "return_line": 95.2,
  "hot_valid": true,
  "return_valid": true,
  "hot_sensor_id": "28-0123456789ab",
  "return_sensor_id": "28-abcdef012345",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### FlowData

```json
{
  "node_id": "sensor-kitchen",
  "active": true,
  "pulse_count": 150,
  "flow_rate": 2.5,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### PumpCommand

```json
{
  "command": "on",
  "source": "schedule",
  "request_id": "req-abc123",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### PumpStatus

```json
{
  "node_id": "actuator-1",
  "is_on": true,
  "last_on": "2024-01-15T10:00:00Z",
  "last_off": "2024-01-15T09:45:00Z",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### NodeHeartbeat

```json
{
  "node_id": "sensor-kitchen",
  "mode": "sensor",
  "version": "1.0.0",
  "uptime_seconds": 3600,
  "timestamp": "2024-01-15T10:30:00Z"
}
```

## Smart Plug Integration

For WiFi smart plugs, you'll need a bridge that:
1. Subscribes to `smartplug/pump/command`
2. Translates commands to your smart plug's protocol
3. Publishes status to `smartplug/pump/status`

### Example: Kasa Smart Plug with python-kasa

```python
# Simple bridge example (not production-ready)
import asyncio
import json
from kasa import SmartPlug
import paho.mqtt.client as mqtt

plug = SmartPlug("192.168.1.50")

def on_command(client, userdata, msg):
    cmd = json.loads(msg.payload)
    asyncio.run(plug.update())
    if cmd["command"] == "on":
        asyncio.run(plug.turn_on())
    else:
        asyncio.run(plug.turn_off())

client = mqtt.Client()
client.connect("localhost", 1883)
client.subscribe("smartplug/pump/command")
client.on_message = on_command
client.loop_forever()
```

### Home Assistant MQTT Switch

If your smart plug is already in Home Assistant, create an MQTT switch that bridges commands:

```yaml
# configuration.yaml
mqtt:
  switch:
    - name: "SmartPlug Pump Bridge"
      command_topic: "smartplug/pump/command"
      state_topic: "smartplug/pump/status"
      value_template: "{{ value_json.is_on }}"
      payload_on: '{"command":"on","source":"ha"}'
      payload_off: '{"command":"off","source":"ha"}'
```

## Migration Guide

### From All-in-One to Distributed

1. On the existing Pi (becomes sensor node):
   - Update config to `mode: sensor`
   - Remove pump/schedule/web sections
   - Set `node_id` to a unique name
   - Ensure MQTT broker address is correct

2. On the new controller device:
   - Install `smartplug-controller`
   - Configure with `mode: controller`
   - List the sensor node in `sensor_node_ids`
   - Set up smart plug integration or local relay

3. Test MQTT connectivity:
   ```bash
   # On controller, subscribe to sensor topics
   mosquitto_sub -h broker -t "smartplug/+/sensors/data" -v

   # Verify data appears from sensor node
   ```

## Troubleshooting

### Sensor Node Not Publishing

1. Check MQTT connection: `mosquitto_sub -h broker -t "#" -v`
2. Verify node_id is set in config
3. Check sensor node logs: `journalctl -u smartplug -f`

### Controller Not Receiving Data

1. Verify sensor_node_ids matches the sensor's node_id
2. Check MQTT broker is reachable from both devices
3. Ensure topic_prefix matches on both ends

### Stale Data Warnings

The controller marks sensor data as stale after `data_timeout` seconds:

```yaml
deployment:
  data_timeout: 30    # Increase if network is slow
```

### Smart Plug Not Responding

1. Check MQTT bridge is running and subscribed
2. Verify command topic matches
3. Test with manual MQTT publish:
   ```bash
   mosquitto_pub -h broker -t "smartplug/pump/command" \
     -m '{"command":"on","source":"test"}'
   ```
