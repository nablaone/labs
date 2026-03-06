"""
Curated Blofeld parameters for NN training.

Includes musically meaningful params (~85 total).
Skips: arp, mod matrix routing (source/dest), glide, unison,
       envelope mode, LFO sync/phase, pan, keytrack, bend range.

All values are 0-127 integers. Normalized to 0.0-1.0 for training.
"""

import numpy as np

# ---------------------------------------------------------------------------
# Parameter definitions — order matters, must stay stable
# ---------------------------------------------------------------------------

PARAMS = [
    # --- Oscillator 1 ---
    ("osc1_shape",   lambda p: p["oscillators"][0]["shape"]),
    ("osc1_octave",  lambda p: p["oscillators"][0]["octave"]),
    ("osc1_detune",  lambda p: p["oscillators"][0]["detune"]),
    ("osc1_fm",      lambda p: p["oscillators"][0]["fm"]),
    ("osc1_pw",      lambda p: p["oscillators"][0]["pw"]),
    ("osc1_pwm",     lambda p: p["oscillators"][0]["pwm"]),

    # --- Oscillator 2 ---
    ("osc2_shape",   lambda p: p["oscillators"][1]["shape"]),
    ("osc2_octave",  lambda p: p["oscillators"][1]["octave"]),
    ("osc2_detune",  lambda p: p["oscillators"][1]["detune"]),
    ("osc2_fm",      lambda p: p["oscillators"][1]["fm"]),
    ("osc2_pw",      lambda p: p["oscillators"][1]["pw"]),
    ("osc2_pwm",     lambda p: p["oscillators"][1]["pwm"]),

    # --- Oscillator 3 ---
    ("osc3_shape",   lambda p: p["oscillators"][2]["shape"]),
    ("osc3_octave",  lambda p: p["oscillators"][2]["octave"]),
    ("osc3_detune",  lambda p: p["oscillators"][2]["detune"]),
    ("osc3_fm",      lambda p: p["oscillators"][2]["fm"]),
    ("osc3_pw",      lambda p: p["oscillators"][2]["pw"]),
    ("osc3_pwm",     lambda p: p["oscillators"][2]["pwm"]),

    # --- Mixer ---
    ("mix_osc1",        lambda p: p["mix_osc1"]),
    ("mix_osc2",        lambda p: p["mix_osc2"]),
    ("mix_osc3",        lambda p: p["mix_osc3"]),
    ("mix_noise",       lambda p: p["mix_noise"]),
    ("mix_ring",        lambda p: p["mix_ring"]),
    ("mix_noise_color", lambda p: p["mix_noise_color"]),

    # --- Filter 1 ---
    ("f1_type",    lambda p: p["filters"][0]["type"]),
    ("f1_cutoff",  lambda p: p["filters"][0]["cutoff"]),
    ("f1_res",     lambda p: p["filters"][0]["res"]),
    ("f1_drive",   lambda p: p["filters"][0]["drive"]),
    ("f1_env_amt", lambda p: p["filters"][0]["env_amt"]),

    # --- Filter 2 ---
    ("f2_type",    lambda p: p["filters"][1]["type"]),
    ("f2_cutoff",  lambda p: p["filters"][1]["cutoff"]),
    ("f2_res",     lambda p: p["filters"][1]["res"]),
    ("f2_drive",   lambda p: p["filters"][1]["drive"]),
    ("f2_env_amt", lambda p: p["filters"][1]["env_amt"]),

    # --- Filter routing ---
    ("filter_routing", lambda p: p["filter_routing"]),

    # --- Envelope 1 (amp env) ---
    ("env1_attack",  lambda p: p["envelopes"][0]["attack"]),
    ("env1_decay",   lambda p: p["envelopes"][0]["decay"]),
    ("env1_sustain", lambda p: p["envelopes"][0]["sustain"]),
    ("env1_release", lambda p: p["envelopes"][0]["release"]),

    # --- Envelope 2 (filter env) ---
    ("env2_attack",  lambda p: p["envelopes"][1]["attack"]),
    ("env2_decay",   lambda p: p["envelopes"][1]["decay"]),
    ("env2_sustain", lambda p: p["envelopes"][1]["sustain"]),
    ("env2_release", lambda p: p["envelopes"][1]["release"]),

    # --- Envelope 3 (free env) ---
    ("env3_attack",  lambda p: p["envelopes"][2]["attack"]),
    ("env3_decay",   lambda p: p["envelopes"][2]["decay"]),
    ("env3_sustain", lambda p: p["envelopes"][2]["sustain"]),
    ("env3_release", lambda p: p["envelopes"][2]["release"]),

    # --- LFO 1 ---
    ("lfo1_shape", lambda p: p["lfos"][0]["shape"]),
    ("lfo1_speed", lambda p: p["lfos"][0]["speed"]),
    ("lfo1_fade",  lambda p: p["lfos"][0]["fade"]),
    ("lfo1_delay", lambda p: p["lfos"][0]["delay"]),

    # --- LFO 2 ---
    ("lfo2_shape", lambda p: p["lfos"][1]["shape"]),
    ("lfo2_speed", lambda p: p["lfos"][1]["speed"]),
    ("lfo2_fade",  lambda p: p["lfos"][1]["fade"]),
    ("lfo2_delay", lambda p: p["lfos"][1]["delay"]),

    # --- LFO 3 ---
    ("lfo3_shape", lambda p: p["lfos"][2]["shape"]),
    ("lfo3_speed", lambda p: p["lfos"][2]["speed"]),
    ("lfo3_fade",  lambda p: p["lfos"][2]["fade"]),
    ("lfo3_delay", lambda p: p["lfos"][2]["delay"]),

    # --- Effect 1 ---
    # Note: params[0]/params[1] are used (not param1/param2) because in patch_to_sdata
    # the params[] loop overwrites param1/param2 at the same SDATA indices.
    ("fx1_type",   lambda p: p["effects"][0]["type"]),
    ("fx1_mix",    lambda p: p["effects"][0]["mix"]),
    ("fx1_param1", lambda p: p["effects"][0]["params"][0]),
    ("fx1_param2", lambda p: p["effects"][0]["params"][1]),

    # --- Effect 2 ---
    ("fx2_type",   lambda p: p["effects"][1]["type"]),
    ("fx2_mix",    lambda p: p["effects"][1]["mix"]),
    ("fx2_param1", lambda p: p["effects"][1]["params"][0]),
    ("fx2_param2", lambda p: p["effects"][1]["params"][1]),

    # --- Amp ---
    ("amp_volume",   lambda p: p["amp_volume"]),
    ("amp_velocity", lambda p: p["amp_velocity"]),

    # --- Mod matrix: amounts only (source/dest skipped for v1) ---
    *[(f"mod{i+1}_amount", (lambda i: lambda p: p["mod_matrix"][i]["amount"])(i))
      for i in range(16)],
]

