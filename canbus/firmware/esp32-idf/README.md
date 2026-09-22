# esp32-idf

Plain ESP-IDF app for the ESP32 DevKit, replacing the earlier
[Zephyr version](../zephyr-canbus/) — see [../../CLAUDE.md](../../CLAUDE.md).
ESP32-only now: no Zephyr, no Pico, no shared board-agnostic abstraction to
maintain. Everything runs through Docker via the official `espressif/idf`
image; no host IDF install needed for building.

This app is a FreeRTOS threading/shared-memory demo, not just a blink loop
— it's here to exercise the concurrency model, task/file structure, and
CLI/logging pattern every future node (motor/controller/panel, see
[../../docs/project-charter.md](../../docs/project-charter.md)) is meant to
reuse, using a 32-bit "excitement counter" as a stand-in for later
CAN-driven events.

## Module layout (`main/`)

One file (`.c`/`.h`) per module. Each task module follows the same shape:
an `xxx_task_init()` (one-time hardware setup, called from `app_main()`
before the task starts) and `xxx_task()` (the actual FreeRTOS task loop,
passed to `xTaskCreate()`). Non-task hardware modules (`can.c`) follow the
same file-per-module convention without the task-loop part, since they're
driven by CLI commands rather than a background loop. `main.c` itself is
just orchestration — init each enabled module, register CLI commands,
create tasks — and should stay identical across nodes; only
`node_config.h` (pins, which modules are enabled) and which module files
exist are meant to change per node.

- **`node_config.h`** — the file a new node copies and edits: compile-time
  `NODE_ENABLE_*` flags for which modules this build includes, pin
  assignments, `FIRMWARE_VERSION`. Per-*unit* identity (which two units
  running the identical binary need to differ on) is runtime/NVS-backed
  instead — see `identity.c` below.
- **`state.c`/`.h`** — the shared "excitement counter", mutex-protected
  (`SemaphoreHandle_t`) since tasks run concurrently across the ESP32's
  two cores. Owns the `counter` CLI command, and also publishes the same
  value as the `counter` metric (see below) from `state_init()`/
  `state_counter_increment()`.
- **`metrics.c`/`.h`** — generic key → typed-value status registry
  (`bool`/`int32_t`/`float`/short string), mutex-protected like `state.c`.
  Any module can `metrics_register_*()` a key (typically from its own
  `_init()`) and `metrics_set_*()` it as it runs; dotted, module-prefixed
  keys (`"identity.role"`) avoid collisions where a name isn't already
  unambiguous on its own (`counter`, `version`). Owns the `metrics` CLI
  command, which dumps the whole table — the only consumer, so there's no
  public read/getter API. `main.c` calls `metrics_init()` **first**, before
  `state_init()`/`identity_init()`, since both of those register a metric
  from their own `_init()` and need the registry's mutex to already exist.
  Keys registered so far: `main.c` registers `version` (`FIRMWARE_VERSION`,
  set once, never changes); `state.c` registers/updates `counter`;
  `identity.c` registers `identity.role` in `identity_init()` (`"unset"`
  until configured) and updates it in `identity_role_set()`. Migrating
  `pingpong_task`'s status/seq/rtt to also publish here is still a
  follow-up, not done in this change; `display_task`'s
  LCD tabs keep reading those directly either way, since that's a
  different, latency-sensitive consumer this registry isn't meant to
  replace.
- **`heartbeat_task.c`/`.h`** — ticks every `HB_MS_PER_TICK` (100ms);
  increments the counter once every 5 ticks (500ms), and separately
  toggles the LED whenever a read-back shows the counter actually
  changed — which also catches a change from elsewhere (`can_rx_task`
  bumping it on CAN RX), not just this task's own tick. No runtime-
  configurable period (no CLI command owned here) — used to be two
  separate tasks (`led_task` polling the counter every 100ms,
  `heartbeat_task` ticking it on its own via a `rate`-settable period)
  folded into one after `button_task` was removed.
