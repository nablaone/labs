# One Zephyr app, both boards

Zephyr applications are board-agnostic by design, so the Pico and ESP32
Zephyr firmware live in **one directory**,
`firmware/zephyr-canbus/`, not two. Only the FreeRTOS side needs separate
per-board trees (see [cli-toolchain.md](cli-toolchain.md) /
[freertos-notes.md](freertos-notes.md)) — Pico-FreeRTOS (pico-sdk/CMake) and
ESP32-FreeRTOS (ESP-IDF/`idf.py`) are genuinely different build systems
with nothing Zephyr-style underneath to unify them.

## Layout

```
firmware/zephyr-canbus/
  CMakeLists.txt
  prj.conf                                    # config shared by all boards
  src/
    main.c                                    # board-agnostic app code
  boards/
    rpi_pico.overlay                          # Pico: LED/button GPIO wiring
    rpi_pico2.overlay                         # Pico: MCP2515 SPI wiring, zephyr,canbus -> spi CAN node (not yet added)
    rpi_pico2.conf                             # Pico: enable SPI CAN controller driver (not yet added)
    esp32_devkitc_esp32_procpu.overlay        # ESP32: LED/button GPIO wiring (TWAI overlay not yet added)
    native_sim_native_64.overlay/.conf        # native_sim: emulated GPIO for `make run`
```

`boards/<board>.overlay` and `boards/<board>.conf` are picked up
automatically by `west build -b <board>` when the filename matches the
*full qualified* board target (with `/` replaced by `_`) exactly — no
CMakeLists.txt wiring needed for the common case. (Confirmed the hard way:
see the `native_sim` naming gotcha in
[../firmware/zephyr-canbus/README.md](../firmware/zephyr-canbus/README.md#running-without-hardware-make-run).)

## How app code stays board-agnostic

Devicetree indirection, not `#ifdef`s:

- **CAN**: overlay sets the `zephyr,canbus` chosen node to whichever CAN
  controller node applies (`&twai0` on ESP32, the SPI-attached MCP2515 node
  on Pico). App code always does
  `DEVICE_DT_GET(DT_CHOSEN(zephyr_canbus))` — identical call, different
  hardware underneath. Not yet added to this app; still LED/button-only.
- **LED / button**: standard `led0` / `sw0` devicetree aliases, same idea —
  already in use, see `boards/*.overlay`.

## Build invocations

```
# Pico
west build -b rpi_pico firmware/zephyr-canbus
# Pico + CAN, once added (MCP2515 is a Zephyr shield, not part of the board itself)
west build -b rpi_pico2 firmware/zephyr-canbus -- -DSHIELD=<mcp2515-shield-name>

# ESP32
west build -b esp32_devkitc/esp32/procpu firmware/zephyr-canbus
```

Same thing via the Makefile: `make build BOARD=esp32_devkitc/esp32/procpu`.
Both board targets build-verified end to end (2026-08-01), see
[../firmware/zephyr-canbus/README.md](../firmware/zephyr-canbus/README.md).
Exact MCP2515 shield name (Adafruit PiCowbell CAN Bus shield vs. a custom
overlay-only wiring) and TWAI pin wiring on the ESP32 side are still TBD —
open questions in [../CLAUDE.md](../CLAUDE.md#open-questions).

## Sources

- [Zephyr Application Development docs](https://docs.zephyrproject.org/latest/develop/application/index.html)
- [How to Optimize Zephyr Configuration and Overlays — Zephyr Project blog](https://www.zephyrproject.org/how-to-optimize-zephyr-configuration-and-overlays/)
- [STM32H7 Zephyr CAN Driver — zephyr-canbus](https://github.com/zephyrproject-rtos/zephyr/discussions/69073) (worked `zephyr,canbus` chosen-node example — from the earlier STM32 research, pattern still applies generally)
- [can_esp32_twai.c — Zephyr driver source](https://github.com/zephyrproject-rtos/zephyr/blob/main/drivers/can/can_esp32_twai.c)
