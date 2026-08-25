# On-board buses: I2C vs SPI (and why long runs go on CAN)

Decision notes for wiring local peripherals to a node's MCU (ESP32 in this
lab). Distilled from research 2026-08-20. Applies to peripherals sitting
**on or right next to** a board's MCU — anything physically distant is a
CAN node instead, not a bus extension (see the last section, and
[../CLAUDE.md](../CLAUDE.md)).

Context for this project: three boards, each its own CAN node — **motor
board** (power stage), **controller board** (velocity Hall + signal + speed
loop), **panel board** (switches + display). CAN carries the long,
noisy board-to-board runs; I2C/SPI stay short and local to each board.


## The one-line rule

- **I2C** — low-speed local peripherals where **pins are tight**: character
  display, temp/current sensors, RTC. 2 wires total, no matter how many
  devices.
- **SPI** — when you need **speed** (graphical display, SD-card logging,
  fast/precise ADC or encoder) or **no shared-bus failure mode**. 3 shared
  wires + 1 chip-select per device.
- **CAN** — anything **physically distant** or in the traction-noise path.
  Not an on-board bus; it's the backbone between boards.

Both I2C and SPI can run on the ESP32 simultaneously — the choice is
**per-peripheral**, not per-board.


## Pin cost

| | Wires | Adding a device |
|---|---|---|
| **I2C** | 2 (SDA, SCL) | +0 wires — new device = new *address* on the same 2 lines |
| **SPI** | 3 shared (SCLK, MOSI, MISO) | +1 wire — a unique chip-select (CS) per device |

SPI's pin cost is **3 + N** (N = device count): 1 device = 4 pins, 3
devices = 6 pins. I2C stays at 2 forever. That difference is the main
reason I2C is the default for a cluster of small peripherals on a
pin-constrained WROOM-32.

Notes:
- SPI **MISO is optional** for a write-only device (e.g. a display you only
  send to) — but it's still one shared line across the devices that do read
  back, so it doesn't change the "3 shared" count.
- SPI **CS lines can be almost any GPIO** (plain outputs) — only
  SCLK/MOSI/MISO want the SPI-capable pins. Handy for dodging strapping
  pins. Keep CS off the ESP32's **input-only pins (GPIO34–39)** since CS
  must drive.


## Can devices share the same wiring?

**I2C: yes — that's the point.** Every device hangs off the same SDA/SCL in
parallel; the master addresses one at a time. Practical gotchas:

- **One set of pull-ups for the whole bus** (~4.7 kΩ to 3.3V), *not* one
  pair per device. SDA/SCL are open-drain and do nothing without pull-ups.
  Cheap modules often ship with their own pull-ups soldered on — daisy-chain
  several and the parallel combination gets too stiff, weakening the edges.
  Fix: desolder the pull-ups on all but one module.
- **Address collisions are the real trap.** Two devices at the same address
  can't coexist — and whole product categories default to the same one
  (many LCD backpacks 0x27, many BMP/BME sensors 0x76). Most modules expose
  2–3 solder jumpers/address pins for an alternate. Check before buying
  duplicates. Last resort for hard-wired duplicates: a **TCA9548A I2C mux**,
  or use the ESP32's **second independent I2C bus** on other pins.
- **Bus ceiling is total capacitance (~400 pF), not a device count** — every
  device and every cm of wire adds a little.
- **Wedge failure mode:** one device stuck holding SDA/SCL low silences the
  *whole* bus, not just itself. A reason to keep I2C short and local on a
  motor-noisy loco.

**SPI: shares the 3 data lines, but each device needs its own CS.** No
addresses, so no collisions; no shared-line wedge (push-pull, no pull-ups).
The cost is the per-device CS pin.


## Maximum distance

There's **no length limit in the spec** — the limit is **capacitance**.

- **I2C**: spec caps total bus capacitance at ~400 pF. Ordinary wire adds
  ~50–100 pF/m, so **keep it under ~1 m**. Board-level or a short ribbon
  (10–30 cm) is trouble-free. Past ~1 m you get rounded edges, missed ACKs,
  flaky reads — worse at 400 kHz.
  - Stretch options if unavoidable: **lower the clock** (100 kHz or less),
    **stronger pull-ups** (e.g. 2.2 kΩ), or **bus buffers/extenders**
    (P82B715, PCA9600) that isolate cable capacitance and reach tens of
    metres at reduced speed.
- **SPI**: also capacitance/edge-limited, and generally kept to board level
  too; higher clock makes it *less* tolerant of long wiring, not more.

