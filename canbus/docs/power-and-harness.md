# Power and harness

The electrical distribution standard for the loco (and any wired trackside).
Compiled 2026-08-20.

## Voltage: 48V native

**Standardized on 48V**, taken directly from the traction battery
(Tesla-style single-battery-voltage-for-everything). Rationale is the same
physics the auto industry and PoE converge on: **48V moves the same power at
~¼ the current of 12V**, so thin conductors suffice and losses stay low.

- **No boost stage** — 48V is native, so we only ever *buck down*.
- **Battery voltage swings with charge** (a "48V" pack runs ~40V empty to
  ~54–58V full). Everything taking raw pack voltage must be rated **~60V
  input** for headroom. Do not run a 50V-max part off a full pack.
- Logic never sees 48V — it's always behind a local buck at 3.3V.

## Harness: Ethernet (Cat) cable + Deutsch connectors

Cheap Cat5e/6 gives 4 twisted pairs at ~100–120 Ω — ideal for CAN, and the
thin conductors are fine at 48V's low current. Connectorized with **Deutsch
DT** (sealed IP67, latching, vibration-proof, auto/off-highway grade).

### Pinout

| Pair | Use | Note |
|---|---|---|
| 1 | CAN-H / CAN-L | ~100 Ω twist, impedance-matched |
| 2 | +48V | feeds each node's local buck (signal/accessory power only) |
| 3 | 48V return | keep as a full pair |
| 4 | **Ground reference** (ties 48V ↔ battery/motor domains for CAN) *or* hardwired e-stop | highest-value use of the spare pair |

- **120 Ω terminators at the two CAN ends only** (a Deutsch shell with a
  resistor across the CAN pins = tidy screw-on terminator).
- **Traction/motor power NEVER goes in the Cat cable** — the motor runs off
  the battery through its own heavy cable. Cat pairs carry only CAN +
  signal/accessory power.
- Keep the CAN pair physically separated from the 48V pairs in the bundle;
  snubber inductive loads to avoid switching transients coupling into CAN.

## Per-node regulation

Distribute high-volts-low-amps, **step down locally at each node** — the same
reason PoE distributes 48V and bucks at the device.

- **Single-stage** (logic-only node): one **60V-rated 48V→5V** buck; the
  ESP board's onboard regulator makes 3.3V.
- **Two-stage** (node needs 12V for a relay coil / point motor / LED strip):
  a **60V-rated 48V→12V** buck feeding a 12V rail, then cheap **12V→5V/3.3V**
  locally. Advantage: the ubiquitous cheap LM2596-class modules (≤40V in) are
  usable on the 12V side, and 12V is independently useful.

### Current budget (why this is trivially safe)

A node bucks 48V→3.3V, so **Cat-cable current is ~15× lower than the node's
actual draw**. A node + LEDs ~1 W ≈ 0.03 A at 48V. **10 nodes ≈ 0.3 A** down
a full pair — a fraction of a 24AWG conductor's ~1–1.5 A limit. Voltage drop
over layout distances is negligible, and a wide-input buck ignores a volt of
sag anyway. You can run dozens of nodes off one 48V injection point.

## Grounding (two power domains)

The 48V-fed control nodes and the battery/motor side must share a **common
ground reference** where CAN crosses between them, or transceivers drift out
of common-mode range. That's the reserved pair 4 job. Isolated DC-DC
converters are the cleaner-but-pricier alternative for full domain isolation.

## Part selection notes

- **48V→5V, 60V-rated**: generic wide-input (8–60V) buck modules; keep ≥60V
  headroom for full-charge.
- **48V→12V, 60V-rated**: dedicated 36/48V→12V modules (32–60V in).
- **Pololu caveat:** the Botland Pololu D36V50F12 (12V out) is rated to only
  **50V in** — fine on a discharged pack, unsafe at full charge; use it only
  on the 12V→5V *second* stage (e.g. D24V22F5), not on raw 48V.

## Sources

- 48V rationale (current/loss vs 12V) — automotive 48V and PoE (IEEE 802.3
  PoE ~44–57V) practice.
- I2C/CAN cable capacitance, 24AWG current — general references; see
  [bus-selection.md](bus-selection.md).
- Buck part specs — Pololu / vendor datasheets (accessed 2026-08-20).
