# minifreak-skill

A [Claude Code](https://claude.ai/claude-code) skill for controlling an
[Arturia MiniFreak](https://www.arturia.com/products/hardware-synths/minifreak/overview)
over MIDI. Design sounds with AI assistance, push them to the synth, audition
them, and capture your knob tweaks back into JSON.

Sibling of [blofeld-skill](https://github.com/nablaone/blofeld-skill).

## How it differs from the Blofeld skill

The MiniFreak has no publicly documented SysEx patch format, so there is no
full patch dump/upload. Instead this skill uses the official MIDI
implementation:

- **CC** for every front-panel knob in Arturia's chart → applied to the edit buffer
- **Bank Select + Program Change** → load any of the 512 presets as a starting point
- **Knob Send CC** → record the knobs you turn into a patch JSON

Things with no CC (oscillator engine Type, filter type, FX types, mod matrix,
voicing) are listed in the patch's `manual` section, and the skill tells you
to set them on the panel.

## Install

```bash
/plugin marketplace add nablaone/minifreak-skill
/plugin install minifreak@nablaone-minifreak-skill
```

Or clone locally:

```bash
git clone https://github.com/nablaone/minifreak-skill
cd minifreak-skill
uv sync
uv run python3 list_ports.py
```

On the MiniFreak, under Utility → MIDI, set **Program Change = On** and
**Knob Send CC = On**. Also set the Input Channel to 1, or pass `--channel`.

## Usage

Example prompts:

- *"Make a dark woody pluck on my MiniFreak"*
- *"Load preset 42 and make it brighter with more reverb"*
- *"I'll tweak some knobs for 30 seconds, capture what I do"*
- *"Play a C minor 7 chord to test the current sound"*

### Tools

| Script | What it does |
|---|---|
| `send_patch.py` | Apply patch JSON (`--patch-json`, `--file` or stdin) as CCs; `--base-preset N`, `--dry-run` |
| `select_preset.py` | `--preset 1..512` |
| `capture_patch.py` | Listen `--seconds N` and record knob CCs (`--merge` into an existing patch) |
| `play_notes.py` | `--test`, `--minor7`, `--industrial`, `--notes "C4 E4 G4"` |
| `list_ports.py` | Show MIDI ports and which ones are used |

All scripts take `--channel` (or `$MINIFREAK_CHANNEL`) and `--port` (or `$MINIFREAK_PORT`).

### Patch format

```json
{
  "name": "Woody Pluck",
  "manual": {
    "osc1_type": "KarplusStr",
    "osc2_type": "Multi Filter, mode BP12",
    "filter_type": "LP",
    "fx1": "Reverb / Room"
  },
  "params": {
    "osc1_wave": 20, "osc1_timbre": 70, "osc1_shape": 55,
    "filter_cutoff": 58, "filter_resonance": 25, "filter_env_amt": 90,
    "env_attack": 0, "env_decay": 50, "env_sustain": 0, "env_release": 40,
    "fx1_time": 40, "fx1_intensity": 70, "fx1_amount": 30
  }
}
```

The full CC map, engine and FX parameter meanings are in
[`skills/minifreak/SKILL.md`](skills/minifreak/SKILL.md). Sent and captured
patches are saved under `patches/`.

## Requirements

- Python 3.9+, [uv](https://docs.astral.sh/uv/)
- [mido](https://mido.readthedocs.io/) and [python-rtmidi](https://spotlightkid.github.io/python-rtmidi/)
- Arturia MiniFreak connected via USB, firmware 2.0+ recommended (CC7 volume, Wavetable engine)
