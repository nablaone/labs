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
- **`state.c`/`.h`** — the shared "excitement counter" + heartbeat period,
  mutex-protected (`SemaphoreHandle_t`) since tasks run concurrently
  across the ESP32's two cores. Owns the `counter` CLI command.
- **`led_task.c`/`.h`** — every 100ms, reads the counter; toggles the LED
  if it changed since the last read.
- **`heartbeat_task.c`/`.h`** — increments the counter every 1s on its
  own. Owns the `rate` CLI command (sets its own period).
- **`button_task.c`/`.h`** — polls the button every 100ms; increments the
  counter on every poll where it reads pressed (holding it down keeps
  incrementing, not just a single bump per press), and (if CAN is
  enabled) broadcasts the new value too via `can_send_u32_nowait()` (0
  timeout -- doesn't block this task's own poll loop waiting for TX
  queue space if the bus is busy or has no listener): 4 bytes,
  little-endian, on ID `0x110` — a scratch ID in
  [../../docs/can-message-spec.md](../../docs/can-message-spec.md)'s
  unallocated gap (`0x100`–`0x6FF`), numerically lower (so
  higher-priority) than `display_task`'s `0x7F0` since a button press is
  a more immediate event than periodic telemetry.
- **`display_task.c`/`.h`** — the *only* module that calls `lcd_display()`
  (see below) — every other module publishes its own state through a
  small getter (`state_counter_read()`, `pingpong_task_status_read()`,
  ...) and this task is the sole place that reads those and formats them
  for the physical display, one rotating "tab" per `DISPLAY_CYCLE_MS`
  (2s): `version` (`FIRMWARE_VERSION`), `counter` (the excitement
  counter), a static `hello`/`world`, and (if `NODE_ENABLE_PINGPONG`) a
  `ping` tab reading `identity_mode_read()`/`identity_node_id_read()`
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
  (`display_task` uses it for its counter broadcast). `can_send_nowait()`/
  `can_send_u32_nowait()` are the same but with a 0 timeout on
  `twai_transmit()` (`button_task` uses these) — that timeout is only
  about waiting for room in the driver's TX queue, not the frame's
  bus-level ACK, but it's still real blocking a frequent caller like a
  held-down button shouldn't eat. Also
  owns `can_rx_task` — the sole reader of `twai_receive()` once running,
  fanning every frame out to one software queue that `can_receive()`
  drains from. The `can sniff` CLI command and `pingpong_task` are both
  `can_receive()` consumers; running `can sniff` while pingpong is active
  will steal frames from it (a manual diagnostic competing with a task,
  not meant to run both at once). `can_rx_task` also bumps the
  excitement counter once for every frame it takes off the bus — see
  `state.c`'s own doc comment ("CAN frame received" is one of the
  events the counter is meant to track); `pingpong_task` doesn't bump it
  again itself when it later matches that same frame to a ping/pong
  exchange, to avoid double-counting.
- **`pingpong_task.c`/`.h`** — the two-node bring-up exercise: sends a
  `PING` (ID `0x120`) and waits for the peer's `PONG` (`0x121`) echoing
  the same sequence number back, logging the round trip; or the reverse,
  replying to every `PING` it sees. Which role — decided by `identity.h`'s
  runtime `mode`, re-read every loop iteration, so `config set-mode` (see
  below) switches a running node between ping and pong with no reboot.
  Requires `NODE_ENABLE_CAN`. Doesn't touch the LCD itself — publishes
  the latest exchange (status/seq/rtt) through the mutex-protected
  `pingpong_task_status_read()`, the same "own module publishes, one
  place renders" shape `state.c` already uses for the counter;
  `display_task`'s `ping` tab (above) is what actually shows it.
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
- **`identity.c`/`.h`** — per-unit runtime identity (`node_id`, `mode`),
  stored in NVS rather than `node_config.h` since the goal is one shared
  binary flashed to every board, differentiated only by what's set over
  the CLI. Owns the `config` command. Survives `make flash` (which only
  rewrites the app partition, not NVS) — a board keeps its identity
  across rebuilds; only `esptool erase_flash` clears it.
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
- **`rate N`** — set `heartbeat_task`'s period to `N*100ms`.
- **`config show`** — print this board's node_id/mode (`unset` if never
  configured).
