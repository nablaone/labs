#!/usr/bin/env python3
"""
Send Patch Tool - apply a patch JSON to the MiniFreak edit buffer via CCs.

Usage:
    python send_patch.py --patch-json '{"name": "Test", "params": {...}}'
    python send_patch.py --file patches/xyz.json
    echo '{...}' | python send_patch.py
    python send_patch.py --file p.json --dry-run     # print MIDI, send nothing

The sound lives only in the edit buffer until you save it on the MiniFreak
(hold Save, pick a slot).
"""

import argparse
import json
import sys

from minifreak import MiniFreak, manual_steps, save_patch, validate_patch


def main():
    ap = argparse.ArgumentParser(description='Send patch CCs to MiniFreak')
    src = ap.add_mutually_exclusive_group()
    src.add_argument('--patch-json', '-j', help='Patch JSON (default: read stdin)')
    src.add_argument('--file', '-f', help='Patch JSON file')
    ap.add_argument('--base-preset', type=int, help='Load this preset (1-512) first; overrides JSON')
    ap.add_argument('--channel', '-c', type=int, help='MIDI channel 1-16 (default 1 / $MINIFREAK_CHANNEL)')
    ap.add_argument('--port', help='MIDI port name substring (default: auto / $MINIFREAK_PORT)')
    ap.add_argument('--dry-run', action='store_true', help='Print messages instead of sending')
    ap.add_argument('--no-save', action='store_true', help='Do not save a copy under patches/')
    args = ap.parse_args()

    if args.file:
        raw = open(args.file).read()
    elif args.patch_json:
        raw = args.patch_json
    else:
        raw = sys.stdin.read()
    if not raw.strip():
        print('Error: No patch JSON provided', file=sys.stderr)
        sys.exit(1)

    try:
        patch = validate_patch(json.loads(raw))
        if args.base_preset:
            patch['base_preset'] = args.base_preset
            patch = validate_patch(patch)

        with MiniFreak(channel=args.channel, port=args.port, dry_run=args.dry_run) as mf:
            n = mf.apply_patch(patch)

        base = f' on top of preset {patch["base_preset"]}' if patch.get('base_preset') else ''
        print(f"Patch '{patch['name']}' applied: {n} CCs sent{base}")
        steps = manual_steps(patch)
        if steps:
            print('\nSet these on the MiniFreak panel (no MIDI CC for them):')
            for s in steps:
                print(f'  - {s}')
        if not args.no_save and not args.dry_run:
            print(f'\nSaved copy: {save_patch(patch)}')
    except Exception as e:
        print(f'Error: {e}', file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
