#!/usr/bin/env python3
"""
Blofeld Synthesizer MIDI/SysEx Library

Core library for communicating with Waldorf Blofeld synthesizer via MIDI SysEx.
"""

import json
import os
import re
import time
from dataclasses import dataclass, field, asdict
from datetime import datetime
from pathlib import Path
from typing import Optional, List, Tuple

import mido

PATCH_SIZE = 383  # Expected SDATA payload size for a Blofeld patch
SYSEX_TIMEOUT = 5.0  # Seconds to wait for SysEx response

# Waldorf Blofeld SysEx constants
WALDORF_ID = 0x3E
BLOFELD_ID = 0x13
MSG_SNDR = 0x00  # Sound Request
MSG_SNDD = 0x10  # Sound Dump


@dataclass
class Oscillator:
    octave: int = 64
    pitch: int = 64
    bend_range: int = 64
    keytrack: int = 64
    detune: int = 64
    shape: int = 0
    pw: int = 64
    pwm: int = 64
    pwm_source: int = 0
    fm: int = 0
    fm_source: int = 0
    limit_wt: int = 0
    brilliance: int = 64


@dataclass
class Filter:
    type: int = 1
    cutoff: int = 127
    res: int = 0
    drive: int = 0
    drive_curve: int = 0
    env_amt: int = 64
    env_vel: int = 64
    keytrack: int = 64
    mod_source: int = 0
    mod_amount: int = 64
    fm_source: int = 0
    fm_amount: int = 0
    pan: int = 64
    pan_source: int = 0
    pan_amount: int = 64


@dataclass
class Envelope:
    mode: int = 0
    attack: int = 0
    attack_level: int = 127
    decay: int = 64
    sustain: int = 127
    decay2: int = 64
    sustain2: int = 64
    release: int = 64


@dataclass
class LFO:
    shape: int = 0
    speed: int = 64
    sync: int = 0
    clocked: int = 0
    start_phase: int = 0
    delay: int = 0
    fade: int = 64
    keytrack: int = 64


@dataclass
class Effect:
    type: int = 0
    mix: int = 64
    param1: int = 0
    param2: int = 0
    params: List[int] = field(default_factory=lambda: [0] * 14)


@dataclass
class ModulationMatrix:
    source: int = 0
    amount: int = 64
    dest: int = 0


@dataclass
class Modifier:
    source_a: int = 0
    source_b: int = 0
    operator: int = 0
    constant: int = 64


@dataclass
class Patch:
    oscillators: List[Oscillator] = field(default_factory=lambda: [Oscillator() for _ in range(3)])
    osc2_sync: int = 0
    osc_pitch_source: int = 0
    osc_pitch_amount: int = 64

    filters: List[Filter] = field(default_factory=lambda: [Filter() for _ in range(2)])

    # Mixer
    mix_osc1: int = 127
    mix_osc1_balance: int = 64
    mix_osc2: int = 0
    mix_osc2_balance: int = 64
    mix_osc3: int = 0
    mix_osc3_balance: int = 64
    mix_noise: int = 0
    mix_noise_balance: int = 64
    mix_noise_color: int = 64
    mix_ring: int = 0
    mix_ring_balance: int = 64

    filter_routing: int = 0
    glide_mode: int = 0
    glide_rate: int = 0
    unison: int = 0
    unison_detune: int = 0

    envelopes: List[Envelope] = field(default_factory=lambda: [Envelope() for _ in range(3)])
    lfos: List[LFO] = field(default_factory=lambda: [LFO() for _ in range(3)])

    mod_matrix: List[ModulationMatrix] = field(default_factory=lambda: [ModulationMatrix() for _ in range(16)])
    modifiers: List[Modifier] = field(default_factory=lambda: [Modifier() for _ in range(4)])

    # Arpeggiator
    arp_mode: int = 0
    arp_pattern: int = 0
    arp_clock: int = 11
    arp_length: int = 11
    arp_range: int = 0
    arp_direction: int = 0
    arp_sort: int = 0
    arp_velocity_mode: int = 0
    arp_timing_factor: int = 64
    arp_pattern_reset: int = 0
    arp_pattern_length: int = 15
    arp_tempo: int = 60
    arp_pattern_steps: List[int] = field(default_factory=lambda: [0] * 16)
    arp_pattern_timing: List[int] = field(default_factory=lambda: [0] * 16)

    # Effects
    effects: List[Effect] = field(default_factory=lambda: [Effect() for _ in range(2)])

    # Amp
    amp_volume: int = 127
    amp_velocity: int = 64
    amp_mod_source: int = 0
    amp_mod_amount: int = 64
    amp_pan: int = 64
    amp_drive: int = 0

    master_tune: int = 64

    name: str = "Init"
    category: int = 0
    subcategory: int = 0


