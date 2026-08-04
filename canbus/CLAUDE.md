# canbus

CAN bus experimentation lab: simple firmware (LED / button apps) on two MCU
platforms, exercised over a physical CAN bus, with the laptop able to join
the bus to sniff and inject traffic.

## Hardware

- **Raspberry Pi Pico (RP2040)** — no on-chip CAN peripheral. Needs an
  external SPI CAN controller (MCP2515 for classic CAN, or MCP2518FD for
  CAN-FD) plus a transceiver. Alternative: the Longan Labs CANBed RP2040
  bundles RP2040 + MCP2515 + transceiver on one board and has a Zephyr board
  definition already, which would skip the wiring.
- **ESP32** (board in hand: [Botland ESP32 WROOM-32 DevKit](https://botland.com.pl/esp32/8893-esp32-wifi-bt-42-platforma-z-modulem-esp-wroom-32-zgodny-z-esp32-devkit-5904422337438.html),
  an ESP32-DevKitC-compatible board) — CAN is built into the chip as the
  **TWAI** peripheral (Espressif's name for a CAN 2.0-compatible
  controller), but like STM32 there's no on-chip transceiver, so an
  external one is still needed.
- **Transceiver**: standardize on **SN65HVD230** breakouts (3.3V logic,
  matches both boards' GPIO directly) unless a specific board dictates
  otherwise.
- **Bus**: 120Ω termination at both physical ends.

Full research notes/citations: [docs/dev-setup-research.md](docs/dev-setup-research.md),
[docs/freertos-notes.md](docs/freertos-notes.md), [docs/cli-toolchain.md](docs/cli-toolchain.md).

## Firmware targets

Both RTOSes on both boards — four combinations:

| | Pico (RP2040) | ESP32 |
|---|---|---|
| **FreeRTOS** | Official SMP port in FreeRTOS-Kernel (`portable/ThirdParty/GCC/RP2040/`), on top of pico-sdk. No built-in CAN driver — bring our own MCP2515 driver. | **ESP-IDF** — Espressif's own fork of FreeRTOS (dual-core SMP, like Pico) is the default RTOS baked into ESP-IDF, built via `idf.py` (CLI-native, no code generator). TWAI driver is built in. |
| **Zephyr** | Supported since Zephyr 3.0 (`rpi_pico` board). MCP2515 has a native Zephyr shield driver. | Supported via the `esp32_devkitc` board (Xtensa toolchain). Native `can_esp32_twai` driver in-tree. |

Apps themselves stay simple: blink an LED, read a button, and use those to
drive/react to CAN frames — the point is exercising the CAN stack and dev
loop on both RTOS/board combos, not the app logic.

**Zephyr shares one app directory across both boards** — board differences
are isolated to devicetree overlays/Kconfig fragments, app code stays
identical. FreeRTOS needs two separate trees since Pico (pico-sdk/CMake) and
ESP32 (ESP-IDF/`idf.py`) are different build systems with no shared
abstraction underneath. See
[docs/zephyr-single-app.md](docs/zephyr-single-app.md).

**Both Zephyr boards can run without hardware.** `make run` (Pico) uses
`native_sim` — a host binary with GPIO faked via `zephyr,gpio-emul`.
`make run-esp32-qemu` (ESP32) uses Espressif's own QEMU fork — real Xtensa
CPU + peripheral emulation, closer to actual hardware, at the cost of no
interactive button-press simulation (unlike `native_sim`'s scripted
auto-toggle). Both verified working (2026-08-01). See
[firmware/zephyr-canbus/README.md](firmware/zephyr-canbus/README.md) and
[docs/esp32-notes.md](docs/esp32-notes.md#emulator-espressifs-qemu-fork-verified-working-2026-08-01).

## Dev environment

- **Linux-based, Docker for all builds, CLI-only — no GUI tools anywhere in
  the loop** (no vendor IDEs at all).
  - Zephyr (both boards): `west build` — already CLI-native by design.
  - Pico + FreeRTOS: CMake + Ninja + `arm-none-eabi-gcc`, already CLI-native.
  - ESP32 + FreeRTOS (via ESP-IDF): `idf.py` — also already CLI-native by
    design, no code generator exists for ESP32 at all. Unlike STM32 (the
    board originally planned here — see below), there was never a
    GUI-tooling problem to solve on this platform.
  - Flashing/debugging: **OpenOCD + GDB** over SWD for Pico. ESP32 flashes
    over plain **USB-serial via `esptool`** — no debug probe needed at all.
- **macOS caveat**: Docker Desktop on macOS runs containers in a Linux VM,
  not on the host kernel, so USB passthrough to debug probes (picoprobe/
  CMSIS-DAP) and CAN adapters is unreliable. Building in a container on the
  Mac is fine; flashing/debugging from that container is the problem.
  - Exception: **Pico UF2 flashing** (drag-and-drop onto the BOOTSEL mass
    storage drive) needs no driver and works natively from macOS — no
    Docker/USB passthrough involved.
  - Exception: **ESP32 flashing via `esptool`** also works natively from
    macOS with no Docker involvement — it's a plain Python tool talking to
    a USB-serial port (the DevKit's onboard CP2102), not a debug-probe
    protocol. `make flash-esp32` in `firmware/zephyr-canbus/` runs it
    natively rather than through the container for exactly this reason.
  - Everything else that needs direct USB access to a probe (OpenOCD,
    picotool over SWD) should run on a **Linux box or spare Raspberry Pi**,
    where Docker shares the host kernel and USB passthrough just works.
- **Plan**: Mac handles ESP32 flashing directly (no detour needed) and is
  the editor/SSH client generally; a Linux box or RPi is still needed for
  Pico SWD debug-probe work and doubles as the CAN gateway (see below).
  Which physical machine that is — TBD.

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
- `firmware/zephyr-canbus/` — the shared Zephyr app (both boards). Working
  Docker + Makefile dev setup, blink/button demo build-verified on both
  `rpi_pico` and `esp32_devkitc/esp32/procpu`. See
  [firmware/zephyr-canbus/README.md](firmware/zephyr-canbus/README.md).
- `firmware/pico-freertos/`, `firmware/esp32-idf/` — not created yet.
- `docker/` — not created yet; toolchain Dockerfiles.

## Open questions

- MCP2515 vs MCP2518FD vs just buying a CANBed RP2040 for the Pico side.
- Which physical machine becomes the permanent Linux/RPi dev+flash+CAN-gateway
  host — this Linux box has been doing that job de facto since 2026-08-04
  (CANable2 bridged to SocketCAN via `canbus/scripts/setup-socketcan.sh`,
  ESP32 flashed and running over USB-serial 2026-08-05), but that's not yet
  a deliberate decision, just what's plugged in. Still needed regardless
  for Pico SWD debug-probe work.
- ESP32 TWAI CAN pin wiring (which GPIOs to use) not yet decided /
  overlay not yet written — GPIO33 (button) was chosen to stay clear of
  the common TWAI default pins; LED moved to GPIO2 (this board's onboard
  LED, confirmed 2026-08-05) instead of the originally-planned GPIO32, see
  [firmware/zephyr-canbus/boards/esp32_devkitc_esp32_procpu.overlay](firmware/zephyr-canbus/boards/esp32_devkitc_esp32_procpu.overlay).
