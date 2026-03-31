# SmartPlug - Open Source Hot Water Recirculation Controller

An open-source, sub-$60 alternative to commercial hot water recirculation controllers like the Leridian SRC32 ($250-505). SmartPlug controls a recirculation pump (e.g., Taco 006-B4) to provide "instant on" hot water while minimizing energy usage.

## Features

- **Flow Meter Demand Detection** - Instantly detects when any hot faucet opens
- **Dual Temperature Monitoring** - Tracks hot outlet and return line temperatures
- **Adaptive Learning** - Learns your routine and creates automatic schedules
- **Manual Scheduling** - Up to 10 programmable time slots
- **Mobile-Responsive Web UI** - PWA that works as a home screen app
- **REST API** - Full API for automation
- **MQTT / Home Assistant** - Native integration with auto-discovery
- **Flexible Deployment** - All-in-one or distributed sensor/controller architecture

## Bill of Materials (~$60)

| Component | Price | Notes |
|-----------|-------|-------|
| Raspberry Pi Zero 2 W | $15 | Quad-core, WiFi, full Linux |
| 2x DS18B20 sensors | $10 | Surface-mount temperature sensors |
| Hall-effect flow meter | $12-15 | 3/4" NPT, demand detection |
| 5V relay module (10A) | $3-5 | Controls pump power |
| 32GB microSD card | $8 | For OS and storage |
| 5V/2.5A power supply | $8 | Powers Pi |
| 4.7kΩ resistor | $0.10 | DS18B20 pull-up |
| Wires, connectors | $3 | Misc |

## Hardware Wiring

```
Raspberry Pi Zero 2 W
├── GPIO4 (pin 7)   ─── DS18B20 Data (1-Wire bus) + 4.7kΩ pull-up to 3.3V
├── GPIO17 (pin 11) ─── Relay IN (pump control)
├── GPIO27 (pin 13) ─── Flow Meter PULSE
├── 3.3V (pin 1)    ─── DS18B20 VCC
├── 5V (pin 2)      ─── Relay VCC + Flow Meter VCC
└── GND (pins 6,9)  ─── All grounds

Temperature Sensors (DS18B20):
- Tape to hot water outlet pipe (near water heater)
- Tape to return line (before pump)
- Use thermal paste + electrical tape for good contact
```

## Quick Start

### 1. Flash Raspberry Pi OS