# Index mappings into the 383-byte SDATA payload (Blofeld spec 3.1)
OSC_FIELD_MAPPING = [
    {"octave": 1, "pitch": 2, "bend_range": 4, "keytrack": 5, "detune": 3, "fm_source": 6, "fm": 7,
     "shape": 8, "pw": 9, "pwm_source": 10, "pwm": 11, "limit_wt": 14, "brilliance": 16},
    {"octave": 17, "pitch": 18, "bend_range": 20, "keytrack": 21, "detune": 19, "fm_source": 22, "fm": 23,
     "shape": 24, "pw": 25, "pwm_source": 26, "pwm": 27, "limit_wt": 30, "brilliance": 32},
    {"octave": 33, "pitch": 34, "bend_range": 36, "keytrack": 37, "detune": 35, "fm_source": 38, "fm": 39,
     "shape": 40, "pw": 41, "pwm_source": 42, "pwm": 43, "limit_wt": -1, "brilliance": 48},
]

FILTER_FIELD_MAPPING = [
    {"type": 77, "cutoff": 78, "res": 80, "drive": 81, "drive_curve": 82, "env_amt": 87, "env_vel": 88,
     "keytrack": 86, "mod_source": 89, "mod_amount": 90, "fm_source": 91, "fm_amount": 92,
     "pan": 93, "pan_source": 94, "pan_amount": 95},
    {"type": 97, "cutoff": 98, "res": 100, "drive": 101, "drive_curve": 102, "env_amt": 107, "env_vel": 108,
     "keytrack": 106, "mod_source": 109, "mod_amount": 110, "fm_source": 111, "fm_amount": 112,
     "pan": 113, "pan_source": 114, "pan_amount": 115},
]

ENVELOPE_FIELD_MAPPING = [
    {"mode": 196, "attack": 199, "attack_level": 200, "decay": 201, "sustain": 202,
     "decay2": 203, "sustain2": 204, "release": 205},
    {"mode": 208, "attack": 211, "attack_level": 212, "decay": 213, "sustain": 214,
     "decay2": 215, "sustain2": 216, "release": 217},
    {"mode": 220, "attack": 223, "attack_level": 224, "decay": 225, "sustain": 226,
     "decay2": 227, "sustain2": 228, "release": 229},
]

LFO_FIELD_MAPPING = [
    {"shape": 160, "speed": 161, "sync": 163, "clocked": 164, "start_phase": 165,
     "delay": 166, "fade": 167, "keytrack": 170},
    {"shape": 172, "speed": 173, "sync": 175, "clocked": 176, "start_phase": 177,
     "delay": 178, "fade": 179, "keytrack": 182},
    {"shape": 184, "speed": 185, "sync": 187, "clocked": 188, "start_phase": 189,
     "delay": 190, "fade": 191, "keytrack": 194},
]

EFFECT_FIELD_MAPPING = [
    {"type": 128, "mix": 129, "param1": 130, "param2": 131, "params_start": 130},
    {"type": 144, "mix": 145, "param1": 146, "param2": 147, "params_start": 146},
]

# Constant indices
OSC_SYNC_IDX = 49
OSC_PITCH_SOURCE_IDX = 50
OSC_PITCH_AMOUNT_IDX = 51
GLIDE_MODE_IDX = 56
GLIDE_RATE_IDX = 57
UNISON_IDX = 58
UNISON_DETUNE_IDX = 59

MIX_OSC1_IDX = 61
MIX_OSC1_BAL_IDX = 62
MIX_OSC2_IDX = 63
MIX_OSC2_BAL_IDX = 64
MIX_OSC3_IDX = 65
MIX_OSC3_BAL_IDX = 66
MIX_NOISE_IDX = 67
MIX_NOISE_BAL_IDX = 68
MIX_NOISE_COLOR_IDX = 69
MIX_RING_IDX = 71
MIX_RING_BAL_IDX = 72

FILTER_ROUTING_IDX = 117

AMP_VOLUME_IDX = 121
AMP_VELOCITY_IDX = 122
AMP_MOD_SOURCE_IDX = 123
AMP_MOD_AMOUNT_IDX = 124

MASTER_TUNE_IDX = 52

MOD_MATRIX_START_IDX = 261
MOD_MATRIX_STRIDE = 3
MODIFIER_START_IDX = 245
MODIFIER_STRIDE = 4

ARP_MODE_IDX = 311
ARP_PATTERN_IDX = 312
ARP_CLOCK_IDX = 314
ARP_LENGTH_IDX = 315
ARP_RANGE_IDX = 316
ARP_DIRECTION_IDX = 317
ARP_SORT_IDX = 318
ARP_VELOCITY_MODE_IDX = 319
ARP_TIMING_FACTOR_IDX = 320
ARP_PATTERN_RESET_IDX = 322
ARP_PATTERN_LENGTH_IDX = 323
ARP_TEMPO_IDX = 326
ARP_PATTERN_STEPS_START_IDX = 327
ARP_PATTERN_TIMING_START_IDX = 343


