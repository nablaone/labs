# Message spec — the cross-toolchain contract

The most important artifact in a mixed-language system. CAN (and ESP-NOW) is
the backbone precisely because a frame is language-agnostic — the transceiver
doesn't care whether a node runs ESP-IDF, Arduino, or bare C. But with no
shared codebase enforcing layouts, **the message definitions must be written
down, canonical, and shared.** This doc is that contract. Compiled
2026-08-20.

## Principle

- **One canonical spec** (this file / a DBC) is the single source of truth
  for every ID, byte layout, unit, and priority. Automotive calls it a DBC;
  the general term is an **ICD** (Interface Control Document).
- **Generate, don't hand-copy.** Ideally each node's message struct is
  generated from this spec, or at minimum a shared plain-C header both
  toolchains `#include`. Re-typing layouts per project = drift = silent bugs.
- **Endianness + packing are defined here explicitly**, never left to a
  compiler default — the classic mixed-toolchain bug. **Convention:
  little-endian, `__attribute__((packed))` structs, byte offsets as stated.**

## CAN ID = priority (lower ID wins arbitration)

The ID ladder falls out of "what must never be starved." Using 11-bit
standard IDs:

| Range | Class | Examples |
|---|---|---|
| `0x000–0x00F` | **Emergency** | e-stop, critical fault |
| `0x010–0x03F` | **Control** | motor command, speed feedback |
| `0x040–0x07F` | **Setpoints** | panel throttle/direction, heartbeats |
| `0x080–0x0BF` | **Status/telemetry** | temps, currents, node health |
| `0x0C0–0x0FF` | **Display/lighting** | non-critical UI/lighting data |
| `0x700–0x7FF` | **Diagnostics** | per-node DTC/heartbeat channel (see diagnostics.md) |

(Exact IDs to be assigned as nodes are built; keep this table updated as the
allocation register.)

## Message layout template

Each message gets an entry like:

```
ID    0x011  MOTOR_COMMAND        priority: control    rate: 50 Hz
  byte 0     command_type   u8    0=coast 1=drive 2=brake
  byte 1     direction      u8    0=fwd 1=rev
  byte 2-3   setpoint       i16   LE, permille of max (-1000..1000)
  byte 4     flags          u8    bit0=enable
  byte 5-7   reserved
```

```
ID    0x020  SPEED_FEEDBACK       priority: control    rate: 50 Hz
  byte 0-1   velocity       i16   LE, mm/s
  byte 2     quality        u8    0..255 signal quality
  byte 3-7   reserved
```

```
ID    0x7NN  NODE_HEARTBEAT       priority: diag       rate: 1-5 Hz
  byte 0     node_id        u8
  byte 1     state          u8    0=init 1=ok 2=degraded 3=fault
  byte 2-3   fault_code     u16   LE, 0=none (see diagnostics.md table)
  byte 4     reset_reason   u8    esp reset reason
  byte 5-7   node-specific
```

## Rules

1. **Every field: type, endianness (LE), unit, offset.** No implicit
   anything.
2. **Reserved bytes are zero-filled** and stay reserved for forward compat.
3. **Heartbeat is mandatory on every node** — it doubles as the failsafe
   liveness signal (a silent node = failed → safe state).
4. **Wireless (ESP-NOW) messages reuse the same struct definitions** where a
   command crosses the bridge, so the bridge translates 1:1 without
   reinterpreting bytes.
5. **The bridge node** is the only place CAN IDs and ESP-NOW payloads are
   mapped to each other; that mapping lives here too.

## Sources

- CAN 11-bit identifier arbitration — ISO 11898.
- DBC/ICD practice — automotive interface-definition convention (general
  reference).
