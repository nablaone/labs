# Project charter — 7¼" gauge electric locomotive control

The single source of truth for *what we're building and why*. Individual
topics have their own docs (linked below); this page holds the decisions and
the shape of the whole system. Last consolidated 2026-08-20.

## What this is

A distributed control system for a rideable 7¼" gauge electric locomotive,
plus a modular trackside layout. Built as a learning project — the point is
to learn control engineering hands-on, borrowing from the **automotive** and
**factory-automation** domains, not to ship a product. Experimentation is a
feature, not overhead.

## Platform decision (the big pivot)

**Standardized on ESP32 + ESP-IDF. Single platform.**

The project began Zephyr-based and multi-platform (ESP32 + RP2040 via
Zephyr's overlay model — see the historical
[zephyr-app-architecture.md](zephyr-app-architecture.md)). It has since
pivoted to **ESP-only, ESP-IDF-only** because:

- Every node — loco *and* trackside — benefits from ESP32's integrated
  connectivity (WiFi/BLE/ESP-NOW), which no other MCU in this class matches.
- One toolchain, one language, one mental model = productive focus for a solo
  builder, versus maintaining Zephyr + ESP-IDF bilingually.
- The portability Zephyr bought us (RP2040 target) was theoretical; we were
  never going to ship on non-ESP silicon.

Trade accepted: we give up Zephyr's `zephyr,canbus` abstraction and Pico
support. On ESP-IDF we drive the **TWAI** controller directly and hand-roll
the message layer — which is *better* for learning anyway. Concurrency model
is FreeRTOS (ESP-IDF's kernel): see
[esp-idf-architecture.md](esp-idf-architecture.md).

### Chip variants standardized

| Variant | Role |
|---|---|
| **ESP32-S3** | Compute-heavier loco nodes; native USB; BLE 5 |
| **ESP32-C3** | Cheap small wireless nodes (turnouts, accessories) |
| **ESP32-C6** | Only if trackside grows into a Thread mesh (802.15.4) |

The original WROOM-32 is used for existing prototypes but is marked NRND by
Espressif — not for new nodes.

## System shape

Two physically separate domains that meet at one bridge:

```
      LOCO (wired CAN backbone)                MODULAR TRACKSIDE (wireless)
  ┌─────────────────────────────┐          ┌──────────────────────────────┐
  │  motor node ── CAN ── ...    │          │  turnout node (ESP-NOW) x6   │
  │  controller node ── CAN ──   │          │  each: throw + confirm pos   │
  │  panel node ── CAN ──        │          │  battery/local power         │
  └──────────────┬──────────────┘          └───────────────┬──────────────┘
                 │                                          │
                 └────────── BRIDGE NODE (CAN + radio) ─────┘
```

- **Loco:** three nodes on a wired **CAN** backbone (motor / controller /
  panel). CAN is deliberate: differential, deterministic, noise-immune —
  right for a machine carrying people. See [bus-comparison.md](bus-comparison.md),
  [bus-selection.md](bus-selection.md).
- **Trackside:** the track is modular and rearranged in minutes, so a fixed
  CAN trunk is the wrong tool. Turnouts are **wireless (ESP-NOW)**,
  self-contained, locally powered. 6 turnouts, **0 signals/semaphores**
  (single loco → no conflicting movements → interlocking's safety purpose is
  absent). See [wireless-architecture.md](wireless-architecture.md),
  [trackside-control.md](trackside-control.md).
- **Bridge:** one node speaks both CAN and the wireless radio, the single
  place the two domains touch.

## Cross-cutting standards

Decisions that apply everywhere, each with its own doc:

- **Message contract** — one canonical CAN/ESP-NOW message spec (IDs,
  priorities, byte layout), the backbone across mixed toolchains:
  [can-message-spec.md](can-message-spec.md).
- **Power** — 48V native (Tesla/PoE rationale), Cat-cable harness, Deutsch
  connectors, local buck per node, motor on its own battery/cable:
  [power-and-harness.md](power-and-harness.md).
- **Lighting** — WS2812/SK6812 addressable LEDs via the ESP32 RMT driver:
  [lighting-standard.md](lighting-standard.md).
- **Packaging** — DIN-rail bay for clustered loco electronics; sealed IP67
  boxes for isolated nodes: [hardware-packaging.md](hardware-packaging.md).
- **Diagnostics** — status LED + heartbeat + fault codes + bus diag channel,
  the automotive OBD model: [diagnostics.md](diagnostics.md).
- **Off-the-shelf CAN parts** (build-vs-buy reference):
  [can-components.md](can-components.md).

## Safety posture (non-negotiable)

This machine carries people. Therefore:

- **Vital control and e-stop are never wireless-only.** CAN (wired) for
  anything that moves or stops a train; radio only for telemetry and the
  physically un-wireable (see wireless doc for the boundary).
- **Fail-safe defaults:** loss of comms → safe state (motor ramps to stop,
  tail lamp stays lit, turnout reports "unconfirmed"). Heartbeat timeout
  drives this everywhere.
- **Confirmed feedback before action:** a turnout's *sensed* position, not
  the fact a command was sent, gates driving over it. Command ≠ done.
- **E-stop** ideally has a hardwired path independent of software/radio.

## Doc index

| Doc | Topic |
|---|---|
| [bus-comparison.md](bus-comparison.md) | I2C/SPI/CAN/RS-232/RS-485/USB/Ethernet landscape |
| [bus-selection.md](bus-selection.md) | On-board I2C vs SPI detail |
| [canbus-vs-modbus.md](canbus-vs-modbus.md) | CAN vs Modbus/RS-485 |
| [can-message-spec.md](can-message-spec.md) | The message contract (IDs, layout) |
| [esp-idf-architecture.md](esp-idf-architecture.md) | FreeRTOS concurrency, tasks, SMP, sync |
| [zephyr-app-architecture.md](zephyr-app-architecture.md) | (historical) Zephyr model, pre-pivot |
| [wireless-architecture.md](wireless-architecture.md) | BLE/ESP-NOW/Thread/LoRa, safety boundaries |
| [trackside-control.md](trackside-control.md) | Trackside protocol, event model, learning concepts |
| [power-and-harness.md](power-and-harness.md) | 48V, Cat cable, Deutsch, buck tiers |
| [lighting-standard.md](lighting-standard.md) | Addressable LED standard |
| [hardware-packaging.md](hardware-packaging.md) | Enclosures, DIN, mounting |
| [diagnostics.md](diagnostics.md) | Debug/diagnose strategy |
| [can-components.md](can-components.md) | Ready-made CAN hardware |
