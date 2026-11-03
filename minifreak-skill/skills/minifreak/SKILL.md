---
name: minifreak
description: Control an Arturia MiniFreak synthesizer over USB MIDI — design sounds by sending CC parameter changes to the edit buffer, select presets, capture knob positions, and play test notes. Use when the user wants to create, tweak or audition sounds on their MiniFreak.
---

# MiniFreak Synthesizer Control

Control an Arturia MiniFreak over MIDI. Design sounds from a text description,
push them to the synth as CCs, audition them, and capture knob tweaks back
into JSON.

## Important: what MIDI can and cannot do on the MiniFreak

Arturia publishes **no SysEx patch format** for the MiniFreak, so there is no
"dump preset" / "write preset" like on a Blofeld. What works over MIDI:

| Can do over MIDI | Must be done on the panel |
|---|---|
| All front-panel knobs in the CC table below (osc Wave/Timbre/Shape/Volume/Tune, cutoff, resonance, env, cycling env, LFO rates, FX knobs, macros, glide) | **Osc 1 / Osc 2 Type** (engine) |
| Load any of the 512 presets (Bank Select + Program Change) | **Analog filter type** (LP / BP / HP) |
| Record knobs the user turns (Knob Send CC) | **FX Type and Subtype**, FX on/off, Delay/Reverb routing |
| Notes, mod wheel, sustain | Mod matrix routings, LFO shape/sync, voicing mode (Poly/Para/Mono/Uni), arp/seq patterns, saving the preset |

So a patch = `params` (sent as CCs) + `manual` (panel steps you tell the user
to do). Work this way:

1. Choose a **base preset** whose engine types / FX already fit, or tell the
   user what to set in `manual` (engine types, filter type, FX types).
2. Send `params` as CCs. Always print the `manual` steps to the user.
3. Audition with `play_notes.py`, iterate on `params`.
4. Remind the user that the sound is only in the edit buffer until they **Save**
   on the MiniFreak.

All CC values are raw 0..127. Knob meaning for Wave/Timbre/Shape and the FX
knobs depends on the selected Type (see reference below). Bipolar knobs
(`filter_env_amt`, tunes) have 0 at 64.

## Setup

```bash
uv sync
uv run python3 list_ports.py   # check the MiniFreak is visible
```

On the MiniFreak (Shift + Sound Edit → Utility → MIDI):
- **Input Channel**: 1 (or set `--channel` / `$MINIFREAK_CHANNEL` to match; `All` works too)
- **Program Change**: On (needed for `select_preset.py` / `base_preset`)
- **Knob Send CC**: On (needed for `capture_patch.py`)
- **MIDI From**: USB or USB+MIDI

Port auto-detect matches "MiniFreak"; override with `--port <substring>` or
`$MINIFREAK_PORT`. Incoming CCs are ignored while MiniFreak V is connected.

## Tools

### Send Patch
Apply a patch to the edit buffer (JSON via stdin, `--patch-json` or `--file`).
A copy is saved to `patches/`.
```bash
uv run python3 send_patch.py --patch-json '{"name": "Warm Pad", "params": {...}, "manual": {...}}'
uv run python3 send_patch.py --file patches/xyz.json --base-preset 12
uv run python3 send_patch.py --file p.json --dry-run    # print MIDI only
```

### Select Preset
```bash
uv run python3 select_preset.py --preset 242     # 1..512
```

### Capture Patch
Closest thing to "get patch": listens while the user turns knobs and records
the last value of each. Untouched knobs are not captured.
```bash
uv run python3 capture_patch.py --seconds 30 --name "My Bass"
uv run python3 capture_patch.py --seconds 20 --merge patches/xyz.json
```

### Play Notes
```bash
uv run python3 play_notes.py --test              # C4, E4, G4
uv run python3 play_notes.py --minor7            # Cm7 chord (10 sec)
uv run python3 play_notes.py --industrial        # Dissonant cluster F3, F#3, B3
uv run python3 play_notes.py --notes "C2 C2 Eb2 G2 R Bb1"   # melody, R = rest
```

## Patch JSON

```json
{
  "name": "Warm Pad",
  "base_preset": null,
  "manual": {
    "osc1_type": "SuperWave",
    "osc2_type": "BasicWaves, Osc 2 Tune -12 st",
    "filter_type": "LP",
    "fx1": "Chorus / Lush",
    "fx2": "Reverb / Hall",
    "fx3": "off",
    "voicing": "Poly"
  },
  "params": {
    "osc1_wave": 0, "osc1_timbre": 45, "osc1_shape": 80, "osc1_volume": 100,
    "osc2_wave": 70, "osc2_timbre": 64, "osc2_shape": 40, "osc2_volume": 70,
    "filter_cutoff": 62, "filter_resonance": 20, "filter_env_amt": 78,
    "env_attack": 70, "env_decay": 80, "env_sustain": 100, "env_release": 85,
    "lfo1_rate": 30,
    "fx1_time": 30, "fx1_intensity": 60, "fx1_amount": 50,
    "fx2_time": 90, "fx2_intensity": 50, "fx2_amount": 45
  }
}
```

