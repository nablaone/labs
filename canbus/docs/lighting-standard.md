# Lighting standard — addressable LEDs

One scheme for all signal lamps, marker lights, and indicators across loco
and trackside. Compiled 2026-08-20.

## Choice

**Default: WS2812B / SK6812 driven via the ESP32 RMT `led_strip` driver.**
Fallback: APA102/SK9822 over SPI where robustness or long runs demand it.

Addressable LEDs each contain a controller, individually settable over **one
data wire**, chained. One GPIO drives a whole string of lamps.

- **WS2812B** — one-wire, ubiquitous, cheap; strict timing, no clock.
- **SK6812 (RGBW)** — WS2812 variant with a true white channel; better for
  signal lamps needing clean white / accurate color. Preferred for signals.
- **APA102 / SK9822** — two-wire (data + clock); timing not critical (shift
  over SPI at any speed), more robust, higher refresh. Use for long runs or
  where interrupt timing can't be guaranteed.

## Why RMT solves WS2812's weakness

WS2812's bit timing is tight (~0.4 µs vs ~0.8 µs pulses) and has no clock, so
software bit-banging glitches under interrupts (WiFi/CAN). The ESP32 **RMT
peripheral generates the pulse train in hardware**, immune to interrupt
activity — ESP-IDF's built-in RMT-based `led_strip` driver is the clean,
rock-solid way. This removes the only real WS2812 downside. (APA102 over SPI
is likewise interrupt-proof.)

## Mandatory wiring rules (bake into every lamp)

1. **Level-shift the data line.** 3.3V ESP32 data into 5V LEDs is marginal —
   the first LED often misreads it. Use a **74AHCT125-class shifter**, or
   power the strip at ~4.5V, or use 3.3V-native LEDs. On a noisy loco, shift
   properly — don't rely on "usually works."
2. **Separate power, sized.** ~60 mA per RGB LED at full white. LED power
   comes from the node's 5V rail (see [power-and-harness.md](power-and-harness.md)),
   **never through the data-carrying logic**. Inject at both ends of long
   runs.
3. **Common ground** between ESP32 and the LED supply — the classic "first
   LED works, rest are garbage" bug is a missing shared ground.

## Integration

Each signal/lamp = a few addressable LEDs on one node's GPIO, driven by the
RMT `led_strip` driver, powered from that node's 5V rail, level-shifted data.
A "set aspect / set state" command (CAN or ESP-NOW) writes colors. **One
reusable lighting module** across loco marker lights, cab lights, and
trackside signals — standardize the data format (GRB / RGBW, 8-bit/channel,
chained).

## Sources

- ESP-IDF `led_strip` RMT driver, WS2812/SK6812 timing — Espressif ESP-IDF
  docs (accessed 2026-08-20).
- APA102/SK9822 two-wire protocol — vendor datasheets.
