# CAN vs Modbus over RS-485

Why the loco backbone is CAN and not the other obvious multi-drop option.
Compiled 2026-08-20. See also [bus-comparison.md](bus-comparison.md).

## The decisive difference: access method

- **CAN** — multi-master, event-driven. Any node transmits when it has
  something to say; **priority arbitration** (lowest ID wins,
  non-destructively) resolves contention *in hardware*.
- **Modbus/RS-485** — single-master, **polled**. Passive slaves never speak
  unless the master asks. A slave with an urgent fault waits its turn in the
  poll loop.

For distributed real-time control with an e-stop, that gap is decisive.

## Head to head

| | CAN | Modbus RTU / RS-485 |
|---|---|---|
| Access | Multi-master, priority-arbitrated | Single-master, polled req/response |
| Urgent-event latency | Near-immediate (high-priority frame wins) | Waits for poll slot; grows with node count |
| Async fault reporting | Yes — node just transmits | No — master must poll to discover |
| Error handling | In silicon: CRC, ACK, **auto-retransmit**, fault confinement (bad node self-isolates) | CRC-16 detects; retry/recovery is your app's job; no node isolation |
| Data model | Message/content-addressed, ≤8 B/frame (64 CAN-FD) | Register/coil tables, node-addressed (1–247) |
| Peer-to-peer | Native | None (no slave-to-slave) |
| Physical | CAN_H/L diff pair, 120 Ω ×2, transceiver, **controller on-chip (TWAI)** | A/B diff pair, 120 Ω ×2, UART + software stack + DE/RE direction timing |
| Raw speed/reach | 1 Mbit/s; 40 m→~1 km by rate | up to 10 Mbit/s / 1200 m physically (RTU usually 9600–115200) |

## Why CAN for this loco

- **E-stop wins the bus automatically** — lowest ID, hardware arbitration.
  On Modbus you'd poll for it or add a separate hardwire.
- **Motor board reports a fault the instant it happens** — no waiting to be
  polled.
- **Auto-retransmit + fault confinement** matter amid motor PWM noise.
- **Peer nodes** — all three boards are equals; Modbus forces one master +
  passive slaves.
- **Free on ESP32** (TWAI on-chip); Modbus is a software stack plus the
  half-duplex direction-control timing bug everyone hits.

## Where Modbus would win (not here)

Slow, master-polled reads of registers off industrial sensors/PLCs where
nothing is time-critical, simplicity is paramount, and a human-readable
register map is wanted. A safety-relevant distributed motion controller isn't
that.

**Bottom line:** RS-485 is the only real physical alternative to CAN, but
it's *only the wire* — no arbitration, addressing, or error handling. CAN
gives all of that in silicon, and its priority arbitration is exactly what
makes e-stop always win.

## Sources

- CAN arbitration, error confinement — ISO 11898.
- Modbus RTU master/slave polling model — Modbus Organization spec.
- RS-485 physical layer — TIA/EIA-485-A.
