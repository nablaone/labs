# ESP-IDF app architecture: FreeRTOS concurrency on ESP32

The concurrency model for our nodes now that the platform is **ESP-IDF**
(see the pivot in [project-charter.md](project-charter.md)). ESP-IDF is
Espressif's full framework; its **kernel is FreeRTOS** (an SMP-modified
fork). So "concurrency in ESP-IDF" = "FreeRTOS tasks on a dual-core ESP32."
Maps closely onto the Zephyr model in
[zephyr-app-architecture.md](zephyr-app-architecture.md) — same concepts,
renamed APIs. Compiled 2026-08-20.

## Where things sit

```
your node code  →  (optionally Arduino layer)  →  ESP-IDF components
                                                   (drivers, WiFi/BLE, NVS…)
                                                        │
                                                   FreeRTOS (kernel)
                                                        │
                                                     hardware
```

ESP-IDF gives **full, direct access to the real FreeRTOS API** — you
`#include "freertos/FreeRTOS.h"` and call the genuine `xTaskCreate`,
`xQueue*`, `xSemaphore*`, event groups, task notifications, exactly as the
FreeRTOS docs describe. It's the SMP fork, so you also get **extras** for
dual-core (core affinity), not a reduced subset.

## Scheduling: priority-preemptive, dual-core SMP

- **Tasks** = Zephyr threads. `xTaskCreate()` /
  `xTaskCreatePinnedToCore()`. Each has a stack + priority.
- **Priority convention is INVERTED vs Zephyr: higher number = higher
  priority.** (Zephyr: lower = higher.) Easy to trip on.
- **Preemptive:** highest-priority ready task runs immediately, preempts
  lower ones. Blocking (`vTaskDelay`, queue wait) yields the CPU.
- **Two cores (SMP):** PRO_CPU (core 0) and APP_CPU (core 1). By default the
  **WiFi/BLE stack and `app_main` run on core 0**. Standard practice: **pin
  time-critical work (TWAI/CAN, control loop) to core 1** to keep it away
  from radio bursts on core 0, via `xTaskCreatePinnedToCore(..., 1)`. Use
  `tskNO_AFFINITY` to let a task float.

## Sync primitives (FreeRTOS ↔ Zephyr map)

| Need | ESP-IDF / FreeRTOS | Zephyr equiv |
|---|---|---|
| Event/message hand-off | `xQueueSend/Receive` | `k_msgq` |
| Signal from ISR | binary/counting sem, `xSemaphoreGiveFromISR` | `k_sem` |
| Mutual exclusion (priority inheritance) | `xSemaphoreCreateMutex` | `k_mutex` |
| Wait on multiple events | event groups (`xEventGroupWaitBits`) | `k_event`/`k_poll` |
| Fast single-task wake | **task notifications** | (no direct equiv) |
| Short SMP-safe critical section | `portENTER_CRITICAL(&spinlock)` | `k_spinlock` |
| Lock-free flag | atomics / `portMUX` | `atomic_t` |

### Practical subset per node

- **`xQueue`** — CAN-RX / ESP-NOW-RX callback → logic task.
- **`xSemaphoreCreateMutex`** — guard the small "current state" struct
  (latest velocity, setpoint) read by multiple tasks.
- **Task notification** — the fast path for "ISR wakes one known task"
  (lighter than a queue). Ideal for the limit-switch ISR → actuator task.
- **atomics / critical section** — single-word flags (e-stop, fault).

### ISR rules (explicit in FreeRTOS)

- From an ISR, call the **`...FromISR`** variants only
  (`xQueueSendFromISR`, `xSemaphoreGiveFromISR`). Normal versions are
  forbidden in interrupt context.
- If a `FromISR` call woke a higher-priority task, call
  **`portYIELD_FROM_ISR()`** on ISR exit to switch to it immediately.
- On dual-core, thread-vs-ISR shared data needs a **spinlock/critical
  section or atomic**, not just interrupt disable — the other core can run
  the ISR concurrently.

## Node skeleton shape

```
GPIO ISR (limit switch)      → xTaskNotifyFromISR  → actuator task (core 1)
ESP-NOW / TWAI RX callback   → xQueueSendFromISR   → logic task   (core 1)
control loop task (core 1)   ← driven by esp_timer / FreeRTOS timer, fixed rate
radio/telemetry task (core 0)
state struct                 ← guarded by a mutex; e-stop/fault as atomics
```

Fixed-rate loops use a **timer** (`esp_timer` or FreeRTOS software timer),
not `vTaskDelay` in a loop, to avoid period drift.

## Config note

Some FreeRTOS features (task notifications, timers) and the SMP options are
Kconfig-gated — set them in `idf.py menuconfig` (Kconfig, like Zephyr's).

## Portability bonus

The FreeRTOS API here is the **upstream** API, so this knowledge transfers to
any FreeRTOS target (STM32, etc.) — unlike Zephyr's `k_*` API which is
Zephyr-specific.

## Sources

- ESP-IDF FreeRTOS (SMP) and `xTaskCreatePinnedToCore` — Espressif ESP-IDF
  programming guide, FreeRTOS section (accessed 2026-08-20).
- Task notifications, `...FromISR` + `portYIELD_FROM_ISR` — FreeRTOS.org API
  reference.
- Core assignment of WiFi/BT stack (core 0) — Espressif ESP-IDF docs.
- Existing FreeRTOS CAN-RX pattern — see `freertos-notes.md` in this repo.
