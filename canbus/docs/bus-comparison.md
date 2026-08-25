# Bus comparison: I2C, SPI, CAN, RS-232, RS-485, USB, Ethernet

Landscape reference for choosing a bus, across the seven that come up for an
embedded / distributed-control build. For the on-board **I2C vs SPI** detail
(pin cost, pull-ups, addresses) see [bus-selection.md](bus-selection.md);
this doc is the wider picture including the serial and networking buses.
Compiled 2026-08-20.

Rule of thumb for reading it: buses split into three scopes —
**on-board chip-to-chip** (I2C, SPI), **field / inter-board** (CAN, RS-485,
RS-232), and **high-bandwidth host links** (USB, Ethernet). They are not
substitutes for each other across scopes.


## Comparison table

| Bus | Max length | Throughput | Wires / signalling | Topology | Cost (per node) | Typical usage |
|---|---|---|---|---|---|---|
| **I2C** | ~1 m (capacitance-limited, ~400 pF); tens of m with buffers at low speed | 100 kbit/s / 400 kbit/s / 1 Mbit/s (3.4 Mbit/s HS, rare) | 2 (SDA, SCL), single-ended, open-drain + pull-ups | Multi-master, multi-drop, **7-bit addresses** | ~free — on-chip, **no transceiver** | On-board sensors, RTC, EEPROM, character/OLED displays |
| **SPI** | Board-level, ~10–30 cm | Very high — tens of MHz (~1–50+ Mbit/s) | 3 shared (SCLK, MOSI, MISO) + **1 CS per device**, single-ended | Single-master, CS-selected | ~free — on-chip, no transceiver | Fast on-board: TFT displays, SD cards, flash, ADCs, SPI encoders |
| **CAN** (classic 2.0) | **40 m @ 1 Mbit/s → ~500 m @ 125 kbit/s → ~1 km @ ~40 kbit/s** | 1 Mbit/s (CAN-FD: 5–8 Mbit/s data phase, shorter runs) | 2 (CAN_H, CAN_L), **differential** twisted pair, 120 Ω both ends | **Multi-master, message-priority arbitration** (no node addresses needed) | low — transceiver ~$0.5–2; controller often **on-chip (ESP32 TWAI)** | Vehicles, industrial, **distributed real-time control in electrical noise** |
| **RS-232** | ~15 m at low baud (shorter at high baud) | ≤115.2 kbit/s typical (spec ~20 kbit/s; ~1 Mbit/s over short good cable) | 3+ (TX, RX, GND), single-ended, **bipolar ±3…±15 V** | **Point-to-point only (1:1)**, full-duplex | low — needs level shifter (MAX232-class) | Legacy serial: console/config ports, modems, bench instruments |
| **RS-485** | **~1200 m** at low speed (10 Mbit/s @ ~12 m ↔ ~100 kbit/s @ 1200 m) | up to 10 Mbit/s (short) | 2 (A, B) half-duplex or 4-wire full, **differential**, 120 Ω term. | Multi-drop, up to **32 (to 256) nodes**; **physical layer only** | low — transceiver (MAX485-class) ~$0.5–1 | Industrial long-run multi-drop: Modbus, DMX512, Profibus |
| **USB** | ~5 m (USB 2.0 cable); ~3 m (USB 3.x); further via hubs/active cables | 12 Mbit/s (1.1) · **480 Mbit/s (2.0)** · 5–10 Gbit/s (3.x) · 40 Gbit/s (USB4) | Differential pair(s) + **5 V VBUS** + GND | **Host/device** (one host), tiered star via hubs, host-addressed | moderate — USB PHY/controller; **WROOM-32 has none** (uses CP2102 bridge); S2/S3/C3 have native USB | PC peripherals, flashing, mass storage, HID, host↔device links |
| **Ethernet** | **100 m per copper segment** (Cat5e/6); further via switches/fibre | 10 / 100 Mbit/s · 1 Gbit/s common · 10 Gbit/s+ high end | Twisted pairs (2–4) + RJ45, differential, needs **PHY + magnetics** | Switched star, **MAC/IP-addressed**, packet | higher — external PHY (e.g. LAN8720) + magnetics + connector; ESP32 EMAC needs external RMII PHY | LAN / TCP-IP, high bandwidth, industrial Ethernet (EtherCAT, PROFINET) |


## Per-bus notes that matter for a distributed loco

**I2C / SPI — on-board only.** Both are chip-to-chip buses meant for
short traces next to the MCU. Neither is a legitimate way to reach another
board across a metre-plus of PWM-motor noise (see the distance analysis in
[bus-selection.md](bus-selection.md)). Use them *within* a board, not
between boards.

