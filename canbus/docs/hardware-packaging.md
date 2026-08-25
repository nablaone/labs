# Hardware packaging — enclosures and mounting

How nodes are physically housed and held in a hostile environment. Compiled
2026-08-20.

## Environment we design against

A 7¼" loco / garden layout is hard on bare boards: **vibration** (the #1
killer — connectors back out, joints crack), **moisture** (outdoor,
condensation), **conductive brake/rail dust**, **heat** (sun + motor + buck
converters), and **motor PWM EMI**.

## Two-tier enclosure standard

- **Clustered loco electronics → DIN-rail bay.** Several nodes + buck
  converters + terminals on a DIN rail inside **one sealed compartment**.
  Modular, serviceable (pull a module to swap/reflash), matches the "factory"
  aesthetic. **DIN modules are not individually IP-sealed** — the surrounding
  cabinet/bay provides weatherproofing.
- **Isolated exposed node (a lone trackside turnout) → sealed IP67 box.**
  Polycarbonate (UV/impact) with gasketed lid and cable glands / Deutsch
  bulkhead penetrations. Not DIN.

Both feed the same Deutsch-connector harness
([power-and-harness.md](power-and-harness.md)).

### Ready-made DIN options

- **ArduiBox ESP (Zihatec)** — enclosure kit with a carrier PCB (onboard
  regulator + prototyping area), fits a 4-module DIN box; "cabinet DC in,
  regulate onboard" maps onto our 48V→local-buck scheme. Fastest professional
  result.
- **Camden Boss CNMB / Italtronic / Phoenix** — empty polycarbonate DIN
  module boxes (2/4/6-module widths, UL94-V0) for your own carrier board.
- **3D-printed DIN clips** — free STLs; print in **PETG/ASA, not PLA** (PLA
  softens in loco/sun heat).
- For harsh EMI/heat spots (controller node near motor): **die-cast
  aluminium** (shielded, heat-sinking) or a **custom-fabricated metal bay**
  (plays to metalworking skills).

## Board mounting

- **Carrier/base board per node type.** Socket the ESP module on **female
  headers** (pull/swap/reflash without desoldering) on a small base that also
  carries the transceiver, buck, level-shifter, and connectors, with a
  **standard 4-hole M3 standoff pattern**. Standoff the carrier into the
  enclosure. Bare dev boards (esp. C3 Super Mini) often lack usable mounting
  holes — the carrier solves that and makes every node physically
  interchangeable.
- **Standoffs/spacers** (M2.5/M3 brass or nylon) — nylon near metal cases to
  avoid shorts. Avoid snap-in/adhesive standoffs on the loco (release under
  vibration).

## Sealing / heat / vibration rules

- **Cable entry is the weak point** — seal at the box wall via **cable
  glands** or **Deutsch bulkhead connectors**, not by cramming wire through a
  hole. Lets you unplug a node without unsealing the harness.
- **Vibration** — threadlocker / nylon-insert nuts on screws, all 4 holes
  used, strain-relieve every cable, consider anti-vibration mounts.
- **Heat** — a sealed box traps buck-converter heat. Heat-sink to a metal
  case, or use **IP-rated / Gore membrane vents** that breathe but block
  water; don't just drill holes.
- **Conformal-coat the boards** — sprayed lacquer sealing against moisture
  and conductive dust; cheap insurance, arguably more important than the
  box's IP rating.
- **Serviceability** — you're still developing: gasketed hinged lids and an
  internal serial/JTAG header + power test points, so you open just the lid,
  not the whole seal.

## Sources

- ArduiBox ESP / Camden Boss CNMB DIN modules — vendor pages (accessed
  2026-08-20).
- IP ratings, conformal coating, cable glands — general enclosure practice.
