# Zephyr app architecture: RTOS model, scheduling, buses, concurrency

Reference for how a node's firmware is structured under Zephyr — where the
"RTOS" actually is, how threads get scheduled, how the I2C/SPI drivers
behave concurrency-wise, and which sync primitives to reach for. Distilled
from research 2026-08-20. Feeds the node apps for the motor / controller /
panel boards (see [../CLAUDE.md](../CLAUDE.md) and
[bus-selection.md](bus-selection.md)).


## Where is the RTOS? It's a library, not a process

Zephyr is a **library kernel** — it compiles *into* the firmware. `main()`,
the drivers, and the kernel all link into one `zephyr.bin`. There is no OS
running "underneath" the app the way Linux sits under a process.

- On boot, the ESP32 reset vector runs Zephyr's startup, which initializes
  the kernel and starts threads. Between kernel calls, **the running
  thread's own code is what the CPU executes** — the kernel isn't a
  background task getting time slices; it's code entered at well-defined
  points (a kernel call, an interrupt, a timer expiry).
- **`main()` is just a thread** (the "main thread"). If it returns, the
  system does **not** halt — other threads keep running. Many apps spawn
  their worker threads and let `main` return or become one worker.


## Scheduling: priority-based, preemptive

Core rule: **the highest-priority ready thread runs, always, immediately.**
Not "fair" — "most important ready thread runs now." That's the defining
RTOS property.

- **Priorities are integers; lower number = higher priority.** Two bands:
  **cooperative** (negative) and **preemptible** (zero and positive). You
  assign them; the scheduler obeys.
- **Preemption:** the instant a higher-priority thread becomes ready (e.g.
  a semaphore it waited on is given from an ISR), the scheduler preempts the
  running lower-priority thread mid-execution and switches. This is what
  guarantees the speed loop reacts even while a display thread is busy.
- **Cooperative threads (negative priority)** run until they *voluntarily*
  yield or block — not preemptible by other threads (ISRs still interrupt
  them). For short sequences that must not be interrupted by other threads.
- **Blocking = yielding the CPU.** `k_msgq_get` on an empty queue, `k_msleep`,
  a mutex wait — the thread leaves the ready set and the next-highest ready
  thread runs. Waiting means *not scheduled*; the CPU never spins. This is
  why blocking I2C/SPI calls are fine: the caller sleeps, others run.
- **Equal priority:** by default, two same-priority ready threads run until
  one blocks (no auto time-slicing). Enable **`CONFIG_TIMESLICING`** to
  round-robin them on a quantum if desired.
- **What triggers a scheduling decision:** an interrupt, or a kernel call
  that changes readiness (sem give/take, queue put/get, sleep, mutex
  unlock). Nothing happens in the gaps — a CPU-bound thread that never
  blocks and is never preempted will hog its core, so long work goes at low
  priority or is broken up.

**Interrupts sit above all threads.** ISRs aren't scheduled — they preempt
any thread the moment hardware fires, run to completion, then the scheduler
runs and may switch (e.g. because the ISR gave a semaphore that woke a
high-priority thread). This is the **"ISR enqueues, high-priority thread
processes"** pattern: tiny ISR, and waking the thread is a scheduling event
that runs it the instant the ISR returns. Matches the CAN-RX pattern in
[freertos-notes.md](freertos-notes.md).

**Dual-core SMP (ESP32).** The scheduler keeps the **two highest-priority
ready threads running simultaneously**, one per core — "highest runs"
becomes "the N highest run on N cores." Threads migrate between cores unless
pinned via CPU affinity (`CONFIG_SCHED_CPU_MASK`). Pin only if measurement
shows a need; the scheduler load-balances reasonably on its own.


## How the I2C/SPI drivers behave: no threads, one lock per bus

Zephyr does **not** spawn a thread per bus or per device. The bus drivers
are passive libraries.

- **They run in the caller's thread context.** `i2c_write_dt()` /
  `spi_transceive_dt()` execute the transfer in whatever thread called them.
  No background bus thread does transfers on your behalf.
- **Per-bus mutual exclusion is built in.** Each bus controller has its own
  lock inside the driver, so multiple devices on the *same* bus are safe to
  access from different threads — the driver serializes them. You do **not**
  add your own mutex around bus access. Two *different* buses (`&i2c0` vs
  `&spi2`) have independent locks and run truly concurrently. This is
  per-**bus**, not per-device — correct, since the bus is the shared
  resource.
- **Blocking by default.** Calls are synchronous: the driver typically kicks
  off a DMA/interrupt transfer and blocks the caller on a semaphore (CPU
  free for other threads), returning when done. SPI also has an **async**
  API (`spi_transceive_signal()` / callback) for fire-and-don't-wait — not
  needed for these peripherals; blocking is simpler and correct.

**Consequence for app design:** threading is your decision, not the
framework's. Structure around **threads-per-job, not per-device**, and let
the per-bus lock handle any overlap.


## Shared memory vs message passing

Zephyr threads share **one address space** by default, so shared memory is
always physically available — the question is which discipline you use.

- **Shared memory + a lock** — for **current state read often, updated
  occasionally**: latest velocity, throttle setpoint, fault flags. Cheap to
  read, no copying. Guard with a **mutex** (`k_mutex`), a **spinlock** for
  very short sections, or **atomics** for single words.
- **Message passing** — for **events / hand-offs**: "a CAN frame arrived,"
  "e-stop pressed," "next display line." Kernel handles the synchronization;
  a receiver can block until something arrives. Objects: `k_msgq`,
  `k_fifo`/`k_lifo`, `k_mbox`, `k_pipe`.

