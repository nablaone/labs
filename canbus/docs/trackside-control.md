# Trackside control — protocol, event model, and learning concepts

The trackside accessory side (turnouts; signals/semaphores if ever needed),
and the control-engineering concepts it's a vehicle for learning. Compiled
2026-08-20. Transport decision lives in
[wireless-architecture.md](wireless-architecture.md).

## Current scope (single loco, modular track)

- **6 turnouts, 0 signals/semaphores.** One loco + one driver = no
  conflicting movements, so signals lose their safety purpose and
  interlocking's core rationale is absent. Dropped for now.
- **Turnout position still matters for safety** even with one loco — a
  half-thrown or wrongly-set point derails that loco. So the one piece of
  interlocking we keep is **command + confirmed feedback per turnout**: the
  node reports *sensed* blade position (limit switch), and the driver acts on
  that confirmation, never on "command sent."
- **Track is modular** (rearranged in minutes) → **wireless per turnout
  (ESP-NOW)**, not a fixed CAN trunk. Each turnout is a self-contained module
  that moves with its panel and rejoins by node ID.

### Turnout node
ESP32-C3 + actuator driver (point motor / linear actuator — heftier than a
model servo at this scale) + limit-switch feedback + local/battery power +
status LED. Listens for "throw N," drives the actuator, reports "N thrown/
normal, confirmed" or "N failed."

## Design pattern worth borrowing: producer/consumer events

The mature open railway-control standards (**LCC/OpenLCB**, **MERG CBUS**,
both CAN-based) use a **producer/consumer event model** instead of
node-addressed commands: a device that senses something *produces* an event;
devices that act are taught which events to *consume*. One button press can
set a whole route because multiple consumers react to the same event. We
build our own (learning is the point — see below), but steal this model: it
decouples devices and is more flexible than "node 5, output 3." It maps
cleanly onto CAN or ESP-NOW.

We are **not** adopting LCC/CBUS wholesale (private line, learning-driven, and
their ESP libraries are Arduino/IDF not our stack) — but their event model
and their feedback discipline are the reference.

## The learning frame: auto + factory concepts

This project is a vehicle to learn control engineering by borrowing from two
domains:

**Automotive (existing foundation):** CAN itself; each node as an **ECU**;
distributed control with no central brain; **diagnostics culture** (DTCs, a
diagnostic channel — see [diagnostics.md](diagnostics.md)); heartbeat/
watchdog.

**Factory automation (the frontier):**
- **PLC scan cycle** — read inputs → solve logic → write outputs, repeat,
  deterministically. Implementable as a "soft PLC" on an ESP32; reframes the
  whole problem.
- **Ladder logic (IEC 61131-3)** — relay-style logic (familiar from auto
  electrics); even a tiny interpreter is a good exercise.
- **State machines** — model each turnout/signal/route explicitly.
- **CANopen** — the industrial way to do CAN (object dictionary, PDO/SDO,
  NMT, heartbeat); "CAN from auto, done the factory way."
- **Fail-safe / vital design** — railway signalling *invented* this: default
  to danger on any failure; de-energize-to-trip; dual-channel. Same as a
  factory safety-guard interlock.

**Crown-jewel project (if multi-loco ever happens): interlocking.** Railway
signalling and factory safety logic are the same discipline. The rule — *a
signal may clear only if the route is set, every point is locked and
detected, the block is clear, and no conflicting route is set; any failure →
danger* — contains state machines, input conditioning, scan-cycle evaluation,
fail-safe defaults, and mutual exclusion. Not needed for a single loco, but
the reference target for "control stuff."

### Staged build (one testable thing at a time)
1. One turnout with feedback (command → actuate → sense → confirm).
2. Cab/handset that commands turnouts and displays confirmed state.
3. (If ever multi-loco) one signal as a state machine → two-signal
   interlocking → full route with conflicting-route lockout.

## Sources

- LCC/OpenLCB, MERG CBUS producer/consumer event model — openlcb.org,
  nmra.org/lcc, MERG (general reference).
- IEC 61131-3 (ladder/SFC), PLC scan cycle, CANopen (CiA 301) — general
  control-engineering references.
