#!/usr/bin/env python3
"""
Phase 3: Export trained model to ONNX for browser inference.

Input:  models/model.pt, models/model_meta.json
Output: models/model.onnx
"""

import json
import sys
from pathlib import Path

import torch

sys.path.insert(0, str(Path(__file__).parent))
from train import SynthMapper

ROOT = Path(__file__).parent.parent
MODELS_DIR = ROOT / "models"


def main():
    meta = json.loads((MODELS_DIR / "model_meta.json").read_text())
    n_dims = meta["n_dims"]
    n_params = meta["n_params"]

    model = SynthMapper(n_dims, n_params)
    model.load_state_dict(torch.load(MODELS_DIR / "model.pt", weights_only=True))
    model.eval()

    dummy = torch.zeros(1, n_dims)
    output_path = MODELS_DIR / "model.onnx"

    torch.onnx.export(
        model,
        dummy,
        output_path,
        input_names=["dims"],
        output_names=["params"],
        dynamic_axes={"dims": {0: "batch"}, "params": {0: "batch"}},
        opset_version=15,
        dynamo=False,
    )

    size_kb = output_path.stat().st_size / 1024
    print(f"Exported -> models/model.onnx  ({size_kb:.0f} KB)")
    print(f"  Input:  [{n_dims}]  ({', '.join(meta['dim_names'])})")
    print(f"  Output: [{n_params}] curated Blofeld params")


if __name__ == "__main__":
    main()
