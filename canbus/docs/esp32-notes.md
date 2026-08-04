# ESP32 platform notes

Board in hand (acquired 2026-07-31): [Botland ESP32 WiFi+BT 4.2 DevKit](https://botland.com.pl/esp32/8893-esp32-wifi-bt-42-platforma-z-modulem-esp-wroom-32-zgodny-z-esp32-devkit-5904422337438.html)
— an ESP32-WROOM-32 module on an ESP32-DevKitC-compatible carrier board.
This replaced an originally-planned STM32 board (see
[dev-setup-research.md](dev-setup-research.md) and
[cli-toolchain.md](cli-toolchain.md), both kept as-is for the STM32
research they contain, now superseded for this lab).

## CAN support: TWAI

ESP32 has a built-in CAN 2.0-compatible controller that Espressif calls
**TWAI** (Two-Wire Automotive Interface — CAN by another name, to sidestep
trademark issues). Like STM32's bxCAN/FDCAN, there's no on-chip
transceiver, so an external one (SN65HVD230, same as planned for the Pico
side) is still required. Default TWAI RX/TX pins on the ESP32 DevKitC are
GPIO0/GPIO2, though these are also strapping pins — worth routing to
different GPIOs via devicetree/`idf.py menuconfig` pin config if that
causes boot issues.

Zephyr has an in-tree driver, `can_esp32_twai.c`, compatible with the
original ESP32 as well as newer ESP32-C3/S2/etc. variants.

## Toolchain: already CLI-native, unlike STM32

The STM32 research (`cli-toolchain.md`) spent real effort working around
STM32CubeMX being a GUI tool with no clean headless story. **That problem
doesn't exist for ESP32.** Espressif's own build system, `idf.py` (which
wraps CMake+Ninja under the hood), has never had a GUI code generator —
it's `idf.py menuconfig` for interactive config (a terminal UI, not a GUI,
and entirely optional/scriptable via `sdkconfig` files) and `idf.py build`/
`flash`/`monitor` for everything else. Zephyr's `west` wraps the same
underlying toolchain for Zephyr apps. No Xvfb workaround, no libopencm3-style
alternative needed — this was a non-issue from the start.

## Flashing: plain USB-serial, no debug probe

The DevKit has an onboard **CP2102** USB-to-serial chip, so flashing is
just `esptool.py` talking to a serial port — no ST-Link/picoprobe/CMSIS-DAP
equivalent needed at all, similar in spirit to the Pico's UF2 convenience
(though ESP32 uses a real bootloader protocol over serial, not a
mass-storage drive).

```
esptool.py --chip esp32 -p /dev/tty.usbserial-XXXX write_flash 0x1000 zephyr.bin
```

(Exact offset/args for Zephyr builds are what `west flash`'s `esp32` runner
uses under the hood — confirm against its actual invocation once flashing
is exercised for real; `0x1000` is the conventional default entry point for
non-secure-boot ESP32 images.)

**No Docker/USB-passthrough workaround needed on macOS either** — unlike
SWD debug probes, a USB-serial device plus a plain Python tool (`esptool`)
has no special driver requirements on macOS beyond the CP2102's own driver
(usually already present on modern macOS; Silicon Labs also publishes one).
`make flash-esp32` in `../firmware/zephyr-canbus/Makefile` runs `esptool.py`
natively on the host rather than through the Docker container for exactly
this reason — see [cli-toolchain.md](cli-toolchain.md) and
[zephyr-single-app.md](zephyr-single-app.md) for how the container/host
split works generally in this lab.

## FreeRTOS: ESP-IDF's own fork, dual-core SMP