**RS-232 — point-to-point, so not a node bus.** It connects exactly two
devices. There's no multi-drop, no arbitration, no addressing. Fine for a
single debug/console link to a PC or an instrument; useless as a backbone
for three boards. Also single-ended, so poor noise immunity on a loco.

**RS-485 — the real alternative to CAN, but it's only the wire.** Same
differential, long-run, multi-drop robustness as CAN at the physical layer,
and actually *higher* raw throughput on short runs. The catch: **RS-485 is
only a physical layer.** It has **no built-in arbitration, addressing, error
handling, or message priority** — you must layer a protocol on top (Modbus
etc.), usually with a designated master polling nodes. CAN gives you all of
that *in silicon*: any node can transmit, and **the lowest-ID message wins
arbitration automatically** with no corruption. For a safety-relevant loco
where "e-stop must always win the bus instantly," CAN's priority arbitration
and hardware error handling are a decisive advantage over raw RS-485.

**CAN — the right backbone here, and it's free on the ESP32.** Differential
and noise-immune, multi-master, priority-arbitrated, with automatic
retransmission and fault confinement built in. The controller is **on-chip
(TWAI)** on the ESP32 — only an external SN65HVD230 transceiver is needed.
Distances (tens of m at 500 kbit/s) far exceed a loco's needs. This is why
the project is built on it.

**USB — host-centric, not a peer bus (and absent on WROOM-32).** USB assumes
**one host** and a tree of devices; boards can't be equal peers on it the way
CAN nodes are. On top of that, the classic **ESP32-WROOM-32 has no native USB
peripheral at all** — its USB port is just the onboard CP2102 USB-to-serial
bridge used for flashing/console. So USB's role in this project is exactly
that: **flashing and PC console**, not inter-board communication. (Native USB
would need an ESP32-S2/S3/C3.)

**Ethernet — overkill for control, useful only if you add heavy data.** 100 m
reach and 100 Mbit/s+ are enormous relative to a few control frames a
millisecond. It costs the most (external PHY + magnetics + RJ45), adds a
TCP/IP stack, and on the ESP32 needs an external RMII PHY. Not warranted for
control signalling. Where it *could* earn its place later: streaming a cab
camera, high-rate data logging off-loco, or a Wi-Fi/Ethernet bridge to a
laptop for telemetry — none of which belong on the CAN control bus anyway.


## Verdict for this build

| Role | Bus |
|---|---|
| Chip-to-chip **within** a board (display, local sensors, RTC) | **I2C** (or **SPI** for fast/critical: TFT, SD, encoder) |
| **Between** the three boards (motor / controller / panel), control + telemetry | **CAN** — differential, priority-arbitrated, on-chip TWAI, noise-immune |
| Flashing + PC console | **USB** (CP2102 serial on WROOM-32) — not for inter-board |
| Long multi-drop *if* CAN weren't available | RS-485 — but you'd have to build arbitration/addressing yourself |
| High-bandwidth extras later (camera, bulk telemetry) | Ethernet or Wi-Fi — off the control bus |

Bottom line: **CAN for the backbone, I2C/SPI for on-board, USB only for
flash/console.** RS-232 (point-to-point) and Ethernet (bandwidth/cost
overkill) don't fit the inter-board control role; RS-485 fits physically but
makes you re-invent what CAN already does in hardware.


## Sources

- CAN physical layer, bit-rate vs bus-length tradeoff, arbitration — **ISO 11898** (CAN 2.0 / CAN-FD); on-chip TWAI details in [esp32-notes.md](esp32-notes.md), transceiver/termination in [dev-setup-research.md](dev-setup-research.md).
- I2C electrical limits (~400 pF bus capacitance, open-drain pull-ups, speed grades) — **NXP I2C-bus specification UM10204**.
- RS-232 line lengths and levels — **TIA/EIA-232-F**.
- RS-485 differential multi-drop, ~1200 m and speed/length tradeoff, 32-unit-load fan-out — **TIA/EIA-485-A**. (RS-485 is a physical layer only; protocols such as Modbus/DMX512 sit on top.)
- USB data rates and cable lengths (USB 1.1/2.0/3.x/USB4) — **USB Implementers Forum (USB-IF)** specifications.
- Ethernet segment length (100 m copper) and data rates — **IEEE 802.3** (10/100/1000BASE-T).
- ESP32 EMAC needs an external RMII PHY; WROOM-32 has no native USB (S2/S3/C3 do) — Espressif ESP32 technical reference; see also [esp32-notes.md](esp32-notes.md).

_Figures are standards-typical values; exact numbers vary with cable
quality, transceiver, and bit rate. Access date 2026-08-20._
