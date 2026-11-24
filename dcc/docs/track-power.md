# Powering the decoder from the rails

Right now the ESP32-C3 runs from a battery or USB, and the 6N137 opto is
the only link to the track (primer §6). This note covers the next step:
taking power from the DCC signal itself, which is how a real decoder like
the PIKO #55030 does it.

## Why it works

DCC is a bipolar square wave: the rails swap polarity every 58 µs (a "1")
or ≥100 µs (a "0"), but the *magnitude* is always about 15 V. Rectify it
and you get almost clean DC. Unlike mains AC, the voltage doesn't pass
slowly through zero; it flips in about a microsecond. So the ripple is
tiny, and the bulk capacitor mostly has to cover dirty-rail dropouts and
the RailCom cutout, not the waveform itself.

## Signal flow

```
rail A ──┬───────────────────────────────► opto input (unchanged, still on the raw rails)
         │
rail B ──┼──┬────────────────────────────►
         │  │
       ┌─┴──┴─┐  +V_bus ≈ 13–14 V
       │BRIDGE├──[R_in]─┬────────┬───►|D2─┬──► BUCK 5 V ──► Super Mini 5V pin + 6N137 VCC
       │      │         │        │        │    (≥30 V in)
       └──┬───┘        C1      TVS       C2
          │            470µ   SMBJ28A    220µ + 100n
          │            35V                │
          └─────────────┴────────┴────────┴──► GND (ESP ground = bridge minus)
                        │
                        └──► solenoid driver / CDU (on the bus, before D2)
```

## Stage by stage

### 1. Bridge rectifier

A ready-made bridge (MB6S, DB107) or **four discrete diodes** works. A
bridge is just four diodes in one package:

```
          D1            D2
rail A ───┬──►|──┬──|◄──┬─── rail B
          │      │      │
          │     +V      │
          │      │      │
          └──|◄──┼──►|──┘
          D3     │      D4
                GND
```

Read it like this: each rail has one diode pointing toward +V and one
pointing away from GND. Whichever rail is high at the moment feeds +V
through its upper diode, and the other rail returns current through its
lower diode. Two diodes conduct at any time, so you lose two forward
drops.

| Diode | Verdict |
|---|---|
| **1N5819** (Schottky, 1 A, 40 V) | The best choice for the logic supply. It drops about 0.4 V per diode (≈0.8 V total) and is fast. 40 V covers the 27 V worst-case track (S-9.1). |
| **1N5822** (Schottky, 3 A, 40 V) | Same, with headroom. Use it if the bridge also feeds the turnout coils. |
| **UF4007** (ultrafast silicon) | Fine. It drops about 1 V per diode. |
| **1N4007 / 1N4001** | It works, and many DIY decoders use it. These are slow "mains" diodes, though. On every DCC edge (about 10,000 per second) they conduct backwards for a few µs before turning off, which gives small current spikes and extra heat. Acceptable, but not the nicest option. |

**Could you use just one diode?** A single diode (half-wave) also works,
because the capacitor easily bridges a 58–100 µs half-period. But you
draw current from only one polarity, and the ground ends up tied directly
to one rail. The full bridge costs a few cents more and is cleaner.

### 2. Inrush limiter (R_in) and bulk capacitor C1

C1 is 470 µF, rated **35 V** because track voltage can reach 27 V. It
rides through dirty-rail dropouts and the RailCom cutout.

To size it, use C = I·Δt / ΔV. At 100 mA for 50 ms while the bus sags
from 14 V to 8 V:

    C = 0.1 × 0.05 / 6 ≈ 830 µF

470 µF covers typical dropouts. Go bigger if you see resets on dirty
track.

**Inrush catch:** at power-on, a big uncharged capacitor looks like a
short to the booster, and the PIKO Digi-Power-Box may trip its overload
detection. Put about 10–22 Ω (½ W) or a small NTC in series in front of
C1, or a resistor with a diode bypassing it.

### 3. TVS diode

An SMBJ28A across the bus clamps spikes from derailments and wheels
bridging rail gaps. Its 28 V standoff sits just above the 27 V maximum
track voltage.

### 4. Buck converter to 5 V

- **Input rating ≥ 30 V.** That rules out the common MP1584 modules (28 V
  max). A Pololu D24V5F5 / D24V10F5 (36 V) or anything built on a 40 V-class
  regulator works.
- **No linear 7805.** Going from 14 V to 5 V at 150 mA burns about 1.4 W.
- **Current budget:** the C3 draws about 25–80 mA without Wi-Fi, with
  about 350 mA peaks during Wi-Fi TX. At about 85 % efficiency from a 14 V
  bus, the peak track current is roughly

      5 V × 0.35 A / 0.85 / 14 V ≈ 150 mA

  That's tiny for a booster.

The 5 V output feeds the Super Mini's **5V pin** (its onboard LDO makes
3.3 V) and the 6N137's VCC.

### 5. D2 + C2: separate logic from solenoids

A PIKO turnout coil pulls 0.5–1 A for 100–300 ms and drags the bus down.
D2 (1N5819) stops C2 from being drained into the coil, so the buck keeps
running and the ESP doesn't brown out mid-pulse. It's the same idea as a
decoupling stage in an analog signal chain. The coil drivers
(ULN2803/MOSFETs, or a CDU) take their power from the bus **before** D2.

## What changes on the signal side

- **The opto input stays exactly as drawn in primer §6.** It keeps
  sitting on the raw rails before the bridge, with the same R1 and
  anti-parallel LED.
- **You lose galvanic isolation.** The ESP ground is now the bridge's
  minus, which is connected to whichever rail is lower at that moment
  through one bridge diode. The opto still does level shifting and
  protects the GPIO, but nothing is isolated anymore. Because the ground
  is shared, some DIY decoders replace the opto with a resistor divider
  and clamp from one rail. Keep the opto anyway: it's more robust and
  already built.

## USB while on the track

This is the one thing that can bite you. With the decoder powered from
the rails, plugging USB into the Mac ties the Mac's ground to the
bridge's minus. If the command station or booster is also grounded
(connected to the same Mac, or via a mains-earthed PSU), current can flow
booster → rail → bridge diode → USB ground → back to the booster. That's
a short through half the H-bridge.

For flashing and monitoring, pick one of these:

- Unplug the track when USB is connected.
- Keep running from battery or USB, as now, during development.
- Use a **USB isolator** (ADuM3160-based, about €10) between the Mac and
  the board.

Also, don't let the buck's 5 V and USB VBUS fight each other on the 5V
pin. Check whether the Super Mini has a diode on VBUS, or add a Schottky
in series with the buck output.

## Parts list

| Part | Example |
|---|---|
| Bridge | 4× 1N5819 (or 1N5822 if it also feeds coils), or MB6S |
| R_in (inrush) | 15 Ω ½ W |
| C1 | 470 µF / 35 V electrolytic |
| TVS | SMBJ28A |
| D2 | 1N5819 |
| C2 | 220 µF / 16 V + 100 nF ceramic |
| Buck | Pololu D24V10F5 (5 V, 1 A, 36 V in) |
