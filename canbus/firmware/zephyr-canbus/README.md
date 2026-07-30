# zephyr-canbus

Single Zephyr application shared across boards — see
[../../docs/zephyr-single-app.md](../../docs/zephyr-single-app.md) for why.
Everything runs through Docker; no host Zephyr/SDK install needed.

Current app (`src/main.c`): blinks an LED, polling a button to switch
between a slow (500ms) and fast (100ms) blink rate while it's held down.

## Wiring (default board: rpi_pico)

Per `boards/rpi_pico.overlay`:

- LED (+ resistor) from **GP20** to GND.
- Push-button from **GP21** to GND (internal pull-up enabled in software,
  no external resistor needed).

## Usage

```
make build              # BOARD defaults to rpi_pico, build dir is build-<board>/
make build BOARD=rpi_pico2
make run                 # run in Zephyr's native_sim emulator, no hardware needed
make shell               # drop into the dev container (west/cmake available)
make flash               # untested placeholder -- debug probe not chosen yet
make clean
```

First `make build`/`make run` triggers the Docker image build, which checks
out the full Zephyr workspace and downloads the ARM SDK toolchain — multi-GB
download, expect it to take a while. Later builds reuse the cached image.

`make flash` needs a real debug probe (picoprobe/CMSIS-DAP over SWD, or
similar) and Linux-host USB passthrough — not yet exercised, see the macOS
caveat in [../../docs/dev-setup-research.md](../../docs/dev-setup-research.md).
For now, flashing via UF2 drag-and-drop (copy `build-rpi_pico/zephyr/zephyr.uf2`
onto the Pico's BOOTSEL mass-storage drive) works with no extra setup.

## Running without hardware (`make run`)

RP2040/Pico has no QEMU machine model in Zephyr, so full-chip emulation
isn't available — but `make run` builds for `native_sim/native/64` (a plain
host Linux binary) with LED/button emulated via `zephyr,gpio-emul`
(`boards/native_sim_native_64.overlay` + `.conf`). There's no real button to
press, so `src/main.c` auto-toggles the emulated button every 3s when
`CONFIG_GPIO_EMUL` is set (`gpio_emul_input_set()`), purely to demonstrate
the blink-rate change. Expected output:

```
*** Booting Zephyr OS build v4.4.0 ***
[sim] native_sim: no real button, auto-toggling every 3s to demo the blink-rate change
led on (period=500ms)
led off (period=500ms)
led on (period=500ms)
[sim] button pressed
led off (period=100ms)
...
[sim] button released
led on (period=500ms)
...
```

Ctrl-C to exit. Board-qualifier note: `native_sim`'s default target assumes
a 32-bit userspace, which doesn't exist on arm64 hosts (Apple Silicon) —
hence `/native/64` explicitly. Zephyr's board-specific overlay/conf files
must be named after the *full* board target with `/` replaced by `_`
(`native_sim_native_64.overlay`, not `native_sim.overlay`) or they're
silently not picked up.

## Adding the STM32 target

Once the STM32 board is chosen (see open questions in
[../../CLAUDE.md](../../CLAUDE.md)): add `boards/<board_name>.overlay`
wiring `led0`/`sw0` to real GPIOs on that board, then
`make build BOARD=<board_name>`. No source changes needed — see
[../../docs/zephyr-single-app.md](../../docs/zephyr-single-app.md).
