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

## Hardware

KiCad 10 project in [`hardware/`](hardware/), with a PDF export at
[`hardware/dcc-sniffer.pdf`](hardware/dcc-sniffer.pdf):

1. **Track input and power**: Schottky bridge (4× 1N5819), 470 µF buffer
   (rides through RailCom cutouts) and a 5 V step-down module.
2. **DCC signal input**: 6N137 optocoupler with a 1.5 kΩ series resistor,
   a red LED anti-parallel to its input (reverse protection and "DCC
   present" indicator), 100 nF bypass, and a 2.2 kΩ pull-up to 3V3 on
   GPIO4.
3. **ESP32-C3 Super Mini**: 5V, GND, 3V3, GPIO4.

The two modules (Super Mini, DC-DC) are custom symbols in
`hardware/dcc_sniffer.kicad_sym`; everything else comes from the stock
KiCad libraries. With the step-down module the board's GND is tied to the
rails, so don't plug in USB while it's powered from the track.

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
commands: `help`, `version`, `exit`, plus:

- `dcc sniff [seconds] [-a]`: print decoded DCC packets (default 30 s).
  Idle packets and repeats are hidden unless `-a` is given.
  ```
    12.345  81 F9 78          ACC 1 R=1 straight/green ON (wire 4)
    12.401  03 76 75          LOCO 3 speed 10/28 fwd
  ```
- `ble`: Bluetooth link state (advertising/streaming), MTU, packets sent
  and lost.
- `dcc stats [clear]`: packet and error counters since boot.
- `dcc timing [ms]`: histogram of edge-to-edge times on the DCC input.
  A working opto stage shows peaks at 50–60 µs and 90–120 µs. Use this
  first when bringing up the hardware.

## Web viewer (Bluetooth)

`web/index.html` shows the sniffed traffic in Chrome or Edge over **BLE**
(Web Bluetooth), so the laptop or phone keeps its normal Wi-Fi and
internet:

```sh
make web                # then open http://localhost:8000
```

Click **Connect** and pick `DCC-Sniffer`. The page has two parts:

- **Current state**: one entry per loco/accessory address with the last
  known value of each parameter (speed, direction, functions, turnout
  position, ...). Values that change flash briefly.
- **Log**: every packet with its time, raw bytes, type and description.
  It has filters (loco/acc/other/idle), "hide repeats", pause, clear and
  CSV export.

**Demo** feeds synthetic traffic through the same pipeline, so you can
try the page without hardware. On macOS, Chrome needs Bluetooth
permission (System Settings → Privacy & Security → Bluetooth). Firefox
and Safari, including all iPhone browsers, have no Web Bluetooth.

While a page is connected, `dcc sniff` on the USB console competes for
the same packets, so use one or the other.

## Layout (`main/`)

- `main.c`: orchestration only. Init modules, register CLI commands,
  create tasks.
- `console.c/.h`: USB-Serial/JTAG driver + esp_console/linenoise REPL, run as
  `console_task`. Other modules add commands via
  `esp_console_cmd_register()` after `console_init()`.
- `heartbeat_task.c/.h`: toggles the onboard LED (GPIO8) every 500 ms.
  This is the template for new task modules: `xxx_init()` for one-time
  setup, `xxx_task()` for the FreeRTOS loop.
- `dcc.c/.h`: DCC receiver. A GPIO edge interrupt on `DCC_IN_GPIO`
  (GPIO4, from the 6N137) times every half-bit, pairs halves into bits,
  assembles packets and checks the XOR byte. Verified packets go into a
  queue read by `dcc_receive()`, same shape as canbus's `can.c`.
  `dcc_describe()` turns a packet into text, and the module also
  provides the `dcc` CLI command.
- `ble_link.c/.h`: NimBLE peripheral `DCC-Sniffer`. It streams every
  packet from `dcc_receive()` as batched notifications on one GATT
  characteristic (the record format is in `ble_link.h`), but only while a
  client is subscribed. Provides the `ble` CLI command (state, MTU,
  counters).
- `node_config.h`: pins (with Super Mini pin notes) and `FIRMWARE_VERSION`.

Planned modules (see the primer's §8): accessory address matching, output pulsing, NVS config with a
PIKO-style learn button.
