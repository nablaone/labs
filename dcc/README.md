# dcc

Educational project: a **DCC accessory (turnout) decoder** for a PIKO H0
layout, on an **ESP32-C3 Super Mini** (ESP32-C3FH4: single-core RISC-V
160 MHz, 4 MB embedded flash, native USB-Serial/JTAG on the USB-C port,
blue LED on GPIO8 active-low, BOOT button on GPIO9) with plain ESP-IDF
(FreeRTOS). The goal is to
switch turnouts from the PIKO command station the same way the commercial
PIKO #55030 switch decoder does.

Started as a stripped copy of [`../canbus/firmware/esp32-idf`](../canbus/firmware/esp32-idf).
Only the skeleton is kept: `app_main()`, one example task, and the
serial CLI. All CAN/LCD/metrics/NVS-identity code was removed.

Background reading, the standards (NMRA S-9.x, RailCommunity RCN-21x)
and the PIKO manual are in [docs/](docs/README.md). Start with
[docs/dcc-accessory-primer.md](docs/dcc-accessory-primer.md).

## Build / flash / monitor

Builds run in Docker (official `espressif/idf` image, see `Dockerfile`).
Flashing and the serial monitor run natively on the host, over the
board's USB-C port (`/dev/cu.usbmodem*`; no USB-UART bridge chip, the
console is the C3's built-in USB-Serial/JTAG, set in `sdkconfig.defaults`).

```sh
make build              # docker build + idf.py build
make flash              # esptool over USB-serial (PORT auto-detected)
make monitor            # minicom 115200, Ctrl-A X to quit
make shell              # interactive shell in the IDF container
```

Override the port with `PORT=/dev/cu.usbmodemXXXX` if needed. If flashing
fails to connect, hold BOOT while plugging in (or while tapping RESET).

## CLI

Logs flow normally. Press **Enter** to switch to the `dcc>` prompt (logs
are silenced while you're in it), and type `exit` to leave. Built-in
commands: `help`, `version`, `exit`.

## Layout (`main/`)

- `main.c`: orchestration only. Init modules, register CLI commands,
  create tasks.
- `console.c/.h`: USB-Serial/JTAG driver + esp_console/linenoise REPL, run as
  `console_task`. Other modules add commands via
  `esp_console_cmd_register()` after `console_init()`.
- `heartbeat_task.c/.h`: toggles the onboard LED (GPIO8) every 500 ms.
  This is the template for new task modules: `xxx_init()` for one-time
  setup, `xxx_task()` for the FreeRTOS loop.
- `node_config.h`: pins (with Super Mini pin notes) and `FIRMWARE_VERSION`.

Planned modules (see the primer's §8): DCC signal capture, packet
decoder, accessory address matching, output pulsing, NVS config with a
PIKO-style learn button.
