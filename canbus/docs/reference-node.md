# Reference node — CAN backbone, I2C device chain, minimal wiring

The standard node pattern: **CAN on one face (the network), a Qwiic/STEMMA QT
I2C chain on the other (local sensors/actuators/lights), GPIO for the few
direct things.** One network drop, lots of local I/O, minimal hand-wiring —
the automotive smart-junction / industrial remote-I/O model. Compiled
2026-08-20. Fits the carrier-board idea in
[hardware-packaging.md](hardware-packaging.md).

## Concept

```
   HARNESS (Deutsch)                 NODE (carrier board)              LOCAL DEVICES
  ┌──────────────────┐        ┌──────────────────────────────┐
  │ CAN-H ───────────┼────────┤ TWAI transceiver             │
  │ CAN-L ───────────┼────────┤ (SN65HVD230)                 │
  │ +48V ────────────┼────────┤ buck → 5V/3.3V rails         │      ┌── sensor
  │ 48V return ──────┼────────┤ ESP32 (S3/C3/C6)             │      │
  │ gnd ref / e-stop ┼────────┤   ├─ Qwiic/STEMMA QT port ───┼──────┼── I2C expander→relays
  └──────────────────┘        │   ├─ GPIO: WS2812 status LED │      │
                              │   └─ GPIO: buttons / Hall    │      └── display
                              └──────────────────────────────┘
```

- **CAN side (network): 2 signal wires** (CAN-H/L), in the shared Deutsch
  harness with power ([power-and-harness.md](power-and-harness.md)).
- **I2C side (local expansion): 2 wires** (SDA/SCL), shared by *every* local
  device via a **Qwiic/STEMMA QT** daisy-chain — 10 devices still = one bus.
- **GPIO (direct): 1 wire per thing** — status LED (WS2812), a few buttons, a
  Hall pulse. Used only for the exceptions I2C can't absorb.
- **Power in: the 48V pair**, bucked locally.

**Net to the outside world: ~4 wires** (CAN pair + power pair, one
connector). Device count barely moves the wire total — that's the point.

## Local bus standard: Qwiic / STEMMA QT

Standardized on **Qwiic / STEMMA QT** (same connector — JST-SH 4-pin, 1 mm
pitch — fully interchangeable; treat as one ecosystem). **Grove is avoided**
(bigger connector, ambiguous — a Grove port may be I2C/UART/GPIO). Qwiic
pinout, fixed color code: **red=3.3V, black=GND, blue=SDA, yellow=SCL**;
polarized so it can't be plugged in reversed; no soldering.

### Rules
1. **3.3V logic/power only** on the chain (Qwiic is 3.3V, no level shifting —
   matches the ESP32). **Actuator/relay/LED-strip power comes from the node's
   own 5V rail, never the chain** (see power doc). The chain feeds *logic*.
2. **Keep the chain short and local** (I2C ~1 m / ~400 pF ceiling). Anything
   distant is the next **node on CAN**, not a longer chain.
3. **Unique address per device** — can't chain two identical modules without
   re-addressing (jumpers). Watch default clashes (0x20, 0x27, 0x48, 0x76).
   Out of addresses → the ESP32's second I2C bus, or a TCA9548A mux.
4. **One INT wire** from an input expander (MCP23017) so buttons are
   event-driven, not polled — many inputs for 3 wires total.

## Making it fewer wires (push onto I2C, GPIO for exceptions)

| Local function | Do it via | Wires |
|---|---|---|
| Many lights | I2C PCA9685 (16 PWM) **or** WS2812 chain | 2 (I2C) / 1 (GPIO) |
| Many relays/actuators | I2C MCP23017 + driver, or I2C relay board | 2 (+1 INT) |
| Sensors (temp, current, IMU, RTC) | native I2C | 2 (shared) |
| Many buttons | MCP23017 + INT | 2 + 1 |
| Few buttons / status LED / Hall pulse | GPIO | 1 each |

## Botland parts (wiring + a starter device mix)

**Cables / interconnect** (botland.store):
- **STEMMA QT/Qwiic JST-SH cable, 300 mm** (Adafruit 5384) — the standard
  chain cable; <cite index="151-1">data lines interleaved with power to reduce interference.</cite>
  Also 400 mm (Adafruit 5385) and short 50–100 mm hops.
- **STEMMA QT/Qwiic 5-port hub** (Adafruit 5625) — <cite index="153-1">adds 5 Qwiic connectors to branch the bus to several peripherals, no soldering.</cite>
  Use when a device lacks a second pass-through port.
- (Bridge to a non-Qwiic breakout: a Qwiic-to-male-header or Qwiic-to-Grove
  adapter cable.)

**Node brain + CAN:** ESP32 (S3 compute / C3 cheap / C6 if Thread) + an
SN65HVD230 transceiver (or an onboard-transceiver board — see
[can-components.md](can-components.md)).

**Example device mix on the chain** (all Qwiic where possible):
- Environmental/status: a Qwiic BME280 (temp/pressure/humidity) for the
  electronics-bay monitoring.
- Outputs: an I2C I/O expander (MCP23017) → relay bank, or a Qwiic relay
  board.
- Display (panel node): a small Qwiic OLED/character display.
- GPIO: WS2812 status LED (diagnostics), a couple of buttons, a Hall input.

> Note: some Qwiic breakouts (e.g. Adafruit's MCP2221A board) also expose a
> Qwiic port and a **jumper to switch 3V↔5V logic** — handy, but keep the
> node standard at 3.3V.

## Why this shape

One board = a little **gateway**: CAN network on one face, local I2C device
bus on the other, GPIO for the few direct signals. Add sensors/actuators/
lights by plugging Qwiic cables, not by hand-wiring — the node becomes a
stack of pluggable modules (the "Eurorack for I2C" idea), while the wire
count to the outside world stays at ~4.

## Sources

- Qwiic/STEMMA QT identical connector & pinout, color code, 3.3V-only —
  Adafruit / SparkFun ecosystem docs; Botland product pages (accessed
  2026-08-20).
- Cables/hub — Botland (Adafruit 5384/5385/5625).
- I2C limits, address clashes — see [bus-selection.md](bus-selection.md).
