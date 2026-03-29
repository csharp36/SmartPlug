# Creating SmartPlug Wiring Diagram in TinkerCAD

## Getting Started

1. Go to https://www.tinkercad.com
2. Create a free Autodesk account (or sign in)
3. Click **Circuits** in the left sidebar
4. Click **Create new Circuit**

## Components to Add

Click **Components** on the right panel. Use the dropdown to select **All** components.

| Search Term | Component | Quantity | Notes |
|-------------|-----------|----------|-------|
| `arduino uno` | Arduino Uno R3 | 1 | Stand-in for Pi Zero (TinkerCAD doesn't have Pi) |
| `temperature sensor` | TMP36 | 2 | Stand-in for DS18B20 (no DS18B20 in TinkerCAD) |
| `relay` | Relay SPDT | 1 | Controls the outlet |
| `resistor` | Resistor | 1 | Click to change value to 4.7kΩ |
| `breadboard` | Breadboard Small | 1 | Optional, for cleaner layout |

**Note:** TinkerCAD has limited parts. We'll use Arduino + TMP36 as stand-ins and add labels.

## Alternative: Use "Starter" Templates

1. In Components dropdown, select **Starters**
2. Search `arduino` - some pre-wired templates may help

## Wiring Connections

### Temperature Sensors (TMP36 as stand-in for DS18B20)

| TMP36 Pin | Connect To | Notes |
|-----------|------------|-------|
| +Vs (left) | 5V | Power |
| Vout (middle) | A0 / A1 | Analog pins (one per sensor) |
| GND (right) | GND | Ground |

*Real DS18B20 uses digital GPIO4 with 1-Wire protocol, but TinkerCAD doesn't support this.*

### Relay

| Relay Pin | Connect To |
|-----------|------------|
| Coil + | Digital Pin 7 (stand-in for GPIO17) |
| Coil - | GND |

### Flow Meter (not in TinkerCAD)

Add a text label or use a generic 3-pin component as placeholder.

## Pin Mapping Reference

| Pi Zero 2 W | Arduino Uno (TinkerCAD) | Function |
|-------------|-------------------------|----------|
| Pin 1 (3.3V) | 5V | Power for sensors |
| Pin 2 (5V) | 5V | Power for relay |
| Pin 6 (GND) | GND | Common ground |
| Pin 7 (GPIO4) | A0, A1 | Sensor data |
| Pin 11 (GPIO17) | D7 | Relay control |
| Pin 13 (GPIO27) | D8 | Flow meter pulse |

## Adding Labels/Annotations

1. Click **Text** tool in the toolbar (top)
2. Add labels:
   - "Hot Outlet Sensor (DS18B20)"
   - "Return Line Sensor (DS18B20)"
   - "Relay Module (5V, 10A)"
   - "Flow Meter (YF-S201) - not shown"
   - "To Controlled Outlet → Pump plugs in here"

## Export Your Diagram

1. Arrange components neatly
2. Click **Export** button (top right, download icon)
3. Select **Download PNG** or **Download SVG**
4. Save to `docs/images/wiring-diagram.png`

## Tips for a Clean Diagram

1. **Use a breadboard** - Makes wiring look organized
2. **Color your wires:**
   - Red = Power (5V/3.3V)
   - Black = Ground
   - Yellow/Orange = Signal/Data
   - Green = Control
3. **Spread components out** - Leave room for labels
4. **Add a title** - Use text tool to add "SmartPlug Wiring Diagram"

## Limitations of TinkerCAD

TinkerCAD Circuits is designed for Arduino simulation, so:

- ❌ No Raspberry Pi
- ❌ No DS18B20 (1-Wire sensors)
- ❌ No flow meters
- ❌ No relay modules (only basic SPDT relay)

**Workaround:** Use TinkerCAD for the basic layout concept, then add annotations explaining the real components.

## Better for Diagrams Only: TinkerCAD 3D

If you just want a visual (not a simulation):

1. Go to **3D Design** instead of Circuits
2. Import or create simple 3D shapes representing components
3. More flexible but requires more manual work

## Quick Alternative: Screenshot + Annotate

1. Create basic circuit in TinkerCAD
2. Export PNG
3. Open in free image editor (Photopea.com, Canva.com)
4. Add proper labels, arrows, and component names
5. Add flow meter and correct sensor names manually
