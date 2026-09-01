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
  assignments, `FIRMWARE_VERSION`. Runtime (NVRAM-backed) reconfiguration
  is a planned extension, not implemented yet.
- **`state.c`/`.h`** — the shared "excitement counter" + heartbeat period,
  mutex-protected (`SemaphoreHandle_t`) since tasks run concurrently
  across the ESP32's two cores. Owns the `counter` CLI command.
- **`led_task.c`/`.h`** — every 100ms, reads the counter; toggles the LED
  if it changed since the last read.
- **`heartbeat_task.c`/`.h`** — increments the counter every 1s on its
  own. Owns the `rate` CLI command (sets its own period).
- **`button_task.c`/`.h`** — polls the button every 100ms; increments the
  counter on every poll where it reads pressed (holding it down keeps
  incrementing, not just a single bump per press).
- **`display_task.c`/`.h`** — logs the counter's value every 10s.
- **`can.c`/`.h`** — TWAI driver + self-test + send/sniff. Not a
  background task (no loop of its own) — driven entirely by the `can` CLI
  command. See below.
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
- **`can loop`** — self-test with D21 jumpered directly to D22 (no
  transceiver) — isolates the TWAI peripheral/firmware from the hardware.
- **`can xcvr`** — the same self-test, but with the SN65HVD230 wired
  normally — confirms the transceiver + its wiring.
- **`can sniff`** — print received frames as `<id_hex>#<data_hex>` for a
  fixed 30s, then return.
- **`can send <id_hex>#<data_hex>`** — transmit one frame, e.g.
  `can send 123#DEADBEEF`.
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
