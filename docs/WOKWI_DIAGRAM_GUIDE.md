# Creating SmartPlug Wiring Diagram in Wokwi

## Getting Started

1. Go to https://wokwi.com
2. Click "Start from Scratch"
3. Select "Raspberry Pi Pico" as base (Wokwi doesn't have Pi Zero, but Pico works for diagram purposes)

## Components to Add

Click the **+** button and search for each component:

| Search Term | Component | Quantity | Notes |
|-------------|-----------|----------|-------|
| `raspberrypi pico` | Raspberry Pi Pico | 1 | Stand-in for Pi Zero 2 W |
| `ds18b20` | DS18B20 Temperature Sensor | 2 | One for hot, one for return |
| `relay module` | Relay Module (1-channel) | 1 | Controls the outlet |
| `resistor` | Resistor | 1 | Set to 4.7kΩ (click to edit value) |

**Note:** Wokwi doesn't have the YF-S201 flow meter. Use a generic component or add a text label.

## Wiring Connections

### Temperature Sensors (DS18B20) - Both wired in parallel

| DS18B20 Pin | Connect To |
|-------------|------------|
| VCC (red) | 3.3V |
| GND (black) | GND |
| DATA (yellow) | GPIO4 + 4.7kΩ resistor to 3.3V |

### Relay Module

| Relay Pin | Connect To |
|-----------|------------|
| VCC | 5V (VBUS on Pico) |
| GND | GND |
| IN | GPIO17 |

### Flow Meter (if adding manually)

| Flow Meter Pin | Connect To |
|----------------|------------|
| VCC (red) | 5V |
| GND (black) | GND |
| PULSE (yellow) | GPIO27 |

## Pin Mapping (Pi Zero 2 W → Pico equivalent)

| Pi Zero 2 W | Pico | Function |
|-------------|------|----------|
| Pin 1 (3.3V) | 3V3 | Power for sensors |
| Pin 2 (5V) | VBUS | Power for relay & flow meter |
| Pin 6 (GND) | GND | Common ground |
| Pin 7 (GPIO4) | GP4 | 1-Wire data (sensors) |
| Pin 11 (GPIO17) | GP17 | Relay control |
| Pin 13 (GPIO27) | GP27 | Flow meter pulse |

## Export Your Diagram

1. Arrange components neatly
2. Press **Ctrl+Shift+E** or go to menu → **Download** → **Download as PNG**
3. Save to `docs/images/wiring-diagram.png`

## Alternative: Use Wokwi's Share Feature

1. Click **Save** (requires free account)
2. Click **Share** → Copy link
3. Anyone can view your interactive diagram

## Quick Wokwi Project JSON

You can also paste this into a new Wokwi project's `diagram.json`:

```json
{
  "version": 1,
  "author": "SmartPlug",
  "editor": "wokwi",
  "parts": [
    { "type": "wokwi-pi-pico", "id": "pico", "top": 0, "left": 0 },
    { "type": "wokwi-ds18b20", "id": "temp1", "top": -100, "left": 200, "attrs": { "label": "Hot Outlet" } },
    { "type": "wokwi-ds18b20", "id": "temp2", "top": -100, "left": 300, "attrs": { "label": "Return Line" } },
    { "type": "wokwi-resistor", "id": "r1", "top": -50, "left": 150, "attrs": { "value": "4700" } },
    { "type": "wokwi-relay-module", "id": "relay", "top": 100, "left": 250 }
  ],
  "connections": [
    ["pico:3V3", "temp1:VCC", "red", ["v-20", "h50"]],
    ["pico:3V3", "temp2:VCC", "red", ["v-20", "h100"]],
    ["pico:3V3", "r1:1", "red", ["v-20"]],
    ["pico:GND.1", "temp1:GND", "black", ["v20", "h50"]],
    ["pico:GND.1", "temp2:GND", "black", ["v20", "h100"]],
    ["pico:GP4", "temp1:DQ", "orange", ["v-10", "h30"]],
    ["pico:GP4", "temp2:DQ", "orange", ["v-10", "h80"]],
    ["pico:GP4", "r1:2", "orange", []],
    ["pico:VBUS", "relay:VCC", "red", ["v50"]],
    ["pico:GND.2", "relay:GND", "black", ["v60"]],
    ["pico:GP17", "relay:IN", "green", ["v40"]]
  ]
}
```

## Adding Flow Meter Manually

Since Wokwi doesn't have YF-S201:
1. Add a text annotation: "Flow Meter (YF-S201)"
2. Or use "Custom Chip" feature to create a placeholder
3. Or just add it in post with any image editor
