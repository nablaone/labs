# zephyr-canbus

Single Zephyr application shared across boards — see
[../../docs/zephyr-single-app.md](../../docs/zephyr-single-app.md) for why.
Everything runs through Docker; no host Zephyr/SDK install needed.

Current app (`src/main.c`): blinks an LED, polling a button to switch
between a slow (500ms) and fast (100ms) blink rate while it's held down.

## Wiring

Pico (default board, per `boards/rpi_pico.overlay`):

- LED (+ resistor) from **GP20** to GND.
- Push-button from **GP21** to GND (internal pull-up enabled in software,
  no external resistor needed).

ESP32 DevKitC (per `boards/esp32_devkitc_esp32_procpu.overlay`):

- LED (+ resistor) from **GPIO32** to GND.
- Push-button from **GPIO33** to GND (internal pull-up enabled in software,
  no external resistor needed). Pins chosen to avoid strapping pins and stay
  clear of the default TWAI (CAN) pins for later CAN work.

## Usage

```
make build              # BOARD defaults to rpi_pico, build dir is build-<board>/
make build BOARD=esp32_devkitc/esp32/procpu
make run                 # Pico app: run in Zephyr's native_sim emulator, no hardware needed
make run-esp32-qemu      # ESP32 app: run in Espressif's QEMU fork (real Xtensa CPU emulation)
make shell               # drop into the dev container (west/cmake available)
make flash               # Pico: untested placeholder -- debug probe not chosen yet
make flash-esp32 PORT=/dev/tty.usbserial-XXXX   # ESP32: real esptool flash, native (no Docker)
make clean
```

First `make build`/`make run` triggers the Docker image build, which checks
out the full Zephyr workspace and downloads the ARM + Xtensa SDK toolchains
— multi-GB download, expect it to take a while (has hit transient
mid-download connection failures during development here; the Dockerfile
retries the flaky steps a few times before giving up). Later builds reuse
the cached image.

Both `rpi_pico` and `esp32_devkitc/esp32/procpu` build-verified end to end
(2026-08-01) — `build-rpi_pico/zephyr/zephyr.uf2` and
`build-esp32_devkitc_esp32_procpu/zephyr/zephyr.bin` both come out clean.

`make flash` (Pico) needs a real debug probe (picoprobe/CMSIS-DAP over SWD)
and Linux-host USB passthrough — not yet exercised, see the macOS caveat in
[../../docs/dev-setup-research.md](../../docs/dev-setup-research.md). For
now, flashing via UF2 drag-and-drop (copy `build-rpi_pico/zephyr/zephyr.uf2`
onto the Pico's BOOTSEL mass-storage drive) works with no extra setup.

`make flash-esp32` (ESP32) runs `esptool.py` **natively on the host**, not
through Docker — unlike SWD debug probes, a USB-serial connection (the
DevKit's onboard CP2102 chip) has no special passthrough problems on
macOS, so there's no Linux-box detour needed here. Requires `esptool`
installed on the host (`pip install esptool`) and the board's serial port
(`ls /dev/tty.usbserial-*` on macOS once plugged in). Not yet exercised
against real hardware.

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

## Running the ESP32 build without hardware (`make run-esp32-qemu`)

Unlike RP2040, ESP32 has a real full-chip emulator: [Espressif's own QEMU
fork](https://github.com/espressif/qemu) (`qemu-system-xtensa`), baked into
the Docker image as a prebuilt binary release. This is genuine Xtensa CPU +
peripheral emulation — a step up from `native_sim`'s "host binary with GPIO
faked" approach — and boots the actual ROM bootloader banner before Zephyr
even starts. Verified output (2026-08-01):

```
ets Jul 29 2019 12:21:46

rst:0x1 (POWERON_RESET),boot:0x12 (SPI_FAST_FLASH_BOOT)
...
I (spi_flash): detected chip: gd
I (spi_flash): flash io: dio
*** Booting Zephyr OS build v4.4.0 ***
led on (period=500ms)
led off (period=500ms)
led on (period=500ms)
...
```

`make run-esp32-qemu` builds the real `esp32_devkitc/esp32/procpu` target,
pads `zephyr.bin` into a 4MB flash image with the binary placed at flash
offset `0x1000` (the real ESP32 bootloader offset — same offset
`make flash-esp32` writes to on real hardware), and boots it in QEMU.
Ctrl-A then X to exit.

Two things had to be fixed to get this working, both now baked in:

- QEMU emulates ESP32 **chip revision 0**; Zephyr's `esp32` SoC init
  rejects that as unsupported by default (real ESP32-WROOM-32 modules are
  typically newer revisions). Fixed via `CONFIG_ESP32_USE_UNSUPPORTED_REVISION=y`
  in `boards/esp32_devkitc_esp32_procpu.conf` — harmless on real hardware,
  it only widens which revisions are accepted.
- `zephyr.bin` has to be written at offset `0x1000` in the padded image,
  not offset `0` — the ROM bootloader looks for its header there. Getting
  this wrong produces `invalid header: 0x00000100` and nothing boots.

**No interactive button-press demo here**, unlike `make run`: QEMU's
keyboard-driven touch-pin simulation (keys 7/8/9/0 map to fixed GPIO01/02/
12/13) doesn't reach this app's button pin (GPIO33, chosen for real-hardware
wiring reasons, not QEMU compatibility) — so this target is a boot/sanity
check against a much more realistic hardware model, not a substitute for
`native_sim`'s scripted blink-rate-change demo.

## Adding CAN

Not yet added to this app (still LED/button-only). See open questions in
[../../CLAUDE.md](../../CLAUDE.md) and
[../../docs/zephyr-single-app.md](../../docs/zephyr-single-app.md) for how
the `zephyr,canbus` chosen-node pattern will keep `src/main.c` board-agnostic
once it's wired up on both the Pico (MCP2515 shield) and ESP32 (built-in
TWAI) sides.