- **`display_task.c`/`.h`** — the *only* module that calls `lcd_display()`
  (see below) — every other module publishes its own state through a
  small getter (`state_counter_read()`, `pingpong_task_status_read()`,
  ...) and this task is the sole place that reads those and formats them
  for the physical display, one rotating "tab" per `DISPLAY_CYCLE_MS`
  (2s): `version` (`FIRMWARE_VERSION`), `counter` (the excitement
  counter), a static `hello`/`world`, and (if `NODE_ENABLE_PINGPONG`) a
  `ping` tab reading `identity_role_read()`/`identity_node_id_read()`
  and `pingpong_task_status_read()` — `"<ping|pong> id<N>"` on line 1,
  the latest exchange's outcome on line 2 (`"seq3 12ms"` /
  `"seq3 timeout"` / `"seq3 replied"` / `"unconfigured"`). Each tab is
  logged too. Only the counter's turn also broadcasts over CAN (if
  enabled): 4 bytes, little-endian, on ID `0x7F0`. Deliberately a high
  11-bit ID — CAN arbitration is lowest-ID-wins, so this is the
  *lowest*-priority traffic on the bus, as fits a non-critical periodic
  debug broadcast. Sits in
  [../../docs/can-message-spec.md](../../docs/can-message-spec.md)'s
  diagnostics band (`0x700`–`0x7FF`) but away from that doc's `0x7NN`
  `NODE_HEARTBEAT` pattern — this is an experimental broadcast, not a
  registered message.
- **`can.c`/`.h`** — TWAI driver + self-test + send/sniff, plus
  `can_send(id, data, len)` and the `can_send_u32(id, value)` convenience
  wrapper (little-endian 32-bit payload) other modules call directly
  (`display_task` uses it for its counter broadcast). Also
  owns `can_rx_task` — the sole reader of `twai_receive()` once running,
  fanning every frame out to one software queue that `can_receive()`
  drains from. The `can sniff` CLI command and whichever of
  `ping_task`/`pong_task` is running are both `can_receive()` consumers;
  running `can sniff` while one of them is active will steal frames from
  it (a manual diagnostic competing with a task, not meant to run both at
  once). `can_rx_task` also bumps the excitement counter once for every
  frame it takes off the bus — see `state.c`'s own doc comment ("CAN
  frame received" is one of the events the counter is meant to track);
  neither `ping_task` nor `pong_task` bumps it again itself when it later
  matches that same frame to a ping/pong exchange, to avoid
  double-counting.
- **`pingpong_task.c`/`.h`** — the two-node bring-up exercise, split into
  two task entry points: `ping_task` sends a `PING` (ID `0x120`) and waits
  for the peer's `PONG` (`0x121`) echoing the same sequence number back,
  logging the round trip; `pong_task` does the reverse, replying to every
  `PING` it sees. Which role a board plays is decided once, at boot, by
  `main.c`'s switch on `identity.h`'s runtime `role` — neither task
  re-checks it itself, so `config set-role` (see below) reboots the board
  immediately rather than switching roles live. Requires `NODE_ENABLE_CAN`.
  Doesn't touch the LCD itself — publishes the latest exchange
  (status/seq/rtt) through the mutex-protected `pingpong_task_status_read()`
  (set up by whichever of `ping_task_init()`/`pong_task_init()` `main.c`
  called for the role that started — both just create the same shared
  mutex, so either is enough), the same
  "own module publishes, one place renders" shape `state.c` already uses
  for the counter; `display_task`'s `ping` tab (above) is what actually
  shows it.
