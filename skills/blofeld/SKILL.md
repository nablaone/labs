# Blofeld Synthesizer Control

Control a Waldorf Blofeld synthesizer via MIDI SysEx. Read and write patches, play test notes, and explore synthesizer programming with AI assistance.

## Setup

```bash
uv sync
```

Ensure your Blofeld is connected via USB MIDI and powered on.

## Tools

### Get Patch
Retrieve a patch from the synthesizer:
```bash
uv run python3 get_patch.py --bank A --program 1
```

### Send Patch
Send a patch to the synthesizer (JSON via stdin or --patch-json):
```bash
uv run python3 send_patch.py --bank A --program 1 --patch-json '{"name": "MyPatch", ...}'
```

### Play Notes
Test the current sound:
```bash
uv run python3 play_notes.py --test              # C4, E4, G4
uv run python3 play_notes.py --minor7            # Cm7 chord (10 sec)
uv run python3 play_notes.py --industrial        # Dissonant cluster F3, F#3, B3 (industrial/dark pads)
uv run python3 play_notes.py --notes "C4 E4 G4"  # Custom melody
```

## Patch JSON Structure

A Blofeld patch contains these sections:

- **oscillators** (3): shape, octave, pitch, detune, FM, PWM, brilliance
- **filters** (2): type, cutoff, resonance, drive, envelope amount, keytrack
- **envelopes** (3): ADSR with attack level, decay2, sustain2
- **lfos** (3): shape, speed, sync, delay, fade
- **effects** (2): type, mix, parameters
- **mod_matrix** (16 slots): source, destination, amount
- **modifiers** (4): source A/B, operator, constant
- **arpeggiator**: mode, pattern, clock, range, direction
- **mixer**: osc levels, noise, ring mod, balances
- **amp**: volume, velocity, pan

## Example: Creating a Pad Sound

