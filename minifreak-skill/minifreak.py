#!/usr/bin/env python3
"""
Arturia MiniFreak MIDI Library

Core library for talking to an Arturia MiniFreak over MIDI.

The MiniFreak has no publicly documented SysEx patch format, so unlike a
Blofeld we cannot dump or upload a whole preset. What we *can* do, per the
official MIDI implementation chart (MiniFreak manual v2.0, section 16.2.5):

  * set the panel parameters of the current edit buffer via Control Change
  * select one of the 512 presets via Bank Select (CC0) + Program Change
  * listen to CCs the MiniFreak sends when its knobs are turned
    (requires Utility > MIDI > Knob Send CC = On)

Parameters with no CC (oscillator Type, analog filter type, FX Type/Subtype,
mod matrix routings, LFO shapes, voicing mode...) must be set on the panel.
A patch therefore has two parts: `params` (sent as CCs) and `manual`
(instructions shown to the user).
"""

import json
import os
import re
import time
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional

import mido

# ---------------------------------------------------------------------------
# CC map — straight from the official MiniFreak MIDI implementation chart.
# ---------------------------------------------------------------------------

# name -> (cc, description)
CC_MAP: Dict[str, tuple] = {
    # General
    'mod_wheel':          (1,   'Mod wheel'),
    'glide':              (5,   'Glide amount'),
    'volume':             (7,   'Master volume (firmware 2.0+)'),
    'sustain':            (64,  'Sustain pedal (>=64 on)'),
    # Oscillator 1
    'osc1_tune':          (70,  'Osc 1 tune (64 = centre)'),
    'osc1_wave':          (14,  'Osc 1 Wave knob (meaning depends on Type)'),
    'osc1_timbre':        (15,  'Osc 1 Timbre knob (meaning depends on Type)'),
    'osc1_shape':         (16,  'Osc 1 Shape knob (meaning depends on Type)'),
    'osc1_volume':        (17,  'Osc 1 level'),
    # Oscillator 2
    'osc2_tune':          (73,  'Osc 2 tune (64 = centre)'),
    'osc2_wave':          (18,  'Osc 2 Wave knob (meaning depends on Type)'),
    'osc2_timbre':        (19,  'Osc 2 Timbre knob (meaning depends on Type)'),
    'osc2_shape':         (20,  'Osc 2 Shape knob (meaning depends on Type)'),
    'osc2_volume':        (21,  'Osc 2 level (dry/wet when Osc 2 is a processor)'),
    # Analog filter
    'filter_cutoff':      (74,  'Analog filter cutoff (~20 Hz .. 20 kHz)'),
    'filter_resonance':   (71,  'Analog filter resonance'),
    'filter_env_amt':     (24,  'Filter envelope amount (bipolar, 64 = 0)'),
    'velocity_env_mod':   (94,  'Velocity -> envelope modulation'),
    # Cycling envelope
    'cycenv_rise':        (76,  'Cycling env rise time'),
    'cycenv_fall':        (77,  'Cycling env fall time'),
    'cycenv_hold':        (78,  'Cycling env hold / sustain'),
    'cycenv_rise_shape':  (68,  'Cycling env rise curve'),
    'cycenv_fall_shape':  (69,  'Cycling env fall curve'),
    # Envelope (amp + filter)
    'env_attack':         (80,  'Envelope attack'),
    'env_decay':          (81,  'Envelope decay'),
    'env_sustain':        (82,  'Envelope sustain'),
    'env_release':        (83,  'Envelope release'),
    # LFOs
    'lfo1_rate':          (85,  'LFO 1 rate'),
    'lfo2_rate':          (87,  'LFO 2 rate'),
    # FX (knob meaning depends on FX Type)
    'fx1_time':           (22,  'FX 1 Time knob'),
    'fx1_intensity':      (23,  'FX 1 Intensity knob'),
    'fx1_amount':         (25,  'FX 1 Amount (dry/wet or send)'),
    'fx2_time':           (26,  'FX 2 Time knob'),
    'fx2_intensity':      (27,  'FX 2 Intensity knob'),
    'fx2_amount':         (28,  'FX 2 Amount (dry/wet or send)'),
    'fx3_time':           (29,  'FX 3 Time knob'),
    'fx3_intensity':      (30,  'FX 3 Intensity knob'),
    'fx3_amount':         (31,  'FX 3 Amount (dry/wet or send)'),
    # Sequencer / arp
    'seq_gate':           (115, 'Arp/Seq gate length'),
    'seq_spice':          (116, 'Arp/Seq spice amount'),
    # Macros
    'macro1':             (117, 'Macro M1'),
    'macro2':             (118, 'Macro M2'),
}