- **`config set-id <n>`** — set and persist (NVS) this board's node_id
  (0-255).
- **`config set-mode <ping|pong>`** — set and persist (NVS) this board's
  mode.
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

LED and button are onboard, no breadboard wiring needed:

- LED: onboard LED on **GPIO2**.
- Button: onboard **BOOT** button, wired to **GPIO0**.

Both are strapping pins (sampled at boot to select flash/boot mode) — the
earlier Zephyr app avoided them in favor of an external button on GPIO33,
but once the app is running, GPIO0 reads like any other input (it's only
sampled at reset), and the onboard LED's light loading on GPIO2 doesn't
disturb boot-mode sensing in practice. GPIO0 already has an external
pull-up on the board for the BOOT button.

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

Two boards, one binary — `identity.c`'s runtime `mode` (not a rebuild) is
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
| A (`node_id=0`, `mode=ping`) | `58:2a:bd:80:87:d4` |
| B (`node_id=1`, `mode=pong`) | `20:9b:a9:6f:bc:90` |

1. Wire the two boards' SN65HVD230 transceivers together: CAN-H to CAN-H,
   CAN-L to CAN-L, common GND. 120Ω termination at both physical ends —
   if a CANable stays in the loop as a passive sniffer, it sits mid-bus
   (no termination there); the two ESP32 transceivers become the bus's
   two ends.
2. `make build` once; `make flash PORT=...` both boards with the exact
   same binary.
3. On board A's CLI: `config set-id 0`, `config set-mode ping`.
4. On board B's CLI: `config set-id 1`, `config set-mode pong`.
   Both `config set-*` calls persist to NVS and update the running
   node's identity immediately — `pingpong_task` re-reads `mode` every
   loop iteration, so no reboot is needed for either board.
5. Watch the logs (or LCDs, if wired): board A logs `seq=N rtt=Xms` once
   a second; board B logs `seq=N replied` as it echoes each one back.
   `can sniff` from either board's CLI (or `cantool.py sniff` from the
   Mac) confirms the frames on the wire independently of the app logic —
   but see `can.c`'s note above about it competing with `pingpong_task`
   for the same queue.

`config set-id` isn't used by `pingpong_task` yet (only `mode` is) — it's
there for whichever future exercise needs to tell the two nodes apart by
more than role (e.g. a 3+ node test, or once messages carry a sender ID).

### Alternative to the CLI: pre-provisioning identity via a CSV

Steps 3/4 above go through the `config` CLI, which is the default because
it needs no extra tooling. `nvs-board-a.csv` / `nvs-board-b.csv` (same
directory) are the declarative alternative — one `key,type,encoding,value`
row per NVS key, in the format ESP-IDF's own `nvs_partition_gen.py`
expects, pre-filled with each board's `identity` namespace (`node_id`,
`mode` — `0`/`1` for `ping`/`pong`, matching `identity_mode_t` in
`identity.h`). Useful for factory-style provisioning without ever
touching the serial console, or for restoring identity after an
`esptool erase_flash`. Generate and flash (each board's default `nvs`
partition is 24576 bytes / `0x6000`, per the default single-app
partition table — confirm with `idf.py partition-table` if that's ever
changed):

```
make shell   # ESP-IDF's python env (protobuf etc.) lives in the container
python3 $IDF_PATH/components/nvs_flash/nvs_partition_generator/nvs_partition_gen.py \
    generate nvs-board-a.csv nvs-board-a.bin 0x6000
exit

python3 -m esptool --chip esp32 -p /dev/tty.usbserial-XXXX \
    write_flash 0x9000 nvs-board-a.bin
```

(swap in `nvs-board-b.csv`/`nvs-board-b.bin` for board B). This *replaces*
the whole NVS partition, so it also overwrites anything else stored
there — fine here since `identity` is the only thing this app keeps in
NVS. Verify with `config show` over the CLI afterward.

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
