# One Zephyr app, both boards

Zephyr applications are board-agnostic by design, so the Pico and STM32
Zephyr firmware live in **one directory**,
`firmware/zephyr-canbus/`, not two. Only the FreeRTOS side needs separate
per-board trees (see [cli-toolchain.md](cli-toolchain.md) /
[freertos-notes.md](freertos-notes.md)) — Pico-FreeRTOS (pico-sdk/CMake) and
STM32-FreeRTOS (libopencm3/Makefile) are genuinely different build systems
with nothing Zephyr-style underneath to unify them.

## Layout

```
firmware/zephyr-canbus/
  CMakeLists.txt
  prj.conf                       # config shared by both boards
  src/
    main.c                       # board-agnostic app code
  boards/
    rpi_pico2.overlay            # Pico: MCP2515 SPI wiring, zephyr,canbus -> spi CAN node
    rpi_pico2.conf                # Pico: enable SPI CAN controller driver
    <stm32_board_name>.overlay   # STM32: zephyr,canbus -> &fdcan1 (or &can1), pinctrl
    <stm32_board_name>.conf      # STM32: nothing extra needed, FDCAN driver is native
```

`boards/<board>.overlay` and `boards/<board>.conf` are picked up
automatically by `west build -b <board>` when the filename matches the
board target exactly — no CMakeLists.txt wiring needed for the common case.

## How app code stays board-agnostic

Devicetree indirection, not `#ifdef`s:

- **CAN**: overlay sets the `zephyr,canbus` chosen node to whichever CAN
  controller node applies (`&fdcan1` on STM32, the SPI-attached MCP2515 node
  on Pico). App code always does
  `DEVICE_DT_GET(DT_CHOSEN(zephyr_canbus))` — identical call, different
  hardware underneath.
- **LED / button**: standard `led0` / `sw0` devicetree aliases, same idea.

## Build invocations

```
# Pico (MCP2515 is a Zephyr shield, not part of the board itself)
west build -b rpi_pico2 firmware/zephyr-canbus -- -DSHIELD=<mcp2515-shield-name>

# STM32
west build -b <stm32_board_name> firmware/zephyr-canbus
```

Exact shield name (Adafruit PiCowbell CAN Bus shield vs. a custom
overlay-only wiring, if not using an off-the-shelf shield board) and the
STM32 board target name are still TBD — pending the open hardware
questions in [../CLAUDE.md](../CLAUDE.md#open-questions).

## Sources

- [Zephyr Application Development docs](https://docs.zephyrproject.org/latest/develop/application/index.html)
- [How to Optimize Zephyr Configuration and Overlays — Zephyr Project blog](https://www.zephyrproject.org/how-to-optimize-zephyr-configuration-and-overlays/)
- [STM32H7 Zephyr CAN Driver — zephyr-canbus](https://github.com/zephyrproject-rtos/zephyr/discussions/69073) (worked `zephyr,canbus` chosen-node example)
