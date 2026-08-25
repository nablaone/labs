# CAN bus bring-up plan — ESP32 TWAI ↔ Linux SocketCAN

Plan for wiring the SN65HVD230 transceiver to the ESP32 and exchanging real
CAN frames with the Linux box's CANable2 (`can0`, already configured by
[../scripts/setup-socketcan.sh](../scripts/setup-socketcan.sh) — see
`make setup-socketcan` in [../Makefile](../Makefile)). Not yet implemented
— this is the plan to execute next, written before any of it has been
tried. Once it's underway, log actual results/deviations in
`../notebook/`.

This is pure connectivity bring-up on the one WROOM-32 board in hand — not
yet a real node per [project-charter.md](project-charter.md)'s system (that
calls for ESP32-S3/C3/C6, WROOM-32 being NRND for new nodes). The message
IDs below are scratch/test values, not registrations in
[can-message-spec.md](can-message-spec.md) — see the note in "What to send
/ receive."

## Pin assignment

| ESP32 pin | Function | SN65HVD230 pin |
|---|---|---|
| **GPIO21** | TWAI TX (controller → transceiver) | TXD |
| **GPIO22** | TWAI RX (transceiver → controller) | RXD |
| 3V3 | power | VCC |
| GND | ground | GND |
| — | — | CANH → bus |
| — | — | CANL → bus |

Why GPIO21/22:

- Not strapping pins (ESP32's strapping set is GPIO0, 2, 5, 12, 15 — all
  either already used here or worth avoiding regardless).
