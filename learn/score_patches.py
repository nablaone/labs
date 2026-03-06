#!/usr/bin/env python3
"""
Phase 1b: Score patches against semantic dimensions using Claude.

Input:  data/patches.json, learn/dimensions.json
Output: data/scored_patches.json

Resumable: re-running skips already-scored patches.
"""

import argparse
import json
import sys
import time
from pathlib import Path

import anthropic

ROOT = Path(__file__).parent.parent
PATCHES_FILE = ROOT / "data" / "patches.json"
DIMENSIONS_FILE = Path(__file__).parent / "dimensions.json"
OUTPUT_FILE = ROOT / "data" / "scored_patches.json"


def patch_summary(patch: dict) -> dict:
    """Extract musically relevant parameters — skips arp, mod matrix noise."""
    return {
        "name": patch.get("name", ""),
        "oscillators": [
            {"shape": o["shape"], "octave": o["octave"], "detune": o["detune"],
             "fm": o["fm"], "pw": o["pw"]}
            for o in patch.get("oscillators", [])
        ],
        "mixer": {
            "osc1": patch.get("mix_osc1"), "osc2": patch.get("mix_osc2"),
            "osc3": patch.get("mix_osc3"), "noise": patch.get("mix_noise"),
            "ring": patch.get("mix_ring"),
        },
        "filters": [
            {"type": f["type"], "cutoff": f["cutoff"], "res": f["res"],
             "drive": f["drive"], "env_amt": f["env_amt"]}
            for f in patch.get("filters", [])
        ],
        "envelopes": [
            {"attack": e["attack"], "decay": e["decay"],
             "sustain": e["sustain"], "release": e["release"]}
            for e in patch.get("envelopes", [])
        ],
        "lfos": [
            {"shape": l["shape"], "speed": l["speed"], "fade": l["fade"]}
            for l in patch.get("lfos", [])
        ],
        "effects": [
            {"type": e["type"], "mix": e["mix"]}
            for e in patch.get("effects", [])
        ],
        "mod_matrix": [
            {"source": m["source"], "dest": m["dest"], "amount": m["amount"]}
            for m in patch.get("mod_matrix", [])
            if m["source"] != 0 or m["dest"] != 0  # skip empty slots
        ],
    }


def build_prompt(patch: dict, dimensions: list[dict]) -> str:
    summary = patch_summary(patch["patch"])
    dims = "\n".join(
        f'  "{d["name"]}": 0.0-1.0  // {d["low"]} → {d["high"]}'
        for d in dimensions
    )
    return f"""Score this Waldorf Blofeld synthesizer patch on perceptual dimensions.
Parameters are 0-127 integers (64 = center/neutral).

Patch:
{json.dumps(summary, indent=2)}

Score each dimension from 0.0 to 1.0:
{dims}

Return only a JSON object with the dimension scores, no explanation."""


def score_patch(client: anthropic.Anthropic, patch: dict, dimensions: list[dict]) -> dict:
    msg = client.messages.create(
        model="claude-sonnet-4-6",
        max_tokens=256,
        messages=[{"role": "user", "content": build_prompt(patch, dimensions)}],
    )
    text = msg.content[0].text.strip()
    if text.startswith("```"):
        text = text.split("```")[1]
        if text.startswith("json"):
            text = text[4:]
    return json.loads(text)


def patch_key(p: dict) -> tuple:
    return (p["meta"]["source"], p["meta"]["bank"], p["meta"]["program"])


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--limit", type=int, help="Score only N patches (for testing)")
    parser.add_argument("--delay", type=float, default=0.3, help="Seconds between API calls")
    args = parser.parse_args()

    dimensions = json.loads(DIMENSIONS_FILE.read_text())
    patches = json.loads(PATCHES_FILE.read_text())

    # Resumable: load existing output
    if OUTPUT_FILE.exists():
        scored = json.loads(OUTPUT_FILE.read_text())
        done = {patch_key(r) for r in scored}
    else:
        scored = []
        done = set()

    to_score = [p for p in patches if patch_key(p) not in done]
    if args.limit:
        to_score = to_score[: args.limit]

    print(f"Scoring {len(to_score)} patches ({len(done)} already done)")

    client = anthropic.Anthropic()

    for i, patch in enumerate(to_score):
        name = patch["patch"].get("name", "?")
        label = f"{patch['meta']['bank']}{patch['meta']['program']:03d} {name}"
        print(f"  [{i + 1}/{len(to_score)}] {label} ...", end=" ", flush=True)

        try:
            vector = score_patch(client, patch, dimensions)
            patch["vector"] = vector
            scored.append(patch)
            OUTPUT_FILE.write_text(json.dumps(scored, indent=2))
            print("ok")
        except Exception as e:
            print(f"FAILED: {e}", file=sys.stderr)

        if i < len(to_score) - 1:
            time.sleep(args.delay)

    print(f"\nDone: {len(scored)} patches -> {OUTPUT_FILE.relative_to(ROOT)}")


if __name__ == "__main__":
    main()