def parse_sdata(data: bytes) -> Patch:
    """Parse 383-byte SDATA into a Patch object."""
    if len(data) != PATCH_SIZE:
        raise ValueError(f"Invalid SDATA length: {len(data)} (expected {PATCH_SIZE})")

    p = Patch()

    # Oscillators
    for i, m in enumerate(OSC_FIELD_MAPPING):
        if i >= len(p.oscillators):
            break
        osc = p.oscillators[i]
        if m["octave"] >= 0:
            osc.octave = data[m["octave"]]
        osc.pitch = data[m["pitch"]]
        if m["bend_range"] >= 0:
            osc.bend_range = data[m["bend_range"]]
        if m["keytrack"] >= 0:
            osc.keytrack = data[m["keytrack"]]
        osc.detune = data[m["detune"]]
        osc.fm_source = data[m["fm_source"]]
        osc.fm = data[m["fm"]]
        osc.shape = data[m["shape"]]
        osc.pw = data[m["pw"]]
        if m["pwm_source"] >= 0:
            osc.pwm_source = data[m["pwm_source"]]
        osc.pwm = data[m["pwm"]]
        if m["limit_wt"] >= 0:
            osc.limit_wt = data[m["limit_wt"]]
        if m["brilliance"] >= 0:
            osc.brilliance = data[m["brilliance"]]

    p.osc2_sync = data[OSC_SYNC_IDX]
    p.osc_pitch_source = data[OSC_PITCH_SOURCE_IDX]
    p.osc_pitch_amount = data[OSC_PITCH_AMOUNT_IDX]

    # Mixer
    p.mix_osc1 = data[MIX_OSC1_IDX]
    p.mix_osc1_balance = data[MIX_OSC1_BAL_IDX]
    p.mix_osc2 = data[MIX_OSC2_IDX]
    p.mix_osc2_balance = data[MIX_OSC2_BAL_IDX]
    p.mix_osc3 = data[MIX_OSC3_IDX]
    p.mix_osc3_balance = data[MIX_OSC3_BAL_IDX]
    p.mix_noise = data[MIX_NOISE_IDX]
    p.mix_noise_balance = data[MIX_NOISE_BAL_IDX]
    p.mix_noise_color = data[MIX_NOISE_COLOR_IDX]
    p.mix_ring = data[MIX_RING_IDX]
    p.mix_ring_balance = data[MIX_RING_BAL_IDX]

    # Filters
    for i, m in enumerate(FILTER_FIELD_MAPPING):
        if i >= len(p.filters):
            break
        f = p.filters[i]
        f.type = data[m["type"]]
        f.cutoff = data[m["cutoff"]]
        f.res = data[m["res"]]
        f.drive = data[m["drive"]]
        f.drive_curve = data[m["drive_curve"]]
        f.env_amt = data[m["env_amt"]]
        f.env_vel = data[m["env_vel"]]
        f.keytrack = data[m["keytrack"]]
        f.mod_source = data[m["mod_source"]]
        f.mod_amount = data[m["mod_amount"]]
        f.fm_source = data[m["fm_source"]]
        f.fm_amount = data[m["fm_amount"]]
        f.pan = data[m["pan"]]
        f.pan_source = data[m["pan_source"]]
        f.pan_amount = data[m["pan_amount"]]

    p.filter_routing = data[FILTER_ROUTING_IDX]
    p.glide_mode = data[GLIDE_MODE_IDX]
    p.glide_rate = data[GLIDE_RATE_IDX]
    p.unison = data[UNISON_IDX]
    p.unison_detune = data[UNISON_DETUNE_IDX]

    # Envelopes
    for i, m in enumerate(ENVELOPE_FIELD_MAPPING):
        if i >= len(p.envelopes):
            break
        env = p.envelopes[i]
        env.mode = data[m["mode"]]
        env.attack = data[m["attack"]]
        env.attack_level = data[m["attack_level"]]
        env.decay = data[m["decay"]]
        env.sustain = data[m["sustain"]]
        env.decay2 = data[m["decay2"]]
        env.sustain2 = data[m["sustain2"]]
        env.release = data[m["release"]]

    # LFOs
    for i, m in enumerate(LFO_FIELD_MAPPING):
        if i >= len(p.lfos):
            break
        lfo = p.lfos[i]
        lfo.shape = data[m["shape"]]
        lfo.speed = data[m["speed"]]
        lfo.sync = data[m["sync"]]
        lfo.clocked = data[m["clocked"]]
        lfo.start_phase = data[m["start_phase"]]
        lfo.delay = data[m["delay"]]
        lfo.fade = data[m["fade"]]
        lfo.keytrack = data[m["keytrack"]]

    # Effects
    for i, m in enumerate(EFFECT_FIELD_MAPPING):
        if i >= len(p.effects):
            break
        eff = p.effects[i]
        eff.type = data[m["type"]]
        eff.mix = data[m["mix"]]
        eff.param1 = data[m["param1"]]
        eff.param2 = data[m["param2"]]
        for j in range(14):
            idx = m["params_start"] + j
            if idx < len(data):
                eff.params[j] = data[idx]

    # Modulation matrix
    for i in range(16):
        base = MOD_MATRIX_START_IDX + i * MOD_MATRIX_STRIDE
        if base + 2 >= len(data):
            break
        p.mod_matrix[i].source = data[base]
        p.mod_matrix[i].dest = data[base + 1]
        p.mod_matrix[i].amount = data[base + 2]

    # Modifiers
    for i in range(4):
        base = MODIFIER_START_IDX + i * MODIFIER_STRIDE
        if base + 3 >= len(data):
            break
        p.modifiers[i].source_a = data[base]
        p.modifiers[i].source_b = data[base + 1]
        p.modifiers[i].operator = data[base + 2]
        p.modifiers[i].constant = data[base + 3]

    # Arpeggiator
    p.arp_mode = data[ARP_MODE_IDX]
    p.arp_pattern = data[ARP_PATTERN_IDX]
    p.arp_clock = data[ARP_CLOCK_IDX]
    p.arp_length = data[ARP_LENGTH_IDX]
    p.arp_range = data[ARP_RANGE_IDX]
    p.arp_direction = data[ARP_DIRECTION_IDX]
    p.arp_sort = data[ARP_SORT_IDX]
    p.arp_velocity_mode = data[ARP_VELOCITY_MODE_IDX]
    p.arp_timing_factor = data[ARP_TIMING_FACTOR_IDX]
    p.arp_pattern_reset = data[ARP_PATTERN_RESET_IDX]
    p.arp_pattern_length = data[ARP_PATTERN_LENGTH_IDX]
    p.arp_tempo = data[ARP_TEMPO_IDX]
    for i in range(16):
        idx = ARP_PATTERN_STEPS_START_IDX + i
        if idx < len(data):
            p.arp_pattern_steps[i] = data[idx]
    for i in range(16):
        idx = ARP_PATTERN_TIMING_START_IDX + i
        if idx < len(data):
            p.arp_pattern_timing[i] = data[idx]

    # Name and metadata
    name_bytes = data[363:363 + 16]
    p.name = bytes(name_bytes).rstrip(b'\x00').decode('ascii', errors='replace')
    p.category = data[379]
    p.subcategory = data[380] if len(data) > 380 else 0

    # Amp
    p.amp_volume = data[AMP_VOLUME_IDX]
    p.amp_velocity = data[AMP_VELOCITY_IDX]
    p.amp_mod_source = data[AMP_MOD_SOURCE_IDX]
    p.amp_mod_amount = data[AMP_MOD_AMOUNT_IDX]
    p.amp_drive = p.amp_mod_amount
    p.master_tune = data[MASTER_TUNE_IDX]

    return p