`params` keys are exactly the names in the CC table. Only include what you
want to change; everything else keeps its current value (from the base
preset, if any).

---

## MiniFreak MIDI reference (manual v2.0, §16.2)

### Control Change chart

| Param name | CC | Section / notes |
|---|---|---|
| `mod_wheel` | 1 | Mod wheel |
| `glide` | 5 | Glide |
| `volume` | 7 | Master volume (FW 2.0+) |
| `sustain` | 64 | Sustain pedal |
| `osc1_tune` | 70 | Osc 1 Tune (±48 st, 64 = 0) |
| `osc1_wave` | 14 | Osc 1 Wave |
| `osc1_timbre` | 15 | Osc 1 Timbre |
| `osc1_shape` | 16 | Osc 1 Shape |
| `osc1_volume` | 17 | Osc 1 Volume |
| `osc2_tune` | 73 | Osc 2 Tune |
| `osc2_wave` | 18 | Osc 2 Wave |
| `osc2_timbre` | 19 | Osc 2 Timbre |
| `osc2_shape` | 20 | Osc 2 Shape |
| `osc2_volume` | 21 | Osc 2 Volume (dry/wet when Osc 2 is a processor) |
| `filter_cutoff` | 74 | Analog filter cutoff, ~20 Hz–20 kHz, 12 dB/oct |
| `filter_resonance` | 71 | Resonance (self-oscillates at top) |
| `filter_env_amt` | 24 | VCF Env Amt, bipolar (64 = 0) |
| `velocity_env_mod` | 94 | Velocity → env mod |
| `cycenv_rise_shape` | 68 | Cycling env rise curve |
| `cycenv_rise` | 76 | Cycling env rise |
| `cycenv_fall` | 77 | Cycling env fall |
| `cycenv_hold` | 78 | Cycling env hold |
| `cycenv_fall_shape` | 69 | Cycling env fall curve |
| `env_attack` | 80 | Envelope attack |
| `env_decay` | 81 | Envelope decay |
| `env_sustain` | 82 | Envelope sustain |
| `env_release` | 83 | Envelope release |
| `lfo1_rate` | 85 | LFO 1 rate |
| `lfo2_rate` | 87 | LFO 2 rate |
| `fx1_time` / `fx1_intensity` / `fx1_amount` | 22 / 23 / 25 | FX 1 knobs |
| `fx2_time` / `fx2_intensity` / `fx2_amount` | 26 / 27 / 28 | FX 2 knobs |
| `fx3_time` / `fx3_intensity` / `fx3_amount` | 29 / 30 / 31 | FX 3 knobs |
| `seq_gate` | 115 | Arp/Seq gate |
| `seq_spice` | 116 | Arp/Seq spice |
| `macro1` / `macro2` | 117 / 118 | Macros M1 / M2 |

Presets: 512 = 4 banks × 128. Preset N → Bank Select (CC0) = (N-1) // 128,
Program Change = (N-1) % 128. Six voices; the single Envelope drives both VCA and
the filter env amount.

### Oscillator Types — what Wave / Timbre / Shape do

Panel values are 0–100; CC 0–127 maps across the same range (CC 64 ≈ 50).

| Type | Wave (CC14/18) | Timbre (CC15/19) | Shape (CC16/20) | Character |
|---|---|---|---|---|
| BasicWaves | Morph: square (0) → saw (50) → double saw (100) | Sym: pulse width / saw phasing | Sub: sine one octave down | Classic analog |
| SuperWave | Wave: saw / square / tri / sine | Detune | Volume of copies | Supersaw, trance, pads |
| Harmo | Content (harmonic table) | Sculpting (sine → tri) | Chorus | Additive, organ-ish |
| KarplusStr | Bow | Position | Decay | Plucks, bowed strings, drums |
| VAnalog | Detune (saw vs pulse, up to ±2 oct) | Shape (pulse width → sync) | Wave (tri → saw) | Virtual analog |
| Waveshaper | Wave | Amount (wavefold) | Asym | West-coast, folded |
| Two Op. FM | Ratio | Amount (index) | Feedback | Bells, e-pianos, metallic |
| Formant | Interval | Formant | Shape | Vocal, granular |
| Chords (Osc 2 only) | Interval (oct, 5th, sus4, m, m7, m9, m11, 69, M9, M7, M) | Inv/Trsp | Waveform (>50 = wavetable) | One-finger chords |
| Speech | Type (library) | Timbre (formant shift) | Word | Speak & Spell |
| Modal | Inharm (woody → metallic) | Timbre (exciter brightness) | Decay | Mallets, bells, percussion |
| Noise | Type (particle → white → metallic) | Rate | Balance (noise + sine/tri/square) | Noise, wind, hats |
| Bass | Saturate | Fold | Noise | Gritty basses |
| SawX | SawMod | Shape (chorus) | Noise | Rich, evolving saws |
| Harm | Spread | Rectify | Noise | Harmonic/ring-mod-ish |
| Audio In (Osc 1 only) | Fold | Decimate | Noise | Process external audio |
| Wavetable (Osc 1 only) | Wavetable position | Pulse width | LPF/HPF 1-pole | Wavetable scans |
| FM/RM (Osc 2 only) | Osc 2 waveform | FreqMod (Osc1 → Osc2) | RingMod | FM / ring mod from Osc 1 |