- **`lcd_task.c`/`.h`** — drives a 16x2 HD44780 character LCD over a
  PCF8574 I2C backpack. Exposes `lcd_display(line1, line2)` for other
  modules to call directly (mutex-protected, non-blocking — it only
  updates in-memory state and returns; the actual I2C write happens
  asynchronously off `lcd_task`'s own loop, which wakes every
  `LCD_UPDATE_MS` and only touches the display if the content actually
  changed since the last write). `lcd_task_init()` scans the whole
  7-bit I2C address range at boot and logs what it finds — a bring-up
  aid for backpacks that don't ship at the expected `LCD_I2C_ADDR` —
  but that scan is informational only. `i2c_master_probe()` (the
  scan's underlying call) was found on real hardware to report "no
  response" for every address, including a confirmed-good, confirmed-
  wired backpack independently verified alive at the same address via
  a Raspberry Pi's `i2cdetect` — so presence is decided by a real
  `i2c_master_transmit()` write instead. That bus also turned out to be
  marginal on real writes (occasional genuine `I2C software timeout`),
  confirmed to be weak pull-ups (only the ESP32's internal ~45kΩ ones
  were engaged) once adding real external pull-up resistors cut the
  failure rate drastically — standard 100kHz/`glitch_ignore_cnt=7`
  settings, with the pull-ups, produce noticeably *fewer* failures than
  an earlier attempt at a slower clock + higher glitch tolerance did
  without them, so that wasn't a useful mitigation on its own.
  `pcf8574_write()` still retries each byte a few times with a short
  gap between attempts as a safety net (the failure rate didn't reach
  zero), and `hd44780_init_sequence()` explicitly space-fills both rows
  on top of the normal HD44780 clear command, rather than trusting a
  single clear to leave a clean screen on a bus that can still glitch.
- **`identity.c`/`.h`** — per-unit runtime identity (`node_id`, `role`),
  stored in NVS rather than `node_config.h` since the goal is one shared
  binary flashed to every board, differentiated only by what's set over
  the CLI. Owns the `config` command. Survives `make flash` (which only
  rewrites the app partition, not NVS) — a board keeps its identity
  across rebuilds; only `esptool erase_flash` clears it. `identity_role_set()`
  reboots the board (`esp_restart()`) on every successful change — see
  `config set-role` below.
- **`console.c`/`.h`** — `esp_console`/linenoise setup and the
  `console_task` loop (below), plus the core `help`/`version`/`exit`
  commands common to every node. This is "the same debug strategy" every
  node's firmware shares.

`console_task` waits for Enter on the serial console; once seen, it
silences logging (`esp_log_level_set("*", ESP_LOG_NONE)`) and runs an
`esp_console`/linenoise REPL until the `exit` command is typed, at which
point logging resumes and `console_task` goes back to waiting for the next
Enter. Commands (registered by the module that owns each one):

- **`help`** — list all commands (built into `esp_console`).
- **`version`** — firmware + ESP-IDF version.
- **`counter`** — current excitement counter value.
- **`metrics`** — dump the whole `metrics.c` status table (key, type,
  value); `version`, `counter`, and `identity.role` are registered by
  default, more as other modules adopt it.
- **`config show`** — print this board's node_id/role (`unset` if never
  configured).
- **`config set-id <n>`** — set and persist (NVS) this board's node_id
  (0-255). Does not reboot — nothing reads `node_id` yet.
- **`config set-role <ping|pong>`** — set and persist (NVS) this board's
  role, then **reboot immediately** so `main.c`'s boot-time switch (see
  `pingpong_task.c` above) picks it up.
- **`can loop`** — self-test with D21 jumpered directly to D22 (no
  transceiver) — isolates the TWAI peripheral/firmware from the hardware.
- **`can xcvr`** — the same self-test, but with the SN65HVD230 wired
  normally — confirms the transceiver + its wiring.
- **`can sniff`** — print received frames as `<id_hex>#<data_hex>` for a
  fixed 30s, then return.
- **`can send <id_hex>#<data_hex>`** — transmit one frame, e.g.
  `can send 123#DEADBEEF`.
- **`lcd <line1> <line2>`** — show text on the LCD (16 chars/line,
  space-padded), e.g. `lcd hello world`.
- **`exit`** — leave CLI mode, resume logging.

