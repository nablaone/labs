# docs

Reference material for the DCC accessory decoder. The PDFs are fetched
copies of the official documents, kept locally but gitignored (NMRA,
RailCommunity and PIKO hold the copyright). Re-download them from the URLs
below. The study notes in [dcc-accessory-primer.md](dcc-accessory-primer.md)
are this lab's own summary.

## Reading order

1. **[dcc-accessory-primer.md](dcc-accessory-primer.md)**: this lab's own
   summary of everything below that an accessory decoder needs.
2. **S-9.1**: what the track signal looks like electrically (bit timing).
3. **S-9.2**: packet framing (preamble, start/stop bits, XOR checksum),
   plus reset, idle and broadcast packets.
4. **RCN-213** (German) or **S-9.2.1 §2.4**: the accessory decoder packet
   formats and address mapping. RCN-213 is the clearer of the two and has
   the full user-address ↔ packet ↔ CV1/CV9 tables.
5. **S-9.2.2 / RCN-225**: configuration variables (CV1/CV9 address,
   CV29 config, output pulse time CVs).
6. **PIKO 55030 manual**: the commercial decoder this project imitates.

## NMRA (English), `nmra/`

Index page: <https://www.nmra.org/index-nmra-standards-and-recommended-practices>

| File | Document | URL |
|---|---|---|
| `s-9.1_electrical_standards_for_digital_command_control.pdf` | S-9.1 Electrical (Jun 2026 draft) | https://www.nmra.org/sites/default/files/standards/sandrp/DCC/S/s-9.1_electrical_standards_for_digital_command_control.pdf |
| `s-9.2_dcc_communications_standard.pdf` | S-9.2 Communications (2004) | https://www.nmra.org/sites/default/files/standards/sandrp/DCC/S/s-92-2004-07.pdf |
| `s-9.2.1_dcc_extended_packet_formats.pdf` | S-9.2.1 Extended Packet Formats (2025) | https://www.nmra.org/sites/default/files/standards/sandrp/DCC/S/s-9.2.1_dcc_extended_packet_formats.pdf |
| `s-9.2.2_decoder_cvs.pdf` | S-9.2.2 Configuration Variables | https://www.nmra.org/sites/default/files/standards/sandrp/DCC/S/s-9.2.2_decoder_cvs_2012.07.pdf |
| `s-9.2.3_service_mode_programming.pdf` | S-9.2.3 Service Mode | https://www.nmra.org/sites/default/files/standards/sandrp/DCC/S/S-9.2.3_2012_07.pdf |
| `s-9.2.4_fail-safe.pdf` | S-9.2.4 Fail-Safe | https://www.nmra.org/sites/default/files/standards/sandrp/DCC/S/s-9_2_4_fail-safe.pdf |
| `tn-9_wiring_for_dc_and_dcc.pdf` | TN-9 Wiring for DC and DCC | https://www.nmra.org/sites/default/files/standards/sandrp/Elec/TN/tn-9.pdf |

## RailCommunity RCN (German, European manufacturers incl. PIKO), `rcn/`

Index page: <https://normen.railcommunity.de/>

| File | Document |
|---|---|
| `RCN-210.pdf` | DCC Bit-Übertragung (bit transmission), https://normen.railcommunity.de/RCN-210.pdf |
| `RCN-211.pdf` | DCC Paketstruktur, Adressbereiche, globale Befehle, https://normen.railcommunity.de/RCN-211.pdf |
| `RCN-213.pdf` | DCC Betriebsbefehle für Zubehördecoder (accessory packets), https://normen.railcommunity.de/RCN-213.pdf |
| `RCN-225.pdf` | DCC Konfigurationsvariablen (CVs), https://normen.railcommunity.de/RCN-225.pdf |

## PIKO, `piko/`

| File | Document |
|---|---|
| `piko-55030-55031-switch-decoders.pdf` | Manual for #55030 (2× double-coil turnout outputs) and #55031 (2× relay outputs), DE/EN, https://www.piko-shop.de/is.php?id=2778 |

## Parts datasheets, `parts/`

| File | Document |
|---|---|
| `vishay-6n137.pdf` | Vishay 6N137 / VO2601 / VO2611 10 MBd high-speed optocoupler (DCC input stage), https://www.vishay.com/docs/84732/6n137.pdf |

## Not fetched yet (worth reading)

- **RCN-214**, configuration commands (POM/service mode packets):
  https://normen.railcommunity.de/RCN-214.pdf
- **RCN-217**, RailCom (decoder → command station feedback), only
  needed if feedback is ever added.
- **NmraDcc** Arduino library (MIT-ish, widely used reference decoder
  implementation): https://github.com/mrrwa/NmraDcc. Reading its ISR
  is a good sanity check for your own bit decoder.
