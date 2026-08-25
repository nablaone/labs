# Diagnostics — debugging sealed, scattered nodes

Diagnosability is designed *in*, not bolted on. These nodes are sealed, some
wireless, some headless in a moving loco — a different problem from debugging
one board on the bench. The model is automotive: every "ECU" has a warning
light, reports live status on the bus, and stores fault codes read with a
scan tool. Our scan tool = **Pi + CANable** (wired) and a **phone via BLE**
(wireless). Compiled 2026-08-20.

## Four levels of access

### 1. Bench (full access)
- **Serial console** (`ESP_LOGI` over USB-serial, `idf.py monitor`) — 90% of
  debugging.
- **JTAG** — on-chip; **S3/C3/C6 expose it over native USB** (no external
  probe), OpenOCD + GDB in VS Code. Real breakpoints/stepping. (A reason we
  standardized on S3/C3.)
- **Core dumps** — ESP-IDF saves a core dump to flash on crash; pull it later
  and GDB shows where it died, even from a field crash.

### 2. Sealed but on the bench
- **Remote logging** — redirect logs over **WiFi/UDP, MQTT, TCP console, or
  BLE serial**; watch a sealed node live without opening it.
- **Log over CAN** — reserve a diagnostic ID; every node emits periodic
  status/heartbeat/fault frames. The Pi + CANable sees the whole system's
  health on the bus (the OBD/DTC model).

### 3. Field (sealed, scattered, no physical access)
- **Status RGB LED per node** — the single most valuable field diagnostic.
  Encodes state by color/blink; glance and know. Uses the WS2812 standard.
- **Heartbeat** — every node broadcasts "alive + state" every N ms; a silent
  node is instantly identifiable (also the failsafe liveness signal).
- **Fault codes (DTCs)** — numbered codes, not "broken"; read the code, know
  what's wrong without opening anything.
- **BLE diagnostic service** — walk up with a phone, read state/logs/faults
  from wireless nodes.

### 4. Structural / electrical
- **60 Ω CAN check** — meter across CAN-H/L, powered off: 60 Ω = healthy,
  120 Ω = a terminator missing, open = a break. First test on a dead bus.
- **Power rail check** + **reset reason** — ESP-IDF reports brownout vs panic
  vs watchdog; log it on boot (catches sagging-buck resets).
- **Internal test points/header** — reach power/serial by opening just the
  lid.
- **CANable / logic analyzer** — for signal-level bus or I2C/SPI/WS2812
  timing faults.

## Design rules (bake in NOW)

1. **Every node: status RGB LED + heartbeat frame + numbered fault codes.**
   The non-negotiable trio that keeps a sealed, scattered system diagnosable.
2. **Reserved diagnostic CAN ID** (`0x700–0x7FF`, see
   [can-message-spec.md](can-message-spec.md)) + **BLE diag service** on
   wireless nodes — health always readable remotely.
3. **Serviceable access** — open the lid to a serial/JTAG header + power test
   points without unsealing the harness (Deutsch bulkheads already allow
   unplugging a node whole).
4. **Log reset reason + enable core dumps** — a misbehaving node tells you
   *why* on next connect.
5. **Standardize the diagnostic behavior** — same LED color scheme, same
   heartbeat format, same fault-code table across all nodes.

## Standard status-LED scheme (proposed)

| Color / pattern | Meaning |
|---|---|
| Solid green | Healthy, comms OK |
| Blinking green | Running, no bus/mesh comms |
| Blinking amber | Degraded (e.g. feedback mismatch, low rail) |
| Solid/blinking red | Fault (see reported DTC) |
| Blue pulse | Config/BLE session active |

## Starter fault-code table (u16, extend as built)

| Code | Meaning |
|---|---|
| `0x0000` | No fault |
| `0x0001` | Comms timeout (lost heartbeat from expected peer) |
| `0x0010` | Actuator failed to reach position (turnout/semaphore) |
| `0x0011` | Position sensor open/invalid |
| `0x0020` | Supply rail out of range (buck fault / brownout history) |
| `0x0030` | CAN bus-off / error-passive |
| `0x0040` | Over-temperature |

## Sources

- ESP-IDF JTAG-over-USB (S3/C3/C6), core dump, reset reason — Espressif
  ESP-IDF docs (accessed 2026-08-20).
- 60 Ω CAN termination diagnostic — ISO 11898 practice.