CC_TO_NAME: Dict[int, str] = {cc: name for name, (cc, _) in CC_MAP.items()}

# Order in which a patch's CCs are sent. Levels go last-ish and volume very
# last so a half-applied patch doesn't blast at full level.
SEND_ORDER: List[str] = [
    'osc1_tune', 'osc1_wave', 'osc1_timbre', 'osc1_shape',
    'osc2_tune', 'osc2_wave', 'osc2_timbre', 'osc2_shape',
    'filter_cutoff', 'filter_resonance', 'filter_env_amt', 'velocity_env_mod',
    'env_attack', 'env_decay', 'env_sustain', 'env_release',
    'cycenv_rise', 'cycenv_fall', 'cycenv_hold', 'cycenv_rise_shape', 'cycenv_fall_shape',
    'lfo1_rate', 'lfo2_rate',
    'fx1_time', 'fx1_intensity', 'fx1_amount',
    'fx2_time', 'fx2_intensity', 'fx2_amount',
    'fx3_time', 'fx3_intensity', 'fx3_amount',
    'glide', 'seq_gate', 'seq_spice', 'macro1', 'macro2', 'mod_wheel',
    'osc1_volume', 'osc2_volume', 'volume',
]
assert set(SEND_ORDER) | {'sustain'} == set(CC_MAP), 'SEND_ORDER out of sync with CC_MAP'

NUM_PRESETS = 512
PRESETS_PER_BANK = 128

PATCHES_DIR = Path(__file__).resolve().parent / 'patches'


# ---------------------------------------------------------------------------
# Patch model
# ---------------------------------------------------------------------------

def validate_patch(patch: dict) -> dict:
    """Validate and normalise a patch dict. Raises ValueError on problems."""
    if not isinstance(patch, dict):
        raise ValueError('Patch must be a JSON object')

    params = patch.get('params', {})
    if not isinstance(params, dict):
        raise ValueError('"params" must be an object of name -> 0..127')

    clean = {}
    for name, value in params.items():
        if name not in CC_MAP:
            raise ValueError(f'Unknown parameter "{name}". Known: {", ".join(sorted(CC_MAP))}')
        if not isinstance(value, int) or isinstance(value, bool) or not 0 <= value <= 127:
            raise ValueError(f'Parameter "{name}" must be an integer 0..127, got {value!r}')
        clean[name] = value

    base = patch.get('base_preset')
    if base is not None and (not isinstance(base, int) or not 1 <= base <= NUM_PRESETS):
        raise ValueError(f'"base_preset" must be 1..{NUM_PRESETS}, got {base!r}')

    manual = patch.get('manual', {})
    if not isinstance(manual, dict):
        raise ValueError('"manual" must be an object of setting -> description')

    out = dict(patch)
    out['params'] = clean
    out['manual'] = manual
    out.setdefault('name', 'Untitled')
    return out


def manual_steps(patch: dict) -> List[str]:
    """Human-readable list of settings that must be done on the panel."""
    return [f'{k}: {v}' for k, v in patch.get('manual', {}).items()]


def normalize_patch_name(name: str) -> str:
    s = re.sub(r'[^a-z0-9]+', '_', name.strip().lower()).strip('_')
    return s or 'patch'


def save_patch(patch: dict) -> Path:
    """Save a patch JSON under patches/ with a timestamped filename."""
    PATCHES_DIR.mkdir(exist_ok=True)
    stamp = datetime.now().strftime('%Y%m%d_%H%M%S')
    path = PATCHES_DIR / f'{stamp}_{normalize_patch_name(patch.get("name", ""))}.json'
    path.write_text(json.dumps(patch, indent=2) + '\n')
    return path


def preset_to_bank_program(preset: int) -> tuple:
    """Preset 1..512 -> (bank_select 0..3, program 0..127)."""
    if not 1 <= preset <= NUM_PRESETS:
        raise ValueError(f'Preset must be 1..{NUM_PRESETS}')
    idx = preset - 1
    return idx // PRESETS_PER_BANK, idx % PRESETS_PER_BANK


# ---------------------------------------------------------------------------
# MIDI
# ---------------------------------------------------------------------------

