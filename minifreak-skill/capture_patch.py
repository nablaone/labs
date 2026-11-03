#!/usr/bin/env python3
"""
Capture Patch Tool - record the knob values the MiniFreak sends as CCs.

The MiniFreak can't dump a preset over MIDI, so this is the closest thing to
"get patch": it listens while you turn knobs and records the last value of
each known CC. Knobs you don't touch are not captured.

Requires Utility > MIDI > Knob Send CC = On.

Usage:
    python capture_patch.py --seconds 30 --name "My Bass"
    python capture_patch.py --seconds 30 --merge patches/xyz.json
"""

import argparse
import json
import sys

from minifreak import CC_TO_NAME, MiniFreak, save_patch, validate_patch


def main():
    ap = argparse.ArgumentParser(description='Capture knob CCs from MiniFreak')
    ap.add_argument('--seconds', '-s', type=float, default=30.0, help='How long to listen (default 30)')
    ap.add_argument('--name', '-n', default='Captured', help='Patch name')
    ap.add_argument('--merge', help='Existing patch JSON to update with captured values')
    ap.add_argument('--channel', '-c', type=int, help='Only accept this MIDI channel 1-16 (default: any)')
    ap.add_argument('--port', help='MIDI port name substring')
    args = ap.parse_args()

    try:
        patch = json.load(open(args.merge)) if args.merge else {'name': args.name, 'params': {}, 'manual': {}}
        patch = validate_patch(patch)
        if not args.merge:
            patch['name'] = args.name

        captured = {}
        print(f'Listening for {args.seconds:.0f}s — turn knobs on the MiniFreak...', file=sys.stderr)
        with MiniFreak(port=args.port, need_input=True, need_output=False) as mf:
            for msg in mf.iter_incoming(args.seconds):
                if msg.type != 'control_change':
                    continue
                if args.channel and msg.channel != args.channel - 1:
                    continue
                name = CC_TO_NAME.get(msg.control)
                if name and name != 'sustain':
                    captured[name] = msg.value

        if not captured:
            print('No knob CCs received. Is Utility > MIDI > Knob Send CC set to On?', file=sys.stderr)
            sys.exit(2)

        patch['params'].update(captured)
        print(json.dumps(patch, indent=2))
        print(f'Captured {len(captured)} params: {", ".join(sorted(captured))}', file=sys.stderr)
        print(f'Saved: {save_patch(patch)}', file=sys.stderr)
    except Exception as e:
        print(f'Error: {e}', file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
