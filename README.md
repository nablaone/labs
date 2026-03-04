# blofeld-skill

A [Claude Code](https://claude.ai/claude-code) skill for controlling a [Waldorf Blofeld](https://waldorfmusic.com/blofeld/) synthesizer via MIDI SysEx. Read and write patches, play test notes, and design sounds with AI assistance.

## Install

```bash
/plugin marketplace add nablaone/blofeld-skill
/plugin install blofeld@nablaone-blofeld-skill
```

Or clone locally:

```bash
git clone https://github.com/nablaone/blofeld-skill
cd blofeld-skill
uv sync
```

Make sure your Blofeld is connected via USB MIDI and powered on before use.

## Usage

Once installed, Claude Code can interact with your Blofeld directly. Example prompts:

- *"Read patch A1 from my Blofeld"*
- *"Create a warm pad sound and send it to bank B program 5"*
- *"Play a C minor 7 chord to test the current sound"*
- *"Modify the filter cutoff on the current patch and add some reverb"*

### Tools

The skill exposes three tools:

**`get_patch`** — Retrieve a patch from the synthesizer:
```
bank: A-H
program: 1-128
```

**`send_patch`** — Send a patch JSON to the synthesizer:
```
bank: A-H
program: 1-128
patch: JSON patch object
```

**`play_notes`** — Audition the current sound:
```bash
python3 play_notes.py --test              # C major triad
python3 play_notes.py --minor7            # Cm7 chord
python3 play_notes.py --notes "C4 E4 G4"  # Custom notes
```

### Patch format

Patches are represented as JSON. The main sections are:

| Section | Description |
|---------|-------------|
| `oscillators` | 3 oscillators — shape, octave, pitch, detune, FM, PWM |
| `filters` | 2 filters — type, cutoff, resonance, drive, envelope |
| `envelopes` | 3 envelopes — ADSR with attack level and dual decay/sustain |
| `lfos` | 3 LFOs — shape, speed, sync, delay, fade |
| `effects` | 2 effects slots — chorus, flanger, phaser, overdrive, delay, reverb |
| `mod_matrix` | 16 modulation slots — source, destination, amount |
| `modifiers` | 4 modifiers for combining modulation sources |
| `arpeggiator` | Mode, clock, range, direction, pattern steps |
| `amp` | Volume, velocity, pan |

Example — a warm pad:

```json
{
  "name": "Warm Pad",
  "oscillators": [
    {"shape": 2, "octave": 64, "pitch": 64, "detune": 60},
    {"shape": 2, "octave": 64, "pitch": 64, "detune": 68},
    {"shape": 4, "octave": 52, "pitch": 64, "detune": 64}
  ],
  "filters": [
    {"type": 1, "cutoff": 80, "res": 20, "env_amt": 30, "keytrack": 64}
  ],
  "envelopes": [
    {"attack": 60, "decay": 80, "sustain": 100, "release": 90},
    {"attack": 50, "decay": 70, "sustain": 127, "release": 80}
  ],
  "mix_osc1": 100,
  "mix_osc2": 100,
  "mix_osc3": 60,
  "amp_volume": 127,
  "category": 9
}
```

Retrieved patches are automatically saved as JSON files in the `patches/` directory.

## Requirements

- Python 3.8+
- [mido](https://mido.readthedocs.io/) and [python-rtmidi](https://spotlightkid.github.io/python-rtmidi/)
- Waldorf Blofeld connected via USB MIDI
