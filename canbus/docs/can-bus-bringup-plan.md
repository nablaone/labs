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

### Stage A — transceiver self-test (`TWAI_MODE_NO_ACK`) — CONFIRMED PASSING 2026-08-26

Answers "is the transceiver actually connected and wired correctly?"
*without* needing the Linux box or bus connected at all. Implemented in
`firmware/esp32-idf/main/main.c` (`can_selftest()`, exposed as the
`can loop`/`can xcvr` CLI commands), confirmed against real hardware: the
Waveshare 3945 (SN65HVD230) transceiver on GPIO21/22 (D21/D22).

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

twai_message_t tx_msg = {
    .self = 1,  // REQUIRED -- see gotcha below
    .identifier = 0x100,
    .data_length_code = 1,
    .data = { 0xA5 },
};
twai_transmit(&tx_msg, pdMS_TO_TICKS(100));
// twai_receive() with a short timeout -- getting the same frame back
// confirms TX pin, RX pin, and the transceiver chip are all wired and
// working.
```

**Gotcha that cost real debugging time:** `TWAI_MODE_NO_ACK` alone does
*not* cause self-reception. `twai_transmit()` reports success (frame
"sent," zero errors) regardless of whether it actually reached the
transceiver correctly — that status is about the controller accepting the
frame, not about physical loopback. Without `.self = 1` set on the
message, the frame is never queued for the local `twai_receive()` to see,
*no matter how good the wiring is* — confirmed by the fact that even a
bare jumper wire from GPIO21 straight to GPIO22 (bypassing the
transceiver entirely) still failed identically until this flag was added.
Every failure before that fix looked exactly like a hardware problem
(`msgs_to_tx` draining to 0, zero errors on every counter) and wasn't
one. Reference: ESP-IDF's own
`examples/peripherals/twai/twai_self_test`, which sets this flag.

- **Pass** (frame loops back cleanly): transceiver + GPIO wiring is good —
  move to Stage B.
- **Fail** (`twai_receive()` times out, or `twai_get_status_info()` shows
  bus/bit errors) *with `.self = 1` already set*: now a real wiring
  problem — check TX/RX aren't swapped, transceiver VCC/GND, and that
  CANH/CANL aren't shorted or floating in a way that corrupts the
  loopback signal.

This isolates "is the ESP32 half of this working" from "is there a live
peer" — worth doing before ever touching the Linux box, since it rules out
a whole class of wiring mistakes on its own.

### Stage B — real bus, `TWAI_MODE_NORMAL` — CONFIRMED WORKING 2026-08-26

Implemented as `can send`/`can sniff` CLI commands
(`firmware/esp32-idf/main/main.c`), with `cantool.py` (same directory) as
the Mac-side counterpart — same `<id_hex>#<data_hex>` frame syntax on
both, so a frame copy-pastes cleanly between them. The driver now runs in
`TWAI_MODE_NORMAL` by default (switching to `TWAI_MODE_NO_ACK` only
temporarily for `can loop`/`can xcvr`, then back — see `can_reinit()`).

**Confirmed both directions against a CANable2 on the Mac** (not the
Linux box — see the open question below): `can send 123#DEADBEEF` on the
ESP32 → received via `cantool.py sniff` on the Mac; `cantool.py send
321#CAFEBABE` on the Mac → received via `can sniff` on the ESP32. Real
bus, real ACKs, both ways.

With the CANH/CANL run made to the CANable2 and both ends terminated,
switch to normal mode and bring `can0` up on the Linux box (`ip link set
can0 up type can bitrate 500000` — already handled automatically by
`setup-socketcan.sh` on hotplug) if using that path instead of the Mac.

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

**Implemented as ad hoc `can send`/`can sniff` CLI commands** (and
`cantool.py` on the Mac side) rather than auto-sending the counter on
every heartbeat tick — simpler to get Stage B working first, and ad hoc
send/sniff is more useful for bring-up than a fixed periodic payload
anyway. Confirmed both directions with arbitrary test frames (see Stage B
above: `123#DEADBEEF` ESP32→Mac, `321#CAFEBABE` Mac→ESP32).

**Not yet done** (still a reasonable next step, not part of this
bring-up): tying CAN traffic into the excitement-counter mechanic
already built — e.g. auto-sending the counter's low byte on every
`heartbeat_task` tick, and/or bumping the counter on receipt of any CAN
frame (an "interesting operation," per the counter's whole reason for
existing), same as the button/heartbeat sources already do.

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

## Step-by-step sequence — DONE 2026-08-26, all in one build

Turned out not to need separate Stage A/B builds — `can_reinit()` switches
the driver mode at runtime, so one build does both:

1. ~~Wire the SN65HVD230 to the ESP32~~ — done (D21/D22, see pin table).
2. ~~Confirm self-test loopback passes~~ — done (`can xcvr`; see the
   `.self = 1` gotcha above and [../notebook/2026-08-26.md](../notebook/2026-08-26.md)
   for the debugging detour it caused).
3. ~~Wire CANH/CANL to a CAN peer, confirm real traffic~~ — done, against
   a CANable2 on the Mac rather than the Linux box's `can0` (open question
   below): `can send`/`can sniff` on the ESP32, `cantool.py send`/
   `cantool.py sniff` on the Mac, confirmed both directions.
4. Note actual results in a dated `../notebook/` entry — done, see
   [../notebook/2026-08-26.md](../notebook/2026-08-26.md).

**Not yet done**: the equivalent run against the Linux box's `can0` (this
plan's original target) — worth doing at some point to confirm that path
too, especially since `can-utils` presence there is still unconfirmed
(see below).

## Open questions

- ~~Exact SN65HVD230 breakout in hand~~ — resolved: Waveshare 3945.
  [Schematic](https://files.waveshare.com/upload/c/c5/SN65HVD230-CAN-Board-Schematic.pdf)
  confirms a **fixed, always-on 120Ω** across CANH/CANL (R2, not a
  jumper) and `Rs` biased through a 10KΩ resistor to GND (slope-control
  mode, not standby) — no jumpers to set on this board at all.
- **Deviation from this plan**: Stage A bring-up/debugging actually used a
  CANable2 plugged directly into the Mac (via `python-can`'s `slcan`
  backend, since it's running slcan/CDC-ACM firmware — see
  [../CLAUDE.md](../CLAUDE.md)'s slcan fallback), not the Linux box's
  `can0`. Handy for a quick independent listener during bring-up debugging
  (see the 2026-08-26 notebook entry); Stage B as planned still targets
  the Linux box's `can0` — revisit whether that's still the right call
  once Stage B actually starts, given the Mac path already works.
- Physical distance/routing between the ESP32 (wherever it's flashed from
  — the Mac's USB) and the Linux box's CANable2 — affects how much the
  "keep it short for bring-up" wiring advice above matters in practice.
- Whether `can-utils` is already installed on the Linux box.
