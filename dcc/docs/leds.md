# Addressable LEDs (wiring test)

5 mm through-hole addressable LEDs (WS2812D-F5 / PL9823): they look like
ordinary LEDs but have four legs and a controller chip inside. One GPIO
drives the whole chain. For now they're switched from the CLI and the
web page. Later they'll show turnout positions. The firmware is in
`main/leds.c`, and the chain's mechanics and bit timing are explained in
`main/leds.h`.

## Pinout

Hold the LED with its legs down and the **flat side on the left**:

```
  1    2    3    4
 DIN  VDD  GND  DOUT
      ▲
  longest leg
```

- **Leg 2 (longest) = +5 V, leg 3 = GND.** Get these right: reversed
  power kills the chip.
- Legs 1 and 4 are swapped on some batches. That's harmless: if the
  LED doesn't respond, move the data wire to the other outer leg.

## Wiring

```
5V   ──●────────────●────────────●──── ... ──┬──
       │            │            │           ═ 470 µF (once, where 5V enters)
GND  ──●────────────●────────────●──── ... ──┴──
       │2,3         │2,3         │2,3
GPIO5─[330Ω]─► 1  LED0  4 ──► 1  LED1  4 ──► 1  LED2  4 ──► (last DOUT open)

each LED: 100 nF ceramic between legs 2 and 3, close to the LED
```

- 5V and GND go to every LED **in parallel**. Only the data line is
  daisy-chained, DOUT → DIN.
- 5V comes from the Super Mini's **5V pin** (USB), which is fine for a
  handful of LEDs at the default brightness (40/255). Full white is
  about 60 mA per LED.
- The 330 Ω resistor on the data line damps ringing and protects GPIO5
  if a wire touches the wrong leg.
- **3.3 V data into 5 V LEDs** is below the official 0.7 × VDD
  threshold but usually works. If the LEDs flicker or show random
  colours, put a 1N5819/1N4148 diode in series with the LEDs' 5V feed.
  That gives about 4.4 V, so 3.3 V data reads as a clean "1".

## Bring-up

0. **Check the controller side first** with `led probe`: put a jumper
   from GPIO5 to GPIO6 (the pin right below it) and run it. The RMT
   peripheral's receiver reads back the bits the transmitter sends and
   compares them, so you don't need a scope:

   ```
   sent 192 bits on GPIO5, captured 192 on GPIO6
     high:  0.4 us x96  0.9 us x96
     low :  0.4 us x95  0.9 us x96
   OK: all 192 bits match, the data leaves GPIO5 correctly
   ```

   "Nothing arrived" means the jumper is missing or GPIO5 isn't
   driving. Pulse widths far from 0.4/0.9 µs mean a timing problem. If
   the probe passes, any remaining problem is in the LED wiring. The
   jumper can stay in place while LEDs are connected.
1. Wire **one** LED, flash, and press Enter for the CLI.
2. `led 0 red`. If it lights green instead, run `led order rgb`
   (PL9823 sends red first, WS2812 sends green first), and set
   `LEDS_ORDER_RGB` in `main/node_config.h` to make that the default.
   If nothing lights, swap the wires on legs 1 and 4.
3. Add the rest and run `led test`. It walks red/green/blue along the
   chain, LED 0 first. If the walk stops at some LED, check that LED's
   DOUT → next DIN link and the next LED's power.

## Control

CLI (`led help` prints this list):

```
led                       show state
led <n|all> on|off        colour is kept while off
led <n|all> <colour>      red green blue white yellow orange cyan magenta
led <n|all> <r> <g> <b>   0-255 each
led bright <0-255>        global brightness
led order grb|rgb
led test
led probe                 loopback self-test, jumper GPIO5 -> GPIO6
```

Web page (`make web`, then **Connect**): the **LEDs** panel has one
bulb per LED. Click a bulb to switch it on or off, and use the colour
box under it to set its colour. There's also a brightness slider and
all on / all off. The page follows CLI changes live. Without a
connection the panel works as an offline preview.
