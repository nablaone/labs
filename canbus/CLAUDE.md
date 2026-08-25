# canbus

CAN bus experimentation lab: a simple LED/button firmware app on ESP32,
exercised over a physical CAN bus, with the laptop able to join the bus to
sniff and inject traffic.

**Direction change (2026-08-25):** narrowed from the original two-board
(Pico + ESP32), two-RTOS (FreeRTOS + Zephyr) plan down to **ESP32 only,
plain ESP-IDF only**. The Pico and Zephyr material below is kept only as
history/reference (see "Earlier plan" at the end of each section) — new
work targets `firmware/esp32-idf/`.

## Hardware

- **ESP32** (board in hand: [Botland ESP32 WROOM-32 DevKit](https://botland.com.pl/esp32/8893-esp32-wifi-bt-42-platforma-z-modulem-esp-wroom-32-zgodny-z-esp32-devkit-5904422337438.html),
  an ESP32-DevKitC-compatible board) — CAN is built into the chip as the
  **TWAI** peripheral (Espressif's name for a CAN 2.0-compatible
  controller), but there's no on-chip transceiver, so an external one is
  still needed.
- **Transceiver**: **SN65HVD230** breakouts (3.3V logic, matches the
  board's GPIO directly).
- **Bus**: 120Ω termination at both physical ends.

Full research notes/citations: [docs/dev-setup-research.md](docs/dev-setup-research.md),
[docs/freertos-notes.md](docs/freertos-notes.md), [docs/cli-toolchain.md](docs/cli-toolchain.md).

*Earlier plan*: also had a Raspberry Pi Pico (RP2040) side (needing an
external MCP2515/MCP2518FD SPI CAN controller, or a Longan Labs CANBed
RP2040 to skip the wiring). Dropped along with Zephyr/Pico-FreeRTOS —
ESP32's built-in TWAI peripheral makes the Pico's external-controller path
unnecessary work for this lab's goals.

## Firmware

**ESP-IDF on ESP32** (`firmware/esp32-idf/`) — Espressif's own fork of
FreeRTOS (dual-core SMP) is the default RTOS baked into ESP-IDF, built via
`idf.py` (CLI-native, no code generator). TWAI driver is built in.

The app itself stays simple: blink an LED, read a button, and (next) use
those to drive/react to CAN frames — the point is exercising the CAN stack
and dev loop, not the app logic. See
[firmware/esp32-idf/README.md](firmware/esp32-idf/README.md).

*Earlier plan*: this was going to be one of four RTOS/board combinations
(FreeRTOS/Zephyr × Pico/ESP32), with a single Zephyr app shared across both
boards via devicetree overlays and separate FreeRTOS trees per board (no
shared build system between pico-sdk/CMake and ESP-IDF/`idf.py`). See
[docs/zephyr-single-app.md](docs/zephyr-single-app.md) for that design and
[firmware/zephyr-canbus/](firmware/zephyr-canbus/) for the working
Zephyr app it produced (LED/button demo build-verified on both
`rpi_pico` and `esp32_devkitc/esp32/procpu`, plus `native_sim` and
Espressif's QEMU fork for hardware-free runs — see
[firmware/zephyr-canbus/README.md](firmware/zephyr-canbus/README.md)).
Kept as reference, not maintained going forward.

## Dev environment

- **Linux-based, Docker for all builds, CLI-only — no GUI tools anywhere in
  the loop** (no vendor IDEs at all). ESP-IDF's `idf.py` is CLI-native by
  design, no code generator exists for ESP32 at all.
- **Flashing**: plain **USB-serial via `esptool`** — no debug probe needed.
  This also means no macOS/Docker USB-passthrough problem: `esptool` talks
  to a USB-serial port (the DevKit's onboard CP2102 chip) directly, so
  `make flash` in `firmware/esp32-idf/` runs it natively on the host Mac
  rather than through the container. Building still happens in Docker (the
  official `espressif/idf` image).

*Earlier plan*: the Pico side needed OpenOCD + GDB over SWD, which *does*
hit the macOS Docker-Desktop USB-passthrough problem (containers run in a
Linux VM there, not on the host kernel) — that pushed toward needing a
Linux box or spare Raspberry Pi for Pico debug-probe work specifically.
Dropping Pico removes that requirement; a Linux box is still wanted for the
CAN gateway role (see below), just no longer *load-bearing* for flashing.

**Board history**: this lab originally planned an STM32 board (Nucleo,
TBD), which drove the `docs/cli-toolchain.md` research into libopencm3 vs.
CubeMX. That STM32 plan was replaced with the ESP32 DevKit actually in
hand — kept `docs/cli-toolchain.md` as-is since it documents a real decision
process worth having on file, even though it no longer describes the
board currently in use.

## Laptop ↔ CAN bus (sniff / send)

SocketCAN is a Linux kernel subsystem and doesn't exist on macOS, so the
Mac can't join the bus natively.

- Adapter: **CANable** (open-source USB-CAN, ~$30).
- Plan: run it in **candleLight firmware** mode on the Linux box/RPi →
  native SocketCAN interface there → full `can-utils` (`candump`, `cansend`,
  `cangen`) and Wireshark. SSH in from the Mac for interactive sniffing/
  sending.
- Fallback if working directly from the Mac is ever needed: reflash the
  CANable to **slcan firmware** and use `python-can`'s `slcan` backend
  (works cross-platform over a plain serial connection, no SocketCAN
  required).

## Repo layout

- `notebook/` — dated lab notebook entries (chronological log of what was
  tried). See [notebook/README.md](notebook/README.md).
- `docs/` — reference material as markdown: datasheet notes, scrubbed web
  excerpts, protocol references, cited. See [docs/README.md](docs/README.md).
- `firmware/esp32-idf/` — the active app: plain ESP-IDF on the ESP32
  DevKit. See [firmware/esp32-idf/README.md](firmware/esp32-idf/README.md).
- `firmware/zephyr-canbus/` — earlier Zephyr app (Pico + ESP32), kept as
  reference, not maintained going forward. See
  [firmware/zephyr-canbus/README.md](firmware/zephyr-canbus/README.md).
- `firmware/pico-freertos/` — never created; dropped along with the Pico
  side.
- `docker/` — not created yet; toolchain Dockerfiles.

## Open questions

- ESP32 TWAI CAN pin wiring (which GPIOs to use) not yet decided. GPIO33
  (button) was chosen to stay clear of the common TWAI default pins; LED
  is on GPIO2 (this board's onboard LED, confirmed 2026-08-05). Same
  wiring carried over to `firmware/esp32-idf/`, see
  [firmware/esp32-idf/main/main.c](firmware/esp32-idf/main/main.c).
- Which physical machine becomes the permanent CAN-gateway host (CANable
  bridged to SocketCAN via `canbus/scripts/setup-socketcan.sh`) — this
  Linux box has been doing that job de facto since 2026-08-04 (ESP32
  flashed and running over USB-serial since 2026-08-05), but that's not yet
  a deliberate decision, just what's plugged in. No longer needed for
  flashing/debugging now that Pico/SWD is out of scope — purely a CAN
  gateway question now.
