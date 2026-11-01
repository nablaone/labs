# DCC accessory decoder primer

Study notes for building a DCC accessory (turnout) decoder on the ESP32,
distilled from the standards in this folder (see [README.md](README.md)).
Section references point at those PDFs.

## 1. The track signal (S-9.1 §2, RCN-210)

DCC is a **bipolar square wave** on the two rails. It carries power and
data at the same time. There's no ground reference. Each bit is two
half-periods of equal length, and **the length is what encodes the value**:

| | Command station sends | Decoder must accept |
|---|---|---|
| "1" half-bit | 55–61 µs (nominal 58) | **52–64 µs** |
| "0" half-bit | 95–123 µs (nominal 100) | **90–10 000 µs** |
| "1" half-bit A/B mismatch | ≤ 3 µs | ≤ 6 µs |

(The 10 ms upper limit on "0" exists because of *stretched zeros*, the
old trick for running one analog loco on a DCC layout, S-9.1 §2.1.)

Consequences for the decoder:

- Polarity is unknown: the decoder can sit either way round on the
  rails. Measure **time between edges** and ignore which way they go.
- A simple robust approach: interrupt on **one** edge direction only and
  measure the full bit period (1 ≈ 116 µs, 0 ≥ 200 µs, threshold ≈ 160 µs).
  An alternative is to timestamp *every* edge and classify each half-bit,
  which is more precise and catches asymmetric junk.
- H0 track voltage is nominally **15 V peak** (decoder must survive up to
  27 V, S-9.1 Table 2.3). **Never** connect the rails to an ESP32 pin
  directly. See §6.

## 2. Packet framing (S-9.2 §A, RCN-211)

```
preamble       start  byte 1    start  byte 2   ...  start  checksum  end
1111111111...  0      AAAAAAAA  0      DDDDDDDD ...  0      EEEEEEEE  1
```

- **Preamble**: ≥ 10 "1"s accepted (station sends ≥ 14). The decoder must
  not *require* more than 12.
- Each byte is preceded by a **0** and sent **MSB first**.
- Last byte is the **XOR of all previous bytes**. Drop the packet on mismatch.
- Packet ends with a **1** (which can also count toward the next preamble).
- Special packets every decoder must handle:
  - **Reset** `00000000 00000000 00000000`: clear volatile state.
  - **Idle** `11111111 00000000 11111111`: ignore.
- Packets are **repeated** by the command station. A decoder will see the
  same command several times, so the action must be idempotent.

A natural decoder is a small state machine:
`PREAMBLE (count 1s) → BYTE (shift 8 bits) → expect 0 (next byte) or 1 (end)`.

## 3. Address partitions (RCN-211, S-9.2.1 §2.1)

The first byte says what kind of decoder the packet is for:

| First byte | Meaning |
|---|---|
| `00000000` | broadcast |
| `0AAAAAAA` (1–127) | multifunction (loco), short address |
| **`10AAAAAA`** (128–191) | **accessory decoders** ← this project |
| `11AAAAAA` (192–231) | loco, long address (first byte of two) |
| `11111111` | idle |

So an accessory decoder can drop every packet whose first byte isn't
`10xxxxxx` almost for free.

## 4. Basic accessory packet (RCN-213 §2.1, S-9.2.1 §2.4.1)

```
byte 1     byte 2
10AA-AAAA  1AAA-DAAR      (+ XOR checksum byte)
```

