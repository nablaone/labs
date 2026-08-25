# Ready-made CAN components — build vs buy

Reference for off-the-shelf CAN hardware, mostly from the automotive/
off-highway and factory-automation worlds. Nearly all speak **CANopen or
J1939**, so they plug into the CANopen layer and double as hands-on protocol
learning. Compiled 2026-08-20.

## Categories

### Buttons → CAN keypads (mature, plug-and-play)
Sealed, backlit button panels putting key states on the bus. Self-announce
and accept an address.
- **Blink Marine** PKP / PowerKey / Powertrack — IP67, RGB LED/key, CANopen +
  J1939; some add a rotary encoder.
- **Grayhill 3K** — 6/8/12/15-button, IP67, J1939/CANopen, custom artwork.
- ~€150–300. Overkill for trackside toys; plausible for a serious panel.

### Power relays → two flavors
- **Industrial CANopen relay I/O modules** (DIN-rail): **Datexel DAT7130**
  (8 DI / 4 relay, CiA 301/401, isolated); **Axiomatic** (12 in / 8 relay).
- **CAN Power Distribution Modules (PDMs)** — automotive, **solid-state**,
  replace the fuse/relay box: **Murphy/Enovation IX3212** (12× 15A outputs,
  H-bridge pairs, over-current shutdown, 12 in); **In-CarPC i-PDM** (connects
  keypads directly). Several hundred €. Best fit for switching traction/
  accessory power on a machine carrying people.

### Speed/position → CANopen encoders
No real bare "CAN Hall sensor" market (a Hall is cheap/analog). Ready
products are **CANopen absolute rotary encoders** (CiA 406 — Kübler, Posital,
Pepperl+Fuchs). For wheel speed from a Hall: feed the pulse into a CAN I/O
module's frequency input, or keep it on your own controller node (the
interesting part to build).

### Model-railway CAN kits (cheap trackside end)
**MERG CBUS** kits — turnout drivers, input modules, signal drivers as
assemble-yourself boards. The hobby-priced ready option for wired trackside
signalling.

### ESP32 boards with onboard CAN transceiver
Skip wiring an external SN65HVD230:
- **LILYGO T-CAN485** — ESP32 + CAN (SN65HVD231) + RS485, 5–12V in, CAN on
  GPIO4/5. ~$15–20. (Uses WROOM-32, now NRND.)
- **Autosport Labs ESP32-CAN-X2** — **ESP32-S3**, dual CAN, ruggedized supply
  to 40V in, BLE5 + WiFi. ~$25–30. The future-proof pick.
- All still use the on-chip **TWAI** controller — identical in software to a
  bare ESP32 + transceiver.

## Build-vs-buy line (given the learning goal)

- **Buy** the tedious/safety-relevant hardware: a sealed, over-current-
  protected **PDM** switching traction power; a weatherproof RGB **keypad**.
- **Build** the brains: the control loop, node behavior, interlocking-style
  logic. That's the craft.
- Because it's all CANopen/J1939, hand-built ESP-IDF nodes and bought modules
  share one bus and one protocol.

**Field caution:** builders regularly hit TWAI "sends but nothing received"
(0 ms TX, no bus output) — usually wiring/termination/pin-mapping, not a dead
board. Verify two nodes talk end-to-end before building six.

## Sources

- Blink Marine / Grayhill keypads; Datexel DAT7130; Murphy IX3212; In-CarPC
  i-PDM; LILYGO T-CAN485; Autosport Labs ESP32-CAN-X2 — vendor pages
  (accessed 2026-08-20).
- CANopen device profiles CiA 301/401 (I/O), CiA 406 (encoders) — CAN in
  Automation.