The driver comes up in `TWAI_MODE_NORMAL` at boot (GPIO21/22, 500 kbit/s)
after running the self-test once automatically (temporarily switching to
`TWAI_MODE_NO_ACK` and back — see `can_reinit()`/`can_run_selftest()` in
`main/can.c`), logging PASS/FAIL. `can loop`/`can xcvr` do the same
temporary switch on demand. See
[../../docs/can-bus-bringup-plan.md](../../docs/can-bus-bringup-plan.md)
for the reasoning and Stage A/B plan.

**Confirmed on real hardware, 2026-08-26**: Stage A (SN65HVD230
transceiver, D21/D22 wiring, `can xcvr`) and Stage B (`can send`/
`can sniff` against a CANable2 on the actual bus, both directions) both
pass. Self-test needs a frame with the self-reception flag set
(`.self = 1`) to actually queue a received frame under
`TWAI_MODE_NO_ACK` — easy to miss (cost real debugging time, see
[../../notebook/2026-08-26.md](../../notebook/2026-08-26.md)), ESP-IDF's
own `examples/peripherals/twai/twai_self_test` is the reference.

**`cantool.py`** (same directory) is the Mac-side counterpart —
sniffs/sends on a CANable-style dongle over its slcan serial port
directly (no SocketCAN needed on macOS), using the same
`<id_hex>#<data_hex>` frame syntax as the CLI commands above, so a frame
copy-pastes cleanly between the two. `make can-sniff` (Ctrl-C to stop) /
`make can-send FRAME=123#DEADBEEF` (own venv, auto-created — see
Usage below). Linux/SocketCAN intentionally not covered here; that
already has the real thing (`candump`/`cansend`), see
[../../scripts/setup-socketcan.sh](../../scripts/setup-socketcan.sh).

## Wiring

LED is onboard, no breadboard wiring needed:

- LED: onboard LED on **GPIO2**.

A strapping pin (sampled at boot to select flash/boot mode), but its
light loading doesn't disturb boot-mode sensing in practice.

The onboard **BOOT** button (GPIO0) is unused by firmware now that
`button_task` is gone — free for something else later.

