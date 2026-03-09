# Blofeld Neural

A neural network patch generator for the Waldorf Blofeld synthesizer. Describe a sound using a set of perceptual sliders and the network generates a complete Blofeld patch — instantly, in the browser.

[Demo](https://www.youtube.com/shorts/oeYzk1UsxQk)


## What it does

Instead of tweaking hundreds of parameters, you work with human concepts: *brightness*, *texture*, *movement*, *weight*, *harshness* and eleven more. The neural network, trained on over a thousand real Blofeld patches, translates these into a full patch and sends it to your synthesizer via MIDI SysEx.

The output is intentionally imprecise — run it multiple times with the same settings and you get variations. Think of it as a smart randomizer with intention rather than a precise recreator. Use it to find a starting point, then tweak from there.

## Web App

**Live app:** TBA

The app runs entirely in the browser. No installation, no plugins. You need:

- A Waldorf Blofeld connected via USB MIDI
- A browser with [Web MIDI API](https://caniuse.com/midi) support (Chrome / Edge recommended)

### Usage

1. Open the app in Chrome or Edge
2. Click **CONNECT** — the browser will ask for MIDI permission, grant it
3. Select the **bank** (A–H) and **patch number** (1–128) where you want to save
4. Adjust the 16 knobs to describe the sound you have in mind:
   - Drag up to increase, drag down to decrease
   - Mouse wheel also works
5. Click **SEND** — the patch is written to your Blofeld immediately

### Auditioning sounds

Use the buttons at the bottom to hear the current patch:

| Section | Buttons |
|---------|---------|
| **Rhythm patterns** | 4×4 (four-on-the-floor), Clave, Gallop — click again to stop |
| **Chords (10s)** | Cmaj, Cmin, Cmin7, F·F#·B techno cluster |

> The MIDI channel defaults to 1. Change it in the **CH** field to match your Blofeld's receive channel if needed.

### The 16 dimensions

| Knob | Low → High |
|------|------------|
| Brightness | Dark → Bright |
| Texture | Woody → Metallic |
| Envelope | Pad → Plucky |
| Movement | Static → Evolving |
| Harshness | Mellow → Harsh |
| Weight | Thin → Heavy |
| Density | Sparse → Dense |
| Space | Dry → Spacious |
| Noise | Tonal → Noisy |
| Warmth | Cold → Warm |
| Attack | Soft → Snappy |
| Modulation | Stable → Vibrato |
| Distortion | Clean → Distorted |
| Presence | Recessed → Forward |
| Rhythm | Smooth → Rhythmic |
| Character | Pure → Complex |

## The idea

The Blofeld has 383 patch parameters. Programming it from scratch is powerful but slow. This project compresses that space down to 16 human-meaningful dimensions and trains a small neural network to navigate it — grounded in the character of real patches rather than random values.

The training data comes from the Blofeld factory banks and community patches. Each patch was described along the 16 dimensions by an AI, creating pairs of (intention → parameters) that the network learned from. The result lives in the browser as a 600KB ONNX model that runs inference in milliseconds.

## Requirements

- Waldorf Blofeld (desktop or keys)
- USB MIDI connection
- Chrome or Edge (Web MIDI + WebAssembly)

## License

MIT
