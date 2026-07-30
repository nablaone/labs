# FreeRTOS on Pico and STM32

Short answer: yes, FreeRTOS works on both — officially supported on each,
but through very different mechanisms. See
[dev-setup-research.md](dev-setup-research.md) for the broader platform
research this builds on.

## STM32

Integration is graphical/code-generated, not manual:

- **X-CUBE-FREERTOS** middleware, added from STM32CubeMX either directly in
  the peripherals/middleware list or as a Software Component
  ([ST product page](https://www.st.com/en/embedded-software/x-cube-freertos.html)).
  CubeMX generates the FreeRTOS config and task scaffolding as part of the
  normal HAL project generation — no separate kernel checkout/CMake wiring
  needed, unlike Pico.
- Runs behind the **CMSIS-RTOS2** abstraction layer by default (a
  standardized wrapper API over the native FreeRTOS API). CubeMX supports
  both CMSIS-RTOS v1 and v2; new projects should target v2. It's possible to
  drop down to the native FreeRTOS API instead of CMSIS-RTOS2 if the wrapper
  gets in the way.
- Single-core only (no SMP question here — target STM32s are single-core
  Cortex-M).

**CAN + FreeRTOS pattern**: CAN is interrupt-driven via the HAL
(`HAL_FDCAN_ActivateNotification` / classic CAN RX FIFO interrupt), not
polled. Received-frame callbacks should do the minimum possible (pull the
frame out, push it to a FreeRTOS queue) and return immediately — anything
slower risks dropping frames, since the peripheral's RX FIFO is small and
the ISR blocks other bus handling while it runs. A consumer task then reads
off the queue and does the real handling. This queue-from-ISR pattern is
just the standard FreeRTOS `xQueueSendFromISR` approach applied to CAN RX.

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
  off to a FreeRTOS task the same way as the STM32 case: minimal ISR work,
  hand the frame to a queue.

## Practical differences that matter for this lab

| | STM32 | Pico |
|---|---|---|
| Setup | CubeMX/CubeIDE generates it | Manual CMake + FreeRTOSConfig.h |
| API | CMSIS-RTOS2 wrapper (or native) | Native FreeRTOS API |
| Cores | Single-core | Dual-core SMP available |
| CAN driver | HAL CAN/FDCAN, in-tree | None — MCP2515 over SPI, external driver needed |

Net effect: the STM32 side leans on ST's generated project structure, the
Pico side is closer to "vanilla FreeRTOS-Kernel plus pico-sdk," so the two
firmware trees under `../firmware/` will likely look quite different in
shape even though both are "FreeRTOS."

## Sources

- [X-CUBE-FREERTOS — ST product page](https://www.st.com/en/embedded-software/x-cube-freertos.html)
- [STM32 FreeRTOS Tutorial: CMSIS-RTOS V2 Setup — controllerstech](https://controllerstech.com/stm32-freertos-tutorial-cubemx-led-example/)
- [FreeRTOS-Kernel RP2040 port README](https://github.com/FreeRTOS/FreeRTOS-Kernel/blob/main/portable/ThirdParty/GCC/RP2040/README.md)
- [sbehnke/freertos-pico-template](https://github.com/sbehnke/freertos-pico-template)
- [racka98/PicoW-FreeRTOS-Template](https://github.com/racka98/PicoW-FreeRTOS-Template)
- [FreeRTOS-SMP-Demos RP2040 demo README](https://github.com/FreeRTOS/FreeRTOS-SMP-Demos/blob/main/FreeRTOS/Demo/CORTEX_M0+_RP2040/README.md)
- [Symmetric Multiprocessing Branch of FreeRTOS Gets Ported to the Pico — Hackster.io](https://www.hackster.io/news/symmetric-multiprocessing-branch-of-freertos-gets-ported-to-the-raspberry-pi-pico-rp2040-7709ab0333d2)
- [Using FreeRTOS with CAN interrupt — ST community](https://community.st.com/t5/stm32-mcus-products/using-freertos-with-can-interrupt/td-p/390559)
- [Understanding FDCAN interrupts grouping — ST community](https://community.st.com/t5/stm32-mcus/understanding-fdcan-interrupts-grouping-in-applicable-stm32-mcus/ta-p/852823)