**For this loco:** don't fight it. A 7¼" gauge loco is a metre-plus long
through PWM-motor noise — the worst case for both I2C and SPI even with
extenders. Long runs are exactly what **CAN** (differential, noise-immune,
built for metres in hostile environments) is for. So:

> **I2C/SPI stay local to their board. Anything distant — far-end lighting,
> rear-truck sensors, a second motor stage — gets its own MCU and joins over
> CAN as a node.**

This is the architectural reason the project is CAN-based in the first
place.


## Reliability / failure modes (why SPI for critical, on a noisy loco)

- **SPI** is push-pull, no pull-ups, no addresses → no bus lockups, no
  collisions. Robust in electrically noisy settings. Favour it for
  anything critical enough that you don't want I2C's shared-bus wedge.
- **I2C** is open-drain and shared → simpler wiring, but a single stuck
  device or a bad pull-up choice can take the bus down.


## Character displays (panel board): parallel vs I2C backpack

Almost all cheap 16x2 / 20x2 displays use the **Hitachi HD44780**
controller, in one of two connection styles — and the choice has a Zephyr
driver-support consequence:

- **Parallel (bare LCD → GPIOs): supported in-tree.** Zephyr's `auxdisplay`
  subsystem has a native HD44780 driver (`auxdisplay_hd44780.c`, compatible
  `hit,hd44780`), 4-bit or 8-bit. Works today, no external code — but even
  4-bit mode needs **RS + E + 4 data = 6 GPIOs**, dodging strapping/
  input-only pins and the reserved TWAI pins.
- **I2C backpack (PCF8574 piggyback board): NOT in mainline Zephyr.** This
  is the version most people buy, and the pin-friendly one (**2 pins,
  SDA/SCL**), but Zephyr's HD44780 driver is the GPIO/parallel one — the
  PCF8574-over-I2C path isn't in-tree. You'd port/write a small shim driver
  or use a community one (Zephyr discussion #72745).
- **In-tree + I2C option:** a natively-I2C character display like the
  **JHD1313** (Grove RGB LCD) is supported out of the box, as is a small
  **SSD1306 OLED** via the `display` subsystem — either gives 2-wire hookup
  without the PCF8574 gap.

Hardware gotchas regardless of route: the HD44780 is a **5V part** (3.3V
ESP32 logic usually drives it fine write-only, but contrast/`VO` may need a
trim pot); with an I2C backpack, mind **SDA/SCL level-shifting** since the
backpack often runs the LCD at 5V.

Either way the display slots into
`firmware/zephyr-canbus/boards/esp32_devkitc_esp32_procpu.overlay` the same
way `led0`/`sw0` already do — keeping `src/main.c` board-agnostic.


## Quick map for this build

- **Panel board:** switches + character LCD. I2C backpack (2 pins, needs a
  driver shim) *or* parallel HD44780 (in-tree, ~6 pins) *or* JHD1313/SSD1306
  (in-tree + 2 pins). Local sensors on the same I2C bus.
- **Controller board:** Hall velocity in. If accuracy pushes you to a
  magnetic rotary encoder (AS5047-class), that's **SPI**.
- **Motor board:** current/temp sense — mostly I2C; a fast current ADC
  could be SPI.
- **Between boards:** always **CAN**, never a stretched I2C/SPI.


## Sources

- [Auxiliary display — Zephyr sample docs](https://docs.zephyrproject.org/latest/samples/drivers/auxdisplay/README.html) (in-tree HD44780 auxdisplay driver + sample overlay; accessed 2026-08-20)
- [hit,hd44780 devicetree binding — Zephyr docs](https://docs.zephyrproject.org/latest/build/dts/api/bindings/auxdisplay/hit%2Chd44780.html) (`drivers/auxdisplay/auxdisplay_hd44780.c`; accessed 2026-08-20)
- [Implemented the Zephyr HD44780 I2C driver — Cents, Medium (2024-08)](https://medium.com/@centswu/implemented-the-zephyr-hd44780-i2c-driver-16b352207852) (documents the gap: no in-tree PCF8574-over-I2C HD44780 support; references Zephyr discussion #72745)
- I2C bus capacitance limit (~400 pF) and open-drain pull-up requirement — NXP I2C-bus specification (UM10204), general reference.
- P82B715 / PCA9600 I2C bus extenders — NXP, for long-run I2C (general reference).
- SN65HVD230 3.3V CAN transceiver and 120Ω dual-end termination — see [dev-setup-research.md](dev-setup-research.md) and [esp32-notes.md](esp32-notes.md) in this repo.
