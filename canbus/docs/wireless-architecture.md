# Wireless architecture — where radio fits, and where it must not

The system is CAN-wired by design. Radio is used only where it's genuinely
right. This doc defines the boundary and picks the technology per use.
Compiled 2026-08-20. See [project-charter.md](project-charter.md) safety
posture.

## The hard boundary

**CAN (wired) for anything that moves a train or stops one. Radio only for
telemetry and the physically un-wireable.**

- **Never on wireless:** vital control, e-stop, anything safety-timed. Radio
  is best-effort, non-deterministic, and interferes; unfit for vital paths.
- **Fine on wireless:** config, diagnostics, telemetry, and the physically
  un-wireable moving/modular bits — always with a **heartbeat timeout to a
  safe default** on signal loss.

Two things justified the wireless uses we accepted:
- the link crosses a gap a wire *can't* (a moving car; a modular panel), and
- the payload is non-vital (or gated by confirmed feedback before action).

## ESP32 radio options (native unless noted)

| Tech | Range | Rate | Native on | Use for |
|---|---|---|---|---|
| **WiFi** | tens of m | high | all but H2 | base-station telemetry, config |
| **ESP-NOW** | tens of m (1 hop) | low-latency | all (rides WiFi radio) | **wireless turnouts**, tail lamp |
| **BLE** | short | low | all (BT5 on C3/S3/C6) | phone diagnostics/telemetry |
| **802.15.4 / Thread** | tens of m, meshed | low, low-power | **C6/H2 only** | future large trackside mesh |
| **LoRa** (SX126x, SPI) | km | tiny, high-latency | add-on chip | long-range loco status *only* |

## Decisions by use

### Wireless turnouts (trackside) → ESP-NOW

The track is modular (rearranged in minutes), so a fixed CAN trunk is wrong —
you'd re-lay cable and move terminators each time. Turnouts go **wireless per
node**. On a 1.5-panel layout every turnout is within one radio hop of the
cab/handset, so **ESP-NOW point-to-point (or a small BLE star)** beats a full
mesh: simpler, lower-latency, self-rejoins when a panel moves.

Safety here is acceptable because: **one loco, one driver** who both throws
the point and drives — a lost/late command doesn't cause a surprise
conflicting movement. **But the driver acts on confirmed sensed position
only** (the node reports "thrown ✓" from its limit switch, not from the
command). Command wireless; decision-to-drive on feedback. See
[trackside-control.md](trackside-control.md).

### Tail lamp on the last car → ESP-NOW / BLE broadcast

Un-wireable (car moves relative to loco) and cosmetic. Loco advertises
direction/brake/state; the tail car listens. **Fail-safe: stay lit red on
timeout** (dark is the dangerous failure). One hop covers any realistic
consist, so no mesh needed.

### Phone/laptop diagnostics → BLE

Lowest-risk, highest-value first wireless use: a BLE GATT service exposing a
node's state/faults/logs, read from a phone without opening the box. See
[diagnostics.md](diagnostics.md).

### Long-range status (optional) → LoRa

Only if the line spans a large property: a loco node trickles position/
battery/speed/fault every few seconds to a base station, out of WiFi/BLE
range. Use **raw LoRa point-to-point, not LoRaWAN**. Not for control. A
Meshtastic board (ESP32-S3 + SX1262) can be reflashed for this.

### Thread — reserved for later

If the trackside grows into *many* low-power nodes, some out of single-hop
range, wanting battery + sleep, **Thread on ESP32-C6** is the purpose-built
mesh (self-healing, routed, IPv6). Backing is heavyweight (Apple/Google/
Amazon via Matter), but its ecosystem is smart-home, not DIY control — you'd
adopt it for the capabilities, not for community examples. ESP-NOW stays the
default until scale forces the change.

## The bridge node

Loco CAN and wireless trackside don't share a bus (the loco moves). They meet
at **one bridge node** with both a CAN interface and a radio — the single
place the domains touch, and the natural home for the CAN↔ESP-NOW payload
mapping ([can-message-spec.md](can-message-spec.md)).

## Sources

- ESP-NOW, Thread/802.15.4, BLE variants — Espressif ESP-IDF wireless docs.
- LoRa vs LoRaWAN, SX126x over SPI — Semtech references.
- Thread governance/Matter — Thread Group / CSA (general reference).
