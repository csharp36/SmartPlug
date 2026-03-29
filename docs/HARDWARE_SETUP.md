# SmartPlug Hardware Setup & Testing Guide

Complete guide to assembling and testing SmartPlug before connecting to your pump.

## Bill of Materials

### Required Components

| Component | Approx Price | Where to Buy | Notes |
|-----------|--------------|--------------|-------|
| Raspberry Pi Zero 2 W | $15 | [RPi Foundation](https://www.raspberrypi.com/products/raspberry-pi-zero-2-w/), Adafruit, Amazon | Must be Zero 2 W (not original Zero) for performance |
| 2x DS18B20 Temperature Sensors | $10 | Amazon, AliExpress | Get waterproof or surface-mount versions |
| Hall-effect Flow Meter (3/4" NPT) | $12-15 | Amazon ("YF-S201" or similar) | Brass/stainless, NOT plastic |
| 5V Relay Module (10A) | $3-5 | Amazon, AliExpress | Must be opto-isolated, 10A minimum |
| 32GB microSD Card | $8 | Amazon | Class 10 or better |
| 5V/2.5A USB Power Supply | $8 | Amazon | Quality supply recommended |
| 4.7kΩ Resistor | $0.10 | Any electronics store | 1/4W through-hole |
| Jumper Wires | $3 | Amazon | Female-to-female and male-to-female |
| Thermal Paste | $5 | Amazon | For sensor contact |
| Electrical Tape | $2 | Hardware store | For insulating sensors |

**Total: ~$66-73**

### Optional but Recommended

| Component | Price | Purpose |
|-----------|-------|---------|
| Breadboard | $5 | Prototyping before final assembly |
| Multimeter | $15 | Testing connections |
| Pipe Insulation Foam | $5 | Cover sensors for accuracy |
| Project Enclosure | $10 | Protect electronics |
| Terminal Blocks | $5 | Easier wire connections |

## Wiring Diagram

```
                                    +3.3V (Pin 1)
                                       │
                                       ├──────────────┐
                                       │              │
                                    [4.7kΩ]          │
                                       │              │
 ┌─────────────────────────────────────┼──────────────┼─────────────────┐
 │ Raspberry Pi Zero 2 W               │              │                 │
 │                                     │              │                 │
 │  Pin 1 (3.3V) ──────────────────────┘              │                 │
 │  Pin 2 (5V) ────────────────────────────────────┐  │                 │
 │  Pin 6 (GND) ───────────────────────────┐       │  │                 │
 │  Pin 7 (GPIO4) ─────────────────────────┼───────┼──┘                 │
 │  Pin 11 (GPIO17) ───────────────────┐   │       │                    │
 │  Pin 13 (GPIO27) ────────────────┐  │   │       │                    │
 │                                  │  │   │       │                    │
 └──────────────────────────────────┼──┼───┼───────┼────────────────────┘
                                    │  │   │       │
                                    │  │   │       │
    ┌───────────────────────────────┘  │   │       │
    │  Flow Meter (YF-S201)            │   │       │
    │  ┌─────────────────┐             │   │       │
    │  │ RED (VCC) ──────┼─────────────┼───┼───────┘
    │  │ BLACK (GND) ────┼─────────────┼───┘
    │  │ YELLOW (PULSE)──┘             │
    │  └─────────────────┘             │
    │                                  │
    │  Relay Module                    │
    │  ┌─────────────────┐             │
    │  │ VCC ────────────┼─────────────┤ (from 5V)
    │  │ GND ────────────┼─────────────┤ (to GND)
    │  │ IN ─────────────┘             │
    │  │                               │
    │  │ COM ──── To Pump Hot Wire     │
    │  │ NO ───── From Wall Hot Wire   │
    │  │ NC ───── (unused)             │
    │  └─────────────────┘             │
    │                                  │
    │  DS18B20 Sensors (both on same bus)
    │  ┌─────────────────┐             │
    │  │ RED (VCC) ──────┼─────────────┤ (to 3.3V)
    │  │ BLACK (GND) ────┼─────────────┤ (to GND)
    │  │ YELLOW (DATA) ──┼─────────────┘ (to GPIO4 + pull-up)
    │  └─────────────────┘
    │
    │  Second DS18B20 wired in parallel
    │  (same VCC, GND, DATA lines)
```

## Phase 1: Bench Testing (NO PUMP CONNECTED)

### Step 1.1: Flash Raspberry Pi OS

1. Download [Raspberry Pi Imager](https://www.raspberrypi.com/software/)
2. Select "Raspberry Pi OS Lite (64-bit)"
3. Click gear icon for advanced settings:
   - Enable SSH
   - Set username/password
   - Configure WiFi
4. Flash to microSD card
5. Insert card and power on Pi

### Step 1.2: Enable 1-Wire Interface

```bash
# SSH into Pi
ssh pi@raspberrypi.local

# Edit config
sudo nano /boot/config.txt

# Add this line at the end:
dtoverlay=w1-gpio

# Reboot
sudo reboot
```

### Step 1.3: Wire Temperature Sensors (Bench Test)

Wire on a breadboard first:

1. Connect DS18B20 VCC (red) to 3.3V
2. Connect DS18B20 GND (black) to GND
3. Connect DS18B20 DATA (yellow) to GPIO4
4. Add 4.7kΩ resistor between DATA and 3.3V

**Test sensors:**
```bash
# Load modules
sudo modprobe w1-gpio
sudo modprobe w1-therm

# Check for sensors
ls /sys/bus/w1/devices/

# Should see: 28-xxxxxxxxxxxx (one per sensor)

# Read temperature
cat /sys/bus/w1/devices/28-*/w1_slave

# Should show temperature after t=
# Example: t=23500 means 23.5°C
```

### Step 1.4: Test Relay (NO PUMP YET)

Wire relay module:
1. Relay VCC to 5V
2. Relay GND to GND
3. Relay IN to GPIO17

**Test relay clicks:**
```bash
# Export GPIO
echo 17 | sudo tee /sys/class/gpio/export
echo out | sudo tee /sys/class/gpio/gpio17/direction

# Turn relay ON (should click)
echo 1 | sudo tee /sys/class/gpio/gpio17/value

# Turn relay OFF (should click)
echo 0 | sudo tee /sys/class/gpio/gpio17/value

# Cleanup
echo 17 | sudo tee /sys/class/gpio/unexport
```

### Step 1.5: Test Flow Meter

Wire flow meter:
1. Flow meter RED to 5V
2. Flow meter BLACK to GND
3. Flow meter YELLOW to GPIO27

**Test flow meter pulses:**
```bash
# Export GPIO
echo 27 | sudo tee /sys/class/gpio/export
echo in | sudo tee /sys/class/gpio/gpio27/direction

# Read value (blow into meter or spin manually)
cat /sys/class/gpio/gpio27/value

# Watch for changes
watch -n 0.1 cat /sys/class/gpio/gpio27/value

# Cleanup
echo 27 | sudo tee /sys/class/gpio/unexport
```

## Phase 2: Software Testing (Mock Mode)

### Step 2.1: Install SmartPlug

```bash
# Download and install
curl -LO https://github.com/smartplug/smartplug/releases/latest/download/smartplug-linux-arm64.tar.gz
tar xzf smartplug-linux-arm64.tar.gz
cd release
sudo ./install.sh
```

### Step 2.2: Run in Mock Mode First

```bash
# Stop the real service
sudo systemctl stop smartplug

# Run in mock mode to test UI
sudo /opt/smartplug/smartplug --mock --config /etc/smartplug/smartplug.yaml

# Open browser to http://raspberrypi.local:8080
# Verify:
# - Dashboard loads
# - Temperatures show (mock values)
# - "Heat Now" button works
# - Schedule page works
```

### Step 2.3: Test with Real Sensors

```bash
# Edit config to use real sensors
sudo nano /etc/smartplug/smartplug.yaml

# Set your sensor IDs (from ls /sys/bus/w1/devices/)
sensors:
  hot_outlet_id: "28-xxxxxxxxxxxx"
  return_line_id: "28-yyyyyyyyyyyy"

# Start real service
sudo systemctl start smartplug
sudo journalctl -u smartplug -f

# Verify sensor readings in web UI
```

## Phase 3: Integration Testing (Before Pump)

### Step 3.1: Simulate Hot Water Usage

1. Place one sensor in warm water, other in room temp water
2. Verify differential shows in UI
3. Verify relay clicks when differential exceeds threshold
4. Check that relay turns off when differential drops

### Step 3.2: Test Flow Meter

1. Connect flow meter inline with a garden hose
2. Turn on water
3. Verify "Flow Active" shows in UI
4. Verify pump triggers on flow

### Step 3.3: Verify Safety Limits

```bash
# Temporarily set low max runtime for testing
sudo nano /etc/smartplug/smartplug.yaml

pump:
  max_runtime_minutes: 1  # 1 minute for testing

# Restart and verify pump stops after 1 minute
sudo systemctl restart smartplug
```

## Phase 4: Pump Installation

### Safety Checklist

- [ ] Power OFF circuit breaker for pump
- [ ] Verify pump is rated for relay (typically 1/40 HP = ~1A, well under 10A relay rating)
- [ ] Use appropriately rated wire (14 AWG minimum for 15A circuit)
- [ ] Install in waterproof enclosure
- [ ] Ground all metal parts
- [ ] Keep low-voltage (Pi) and high-voltage (pump) wiring separate

### Wiring Pump to Relay

```
Wall Outlet (120V AC)
├── Hot (black) ──────────────► Relay NO terminal
│                                    │
│                              Relay COM terminal ──► Pump Hot wire
│
├── Neutral (white) ─────────────────────────────► Pump Neutral wire
│
└── Ground (green) ──────────────────────────────► Pump Ground wire
```

### Post-Installation Verification

1. Power on circuit breaker
2. Verify pump is OFF initially
3. Use "Heat Now" in web UI
4. Verify pump runs
5. Verify pump stops when differential reaches stop threshold
6. Monitor for 24 hours before relying on it

## Troubleshooting

### Sensors Not Detected
```bash
# Check 1-Wire is enabled
cat /boot/config.txt | grep w1

# Check modules loaded
lsmod | grep w1

# Manual load
sudo modprobe w1-gpio
sudo modprobe w1-therm
```

### Relay Not Clicking
```bash
# Check GPIO permissions
groups  # Should include 'gpio'

# Test GPIO directly
gpioset gpiochip0 17=1  # ON
gpioset gpiochip0 17=0  # OFF
```

### Flow Meter Not Registering
- Check wiring (5V, not 3.3V)
- Verify water is flowing (needs minimum flow rate)
- Check pulse threshold in config

### Web UI Not Loading
```bash
# Check service status
sudo systemctl status smartplug

# Check logs
sudo journalctl -u smartplug -f

# Check port
sudo ss -tlnp | grep 8080
```

## Maintenance

- **Monthly**: Check sensor readings match a thermometer
- **Quarterly**: Test relay operation manually
- **Yearly**: Check all wire connections for corrosion