PARAM_NAMES = [name for name, _ in PARAMS]
N_PARAMS = len(PARAMS)  # 85


def extract(patch: dict) -> np.ndarray:
    """Extract curated params from a patch dict. Returns float32 array, values 0-127."""
    return np.array([fn(patch) for _, fn in PARAMS], dtype=np.float32)


def normalize(v: np.ndarray) -> np.ndarray:
    """Scale 0-127 → 0.0-1.0."""
    return v / 127.0


def denormalize(v: np.ndarray) -> np.ndarray:
    """Scale 0.0-1.0 → 0-127, clipped and rounded to int."""
    return np.clip(np.round(v * 127.0), 0, 127).astype(np.int32)


def apply_to_patch(values: np.ndarray, base_patch: dict) -> dict:
    """Write denormalized values back into a patch dict (modifies in place)."""
    ints = denormalize(values)
    for (name, _), val in zip(PARAMS, ints):
        val = int(val)
        if name.startswith("osc"):
            i = int(name[3]) - 1
            field = name[5:]
            base_patch["oscillators"][i][field] = val
        elif name.startswith("mix_"):
            base_patch[name] = val
        elif name.startswith("f1_"):
            base_patch["filters"][0][name[3:]] = val
        elif name.startswith("f2_"):
            base_patch["filters"][1][name[3:]] = val
        elif name == "filter_routing":
            base_patch["filter_routing"] = val
        elif name.startswith("env"):
            i = int(name[3]) - 1
            field = name[5:]
            base_patch["envelopes"][i][field] = val
        elif name.startswith("lfo"):
            i = int(name[3]) - 1
            field = name[5:]
            base_patch["lfos"][i][field] = val
        elif name.startswith("fx"):
            i = int(name[2]) - 1
            field = name[4:]
            if field == "param1":
                base_patch["effects"][i]["params"][0] = val
            elif field == "param2":
                base_patch["effects"][i]["params"][1] = val
            else:
                base_patch["effects"][i][field] = val
        elif name in ("amp_volume", "amp_velocity"):
            base_patch[name] = val
        elif name.startswith("mod") and name.endswith("_amount"):
            i = int(name[3:-7]) - 1
            base_patch["mod_matrix"][i]["amount"] = val
    return base_patch