def _pick_port(names: List[str], override: Optional[str]) -> Optional[str]:
    if override:
        for n in names:
            if override.lower() in n.lower():
                return n
        return None
    matches = [n for n in names if 'minifreak' in n.lower()]
    # Prefer the plain MIDI port over any DAW/editor-specific one.
    matches.sort(key=lambda n: ('daw' in n.lower(), ' v' in n.lower(), n))
    return matches[0] if matches else None


def find_minifreak_ports(override: Optional[str] = None):
    """Return (input_name, output_name). `override` is a substring match."""
    override = override or os.environ.get('MINIFREAK_PORT')
    return (_pick_port(mido.get_input_names(), override),
            _pick_port(mido.get_output_names(), override))


class MiniFreak:
    """MIDI connection to a MiniFreak.

    channel is 1-based (1..16), matching Utility > MIDI > Input Channel.
    With dry_run=True nothing is opened and messages are printed instead.
    """

    def __init__(self, channel: Optional[int] = None, port: Optional[str] = None,
                 dry_run: bool = False, need_input: bool = False,
                 need_output: bool = True):
        if channel is None:
            channel = int(os.environ.get('MINIFREAK_CHANNEL', '1'))
        if not 1 <= channel <= 16:
            raise ValueError('MIDI channel must be 1..16')
        self.channel = channel - 1
        self.port = port
        self.dry_run = dry_run
        self.need_input = need_input
        self.need_output = need_output
        self._in: Optional[mido.ports.BaseInput] = None
        self._out: Optional[mido.ports.BaseOutput] = None

    # -- lifecycle --------------------------------------------------------

    def open(self) -> None:
        if self.dry_run:
            return
        in_name, out_name = find_minifreak_ports(self.port)
        if self.need_output:
            if not out_name:
                raise RuntimeError(
                    'Could not find a MiniFreak MIDI output port. '
                    'Is it connected via USB and powered on? '
                    f'Available: {mido.get_output_names()}')
            self._out = mido.open_output(out_name)
        if self.need_input:
            if not in_name:
                raise RuntimeError(f'Could not find a MiniFreak MIDI input port. '
                                   f'Available: {mido.get_input_names()}')
            self._in = mido.open_input(in_name)

    def close(self) -> None:
        for p in (self._in, self._out):
            if p:
                p.close()
        self._in = self._out = None

    def __enter__(self):
        self.open()
        return self

    def __exit__(self, *exc):
        self.close()

    # -- sending ----------------------------------------------------------

    def send(self, msg: mido.Message) -> None:
        if self.dry_run:
            print(f'[dry-run] {msg}')
            return
        if not self._out:
            raise RuntimeError('Not connected')
        self._out.send(msg)

    def cc(self, control: int, value: int) -> None:
        self.send(mido.Message('control_change', channel=self.channel,
                               control=control, value=value))

    def set_param(self, name: str, value: int) -> None:
        self.cc(CC_MAP[name][0], value)

    def select_preset(self, preset: int, settle: float = 0.3) -> None:
        """Load preset 1..512 via Bank Select MSB + Program Change.

        Needs Utility > MIDI > Program Change = On on the MiniFreak.
        """
        bank, program = preset_to_bank_program(preset)
        self.cc(0, bank)
        self.send(mido.Message('program_change', channel=self.channel, program=program))
        if not self.dry_run:
            time.sleep(settle)

    def apply_patch(self, patch: dict, interval: float = 0.005) -> int:
        """Send a validated patch. Returns number of CCs sent."""
        if patch.get('base_preset'):
            self.select_preset(patch['base_preset'])
        params = patch['params']
        sent = 0
        for name in SEND_ORDER:
            if name in params:
                self.set_param(name, params[name])
                sent += 1
                if interval and not self.dry_run:
                    time.sleep(interval)
        return sent

    def note_on(self, note: int, velocity: int = 100) -> None:
        self.send(mido.Message('note_on', channel=self.channel, note=note, velocity=velocity))

    def note_off(self, note: int) -> None:
        self.send(mido.Message('note_off', channel=self.channel, note=note, velocity=0))

    def all_notes_off(self) -> None:
        self.cc(123, 0)

    # -- receiving --------------------------------------------------------

    def iter_incoming(self, timeout: float):
        """Yield incoming messages until `timeout` seconds of total time pass."""
        if not self._in:
            raise RuntimeError('Input port not open (need_input=True)')
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            msg = self._in.poll()
            if msg is None:
                time.sleep(0.002)
                continue
            yield msg