11-bit address, with **the three high bits sent inverted** (one's complement):

```
packet:   1 0 A  A  A  A  A  A   | 1 A   A  A  D A  A  R
address:  - - A7 A6 A5 A4 A3 A2  | - /A10 /A9 /A8 - A1 A0 -
```

- **D** (bit 3): 1 = activate output, 0 = deactivate.
- **R** (bit 0): which output of the pair. By convention R = 0 is
  *diverging / "red" / stop* and R = 1 is *straight / "green" / go*.
- Outputs come in **pairs**. One turnout with a double-coil drive = one
  pair = one *user* address.

Decoding in C:

```c
uint16_t a = ((~b2 >> 4) & 0x07) << 8   /* A10..A8, inverted on the wire */
           | (b1 & 0x3F) << 2           /* A7..A2 */
           | (b2 >> 1) & 0x03;          /* A1..A0 */
bool activate = b2 & 0x08;
uint8_t r     = b2 & 0x01;
```

### User address vs. wire address (the classic off-by-four)

For compatibility with old command stations, **wire address 4 is shown
to the user as address 1** (`10000001 1111D00R`). With RCN-213's linear
scheme:

```
user_address = wire_address - 3        (wire 4 → user 1)
wire 0..3    → user 2045..2048         (wire 2047 is skipped)
```

Some older stations insert the 0–3 block after every 256 addresses
instead ("non-linear"). RCN-213 marks that as obsolete, but if the
turnout number on the throttle and the decoder disagree by 4, this is
why. The full table is on pages 4–5 of `rcn/RCN-213.pdf`.

The **decoder** only ever sees the 11-bit wire address. Two ways to
store "my address" (RCN-225 §3.1, S-9.2.2):

- **Decoder addressing**: CV1 (low 6 bits) + CV9 (high bits) hold
  `wire_address >> 2`, so one decoder answers to four consecutive pairs.
  This is the classic 4-turnout decoder.
- **Output addressing**: CV1/CV9 hold the user address of the first
  pair. Finer grained.

The **PIKO 55030** does neither. It *learns* an arbitrary address per
output: press the button, then flip the turnout on the throttle (§7).
Copying that is the easiest way to set addresses without a
programming track.

## 5. Other accessory packets (RCN-213 §2.2–2.3)

- **Extended accessory** `10AAAAAA 0AAA0AA1 DDDDDDDD`: same address
  bits, one 8-bit value per address (signal aspect 0 = stop), or
  `RZZZZZZZ` = output R on for Z × 100 ms (0 = off, 127 = on until
  the next command).
- **NOP** `10AAAAAA 0AAA1AAT`: RailCom service request. Ignore it.
- **Programming on main (POM)** for accessories: see RCN-214 /
  S-9.2.1. Not needed for the first version.

Unknown or unsupported packets **must be ignored**, never acted on
(RCN-213 §1.2).

## 6. Hardware interface (TN-9, common practice)

This project's input stage: 6N137, an ordinary LED as the reverse
diode, ESP32-C3 on a battery supply that provides 5 V (feeding the
6N137 and the Super Mini's 5V pin; the board's own regulator makes the
3.3 V for the ESP and the R2 pull-up).

```
 TRACK SIDE (isolated)               6N137               ESP SIDE (battery / USB)

                                 ┌───────────┐
 rail A ──[R1 1.5k ½W]──●────────┤2 A   VCC 8├──●── 5V
                        │        │           │  │
                     LED1 ▲      │      EN 7 ├──●
                (cathode up,     │           │
                 anode down)     │     OUT 6 ├─────●──[R2 2.2k]── 3V3
                        │        │           │     │
 rail B ────────────────●────────┤3 K   GND 5├──●  └── GPIO4 (DCC_IN)
                                 └───────────┘  │
                                                └── GND
 100nF ceramic directly between pins 8 and 5.
```

LED1 sits anti-parallel to the 6N137's input LED (LED1 anode to pin 3,
cathode to pin 2).

- **Optocoupler**: a 6N137 (10 MBd) is easily fast enough for 58 µs
  half-bits; a PC817 is too slow. It isolates the ESP32 from track
  voltage and doesn't care which rail is positive. The input LED
  conducts on one polarity only, so the output shows exactly one half
  of each bit.
- **Reverse protection with an LED**: the 6N137's input LED only
  tolerates **5 V reverse**. An anti-parallel diode carries the other
  half-cycle and clamps the reverse voltage to its own forward drop.
  An ordinary red/green/yellow LED (Vf ≈ 2 V) does that job and also
  lights up as a "DCC present" indicator. Use a plain LED, not one with
  a built-in resistor.
- **R1**: sets the input current. Per the Vishay datasheet
  (`parts/vishay-6n137.pdf`), the input threshold is 2.4 mA typical and
  5 mA worst case, the recommended "on" current is **5–15 mA**, and the
  absolute maximum is 20 mA. With 1.5 kΩ (input LED Vf ≈ 1.4 V) you get
  about 9–11 mA at 15–18 V track voltage, 14 mA at 22 V, and 4.7 mA at
  S-9.1's 8.5 V minimum, which is still above the typical threshold.
  It dissipates up to about 0.3 W, so use a ½ W part. (The 1 kΩ I
  suggested earlier pushes about 20 mA at 22 V, which is the absolute
  maximum.)
