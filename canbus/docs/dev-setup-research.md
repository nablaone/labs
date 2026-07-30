# CAN bus lab — dev setup research

Findings from initial research (2026-07-30) into toolchains and connectivity
options for the RP2040 (Pico) + STM32 + FreeRTOS/Zephyr CAN bus lab. See
[../CLAUDE.md](../CLAUDE.md) for the decisions distilled from this.

## RP2040 / Raspberry Pi Pico

The RP2040 has **no built-in CAN peripheral**. CAN requires an external
controller chip talked to over SPI, plus a transceiver:

- **MCP2515** — classic CAN (up to 1 Mbit/s), SPI, widely supported, cheap
  breakout boards available. Zephyr has native shield drivers for MCP2515
  ([docs](https://docs.zephyrproject.org/latest/boards/shields/mcp2515/doc/index.html)).
- **MCP2518FD** — adds CAN-FD.
- **Longan Labs CANBed RP2040** — an all-in-one board bundling an RP2040 +
  MCP2515 + transceiver, with a Zephyr board definition already available
  ([docs](https://docs.zephyrproject.org/latest/boards/longan/canbed_rp2040/doc/index.html)).
  Worth considering to skip the wiring step.

Zephyr has supported the Pico board (`rpi_pico`) since Zephyr 3.0
([board docs](https://docs.zephyrproject.org/latest/boards/raspberrypi/rpi_pico/doc/index.html)).
Raspberry Pi also publishes starter scripts:
[raspberrypi/pico-zephyr](https://github.com/raspberrypi/pico-zephyr).

FreeRTOS has an official SMP-capable port for RP2040 living in the
FreeRTOS-Kernel repo under `portable/ThirdParty/GCC/RP2040/`, built on top of
the Pico SDK
([README](https://github.com/FreeRTOS/FreeRTOS-Kernel/blob/main/portable/ThirdParty/GCC/RP2040/README.md)).
It brings pico-sdk sync primitives (mutex/semaphore/queue) into FreeRTOS
tasks. No built-in CAN driver — we'd bring our own MCP2515 driver (port an
existing Arduino one, or write a minimal one) on the FreeRTOS side.

## STM32

STM32 MCUs have CAN built into the silicon, so **no separate CAN
controller chip is needed** — just an external transceiver:

- **bxCAN** (classic CAN 2.0) — F0/F1/F2/F3/F4/F7 series.
- **FDCAN** (CAN-FD capable) — G0/G4/H7/L5 series.

No STM32 part has an on-chip transceiver
([ST community confirmation](https://community.st.com/t5/stm32-mcus-products/stm32f407-integrated-can-transceiver/td-p/774666)) —
every board needs an external one (e.g. SN65HVD230 breakout).

Zephyr has drivers for both peripherals: `can_stm32_bxcan.c` and the FDCAN
driver (`st,stm32-fdcan` devicetree binding,
[docs](https://docs.zephyrproject.org/latest/build/dts/api/bindings/can/st,stm32-fdcan.html)).
Most Nucleo boards work out of the box for building; CAN pins need
devicetree/pinctrl overlay work since the transceiver isn't on-board.

FreeRTOS on STM32 is the well-trodden STM32Cube + FreeRTOS middleware path,
CAN driven through the HAL CAN/FDCAN driver.

## CAN transceivers

- **SN65HVD230** — 3.3V logic, cheap breakout modules, matches both the
  Pico's and STM32's 3.3V GPIO directly. Good default to standardize on for
  both boards.
- **TJA1050 / TJA1042** — common but 5V-logic parts; would need level
  shifting on 3.3V MCUs. Skip unless a specific board already has one.
- Bus needs 120Ω termination resistors at both physical ends — easy to
  forget on a breadboard bus.

## Toolchain containerization (Docker)

Zephyr publishes official dev images
([zephyrproject-rtos/docker-image](https://github.com/zephyrproject-rtos/docker-image)):
a CI base image, a CI image (+ Zephyr SDK), and a heavier `zephyr-build` dev
image. Usage pattern is to mount the workspace and run `west build` inside
the container. Several lighter community alternatives exist too
(`rugo/zephyr-docker`, `pedroishimaru/zephyr_devcontainer`) if the official
image is too heavy.

For Pico/FreeRTOS there's no single official image; the container just needs
`arm-none-eabi-gcc`, `cmake`, `ninja`/`make`, and the Pico SDK + FreeRTOS-Kernel
checked out (submodule or `west`-less script). Same toolchain covers building
Zephyr-for-Pico's underlying arch, so one `arm-none-eabi` image can likely be
shared across Pico-FreeRTOS and STM32-FreeRTOS.

## macOS caveat — why a Linux box/RPi matters

Docker Desktop on macOS runs containers inside a lightweight Linux VM, not
on the host kernel directly — so **USB passthrough is unreliable/unsupported
for the class of devices we need** (ST-Link debug probes, picoprobe/CMSIS-DAP,
CAN adapters). Building inside a container on the Mac is fine; flashing and
debugging from that same container is the pain point.

Practical implications:

- **Pico flashing via UF2** (drag-and-drop onto the USB mass-storage
  BOOTSEL drive) needs no special driver and works fine natively from macOS
  — no container/USB passthrough involved at all. This is the easy path
  regardless of host OS.
- **SWD debugging / OpenOCD / picotool / st-flash** all want direct USB
  access to a probe — this is where Mac + Docker breaks down.
- Net recommendation: do the actual flashing/debugging on a **Linux box or
  spare Raspberry Pi**, where Docker shares the host kernel and `--device`/
  udev-based USB passthrough just works. The Mac remains fine as an editor/
  SSH client into that box.

## Laptop ↔ CAN bus connectivity (sniffing / sending)

To join the physical CAN bus from a computer, the common cheap option is
**CANable** (open-source USB-CAN adapter, ~$30,
[canable.io](https://canable.io/)). It supports two firmware personalities:

- **candleLight firmware** — enumerates as a *native SocketCAN* device on
  Linux via the in-kernel `gs_usb` driver. Gives full `can-utils`
  (`candump`, `cansend`, `cangen`) and Wireshark support. This is Linux-only
  as a native SocketCAN interface.
- **slcan firmware** — enumerates as a plain serial device on Linux/Mac/
  Windows. On Linux still usable via `slcand` → SocketCAN. On Mac/Windows,
  use [python-can](https://python-can.readthedocs.io/en/stable/interfaces/gs_usb.html)
  with the `slcan` backend, or the Java `cantact-app` GUI.

**SocketCAN itself is a Linux kernel subsystem — it does not exist on
macOS.** So the options for the Mac are: (a) talk to a CANable running slcan
firmware directly over serial with `python-can`, no VM required, or (b) let
a Linux box/RPi own the adapter as a native SocketCAN interface and reach it
remotely (SSH in and run `candump`/`cansend` there, or a SocketCAN-over-IP
bridge like `socketcand`).

Given a Linux box/RPi is already the plan for flashing/debugging (previous
section), the simplest setup is to make it double as the CAN gateway too:
CANable with candleLight firmware plugged into that box, full `can-utils` +
Wireshark there, SSH in from the Mac for interactive work. `python-can` +
slcan direct from the Mac stays as a fallback if working untethered from the
Mac is ever needed.

## Sources

- [Microchip MCP2515 CAN bus shields — Zephyr docs](https://docs.zephyrproject.org/latest/boards/shields/mcp2515/doc/index.html)
- [Raspberry Pi Pico — Zephyr board docs](https://docs.zephyrproject.org/latest/boards/raspberrypi/rpi_pico/doc/index.html)
- [raspberrypi/pico-zephyr](https://github.com/raspberrypi/pico-zephyr)
- [CANBed RP2040 — Zephyr docs](https://docs.zephyrproject.org/latest/boards/longan/canbed_rp2040/doc/index.html)
- [FreeRTOS-Kernel RP2040 port README](https://github.com/FreeRTOS/FreeRTOS-Kernel/blob/main/portable/ThirdParty/GCC/RP2040/README.md)
- [STM32F407 integrated CAN transceiver — ST community](https://community.st.com/t5/stm32-mcus-products/stm32f407-integrated-can-transceiver/td-p/774666)
- [st,stm32-fdcan — Zephyr devicetree binding docs](https://docs.zephyrproject.org/latest/build/dts/api/bindings/can/st,stm32-fdcan.html)
- [zephyrproject-rtos/docker-image](https://github.com/zephyrproject-rtos/docker-image)
- [CANable — canable.io](https://canable.io/)
- [CANable Getting Started](https://canable.io/getting-started.html)
- [gs_usb / candleLight — python-can docs](https://python-can.readthedocs.io/en/stable/interfaces/gs_usb.html)