def patch_to_sdata(p: Patch) -> bytes:
    """Convert Patch object to 383-byte SDATA."""
    data = bytearray(PATCH_SIZE)

    # Oscillators
    for i, m in enumerate(OSC_FIELD_MAPPING):
        if i >= len(p.oscillators):
            break
        osc = p.oscillators[i]
        if m["octave"] >= 0:
            data[m["octave"]] = osc.octave
        data[m["pitch"]] = osc.pitch
        if m["bend_range"] >= 0:
            data[m["bend_range"]] = osc.bend_range
        if m["keytrack"] >= 0:
            data[m["keytrack"]] = osc.keytrack
        data[m["detune"]] = osc.detune
        data[m["fm_source"]] = osc.fm_source
        data[m["fm"]] = osc.fm
        data[m["shape"]] = osc.shape
        data[m["pw"]] = osc.pw
        if m["pwm_source"] >= 0:
            data[m["pwm_source"]] = osc.pwm_source
        data[m["pwm"]] = osc.pwm
        if m["limit_wt"] >= 0:
            data[m["limit_wt"]] = osc.limit_wt
        if m["brilliance"] >= 0:
            data[m["brilliance"]] = osc.brilliance

    data[OSC_SYNC_IDX] = p.osc2_sync
    data[OSC_PITCH_SOURCE_IDX] = p.osc_pitch_source
    data[OSC_PITCH_AMOUNT_IDX] = p.osc_pitch_amount

    # Mixer
    data[MIX_OSC1_IDX] = p.mix_osc1
    data[MIX_OSC1_BAL_IDX] = p.mix_osc1_balance
    data[MIX_OSC2_IDX] = p.mix_osc2
    data[MIX_OSC2_BAL_IDX] = p.mix_osc2_balance
    data[MIX_OSC3_IDX] = p.mix_osc3
    data[MIX_OSC3_BAL_IDX] = p.mix_osc3_balance
    data[MIX_NOISE_IDX] = p.mix_noise
    data[MIX_NOISE_BAL_IDX] = p.mix_noise_balance
    data[MIX_NOISE_COLOR_IDX] = p.mix_noise_color
    data[MIX_RING_IDX] = p.mix_ring
    data[MIX_RING_BAL_IDX] = p.mix_ring_balance

    # Filters
    for i, m in enumerate(FILTER_FIELD_MAPPING):
        if i >= len(p.filters):
            break
        f = p.filters[i]
        data[m["type"]] = f.type
        data[m["cutoff"]] = f.cutoff
        data[m["res"]] = f.res
        data[m["drive"]] = f.drive
        data[m["drive_curve"]] = f.drive_curve
        data[m["env_amt"]] = f.env_amt
        data[m["env_vel"]] = f.env_vel
        data[m["keytrack"]] = f.keytrack
        data[m["mod_source"]] = f.mod_source
        data[m["mod_amount"]] = f.mod_amount
        data[m["fm_source"]] = f.fm_source
        data[m["fm_amount"]] = f.fm_amount
        data[m["pan"]] = f.pan
        data[m["pan_source"]] = f.pan_source
        data[m["pan_amount"]] = f.pan_amount

    data[FILTER_ROUTING_IDX] = p.filter_routing
    data[GLIDE_MODE_IDX] = p.glide_mode
    data[GLIDE_RATE_IDX] = p.glide_rate
    data[UNISON_IDX] = p.unison
    data[UNISON_DETUNE_IDX] = p.unison_detune

    # Envelopes
    for i, m in enumerate(ENVELOPE_FIELD_MAPPING):
        if i >= len(p.envelopes):
            break
        env = p.envelopes[i]
        data[m["mode"]] = env.mode
        data[m["attack"]] = env.attack
        data[m["attack_level"]] = env.attack_level
        data[m["decay"]] = env.decay
        data[m["sustain"]] = env.sustain
        data[m["decay2"]] = env.decay2
        data[m["sustain2"]] = env.sustain2
        data[m["release"]] = env.release

    # LFOs
    for i, m in enumerate(LFO_FIELD_MAPPING):
        if i >= len(p.lfos):
            break
        lfo = p.lfos[i]
        data[m["shape"]] = lfo.shape
        data[m["speed"]] = lfo.speed
        data[m["sync"]] = lfo.sync
        data[m["clocked"]] = lfo.clocked
        data[m["start_phase"]] = lfo.start_phase
        data[m["delay"]] = lfo.delay
        data[m["fade"]] = lfo.fade
        data[m["keytrack"]] = lfo.keytrack

    # Effects
    for i, m in enumerate(EFFECT_FIELD_MAPPING):
        if i >= len(p.effects):
            break
        eff = p.effects[i]
        data[m["type"]] = eff.type
        data[m["mix"]] = eff.mix
        data[m["param1"]] = eff.param1
        data[m["param2"]] = eff.param2
        for j in range(len(eff.params)):
            idx = m["params_start"] + j
            if idx < len(data):
                data[idx] = eff.params[j]

    # Modulation matrix
    for i, mod in enumerate(p.mod_matrix):
        base = MOD_MATRIX_START_IDX + i * MOD_MATRIX_STRIDE
        if base + 2 >= len(data):
            break
        data[base] = mod.source
        data[base + 1] = mod.dest
        data[base + 2] = mod.amount

    # Modifiers
    for i, mod in enumerate(p.modifiers):
        base = MODIFIER_START_IDX + i * MODIFIER_STRIDE
        if base + 3 >= len(data):
            break
        data[base] = mod.source_a
        data[base + 1] = mod.source_b
        data[base + 2] = mod.operator
        data[base + 3] = mod.constant

    # Arpeggiator
    data[ARP_MODE_IDX] = p.arp_mode
    data[ARP_PATTERN_IDX] = p.arp_pattern
    data[ARP_CLOCK_IDX] = p.arp_clock
    data[ARP_LENGTH_IDX] = p.arp_length
    data[ARP_RANGE_IDX] = p.arp_range
    data[ARP_DIRECTION_IDX] = p.arp_direction
    data[ARP_SORT_IDX] = p.arp_sort
    data[ARP_VELOCITY_MODE_IDX] = p.arp_velocity_mode
    data[ARP_TIMING_FACTOR_IDX] = p.arp_timing_factor
    data[ARP_PATTERN_RESET_IDX] = p.arp_pattern_reset
    data[ARP_PATTERN_LENGTH_IDX] = p.arp_pattern_length
    data[ARP_TEMPO_IDX] = p.arp_tempo
    for i in range(16):
        idx = ARP_PATTERN_STEPS_START_IDX + i
        if idx < len(data):
            data[idx] = p.arp_pattern_steps[i]
    for i in range(16):
        idx = ARP_PATTERN_TIMING_START_IDX + i
        if idx < len(data):
            data[idx] = p.arp_pattern_timing[i]

    # Name
    name_bytes = p.name.encode('ascii', errors='replace')[:16].ljust(16, b'\x00')
    data[363:363 + 16] = name_bytes

    data[379] = p.category
    if len(data) > 380:
        data[380] = p.subcategory

    # Amp
    data[AMP_VOLUME_IDX] = p.amp_volume
    data[AMP_VELOCITY_IDX] = p.amp_velocity
    data[AMP_MOD_SOURCE_IDX] = p.amp_mod_source
    data[AMP_MOD_AMOUNT_IDX] = p.amp_mod_amount
    data[MASTER_TUNE_IDX] = p.master_tune

    return bytes(data)


