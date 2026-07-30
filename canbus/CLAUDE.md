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
- **STM32** (board TBD) — CAN is built into the MCU (bxCAN on F0/F1/F2/F3/
  F4/F7, FDCAN on G0/G4/H7/L5), but no STM32 part has an on-chip
  transceiver, so an external one is always needed.
- **Transceiver**: standardize on **SN65HVD230** breakouts (3.3V logic,
  matches both boards' GPIO directly) unless a specific board dictates
  otherwise.
- **Bus**: 120Ω termination at both physical ends.

Full research notes/citations: [docs/dev-setup-research.md](docs/dev-setup-research.md).

## Firmware targets

Both RTOSes on both boards — four combinations:

| | Pico (RP2040) | STM32 |
|---|---|---|
| **FreeRTOS** | Official SMP port in FreeRTOS-Kernel (`portable/ThirdParty/GCC/RP2040/`), on top of pico-sdk. No built-in CAN driver — bring our own MCP2515 driver. | Standard STM32Cube + FreeRTOS middleware path; CAN via HAL CAN/FDCAN driver. |
| **Zephyr** | Supported since Zephyr 3.0 (`rpi_pico` board). MCP2515 has a native Zephyr shield driver. | Native `can_stm32_bxcan` / FDCAN drivers in-tree; CAN pins need a devicetree/pinctrl overlay since the transceiver isn't on-board. |

Apps themselves stay simple: blink an LED, read a button, and use those to
drive/react to CAN frames — the point is exercising the CAN stack and dev
loop on both RTOS/board combos, not the app logic.

## Dev environment

- **Linux-based, Docker for all builds.** Zephyr publishes an official dev
  image (`zephyrproject-rtos/docker-image`) built around `west build`. For
  Pico/STM32 FreeRTOS, a container with `arm-none-eabi-gcc` + `cmake` +
  the relevant SDK (pico-sdk / STM32Cube) covers both — likely shareable
  as one `arm-none-eabi` image.
- **macOS caveat**: Docker Desktop on macOS runs containers in a Linux VM,
  not on the host kernel, so USB passthrough to debug probes (ST-Link,
  picoprobe/CMSIS-DAP) and CAN adapters is unreliable. Building in a
  container on the Mac is fine; flashing/debugging from that container is
  the problem.
  - Exception: **Pico UF2 flashing** (drag-and-drop onto the BOOTSEL mass
    storage drive) needs no driver and works natively from macOS — no
    Docker/USB passthrough involved.
  - Everything else that needs direct USB access to a probe (OpenOCD,
    picotool over SWD, st-flash) should run on a **Linux box or spare
    Raspberry Pi**, where Docker shares the host kernel and USB passthrough
    just works.
- **Plan**: Mac is the editor/SSH client; a Linux box or RPi is the actual
  build+flash+debug host (and doubles as the CAN gateway, see below). Which
  physical machine that is — TBD.

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
- `firmware/` — not created yet; will hold one project per
  board × RTOS combination once hardware is in hand.
- `docker/` — not created yet; toolchain Dockerfiles.

## Open questions

- Exact STM32 board (Nucleo model) not chosen yet.
- MCP2515 vs MCP2518FD vs just buying a CANBed RP2040 for the Pico side.
- Which physical machine becomes the permanent Linux/RPi dev+flash+CAN-gateway
  host.