Unlike STM32 (which needed X-CUBE-FREERTOS bolted on via CubeMX), FreeRTOS
is the **default, built-in RTOS in ESP-IDF** — every ESP-IDF app runs on
top of it already, no separate integration step. It's not vanilla
FreeRTOS, though: Espressif maintains "ESP-IDF FreeRTOS," a fork based on
FreeRTOS v10.5.1 with real changes to support **dual-core SMP** across the
ESP32's two Xtensa cores — similar in spirit to the RP2040's SMP port on
the Pico side, though a different codebase/fork rather than the same
upstream FreeRTOS-Kernel port. (Espressif also supports the official Amazon
SMP FreeRTOS as an alternative, opt-in via Kconfig, if strict upstream
compatibility ever matters more than ESP-IDF's own extensions.)

## Build-verified (2026-08-01)

The blink/button demo (`../firmware/zephyr-canbus/`) builds clean for
`esp32_devkitc/esp32/procpu` via the same Docker+Makefile workflow already
used for `rpi_pico` and `native_sim` — `make build BOARD=esp32_devkitc/esp32/procpu`
produces `zephyr.bin`, ready to flash. Board overlay:
[../firmware/zephyr-canbus/boards/esp32_devkitc_esp32_procpu.overlay](../firmware/zephyr-canbus/boards/esp32_devkitc_esp32_procpu.overlay)
(LED on GPIO32, button on GPIO33 — chosen to avoid strapping pins and stay
clear of default TWAI pins for later CAN work).

Not yet tested against real hardware — no probe/flash cable exercised yet.

## Emulator: Espressif's QEMU fork (verified working, 2026-08-01)

Unlike RP2040 (no QEMU support in Zephyr at all), ESP32 has real full-chip
emulation via [Espressif's own QEMU fork](https://github.com/espressif/qemu)
— an actual Xtensa CPU + peripheral emulator (`qemu-system-xtensa`), a step
up from `native_sim`'s "host binary with `gpio_emul`-faked GPIO" approach.
Installed as a prebuilt binary release in the Docker image (not built from
source — much faster, avoids the full X11/SDL/GTK build dependency chain).

Confirmed working end to end via `make run-esp32-qemu` in
`../firmware/zephyr-canbus/` — boots the real ROM bootloader banner
(`ets Jul 29 2019 12:21:46...`) before Zephyr even starts, then the actual
app:

```
*** Booting Zephyr OS build v4.4.0 ***
led on (period=500ms)
led off (period=500ms)
...
```

Getting there took fixing two real issues:

- **Chip revision rejected.** QEMU emulates ESP32 chip revision 0; Zephyr's
  `esp32` SoC init treats that as unsupported by default (`E (soc_init): You
  are using ESP32 chip revision (0) that is unsupported`) since real
  ESP32-WROOM-32 modules are typically newer revisions. Fixed with
  `CONFIG_ESP32_USE_UNSUPPORTED_REVISION=y` in the board conf file — this
  only widens accepted revisions, harmless for the real board too.
- **Flash image offset.** The image handed to QEMU (`-drive
  file=...,if=mtd,format=raw`) has to be a padded 4MB file with
  `zephyr.bin`'s content starting at byte offset `0x1000`, matching the real
  ESP32 bootloader's fixed read offset (and the same offset
  `esptool write_flash` uses on real hardware). Placing it at offset `0`
  instead produces `invalid header: 0x00000100` and nothing boots — this
  wasn't obvious from the general Espressif QEMU docs, which describe the
  ESP-IDF `merge_bin` flow rather than Zephyr's simpler single-`zephyr.bin`
  output.

**No interactive button-press simulation**: QEMU's ESP32 model has
keyboard-driven touch-pin simulation (keys 7/8/9/0 → fixed GPIO01/02/12/13),
but this app's button lives on GPIO33 (chosen for real-hardware wiring
reasons, not QEMU's fixed pin set) — so unlike `native_sim`'s scripted
auto-toggle, this doesn't demonstrate the blink-rate change. It's valuable
for a different reason: it's booting against a much more realistic
hardware/peripheral model (real ROM bootloader, real chip-revision checks,
real flash layout) than `native_sim` ever could.

## Sources

- [ESP32-DevKitC — Zephyr board docs](https://docs.zephyrproject.org/latest/boards/espressif/esp32_devkitc/doc/index.html)
- [can_esp32_twai.c — Zephyr driver source](https://github.com/zephyrproject-rtos/zephyr/blob/main/drivers/can/can_esp32_twai.c)
- [ESP32 Getting Started Guide for Zephyr — community wiki](https://github.com/mahavirj/zephyr/wiki/ESP32-Getting-Started-Guide)
- [Flashing Firmware — esptool docs](https://docs.espressif.com/projects/esptool/en/latest/esp32/esptool/flashing-firmware.html)
- [FreeRTOS (IDF) — ESP-IDF Programming Guide](https://docs.espressif.com/projects/esp-idf/en/latest/esp32/api-reference/system/freertos_idf.html)
- [FreeRTOS Overview — ESP-IDF Programming Guide](https://docs.espressif.com/projects/esp-idf/en/latest/esp32/api-reference/system/freertos.html)
- [espressif/qemu — GitHub repo](https://github.com/espressif/qemu)
- [espressif/qemu releases (prebuilt binaries)](https://github.com/espressif/qemu/releases)
- [esp-toolchain-docs QEMU ESP32 README](https://github.com/espressif/esp-toolchain-docs/blob/main/qemu/esp32/README.md)
- [QEMU Emulator — ESP-IDF Programming Guide](https://docs.espressif.com/projects/esp-idf/en/latest/esp32/api-guides/tools/qemu.html)
- [How to Run an ESP32 Zephyr Application on Espressif's QEMU — Shawn Hymel](https://shawnhymel.com/2807/how-to-run-an-esp32-zephyr-application-on-espressifs-qemu/)