**Osc 2 processor Types** (process Osc 1; Osc 1 Volume = level into Osc 2, Osc 2 Volume = dry/wet):

| Type | Wave | Timbre | Shape |
|---|---|---|---|
| Multi Filter | Cutoff | Resonance | Mode (LP/BP/HP/Notch, 12/24/36 dB; LP6/HP6) |
| Surgeon Filter | Cutoff | Spread (BP/Notch only) | Mode (LP/BP/HP/Notch) |
| Comb Filter | Cutoff (delay time, key-tracked) | Gain | Damping |
| Phaser Filter | Cutoff | Feedback | Poles (2–12) |
| Destroy | Fold | Decimate | Bitcrush |

### Analog filter

SEM-style state-variable, 12 dB/oct, types LP / BP / HP (panel Type button, no
CC). Resonance self-oscillates; key-track via mod matrix (Keyboard → Cutoff ≈ 50).

### FX Types — what Time / Intensity / Amount do

Three slots in series FX1 → FX2 → FX3. Only one each of Reverb, Delay, Multi
Comp. Delay and Reverb can be Insert or Send (Sound Edit → FX).

| Type | Time (22/26/29) | Intensity (23/27/30) | Amount (25/28/31) | Subtypes |
|---|---|---|---|---|
| Chorus | Rate | Depth | Dry/Wet | Default, Lush, Dark, Shaded, Single |
| Phaser | Rate | Feedback | Dry/Wet | Default, Space, SnH (+ Sync variants) |
| Flanger | Rate | Feedback | Dry/Wet | Default, Silly (+ Sync variants) |
| Super Unison | Detune | LPF/HPF | Dry/Wet | Classic, Ravey, Soli, Slow, Slow Trig, Wide Trig, Mono Trig, Wavy |
| Reverb | Decay | Damping | Dry/Wet or Send | Default, Long, Hall, Echoes, Room, Dark Room |
| Delay | Time | Feedback | Dry/Wet or Send | Digital, Stereo, Ping-Pong, Mono, Filtered, Filtered Ping-Pong (+ Sync) |
| Distortion | Gain | HPF/LPF (<64 LP, >64 HP) | Dry/Wet | Classic, Soft Clip, Germanium, Dual Fold, Climb, Tape |
| Bit Crusher | Decimate | BitDepth | Dry/Wet | — |
| 3 Bands EQ | Low | Mid | High (−15…+6 dB) | Default, Wide, Mid 1K |
| Peak EQ | Frequency (30 Hz–15 kHz) | Gain | Width | — |
| Multi Comp | Time | Tone | Amount | OPP, Bass Ctrl, High Ctrl, Tighter |

### Sound design quick guide

- **Pad**: SuperWave or Harmo, slow attack (`env_attack` 60–80), high sustain, long release, cutoff mid, Chorus + Reverb Hall.
- **Pluck**: KarplusStr or BasicWaves, `env_attack` 0, `env_decay` 40–60, `env_sustain` 0–20, `filter_env_amt` > 80.
- **Bass**: Bass / SawX / BasicWaves with Sub (`osc1_shape` high), Osc 2 −12 st, mono voicing, cutoff 40–60, a touch of Distortion.
- **Bell / metallic**: Two Op. FM (non-integer Ratio) or Modal with Inharm high, long decay, Reverb.
- **Industrial / dark**: Waveshaper or Bass with fold, Osc 2 = Destroy, BP filter, Distortion Dual Fold + Bit Crusher, cycling env in loop on cutoff.
- The **Cycling Envelope** is a third modulation source (envelope or looping LFO); its routing is set in the mod matrix on the panel, but its times/shapes are CCs.
