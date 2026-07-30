# CLI-only toolchain

Requirement: no GUI tools anywhere in the dev loop — everything scriptable,
Dockerizable, and reproducible from the command line.

## Where this is a non-issue

- **Zephyr** (both Pico and STM32): always CLI. `west build` / `west flash`
  is the entire workflow — no code generator, no IDE, by design.
- **Pico + FreeRTOS**: already CLI-native. CMake + Ninja + `arm-none-eabi-gcc`
  to build, `picotool` or plain UF2 copy to flash, OpenOCD + GDB to debug.
  No ST-style code generator exists for the Pico side at all.

## Where it's a real fork: STM32 + FreeRTOS

The default STM32 workflow is STM32CubeMX/STM32CubeIDE generating HAL init
code from a graphical peripheral config. Two ways to get a CLI-only
equivalent:

1. **Scripted STM32CubeMX** (`STM32CubeMX -q script.txt`, feeding it
   `config load project.ioc` / `project generate` commands) plus
   **STM32CubeCLT** (ST's official command-line toolset: GCC-for-Arm, GDB,
   STM32CubeProgrammer) for build/flash/debug. Caveat confirmed by ST's own
   community forum: even in headless/`-q` mode, STM32CubeMX's code-gen step
   still opens a display and fails with `No X11 DISPLAY variable was set`
   if there isn't one — the standard workaround is running it under `Xvfb`
   (virtual framebuffer) inside the container.
2. **libopencm3** — an open-source, register-level peripheral library for
   STM32 (and a few other vendors), pure Makefile build, no code generator
   and no X11 dependency ever. CAN/FDCAN init is hand-written against
   libopencm3's API instead of generated. Existing FreeRTOS + libopencm3
   template projects exist to start from (e.g.
   [Electronshik/FreeRTOS_libopencm3](https://github.com/Electronshik/FreeRTOS_libopencm3),
   [ve3wwg/stm32f103c8t6](https://github.com/ve3wwg/stm32f103c8t6)), and
   there's a full book on exactly this combination:
   *[Beginning STM32: Developing with FreeRTOS, libopencm3 and GCC](https://www.amazon.com/Beginning-STM32-Developing-FreeRTOS-libopencm3/dp/1484236238)*.

**Decision: libopencm3.** No ST tooling in the loop at all — avoids the
Xvfb workaround entirely, keeps the whole STM32+FreeRTOS project as plain C
+ Makefile that diffs cleanly in git and runs the same in Docker with no
special-casing. Tradeoff accepted: CAN/FDCAN peripheral setup is hand-rolled
against register-level libopencm3 calls rather than copied from an ST HAL
example, and libopencm3's CAN support needs checking per-chip once the
STM32 board is chosen (bxCAN parts are well trodden in libopencm3; FDCAN
support on G4/H7-class parts is newer and less certain — verify before
picking a board with only FDCAN if this matters).

## Flashing and debugging (all targets)

Standardize on **OpenOCD + GDB** for both boards:

- STM32: `openocd -f interface/stlink.cfg -f target/<mcu>.cfg -c "program firmware.elf verify reset exit"`,
  talking to the on-board ST-Link over SWD.
- Pico: OpenOCD talking to a debug probe (a second Pico running picoprobe,
  or any CMSIS-DAP probe) over SWD, same GDB flow. Plain UF2 drag-and-drop
  stays as the no-probe-needed fallback for quick manual flashing.

Both are plain CLI tools with direct USB access to the probe — this is the
same USB-passthrough concern from [dev-setup-research.md](dev-setup-research.md#macos-caveat--why-a-linux-boxrpi-matters):
fine in a Docker container on a Linux box/RPi, unreliable from Docker
Desktop on macOS.

## Sources

- [STM32CubeCLT — ST product page](https://www.st.com/en/development-tools/stm32cubeclt.html)
- [Project generation from command line, headless — ST community (X11 display requirement)](https://community.st.com/t5/stm32cubemx-mcus/project-generation-from-command-line-headless/td-p/687267)
- [STM32Cube: Generate code from command line with no GUI — ST community](https://community.st.com/t5/stm32cubemx-mcus/stm32cube-generate-code-from-command-line-with-no-gui/td-p/376292)
- [libopencm3 GitHub topic](https://github.com/topics/libopencm3?l=makefile)
- [Electronshik/FreeRTOS_libopencm3](https://github.com/Electronshik/FreeRTOS_libopencm3)
- [ve3wwg/stm32f103c8t6](https://github.com/ve3wwg/stm32f103c8t6)
- [Programming STM32 Flash with OpenOCD and GDB](https://copyprogramming.com/howto/how-to-program-the-stm32-flash-using-openocd-and-gdb)