def patch_to_sndd(p: Patch, device_id: int, bank: int, program: int) -> bytes:
    """Convert Patch to full SNDD SysEx message."""
    sdata = patch_to_sdata(p)

    # Calculate checksum
    checksum = sum(sdata) & 0x7F

    # Build SNDD message
    msg = bytes([0xF0, WALDORF_ID, BLOFELD_ID, device_id, MSG_SNDD, bank, program])
    msg += sdata
    msg += bytes([checksum, 0xF7])

    return msg


def bank_to_byte(bank: str) -> int:
    """Convert bank letter (A-H) to byte value (0-7)."""
    if not bank:
        raise ValueError("Bank must not be empty")
    ch = bank.upper()[0]
    if ch < 'A' or ch > 'H':
        raise ValueError(f"Bank must be A-H, got {bank!r}")
    return ord(ch) - ord('A')


def find_blofeld_ports() -> Tuple[Optional[str], Optional[str]]:
    """Find Blofeld MIDI input and output ports."""
    in_port = None
    out_port = None

    for name in mido.get_input_names():
        if 'blofeld' in name.lower():
            in_port = name
            break

    for name in mido.get_output_names():
        if 'blofeld' in name.lower():
            out_port = name
            break

    return in_port, out_port


class Blofeld:
    """Blofeld synthesizer MIDI interface."""

    def __init__(self, channel: int = 4, device_id: int = 0x00):
        self.channel = channel
        self.device_id = device_id
        self._in_port: Optional[mido.ports.BaseInput] = None
        self._out_port: Optional[mido.ports.BaseOutput] = None

    def open(self) -> None:
        """Open MIDI connections to Blofeld."""
        in_name, out_name = find_blofeld_ports()

        if not in_name:
            raise RuntimeError("Could not find Blofeld MIDI input port")
        if not out_name:
            raise RuntimeError("Could not find Blofeld MIDI output port")

        self._in_port = mido.open_input(in_name)
        self._out_port = mido.open_output(out_name)

    def close(self) -> None:
        """Close MIDI connections."""
        if self._in_port:
            self._in_port.close()
            self._in_port = None
        if self._out_port:
            self._out_port.close()
            self._out_port = None

    def __enter__(self):
        self.open()
        return self

    def __exit__(self, *args):
        self.close()

    def send(self, msg: mido.Message) -> None:
        """Send a MIDI message."""
        if not self._out_port:
            self.open()
        self._out_port.send(msg)

    def send_sysex(self, data: bytes) -> None:
        """Send raw SysEx data."""
        # mido expects sysex data without F0/F7
        if data[0] == 0xF0:
            data = data[1:]
        if data[-1] == 0xF7:
            data = data[:-1]
        msg = mido.Message('sysex', data=list(data))
        self.send(msg)

    def request_patch(self, bank: str, program: int) -> Patch:
        """Request and receive a patch from the Blofeld."""
        if not self._in_port or not self._out_port:
            self.open()

        bank_byte = bank_to_byte(bank)
        if program < 1 or program > 128:
            raise ValueError(f"Program must be 1-128, got {program}")
        prog_byte = program - 1

        # Clear any pending messages
        while self._in_port.poll():
            pass

        # Send request
        req = bytes([0xF0, WALDORF_ID, BLOFELD_ID, self.device_id, MSG_SNDR, bank_byte, prog_byte, 0xF7])
        self.send_sysex(req)

        # Wait for response
        start = time.time()
        while time.time() - start < SYSEX_TIMEOUT:
            msg = self._in_port.poll()
            if msg and msg.type == 'sysex':
                data = bytes([0xF0] + list(msg.data) + [0xF7])
                return self._parse_sndd(data)
            time.sleep(0.01)

        raise TimeoutError("Timed out waiting for patch dump")

    def _parse_sndd(self, msg: bytes) -> Patch:
        """Parse SNDD SysEx message into Patch."""
        expected_len = PATCH_SIZE + 9
        if len(msg) != expected_len:
            raise ValueError(f"Unexpected dump size {len(msg)} (expected {expected_len})")

        if msg[0] != 0xF0 or msg[-1] != 0xF7:
            raise ValueError("Message is not a SysEx frame")

        if msg[1] != WALDORF_ID or msg[2] != BLOFELD_ID:
            raise ValueError("Not a Waldorf Blofeld SysEx")

        if msg[4] != MSG_SNDD:
            raise ValueError(f"Unexpected message type 0x{msg[4]:02X} (expected SNDD 0x10)")

        sdata = msg[7:7 + PATCH_SIZE]
        checksum = msg[7 + PATCH_SIZE]

        # Verify checksum
        calc_checksum = sum(sdata) & 0x7F
        if checksum != 0x7F and calc_checksum != checksum:
            raise ValueError(f"Checksum mismatch: expected 0x{calc_checksum:02X}, got 0x{checksum:02X}")

        return parse_sdata(sdata)

    def send_patch(self, bank: str, program: int, patch: Patch) -> None:
        """Send a patch to the Blofeld."""
        if not self._out_port:
            self.open()

        # Save patch locally
        save_patch(patch)

        bank_byte = bank_to_byte(bank)
        if program < 1 or program > 128:
            raise ValueError(f"Program must be 1-128, got {program}")
        prog_byte = program - 1

        sndd = patch_to_sndd(patch, self.device_id, bank_byte, prog_byte)
        self.send_sysex(sndd)

    def note_on(self, note: int, velocity: int = 100) -> None:
        """Send note on."""
        self.send(mido.Message('note_on', channel=self.channel, note=note, velocity=velocity))

    def note_off(self, note: int) -> None:
        """Send note off."""
        self.send(mido.Message('note_off', channel=self.channel, note=note, velocity=0))