- Not input-only (GPIO34–39 can't drive TWAI TX).
- Not UART0 (GPIO1/3, used by the console) or the flash SPI pins
  (GPIO6–11).
- Free: GPIO2 (LED) and GPIO0 (BOOT button) are already spoken for; GPIO33
  (the *old* external button pin from the Zephyr app, unused now that the
  button moved to GPIO0) stays free for something else later — a
  CAN-activity LED, an interrupt pin, whatever comes up.
- `esp32-devkitc`'s *default* TWAI pins are GPIO0/GPIO2 (see
  [esp32-notes.md](esp32-notes.md#can-support-twai)) — already ruled out
  since both are in use, and they're strapping pins besides.

Double-check the specific SN65HVD230 breakout's slew-rate/`Rs` pin — most
tie it to GND on-board for standard-speed operation with no jumper needed,
but confirm against the board's silkscreen/datasheet rather than assume.

## Bus wiring and termination

This is a **2-node bus** (ESP32 node ↔ Linux box's CANable2 node) — both
physical ends need 120Ω termination across CANH/CANL, none in the middle:

- **ESP32 end**: enable the SN65HVD230 breakout's onboard 120Ω jumper if it
  has one; otherwise add a discrete 120Ω resistor across CANH/CANL at the
  transceiver.
- **CANable2 end**: check for a termination jumper/switch on the dongle
  and enable it.
- Keep the CANH/CANL run short and twisted-pair-ish for this first test —
  it's a breadboard bring-up, not a production bus; not worth debugging
  signal integrity issues on top of everything else that's new here.
- Common ground reference between the ESP32 and the Linux box's CANable2
  is required — the transceiver's GND, not just CANL, needs to tie back
  (CAN is differential, but both transceivers still need a shared ground
  reference to interpret the differential signal correctly).

## Bitrate: 500 kbit/s, non-negotiable

`setup-socketcan.sh` already brings `can0` up at `CAN_BITRATE=500000` — the
ESP32 side has to match that exactly (CAN nodes don't auto-negotiate
bitrate; a mismatch just looks like a broken bus, with a stream of bit/CRC
errors, not a clean "wrong speed" report). Use ESP-IDF's
`TWAI_TIMING_CONFIG_500KBITS()` macro for this.

## Firmware: two-stage bring-up

Bring the TWAI stack up in two stages rather than wiring everything at
once and debugging blind:

### Stage A — transceiver self-test (`TWAI_MODE_NO_ACK`)

Answers "is the transceiver actually connected and wired correctly?"
*without* needing the Linux box or bus connected at all.

A CAN transceiver electrically reflects whatever it transmits back onto
its own RXD pin (that's inherent to the differential-bus physical layer,
not a special feature) — so a lone ESP32 with just the transceiver wired
(no CANH/CANL run to anything else) can transmit and immediately receive
its own frame back. The catch: in normal mode a transmitted frame with no
other node to ACK it is treated as an error and retried forever.
`TWAI_MODE_NO_ACK` (ESP-IDF's self-test mode) suppresses exactly that
check, making this a real "is my wiring/transceiver good" test:

```c
twai_general_config_t g_config =
    TWAI_GENERAL_CONFIG_DEFAULT(GPIO_NUM_21, GPIO_NUM_22, TWAI_MODE_NO_ACK);
twai_timing_config_t t_config = TWAI_TIMING_CONFIG_500KBITS();
twai_filter_config_t f_config = TWAI_FILTER_CONFIG_ACCEPT_ALL();
twai_driver_install(&g_config, &t_config, &f_config);
twai_start();
// transmit a frame, then twai_receive() with a short timeout --
// getting the same frame back confirms TX pin, RX pin, and the
// transceiver chip are all wired and working.
```

- **Pass** (frame loops back cleanly): transceiver + GPIO wiring is good —
  move to Stage B.
- **Fail** (`twai_receive()` times out, or `twai_get_status_info()` shows
  bus/bit errors): wiring problem on the ESP32 side — check TX/RX aren't
  swapped, transceiver VCC/GND, and that CANH/CANL aren't shorted or
  floating in a way that corrupts the loopback signal.

This isolates "is the ESP32 half of this working" from "is there a live
peer" — worth doing before ever touching the Linux box, since it rules out
a whole class of wiring mistakes on its own.

### Stage B — real bus, `TWAI_MODE_NORMAL`

With the CANH/CANL run made to the CANable2 and both ends terminated,
switch to normal mode and bring `can0` up on the Linux box (`ip link set
can0 up type can bitrate 500000` — already handled automatically by
`setup-socketcan.sh` on hotplug).

**Detecting a live peer doesn't require the other side to be actively
*sending* anything.** ACK is a hardware-level thing: any CAN controller
with its interface up (`can0` UP, even with zero applications reading it)
automatically drives the ACK slot dominant for every valid frame it
receives — that happens in silicon, before any userspace code (`candump`
et al.) ever sees the frame. So:

- Send a frame from the ESP32 in `TWAI_MODE_NORMAL`.
- **ACKed cleanly** → `can0` is up and listening on the wire. A live peer
  is present, confirmed, whether or not anyone's running `candump` on the
  Linux side.
- **`TWAI_ALERT_TX_FAILED` / no-ack errors** → either `can0` isn't up yet,
  or the physical bus connection between the two nodes is bad (open
  CANH/CANL, wrong termination, bad ground reference).

## What to send / receive

Simple and easy to eyeball on both ends for the first test:

- **ESP32 → Linux**: send CAN ID `0x123`, single data byte = the current
  low byte of the app's existing "excitement counter"
  (`firmware/esp32-idf/main/main.c`) — ties the CAN work directly into the
  counter/threading demo already built, and gives a payload that visibly
  changes frame-to-frame so it's obvious real traffic is flowing, not a
  stuck value. Send once per `heartbeat_task` tick (every 1s by default,
  adjustable via the existing `rate N` CLI command) to start; a `can send
  <id> <hex>` CLI command for ad hoc sends can come later.
- **Linux → ESP32**: `cansend can0 321#DEADBEEF` (arbitrary distinct ID and
  payload, easy to recognize) — on receipt, the ESP32 logs the frame and
  bumps the excitement counter (an "interesting operation," per the
  counter's whole reason for existing), same as the button/heartbeat
  sources already do.

Once basic bidirectional traffic is confirmed, next steps (not part of
this bring-up): a real CLI command for sending arbitrary frames, and
maybe wiring the received CAN ID/payload into the counter mechanic more
specifically (e.g. different IDs bump the counter by different amounts).

**On the IDs `0x123`/`0x321`:** deliberately picked from the gap
[can-message-spec.md](can-message-spec.md) leaves unallocated
(`0x100`–`0x6FF`, everything between the defined classes and the
`0x700`–`0x7FF` diagnostics band) — this is connectivity bring-up, not a
real message, so it shouldn't borrow meaning (or collide with) an actual
class like control or status/telemetry. When real motor/controller/panel
node messages get defined, their IDs get chosen from the proper class and
registered in that spec's allocation table, not carried over from here.

## Linux box commands

Assumes `make setup-socketcan` has already been run (idempotent, safe to
rerun) and `can0` shows up:

```
ip -details link show can0          # confirm state UP, bitrate 500000
```

**Listen:**

```
candump can0                        # everything, live
candump -td can0                    # with delta timestamps between frames
candump can0,123:7FF                # filter to just ID 0x123 (exact-match mask)
```

**Send:**

```
cansend can0 321#DEADBEEF           # one-shot frame, ID 0x321, 4 data bytes
cangen can0 -g 100 -I 123 -L 1      # generate a frame every 100ms, ID 0x123, 1 data byte -- traffic generator for testing without the ESP32 side ready yet
```

`can-utils` (`candump`/`cansend`/`cangen`) needs to be installed on the
Linux box if it isn't already (`apt install can-utils` on Debian/Ubuntu) —
not yet confirmed present, check before relying on it.

## Step-by-step sequence

1. Wire the SN65HVD230 to the ESP32 per the pin table above — **no bus
   connection yet**.
2. Flash a Stage A (`TWAI_MODE_NO_ACK`) build, confirm self-test loopback
   passes.
3. Confirm/install `can-utils` on the Linux box; confirm `can0` is up at
   500 kbit/s (`make setup-socketcan` if not already done).
4. Wire CANH/CANL between the ESP32 transceiver and the CANable2, with
   termination at both ends.
5. Flash a Stage B (`TWAI_MODE_NORMAL`) build sending the counter-byte
   frame on `0x123` every heartbeat tick.
6. `candump can0` on the Linux box — confirm frames arrive with the
   expected ID and a changing payload.
7. `cansend can0 321#DEADBEEF` from the Linux box — confirm the ESP32
   logs receipt and the excitement counter bumps (`counter` CLI command,
   or watch the log line).
8. Note actual results (bitrate/wiring issues, error counts, anything
   that didn't match this plan) in a dated `../notebook/` entry.

## Open questions

- Exact SN65HVD230 breakout in hand — confirm its `Rs`/slew-rate pin
  wiring and whether it has a populated termination jumper, both vary by
  vendor.
- Physical distance/routing between the ESP32 (wherever it's flashed from
  — the Mac's USB) and the Linux box's CANable2 — affects how much the
  "keep it short for bring-up" wiring advice above matters in practice.
- Whether `can-utils` is already installed on the Linux box.