- **6N137 supply**: the datasheet part is a **5 V device (VCC 4.5–5.5 V)**.
  The output is **open-drain** (VO max 7 V), so the clean solution is
  VCC = 5 V with the R2 pull-up going to **3.3 V**. GPIO4 then never
  sees more than 3.3 V. The recommended pull-up range is 330 Ω–4 kΩ, so
  2.2 kΩ fits. Pin 7 (Enable) works when left unconnected (see the
  truth table), but tying it to VCC is more robust against noise. The
  datasheet recommends 100 nF across pins 8–5, placed right at the chip.
- **Speed**: the propagation delay is about 50 ns and the rise time
  with 2.2 kΩ is about 0.1–0.2 µs, which is negligible against 58 µs
  half-bits.
- **Output polarity**: the output is pulled **low while the input LED
  conducts**. The decoder measures time between edges, so this
  inversion doesn't matter.
- **Power**: the ESP runs from a battery, so the track and the ESP share
  no ground at all, and the opto is the only link between them. During
  bench work, USB power from the Mac is equally fine for the same
  reason. Don't feed the battery into the Super Mini's 5V pin while USB
  is plugged in unless you've checked that the board has a diode on
  VBUS.
- **Later, track power instead of battery**: bridge rectifier →
  capacitor → buck converter to 5 V (which then also feeds the 6N137).
- **Input pin** on the C3 Super Mini: pick a non-strapping GPIO, e.g.
  GPIO3–GPIO7, GPIO10, GPIO20 or GPIO21. Avoid GPIO2, GPIO8 and GPIO9,
  which are strapping pins (and GPIO8 is the LED, GPIO9 is BOOT).
- **Outputs**: double-coil turnout drives need a short pulse (typically
  100–500 ms) at track voltage. Never leave a coil on, it will burn.
  Drive them from the rectified track supply via a ULN2803 or logic-level
  MOSFETs with flyback diodes. RCN-213 requires an automatic switch-off
  timeout (configurable via CV, RCN-225).

## 7. PIKO specifics (`piko/piko-55030-55031-switch-decoders.pdf`)

- **#55030**: 2 outputs, each driving one double-coil drive (A1, A2, plus
  common), max 2 A, short-circuit protected. Each output gets its own
  freely chosen address.
- **#55031**: same, but 2 changeover relays for lamps/continuous loads.
- Address range: DCC 1–2040 (PIKO Digi-Power-Box). Also speaks Motorola
  (not relevant here).
- **Programming**: press the button (LED blinks 3×), press it again
  for DCC (3×), switch the turnout for A1 on the throttle (3×), then
  for A2 (5×). Done.
- Control LED lights when a valid packet for this decoder is received.

These give a sensible feature target for this project.

## 8. Implementation plan (suggested)

1. **Signal capture**: GPIO edge interrupt + `esp_timer_get_time()`, or
   the RMT RX peripheral for hardware-timed pulse widths. The ESP32-C3
   has no MCPWM capture unit, so it's one of those two. Push
   bit values into a FreeRTOS queue/stream buffer. The ISR stays tiny.
2. **Packet decoder task**: preamble/byte state machine, XOR check,
   emits whole packets. CLI: `dcc stats` (packets ok / bad XOR / bad bits),
   `dcc sniff` (print packets).
3. **Accessory layer**: filter `10xxxxxx`, decode the address, match
   against configured addresses, ignore repeats.
4. **Outputs**: pulse a GPIO for N ms per output, with a forced
   timeout. LEDs first, real coils later.
5. **Config**: addresses in NVS, with a learn mode on the BOOT button
   (PIKO-style) plus `config` CLI commands.
6. Later: CV/POM, extended accessory packets (signals), RailCom.

A handy bench setup is a second ESP32 as a **DCC generator** (H-bridge
driver such as an L298N/DRV8871 on a 15 V supply), or simply the PIKO
SmartControl / Digi-Power-Box with a short piece of track.