def normalize_patch_name(name: str) -> str:
    """Convert patch name to filesystem-safe string."""
    normalized = re.sub(r'[^a-zA-Z0-9\-_]+', '_', name)
    normalized = normalized.strip('_').lower()
    return normalized or 'unnamed'


def save_patch(patch: Patch) -> Path:
    """Save patch to patches directory."""
    script_dir = Path(__file__).parent
    patches_dir = script_dir / 'patches'
    patches_dir.mkdir(exist_ok=True)

    timestamp = datetime.now().strftime('%Y%m%d_%H%M%S')
    normalized_name = normalize_patch_name(patch.name)
    filename = f"{timestamp}_{normalized_name}.json"
    filepath = patches_dir / filename

    filepath.write_text(patch_to_json(patch))
    return filepath


def patch_to_dict(patch: Patch) -> dict:
    """Convert Patch to dictionary for JSON serialization."""
    return {
        'oscillators': [asdict(o) for o in patch.oscillators],
        'osc2_sync': patch.osc2_sync,
        'osc_pitch_source': patch.osc_pitch_source,
        'osc_pitch_amount': patch.osc_pitch_amount,
        'filters': [asdict(f) for f in patch.filters],
        'mix_osc1': patch.mix_osc1,
        'mix_osc1_balance': patch.mix_osc1_balance,
        'mix_osc2': patch.mix_osc2,
        'mix_osc2_balance': patch.mix_osc2_balance,
        'mix_osc3': patch.mix_osc3,
        'mix_osc3_balance': patch.mix_osc3_balance,
        'mix_noise': patch.mix_noise,
        'mix_noise_balance': patch.mix_noise_balance,
        'mix_noise_color': patch.mix_noise_color,
        'mix_ring': patch.mix_ring,
        'mix_ring_balance': patch.mix_ring_balance,
        'filter_routing': patch.filter_routing,
        'glide_mode': patch.glide_mode,
        'glide_rate': patch.glide_rate,
        'unison': patch.unison,
        'unison_detune': patch.unison_detune,
        'envelopes': [asdict(e) for e in patch.envelopes],
        'lfos': [asdict(l) for l in patch.lfos],
        'mod_matrix': [asdict(m) for m in patch.mod_matrix],
        'modifiers': [asdict(m) for m in patch.modifiers],
        'arp_mode': patch.arp_mode,
        'arp_pattern': patch.arp_pattern,
        'arp_clock': patch.arp_clock,
        'arp_length': patch.arp_length,
        'arp_range': patch.arp_range,
        'arp_direction': patch.arp_direction,
        'arp_sort': patch.arp_sort,
        'arp_velocity_mode': patch.arp_velocity_mode,
        'arp_timing_factor': patch.arp_timing_factor,
        'arp_pattern_reset': patch.arp_pattern_reset,
        'arp_pattern_length': patch.arp_pattern_length,
        'arp_tempo': patch.arp_tempo,
        'arp_pattern_steps': patch.arp_pattern_steps,
        'arp_pattern_timing': patch.arp_pattern_timing,
        'effects': [{'type': e.type, 'mix': e.mix, 'param1': e.param1, 'param2': e.param2, 'params': e.params} for e in patch.effects],
        'amp_volume': patch.amp_volume,
        'amp_velocity': patch.amp_velocity,
        'amp_mod_source': patch.amp_mod_source,
        'amp_mod_amount': patch.amp_mod_amount,
        'amp_pan': patch.amp_pan,
        'amp_drive': patch.amp_drive,
        'master_tune': patch.master_tune,
        'name': patch.name,
        'category': patch.category,
        'subcategory': patch.subcategory,
    }


