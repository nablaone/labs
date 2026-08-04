# FreeRTOS on Pico and ESP32

Short answer: yes, FreeRTOS works on both — officially supported on each,
but through very different mechanisms. See
[dev-setup-research.md](dev-setup-research.md) for the broader platform
research this builds on.

(This doc originally covered STM32 as the second platform; STM32 was later
replaced with the ESP32 DevKit actually acquired — see
[esp32-notes.md](esp32-notes.md) for the fuller ESP32 writeup this section
summarizes, and [dev-setup-research.md](dev-setup-research.md) for why the
board changed.)

## ESP32 (ESP-IDF)

Integration is automatic, not a separate step:

- FreeRTOS is the **default, built-in RTOS in ESP-IDF** — every ESP-IDF
  app already runs on top of it, no middleware to add and no code
  generator involved (`idf.py` has always been CLI-native).
- It's not vanilla FreeRTOS: Espressif maintains **"ESP-IDF FreeRTOS,"** a
  fork based on FreeRTOS v10.5.1 with real modifications for **dual-core
  SMP** across the ESP32's two Xtensa cores — a different codebase from the
  Pico's official FreeRTOS-Kernel SMP port, but the same idea (both cores
  usable by one FreeRTOS instance).
- The official Amazon SMP FreeRTOS is also available as an ESP-IDF Kconfig
  option, if strict upstream-FreeRTOS compatibility ever matters more than
  ESP-IDF's own extensions/APIs.

**CAN + FreeRTOS pattern**: same shape as it would have been on STM32 —
interrupt-driven RX on the TWAI (ESP32's CAN peripheral) controller, ISR
does the minimum (pull the frame, push to a FreeRTOS queue via
`xQueueSendFromISR`) and returns immediately, a consumer task does the real
handling. Standard producer/consumer-from-ISR pattern, same as it would be
on any FreeRTOS target.

## Raspberry Pi Pico / RP2040

Integration is manual/source-level, no code generator:

- Official **SMP-capable FreeRTOS-Kernel port** lives in the FreeRTOS-Kernel
  repo itself, under `portable/ThirdParty/GCC/RP2040/` — built by Raspberry
  Pi engineer Graham Sanderson directly on top of the Pico SDK.
- Setup is manual: clone `FreeRTOS-Kernel`, then in the project's
  `CMakeLists.txt` (after `pico_sdk_init()`) set `FREERTOS_KERNEL_PATH`,
  `FREERTOS_PORT` (`GCC_RP2040`), and `FREERTOS_HEAP` (heap scheme 1-5),
  and provide a project-local `FreeRTOSConfig.h`. No CubeMX-style generator
  — several community templates exist to start from instead of doing this
  by hand each time (e.g.
  [sbehnke/freertos-pico-template](https://github.com/sbehnke/freertos-pico-template),
  [racka98/PicoW-FreeRTOS-Template](https://github.com/racka98/PicoW-FreeRTOS-Template)).
- **Dual-core (SMP)**: the RP2040's two Cortex-M0+ cores are both usable by
  one FreeRTOS instance. Relevant config: `configNUM_CORES`,
  `configRUN_MULTIPLE_PRIORITIES` (run tasks of different priorities
  simultaneously on different cores), `configUSE_CORE_AFFINITY` (pin a task
  to a specific core). Pico SDK sync primitives (mutex/semaphore/queue from
  `pico_sync`) interoperate with FreeRTOS tasks and with code on a core not
  running FreeRTOS at all, or in IRQ handlers. An AMP alternative exists
  (separate FreeRTOS instance per core, fully independent) but SMP is the
  more natural fit for a single app splitting work across both cores.
- No built-in CAN driver (expected — CAN isn't a Pico SDK peripheral at
  all). The MCP2515 talks over SPI; a driver has to be brought in
  separately and its RX interrupt (MCP2515 INT pin → Pico GPIO IRQ) handed
  off to a FreeRTOS task the same way as the ESP32 case: minimal ISR work,
  hand the frame to a queue.

## Practical differences that matter for this lab

| | ESP32 (ESP-IDF) | Pico |
|---|---|---|
| Setup | Built into ESP-IDF, nothing to add | Manual CMake + FreeRTOSConfig.h |
| API | ESP-IDF FreeRTOS (fork of v10.5.1) | Official upstream FreeRTOS-Kernel SMP port |
| Cores | Dual-core SMP (Xtensa) | Dual-core SMP (Cortex-M0+) |
| CAN driver | TWAI, in-tree/built-in | None — MCP2515 over SPI, external driver needed |

Net effect: both platforms get dual-core SMP FreeRTOS and both need a
device driver written for CAN on the FreeRTOS side (TWAI has a built-in
low-level driver but still needs FreeRTOS-side queue/task wiring; MCP2515
needs a driver from scratch) — the two firmware trees under `../firmware/`
should end up closer in shape than the STM32/Pico pairing would have been,
since STM32 was the one with a fundamentally different (CubeMX-generated,
single-core) project structure.

## Sources

- [FreeRTOS-Kernel RP2040 port README](https://github.com/FreeRTOS/FreeRTOS-Kernel/blob/main/portable/ThirdParty/GCC/RP2040/README.md)
- [sbehnke/freertos-pico-template](https://github.com/sbehnke/freertos-pico-template)
- [racka98/PicoW-FreeRTOS-Template](https://github.com/racka98/PicoW-FreeRTOS-Template)
- [FreeRTOS-SMP-Demos RP2040 demo README](https://github.com/FreeRTOS/FreeRTOS-SMP-Demos/blob/main/FreeRTOS/Demo/CORTEX_M0+_RP2040/README.md)
- [Symmetric Multiprocessing Branch of FreeRTOS Gets Ported to the Pico — Hackster.io](https://www.hackster.io/news/symmetric-multiprocessing-branch-of-freertos-gets-ported-to-the-raspberry-pi-pico-rp2040-7709ab0333d2)
- [FreeRTOS (IDF) — ESP-IDF Programming Guide](https://docs.espressif.com/projects/esp-idf/en/latest/esp32/api-reference/system/freertos_idf.html)
- [FreeRTOS Overview — ESP-IDF Programming Guide](https://docs.espressif.com/projects/esp-idf/en/latest/esp32/api-reference/system/freertos.html)