**Rule of thumb:** *if the receiver must process every item, use a queue; if
it only needs the most recent value, use shared memory.* Queues model
streams of events; shared memory models current state.

**Safety bias:** message passing makes ownership and timing explicit and is
much harder to race than a pile of globals — so use it for the reactive core
(CAN in, commands out), and reserve mutex-protected shared memory for the
handful of "latest reading" values that genuinely want it.


## Sync primitive catalogue

| Primitive | For | ISR-safe? |
|---|---|---|
| `k_mutex` | Mutual exclusion; recursive; **priority inheritance** | No (thread only) |
| `k_sem` | Counting/binary signalling & resource counting | **give: yes** |
| `k_condvar` | "Wait until predicate" (pairs with a mutex) | No |
| `k_msgq` | Fixed-size messages, copied, blocking | **put: yes** |
| `k_fifo`/`k_lifo` | Variable-size linked items, zero-copy by pointer | **put: yes** |
| `k_mbox` | Message passing with sender/receiver identity | Limited |
| `k_pipe` | Byte-stream between threads | — |
| `k_event` | Wait on a **set of event bits** (AND/OR conditions) | post: yes |
| `k_poll` | Wait on **multiple heterogeneous objects** at once | — |
| `k_spinlock` | Very short critical sections; **SMP/ISR-correct** | Yes |
| `atomic_t` | Lock-free single-word flag/counter | Yes |

### Practical subset for a node

Three cover the standard node shape:

- **`k_msgq`** — CAN frames/events from ISR or CAN-RX callback into the logic
  thread.
- **`k_mutex`** — guarding the small "current state" struct (latest velocity,
  setpoint) that multiple threads read.
- **`atomic_t`** — single-bit/word flags (**e-stop**, fault) any context can
  set/check without locking.

Everything else (mailbox, pipe, condvar, poll, events) is there when a
specific need arises. `k_poll` is worth knowing for a node that multiplexes
CAN + timer + local events in one loop without a thread per source.

### Two hard rules (from ISR + SMP reality)

- **From an ISR or CAN-RX callback, ISR-safe ops only:** `k_sem_give`,
  `k_msgq_put`, `k_fifo_put`, atomics, spinlocks. **No mutex, no blocking** —
  those are thread-context only.
- **On dual-core SMP, thread-vs-ISR shared data needs a spinlock or atomic,
  not just interrupt-locking** — the other core can run the ISR
  concurrently. Mutexes synchronize *threads*, not thread-vs-ISR.


## Applying it: priority ladder and the control-loop timer

Priorities fall out of "what must never be starved":

- **Highest:** e-stop / fault handling and the **CAN RX path** (react to
  commands instantly).
- **High:** the **control loop** on the controller board — fixed rate,
  ~50–100 Hz.
- **Medium/low:** telemetry, display updates, SD logging — slow,
  non-critical, must never delay the above.

Because scheduling is preemptive, the guarantee is free: whatever the
display thread is doing, the moment a CAN command or the control-loop timer
fires, the CPU is yanked to the important thread. You set priorities; the
kernel enforces them.

**Fixed-rate loop caveat:** a control loop is **not**
`while (1) { work(); k_msleep(10); }` — that drifts, because the work time
adds to the sleep. Drive it from a **`k_timer`** (or a periodic wakeup) so it
fires on a true fixed period regardless of body duration. Important for a
speed controller.

### Sketch: reactive-core shape (any node)

```
CAN RX callback (ISR context)
    └── k_msgq_put(&can_rx_q, &frame)      # tiny, ISR-safe, returns fast

logic thread (high priority)
    loop:
        k_msgq_get(&can_rx_q, &frame, FOREVER)   # blocks until a frame lands
        update state  (k_mutex around the shared struct)
        act: command motor / update setpoint / raise fault (atomic)

control loop thread (high priority, controller board)
    driven by k_timer @ 50-100 Hz:
        read Hall (local, blocking bus call OK here)
        read latest setpoint (k_mutex)
        run PID -> send motor command over CAN

display/housekeeping thread (low priority, panel board)
    slow LCD writes, telemetry -- never delays the above
```


## Sources

- [Zephyr kernel services — scheduling](https://docs.zephyrproject.org/latest/kernel/services/scheduling/index.html) (priority-based preemptive scheduler, cooperative vs preemptible, time-slicing; accessed 2026-08-20)
- [Zephyr kernel services — synchronization](https://docs.zephyrproject.org/latest/kernel/services/synchronization/index.html) (mutexes, semaphores, condition variables; accessed 2026-08-20)
- [Zephyr kernel services — data passing](https://docs.zephyrproject.org/latest/kernel/services/data_passing/index.html) (message queues, FIFOs/LIFOs, mailboxes, pipes; accessed 2026-08-20)
- [Zephyr kernel — polling API (`k_poll`)](https://docs.zephyrproject.org/latest/kernel/services/polling.html) (waiting on multiple objects; accessed 2026-08-20)
- [Zephyr I2C API](https://docs.zephyrproject.org/latest/hardware/peripherals/i2c/index.html) and [SPI API](https://docs.zephyrproject.org/latest/hardware/peripherals/spi.html) (synchronous driver model, per-controller locking, SPI async signal API; accessed 2026-08-20)
- [Zephyr SMP](https://docs.zephyrproject.org/latest/kernel/services/smp/smp.html) and CPU affinity (`CONFIG_SCHED_CPU_MASK`) — dual-core scheduling (accessed 2026-08-20)
- ESP-IDF FreeRTOS dual-core SMP background — see [esp32-notes.md](esp32-notes.md) and [freertos-notes.md](freertos-notes.md) in this repo.