def patch_from_dict(d: dict) -> Patch:
    """Create Patch from dictionary."""
    p = Patch()

    p.oscillators = [Oscillator(**o) for o in d.get('oscillators', [])]
    while len(p.oscillators) < 3:
        p.oscillators.append(Oscillator())

    p.osc2_sync = d.get('osc2_sync', 0)
    p.osc_pitch_source = d.get('osc_pitch_source', 0)
    p.osc_pitch_amount = d.get('osc_pitch_amount', 64)

    p.filters = [Filter(**f) for f in d.get('filters', [])]
    while len(p.filters) < 2:
        p.filters.append(Filter())

    p.mix_osc1 = d.get('mix_osc1', 127)
    p.mix_osc1_balance = d.get('mix_osc1_balance', 64)
    p.mix_osc2 = d.get('mix_osc2', 0)
    p.mix_osc2_balance = d.get('mix_osc2_balance', 64)
    p.mix_osc3 = d.get('mix_osc3', 0)
    p.mix_osc3_balance = d.get('mix_osc3_balance', 64)
    p.mix_noise = d.get('mix_noise', 0)
    p.mix_noise_balance = d.get('mix_noise_balance', 64)
    p.mix_noise_color = d.get('mix_noise_color', 64)
    p.mix_ring = d.get('mix_ring', 0)
    p.mix_ring_balance = d.get('mix_ring_balance', 64)

    p.filter_routing = d.get('filter_routing', 0)
    p.glide_mode = d.get('glide_mode', 0)
    p.glide_rate = d.get('glide_rate', 0)
    p.unison = d.get('unison', 0)
    p.unison_detune = d.get('unison_detune', 0)

    p.envelopes = [Envelope(**e) for e in d.get('envelopes', [])]
    while len(p.envelopes) < 3:
        p.envelopes.append(Envelope())

    p.lfos = [LFO(**l) for l in d.get('lfos', [])]
    while len(p.lfos) < 3:
        p.lfos.append(LFO())

    p.mod_matrix = [ModulationMatrix(**m) for m in d.get('mod_matrix', [])]
    while len(p.mod_matrix) < 16:
        p.mod_matrix.append(ModulationMatrix())

    p.modifiers = [Modifier(**m) for m in d.get('modifiers', [])]
    while len(p.modifiers) < 4:
        p.modifiers.append(Modifier())

    p.arp_mode = d.get('arp_mode', 0)
    p.arp_pattern = d.get('arp_pattern', 0)
    p.arp_clock = d.get('arp_clock', 11)
    p.arp_length = d.get('arp_length', 11)
    p.arp_range = d.get('arp_range', 0)
    p.arp_direction = d.get('arp_direction', 0)
    p.arp_sort = d.get('arp_sort', 0)
    p.arp_velocity_mode = d.get('arp_velocity_mode', 0)
    p.arp_timing_factor = d.get('arp_timing_factor', 64)
    p.arp_pattern_reset = d.get('arp_pattern_reset', 0)
    p.arp_pattern_length = d.get('arp_pattern_length', 15)
    p.arp_tempo = d.get('arp_tempo', 60)
    p.arp_pattern_steps = d.get('arp_pattern_steps', [0] * 16)
    p.arp_pattern_timing = d.get('arp_pattern_timing', [0] * 16)

    effects_data = d.get('effects', [])
    p.effects = []
    for e in effects_data:
        eff = Effect()
        eff.type = e.get('type', 0)
        eff.mix = e.get('mix', 64)
        eff.param1 = e.get('param1', 0)
        eff.param2 = e.get('param2', 0)
        eff.params = e.get('params', [0] * 14)
        p.effects.append(eff)
    while len(p.effects) < 2:
        p.effects.append(Effect())

    p.amp_volume = d.get('amp_volume', 127)
    p.amp_velocity = d.get('amp_velocity', 64)
    p.amp_mod_source = d.get('amp_mod_source', 0)
    p.amp_mod_amount = d.get('amp_mod_amount', 64)
    p.amp_pan = d.get('amp_pan', 64)
    p.amp_drive = d.get('amp_drive', 0)
    p.master_tune = d.get('master_tune', 64)

    p.name = d.get('name', 'Init')
    p.category = d.get('category', 0)
    p.subcategory = d.get('subcategory', 0)

    return p


def patch_to_json(patch: Patch) -> str:
    """Serialize Patch to JSON string."""
    return json.dumps(patch_to_dict(patch), indent=2)


def patch_from_json(json_str: str) -> Patch:
    """Deserialize Patch from JSON string."""
    return patch_from_dict(json.loads(json_str))
