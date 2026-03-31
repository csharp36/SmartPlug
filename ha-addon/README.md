# SmartPlug - Home Assistant Add-on

This directory contains the Home Assistant add-on for running SmartPlug Controller on Home Assistant (including Home Assistant Green).

## What This Add-on Does

The SmartPlug Controller add-on:
- Receives temperature and flow sensor data from remote sensor nodes via MQTT
- Runs the pump control logic (temperature thresholds, cooldowns, safety checks)
- Sends pump commands via MQTT to smart plugs
- Provides a web UI on port 8080
- Integrates with Home Assistant via MQTT auto-discovery

## Requirements

- Home Assistant with Mosquitto broker add-on installed
- A sensor node (Raspberry Pi) running `smartplug-sensor` at your water heater
- A WiFi smart plug controllable via MQTT (or HA automation bridge)

## Installation

### From Repository (Recommended)

1. Add this repository to Home Assistant:
   - Go to **Settings** → **Add-ons** → **Add-on Store**
   - Click the three dots (⋮) → **Repositories**
   - Add: `https://github.com/smartplug/smartplug`

2. Install "SmartPlug Controller" from the add-on store

3. Configure the add-on options (see below)

4. Start the add-on

### Manual Installation

1. Copy this `ha-addon` directory to `/addons/smartplug` on your HA instance

2. Go to **Settings** → **Add-ons** → **Add-on Store**

3. Click the three dots (⋮) → **Check for updates**

4. Find "SmartPlug Controller" under "Local add-ons"

## Configuration

```yaml
sensor_node_ids:
  - "sensor-kitchen"      # Must match node_id on your sensor Pi
actuator_type: "mqtt"     # "mqtt" for smart plug control
data_timeout: 30          # Seconds before sensor data is stale
start_threshold: 12.0     # Start pump when temp diff >= this
stop_threshold: 8.0       # Stop pump when temp diff <= this
max_runtime_minutes: 15   # Max pump runtime per cycle
cooldown_minutes: 5       # Wait time between cycles
schedule_enabled: true    # Enable time-based scheduling
ha_discovery: true        # Create HA entities automatically
log_level: "info"         # debug, info, warn, error
```

## MQTT Topics

The add-on automatically connects to the Mosquitto broker add-on.

### Subscribed Topics
- `smartplug/{node_id}/sensors/data` - Temperature readings from sensor nodes
- `smartplug/{node_id}/flow/data` - Flow meter state from sensor nodes
- `smartplug/pump/status` - Pump status feedback

### Published Topics
- `smartplug/pump/command` - Pump on/off commands

## Smart Plug Integration

The add-on publishes commands to `smartplug/pump/command`. You need to bridge this to your actual smart plug using a Home Assistant automation:

```yaml
automation:
  - alias: "SmartPlug Pump Command Bridge"
    trigger:
      - platform: mqtt
        topic: "smartplug/pump/command"
    action:
      - choose:
          - conditions:
              - condition: template
                value_template: "{{ trigger.payload_json.command == 'on' }}"
            sequence:
              - service: switch.turn_on
                target:
                  entity_id: switch.pump_smart_plug
          - conditions:
              - condition: template
                value_template: "{{ trigger.payload_json.command == 'off' }}"
            sequence:
              - service: switch.turn_off
                target:
                  entity_id: switch.pump_smart_plug
```

## Building Locally

From the repository root:

```bash
# Build for arm64 (HA Green, Pi 4)
make docker-build-arm64

# Build for amd64
make docker-build-amd64

# Build and push multi-arch (requires docker login)
make docker-push
```

## Troubleshooting

### Add-on won't start
- Check that Mosquitto broker add-on is running
- Verify sensor_node_ids matches your sensor node's node_id

### No sensor data
- Ensure sensor node is publishing to the correct MQTT broker
- Check topic_prefix matches on both sensor and controller
- Use MQTT Explorer to verify messages are flowing

### Pump not responding
- Verify your smart plug integration automation is working
- Check the command topic matches what your automation listens to
- Test manually: `mosquitto_pub -t smartplug/pump/command -m '{"command":"on"}'`
