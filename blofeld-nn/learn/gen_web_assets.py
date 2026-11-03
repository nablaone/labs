#!/usr/bin/env python3
"""
Generate web assets from Python data.

Outputs:
  web/assets/blofeld_map.json  — default SDATA + curated param → SDATA index map
  web/assets/dimensions.json   — copy of learn/dimensions.json
  web/models/model.onnx        — copy of models/model.onnx
"""

import copy
import json
import shutil
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
from blofeld import Patch, patch_to_sdata, patch_to_dict, patch_from_dict
from curated_params import PARAMS, PARAM_NAMES, N_PARAMS, extract, normalize, denormalize, apply_to_patch

ROOT = Path(__file__).parent.parent
WEB = ROOT / "web"


def probe_sdata_index(param_idx: int, default_dict: dict, default_sdata: list[int]) -> int:
    """Find which SDATA byte a curated param maps to by differential probing."""
    sentinel = 42

    # Build a full normalized array at default values
    default_vals = normalize(extract(default_dict)).copy()

    # Swap in the sentinel for this one param
    sentinel_norm = sentinel / 127.0
    if abs(default_vals[param_idx] - sentinel_norm) < 0.01:
        sentinel = 85
        sentinel_norm = sentinel / 127.0

    sentinel_vals = default_vals.copy()
    sentinel_vals[param_idx] = sentinel_norm

    modified = copy.deepcopy(default_dict)
    apply_to_patch(sentinel_vals, modified)
    modified_sdata = list(patch_to_sdata(patch_from_dict(modified)))

    diffs = [i for i in range(383) if default_sdata[i] != modified_sdata[i]]

    if len(diffs) == 1:
        return diffs[0]
    elif len(diffs) == 0:
        raise ValueError(f"No SDATA diff for param '{PARAM_NAMES[param_idx]}' "
                         f"(default_norm={default_vals[param_idx]:.3f}, sentinel={sentinel})")
    else:
        # Multiple diffs — take the first and warn
        print(f"  WARNING: '{PARAM_NAMES[param_idx]}' affects {len(diffs)} bytes {diffs}, using first")
        return diffs[0]


def main():
    (WEB / "assets").mkdir(parents=True, exist_ok=True)
    (WEB / "models").mkdir(parents=True, exist_ok=True)

    # Default patch
    default_patch = Patch()
    default_dict = patch_to_dict(default_patch)
    default_sdata = list(patch_to_sdata(default_patch))

    # Probe SDATA index for each curated param
    print(f"Probing SDATA indices for {N_PARAMS} params...")
    param_indices = []
    for i, (name, _) in enumerate(PARAMS):
        idx = probe_sdata_index(i, default_dict, default_sdata)
        param_indices.append(idx)
        print(f"  {name:20s} -> sdata[{idx:3d}]  default={default_sdata[idx]}")

    blofeld_map = {
        "param_names": PARAM_NAMES,
        "param_indices": param_indices,
        "default_sdata": default_sdata,
    }
    (WEB / "assets" / "blofeld_map.json").write_text(json.dumps(blofeld_map, indent=2))
    print(f"\nblofeld_map.json written")

    # Copy dimensions.json
    src_dims = Path(__file__).parent / "dimensions.json"
    shutil.copy(src_dims, WEB / "assets" / "dimensions.json")
    print("dimensions.json copied")

    # Copy model.onnx
    src_onnx = ROOT / "models" / "model.onnx"
    shutil.copy(src_onnx, WEB / "models" / "model.onnx")
    print("model.onnx copied")


if __name__ == "__main__":
    main()