Flash Raspberry Pi OS Lite (64-bit) to microSD card using [Raspberry Pi Imager](https://www.raspberrypi.com/software/).

Enable SSH and configure WiFi in the imager settings.

### 2. Install SmartPlug

SSH into your Pi and run:

```bash
# Download latest release
curl -LO https://github.com/smartplug/smartplug/releases/latest/download/smartplug-linux-arm64.tar.gz
tar xzf smartplug-linux-arm64.tar.gz
cd release

# Run installer
sudo ./install.sh
```

### 3. Configure

Edit the configuration file:

```bash
sudo nano /etc/smartplug/smartplug.yaml
```

Key settings:
- `pump.start_threshold`: Start pump when temperature differential exceeds this (default: 12°F)
- `pump.stop_threshold`: Stop pump when differential drops below this (default: 8°F)
- `mqtt.enabled`: Enable Home Assistant integration

### 4. Start Service

```bash
sudo systemctl start smartplug
sudo systemctl status smartplug
```

### 5. Access Web Interface

Open `http://smartplug.local:8080` or `http://<ip-address>:8080` in your browser.

## Building from Source

### Prerequisites

- Go 1.21 or later
- Make

### Build

```bash
# Clone repository
git clone https://github.com/smartplug/smartplug.git
cd smartplug

# Install dependencies
make deps

# Build for Raspberry Pi
make build-pi

# Or build for current platform
make build
```

### Run Locally (Mock Mode)

```bash
make run
```

This starts the server in mock mode without hardware, useful for development.

## Configuration

See `configs/smartplug.yaml` for all available options.

### Temperature Thresholds

```yaml
pump:
  start_threshold: 12.0  # Start when differential > 12°F
  stop_threshold: 8.0    # Stop when differential < 8°F
  max_runtime_minutes: 15
  cooldown_minutes: 5
```

### Flow Meter

```yaml
flowmeter:
  enabled: true
  pulses_per_liter: 450
  trigger_threshold: 3
  demand_timeout: 30
```

### MQTT / Home Assistant

```yaml
mqtt:
  enabled: true
  broker: "tcp://homeassistant.local:1883"
  username: "mqtt_user"
  password: "mqtt_pass"
  ha_discovery: true
```

## Deployment Modes

SmartPlug supports three deployment modes for different hardware configurations:

| Mode | Description |
|------|-------------|
| **all-in-one** | Default. Single Pi with sensors, flow meter, and relay connected directly |
| **sensor** | Sensor node that publishes readings via MQTT. No pump control |
| **controller** | Receives sensor data via MQTT, controls pump via MQTT or local relay |

### Distributed Example

```yaml
# Sensor node (at water heater)
deployment:
  mode: sensor
  node_id: "sensor-kitchen"

# Controller (on Home Assistant or separate device)
deployment:
  mode: controller
  sensor_node_ids: ["sensor-kitchen"]
  actuator_type: mqtt
```

See [docs/DEPLOYMENT_MODES.md](docs/DEPLOYMENT_MODES.md) for detailed configuration and MQTT message formats.

## API Reference

### Status
- `GET /api/status` - Full system status
- `GET /api/temperatures` - Current temperatures

### Pump Control
- `POST /api/pump/heat-now` - Manually activate pump
- `POST /api/pump/stop` - Stop pump
- `POST /api/pump/enable` - Enable automatic control
- `POST /api/pump/disable` - Disable automatic control

### Schedule
- `GET /api/schedule` - Get schedule status and slots
- `POST /api/schedule` - Enable/disable scheduling
- `POST /api/schedule/slots` - Add schedule slot
- `DELETE /api/schedule/slots?id=X` - Delete schedule slot

### Learning
- `GET /api/learning/stats` - Learning statistics
- `GET /api/learning/patterns` - Detected patterns
- `POST /api/learning/clear` - Clear learning history

## Home Assistant Integration

SmartPlug supports MQTT auto-discovery for Home Assistant.

### Entities Created

- **switch.smartplug_pump** - Control the pump
- **sensor.smartplug_temp_hot** - Hot water temperature
- **sensor.smartplug_temp_return** - Return line temperature
- **sensor.smartplug_temp_diff** - Temperature differential
- **binary_sensor.smartplug_flow** - Water flow detection
- **sensor.smartplug_controller_state** - Controller state

### Example Automation

```yaml
automation:
  - alias: "Preheat water before morning routine"
    trigger:
      - platform: time
        at: "06:30:00"
    condition:
      - condition: state
        entity_id: binary_sensor.someone_home
        state: "on"
    action:
      - service: switch.turn_on
        entity_id: switch.smartplug_pump
```

## Troubleshooting

### Sensors Not Detected

1. Check wiring - ensure data line has 4.7kΩ pull-up to 3.3V
2. Verify 1-Wire is enabled: `ls /sys/bus/w1/devices/`
3. Add to `/boot/config.txt`: `dtoverlay=w1-gpio`
4. Reboot

### Pump Not Activating

1. Check relay wiring
2. Verify GPIO permissions: user must be in `gpio` group
3. Check logs: `sudo journalctl -u smartplug -f`

### Web Interface Not Loading

1. Check service status: `sudo systemctl status smartplug`
2. Verify port 8080 is not blocked
3. Check firewall: `sudo ufw allow 8080`

## Safety Notes

- Relay controls 120V AC - use appropriately rated relay (10A minimum)
- Ensure galvanic isolation between Pi and pump circuit
- Flow meter is stainless steel - do NOT use galvanized fittings
- Use PTFE tape on all NPT threads

## Contributing

Contributions welcome! Please read [CONTRIBUTING.md](CONTRIBUTING.md) first.

## License

MIT License - see [LICENSE](LICENSE) for details.

## Acknowledgments

- Inspired by the Leridian SRC32
- Built with Go, using only standard library + MQTT client
- Designed for 24/7 reliability
