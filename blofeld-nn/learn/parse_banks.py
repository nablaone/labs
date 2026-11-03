#!/usr/bin/env python3
"""
Phase 1a: Parse Blofeld .syx bank files into a JSON dataset.

Input:  assets/**/*.syx
Output: data/patches.json
"""

import json
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent))
from blofeld import parse_sdata, patch_to_dict, PATCH_SIZE, WALDORF_ID, BLOFELD_ID, MSG_SNDD


def split_sysex(data: bytes) -> list[bytes]:
    """Split raw bytes into individual F0...F7 SysEx messages."""
    messages = []
    i = 0
    while i < len(data):
        if data[i] == 0xF0:
            end = data.find(0xF7, i)
            if end == -1:
                break
            messages.append(data[i:end + 1])
            i = end + 1
        else:
            i += 1
    return messages


def parse_sndd(msg: bytes, source: str) -> dict | None:
    """Parse a SNDD SysEx message. Returns a patch record or None."""
    if len(msg) != PATCH_SIZE + 9:
        return None
    if msg[0] != 0xF0 or msg[-1] != 0xF7:
        return None
    if msg[1] != WALDORF_ID or msg[2] != BLOFELD_ID:
        return None
    if msg[4] != MSG_SNDD:
        return None

    bank = chr(ord('A') + msg[5])
    program = msg[6] + 1
    sdata = msg[7:7 + PATCH_SIZE]

    patch = parse_sdata(sdata)
    return {
        "meta": {"source": source, "bank": bank, "program": program},
        "patch": patch_to_dict(patch),
    }


def parse_syx_file(path: Path) -> list[dict]:
    source = path.stem
    records = []
    for msg in split_sysex(path.read_bytes()):
        record = parse_sndd(msg, source)
        if record:
            records.append(record)
    return records


def main():
    root = Path(__file__).parent.parent
    output = root / "data" / "patches.json"
    output.parent.mkdir(exist_ok=True)

    syx_files = sorted(
        p for p in (root / "assets").rglob("*.syx")
        if "__MACOSX" not in p.parts
    )

    if not syx_files:
        print("No .syx files found in assets/", file=sys.stderr)
        sys.exit(1)

    all_patches = []
    for path in syx_files:
        records = parse_syx_file(path)
        print(f"  {path.relative_to(root)}: {len(records)} patches")
        all_patches.extend(records)

    output.write_text(json.dumps(all_patches, indent=2))
    print(f"\nTotal: {len(all_patches)} patches -> {output.relative_to(root)}")


if __name__ == "__main__":
    main()