CAN needs the external SN65HVD230 transceiver wired in — GPIO21 (TX) /
GPIO22 (RX), silkscreened **D21**/**D22** on this DevKit V1-style board.
Full wiring table, termination, and bring-up sequence:
[../../docs/can-bus-bringup-plan.md](../../docs/can-bus-bringup-plan.md).

The LCD needs a PCF8574 I2C backpack wired in — GPIO26 (SDA) / GPIO27
(SCL), address `0x27` by default (some backpacks ship at `0x3F` —
`LCD_I2C_ADDR` in `node_config.h` if yours differs). `lcd_task_init()`
logs a (non-fatal) warning at boot if a real write to that address
fails, so the rest of the node still comes up fine before the LCD is
wired.

## Two-node bring-up (ping/pong)

Two boards, one binary — `identity.c`'s runtime `role` (not a rebuild) is
what makes them behave differently. Since only one USB cable is in use
(swapped between boards to flash each), and macOS reuses the same
`/dev/cu.usbserial-XXXX` path for either one, there's no way to tell
which physical board is plugged in from the port name alone. Each
board's MAC is fixed and unique, printed by `esptool` on every
connect (`make flash`/`make nvs-flash-a`/etc.'s own output, or
`esptool --chip esp32 -p PORT read_mac`) — check it against this table
before flashing/provisioning rather than assuming:

| Board | MAC |
|---|---|
| A (`node_id=0`, `role=ping`) | `58:2a:bd:80:87:d4` |
| B (`node_id=1`, `role=pong`) | `20:9b:a9:6f:bc:90` |

1. Wire the two boards' SN65HVD230 transceivers together: CAN-H to CAN-H,
   CAN-L to CAN-L, common GND. 120Ω termination at both physical ends —
   if a CANable stays in the loop as a passive sniffer, it sits mid-bus
   (no termination there); the two ESP32 transceivers become the bus's
   two ends.
2. `make build` once; `make flash PORT=...` both boards with the exact
   same binary.
3. On board A's CLI: `config set-id 0`, then `config set-role ping`.
4. On board B's CLI: `config set-id 1`, then `config set-role pong`.
   `config set-id` just persists to NVS; `config set-role` persists too
   but then reboots the board immediately (`identity_role_set()` calls
   `esp_restart()`) so `main.c`'s boot-time switch picks up the new role
   right away — no manual power-cycle needed, but do set the id first on
   each board since the role change ends the CLI session.
5. Watch the logs (or LCDs, if wired): board A logs `seq=N rtt=Xms` once
   a second; board B logs `seq=N replied` as it echoes each one back.
   `can sniff` from either board's CLI (or `cantool.py sniff` from the
   Mac) confirms the frames on the wire independently of the app logic —
   but see `can.c`'s note above about it competing with whichever of
   `ping_task`/`pong_task` is running for the same queue.

`config set-id` isn't used by `ping_task`/`pong_task` yet (only `role` is) — it's
there for whichever future exercise needs to tell the two nodes apart by
more than role (e.g. a 3+ node test, or once messages carry a sender ID).

### Alternative to the CLI: pre-provisioning identity via a CSV

Steps 3/4 above go through the `config` CLI, which is the default because
it needs no extra tooling. `nvs-board-a.csv` / `nvs-board-b.csv` (same
directory) are the declarative alternative — one `key,type,encoding,value`
row per NVS key, in the format ESP-IDF's own `nvs_partition_gen.py`
expects, pre-filled with each board's `identity` namespace (`node_id`,
`role` — `0`/`1` for `ping`/`pong`, matching `identity_role_t` in
`identity.h`). Useful for factory-style provisioning without ever
touching the serial console, or for restoring identity after an
`esptool erase_flash`. Generate and flash (each board's default `nvs`
partition is 24576 bytes / `0x6000`, per the default single-app
partition table — confirm with `idf.py partition-table` if that's ever
changed):

```
make nvs-flash-a PORT=/dev/tty.usbserial-XXXX   # board A: node_id=0, role=ping
make nvs-flash-b PORT=/dev/tty.usbserial-XXXX   # board B: node_id=1, role=pong
```

(`make nvs-board-a.bin`/`make nvs-board-b.bin` generate just the image,
without flashing, if that's ever useful on its own.) This *replaces* the
whole NVS partition, so it also overwrites anything else stored there —
fine here since `identity` is the only thing this app keeps in NVS.
Verify with `config show` over the CLI afterward. Check the board's MAC
first (see the table above) — the CSV path has no cross-check against
what's actually plugged in, unlike `config set-id`/`set-role` where
you're watching the CLI respond live.

## Future ideas

Not implemented, no hardware ordered beyond what's already in the
[Hardware](../../CLAUDE.md#hardware) section — rough next exercises for
this lab, not full designs:

- **Potentiometer reading** — one of the ESP32's ADC1 channels (stay off
  ADC2's pins; per Espressif errata it can't be read at all while WiFi is
  active, and even with WiFi off here, ADC1 is the simpler/less-caveated
  choice), via `esp_adc`/`adc_oneshot`. The simplest possible analog-input
  exercise, and a stand-in for any real setpoint pot before wiring
  anything scooter-specific.
- **Hall sensor throttle reading** — a twist-grip e-scooter throttle is
  usually a 3-wire *analog* Hall sensor (VCC/GND/signal, signal roughly
  0.8–4.2V proportional to twist) — not the pulse-counting Hall input
  [docs/reference-node.md](../../docs/reference-node.md) already
  describes for wheel-speed sensing (same sensor technology, different
  wiring/reading entirely: ADC read here vs. GPIO interrupt/pulse-count
  there). Same ADC path as the potentiometer above; check the signal
  range against the ESP32 ADC's ~3.3V max first (a 5V-railed throttle
  needs a voltage divider). Broadcasting it as a real message would land
  in [docs/can-message-spec.md](../../docs/can-message-spec.md)'s
  Setpoints band (`0x040–0x07F`, "panel throttle/direction").
- **DC motor control (4-wire)** — an H-bridge driver (direction + PWM
  speed, however that node's 4 wires end up split between motor leads
  and control signals depending on the driver chosen) commanded by a
  `MOTOR_COMMAND`-shaped CAN message —
  [docs/can-message-spec.md](../../docs/can-message-spec.md) already
  sketches `0x011 MOTOR_COMMAND` (command_type/direction/setpoint/flags)
  as the template. First real actuator on the bus; would want
  `SPEED_FEEDBACK` (`0x020`) too once there's something to measure speed
  from.
- **Third ESP32 node** — extend the two-node ping/pong bus to three.
  Would finally give `identity.c`'s `node_id` (persisted, but unused
  since only `role` drives behavior so far) a real job telling nodes
  apart, and exercise genuine multi-transmitter bus arbitration instead
  of just two nodes taking turns.

## Usage

```
make build                        # build via Docker, IDF_TARGET defaults to esp32
make flash PORT=/dev/tty.usbserial-XXXX   # real esptool flash, native (no Docker)
make monitor PORT=/dev/tty.usbserial-XXXX # watch the serial console (minicom, native)
make shell                        # drop into the dev container (idf.py available)
make can-sniff                    # Mac-side sniff via cantool.py, Ctrl-C to stop (own venv, auto-created)
make can-send FRAME=123#DEADBEEF  # Mac-side send via cantool.py
make clean
```

`make can-sniff`/`make can-send` create `.cantool-venv/` on first use
(macOS's Homebrew Python refuses unmanaged `pip install`, PEP 668) and
install `python-can`/`pyserial` into it — nothing touches the host Python.
Auto-detects a `/dev/cu.usbmodem*` dongle; override with `CAN_PORT=` if
more than one is attached.

First `make build` triggers the Docker image pull (the official
`espressif/idf` image — toolchain + IDF baked in, multi-GB). Later builds
reuse the cached image and are incremental.

`CMakeLists.txt` sets `COMPONENTS main` before `project()` — by default
`idf.py` compiles ESP-IDF's *entire* bundled component set on a clean
build (WiFi provisioning, MQTT, SPIFFS, FAT, JSON, coredump, etc.), none
of which this app (USB-serial CLI + GPIO + I2C + CAN only) actually
needs or links into the final binary (`idf.py size-components` shows
the real, much smaller linked set) — it's pure build-time cost. This
setting restricts the component search to `main` and whatever it
actually (transitively) requires, cutting a clean build from ~980 to
~580 compile steps.

`make flash` runs `esptool` **natively on the host**, not through Docker —
a USB-serial connection (the DevKit's onboard CP2102 chip) has no special
passthrough problems on macOS, unlike SWD debug probes, so there's no
Linux-box detour needed here (see [../../CLAUDE.md](../../CLAUDE.md)).
Requires `esptool` installed on the host (`pip install esptool` or, on
Debian, `apt install esptool`) and the board's serial port
(`ls /dev/tty.usbserial-*` on macOS, `/dev/ttyUSB*` on Linux, once
plugged in). It reads `build/flash_args` (generated by `idf.py build`) for
the exact offsets/flags rather than hardcoding them.

`make monitor` uses `minicom` directly instead of `idf.py monitor` so it
doesn't need a host IDF install either — same 115200 8N1 as ESP-IDF's
default console. It writes a minimal per-user minicom profile at
`~/.minirc.canbus-esp32-idf` the first time it runs (only if that file
doesn't already exist) that disables hardware and software flow control —
minicom's defaults there are a common cause of "I don't see what I type" on
ESP32 boards, since the board doesn't wire real UART flow control at all
and minicom just withholds display waiting on it. Ctrl-A X to exit.

Press Enter on the console at any time to switch into CLI mode (logging
pauses, `canbus>` prompt appears); type `exit` to leave it and resume
logging. See the CLI command list above.