```json
{
  "name": "Warm Pad",
  "oscillators": [
    {"shape": 2, "octave": 64, "pitch": 64, "detune": 60, "pw": 64},
    {"shape": 2, "octave": 64, "pitch": 64, "detune": 68, "pw": 64},
    {"shape": 4, "octave": 52, "pitch": 64, "detune": 64, "pw": 64}
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

---

## Blofeld SysEx Specification (v1.04)

### 1. General

SysEx Dumps have the following form:

    F0h IDW IDE DEV IDM BB NN --------Data--------- CHK F7h

where:
- IDW: Waldorf Music MIDI ID = 3Eh
- IDE: Equipment ID = 13h for Blofeld
- DEV: Device number, 00h to 7Eh, 7Fh = broadcast
- IDM: Message ID
- BB: Location High Byte (Bank)
- NN: Location Low Byte (Program)
- CHK: Sum of all data bytes truncated to 7 bits

### Message IDs

| Label | Value | Description |
|-------|-------|-------------|
| SNDR  | 00h   | Sound Request |
| SNDD  | 10h   | Sound Dump |
| SNDP  | 20h   | Sound Parameter Change |
| GLBR  | 04h   | Global Request |
| GLBD  | 14h   | Global Dump |

### Banks

| BB | Location |
|----|----------|
| 00 | Bank A (00-7F = programs 1-128) |
| 01 | Bank B |
| ... | ... |
| 07 | Bank H |
| 7F | Edit Buffer |

### 3.1 SDATA - Sound Data (383 bytes)

| Index | Range | Parameter |
|-------|-------|-----------|
| 1 | 16-112 | Osc 1 Octave (128'..1/2' in steps of 12) |
| 2 | 52-76 | Osc 1 Semitone (-12..+12) |
| 3 | 0-127 | Osc 1 Detune (-64..+63) |
| 4 | 40-88 | Osc 1 Bend Range (-24..+24) |
| 5 | 0-127 | Osc 1 Keytrack (-200%..+196%) |
| 6 | 0-11 | Osc 1 FM Source |
| 7 | 0-127 | Osc 1 FM Amount |
| 8 | 0-72 | Osc 1 Shape |
| 9 | 0-127 | Osc 1 Pulsewidth |
| 10 | 0-30 | Osc 1 PWM Source |
| 11 | 0-127 | Osc 1 PWM Amount (-64..+63) |
| 14 | 0-1 | Osc 1 Limit WT |
| 16 | 0-127 | Osc 1 Brilliance |
| 17-32 | | Osc 2 (same layout) |
| 33-48 | | Osc 3 (same layout, shape limited 0-4) |
| 49 | 0-1 | Osc 2 Sync to O3 |
| 50 | 0-30 | Osc Pitch Source |
| 51 | 0-127 | Osc Pitch Amount |
| 56 | 0-3 | Glide Mode |
| 57 | 0-127 | Glide Rate |
| 58 | 0-127 | Allocation/Unison Mode |
| 59 | 0-127 | Unisono Detune |
| 61-72 | | Mixer (Osc1/2/3 levels, balances, noise, ring) |
| 77-95 | | Filter 1 |
| 97-115 | | Filter 2 |
| 117 | 0-1 | Filter Routing (parallel/serial) |
| 121-124 | | Amplifier |
| 128-143 | | Effect 1 |
| 144-159 | | Effect 2 |
| 160-170 | | LFO 1 |
| 172-182 | | LFO 2 |
| 184-194 | | LFO 3 |
| 196-205 | | Filter Envelope |
| 208-217 | | Amplifier Envelope |
| 220-229 | | Envelope 3 |
| 232-241 | | Envelope 4 |
| 245-260 | | Modifiers 1-4 |
| 261-308 | | Modulation Matrix (16 slots x 3 bytes) |
| 311-358 | | Arpeggiator |
| 363-378 | | Name (16 ASCII chars) |
| 379 | 0-12 | Category |

### 4.1 Oscillator Shapes

| Index | Description |
|-------|-------------|
| 0 | off |
| 1 | Pulse |
| 2 | Saw |
| 3 | Triangle |
| 4 | Sine |
| 5-72 | Alt waveforms, sync, FM, wavetables |

### 4.4 Filter Types

| Index | Description |
|-------|-------------|
| 0 | Bypass |
| 1 | LP 24dB |
| 2 | LP 12dB |
| 3 | BP 24dB |
| 4 | BP 12dB |
| 5 | HP 24dB |
| 6 | HP 12dB |
| 7 | Notch 24dB |
| 8 | Notch 12dB |
| 9 | Comb+ |
| 10 | Comb- |
| 11 | PPG LP |

### 4.5 LFO Shapes

| Index | Description |
|-------|-------------|
| 0 | Sine |
| 1 | Triangle |
| 2 | Square |
| 3 | Saw |
| 4 | Random |
| 5 | S&H |

### 4.7 Modulation Sources

| Index | Description |
|-------|-------------|
| 0 | off |
| 1 | LFO 1 |
| 2 | LFO1*MW |
| 3 | LFO 2 |
| 4 | LFO2*Press |
| 5 | LFO 3 |
| 6 | FilterEnv |
| 7 | AmpEnv |
| 8 | Env3 |
| 9 | Env4 |
| 10 | Keytrack |
| 11 | Velocity |
| 12 | Rel. Velo |
| 13 | Pressure |
| 14 | Poly Press |
| 15 | Pitch Bend |
| 16 | Mod Wheel |
| 17-23 | Controllers |
| 24 | Unisono V. |
| 25-28 | Modifiers 1-4 |
| 29 | minimum |
| 30 | MAXIMUM |

### 4.8 Modulation Destinations

| Index | Description |
|-------|-------------|
| 0 | Pitch |
| 1-9 | Osc 1-3 parameters |
| 10-19 | Mixer levels/balances |
| 20-29 | Filter parameters |
| 30 | Volume |
| 31-33 | LFO speeds |
| 34-49 | Envelope parameters |
| 50-53 | Mod amounts |

### 4.16 Categories

| Index | Description |
|-------|-------------|
| 0 | Init |
| 1 | Arp |
| 2 | Atmo |
| 3 | Bass |
| 4 | Drum |
| 5 | FX |
| 6 | Keys |
| 7 | Lead |
| 8 | Mono |
| 9 | Pad |
| 10 | Perc |
| 11 | Poly |
| 12 | Seq |

### 5. Effects

Effect 1 types: Bypass, Chorus, Flanger, Phaser, Overdrive, Triple FX
Effect 2 types: All above + Delay, Clk.Delay, Reverb

#### Chorus/Flanger Parameters
- Speed (0-127)
- Depth (0-127)
- Feedback (flanger only)
- Polarity (positive/negative)

#### Phaser Parameters
- Speed, Depth, Feedback, Center, Spacing, Polarity

#### Overdrive Parameters
- Drive, Post Gain, Cutoff, Curve

#### Delay Parameters
- Length, Feedback, Cutoff, Polarity, Spread

#### Reverb Parameters
- Size, Shape, Decay, Lowpass, Highpass, Diffusion, Damping
