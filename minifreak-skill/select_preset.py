#!/usr/bin/env python3
"""
Select Preset Tool - load one of the MiniFreak's 512 presets.

Sends Bank Select (CC0) + Program Change. Requires
Utility > MIDI > Program Change = On.

Usage:
    python select_preset.py --preset 242
"""

import argparse
import sys

from minifreak import MiniFreak, preset_to_bank_program


def main():
    ap = argparse.ArgumentParser(description='Select MiniFreak preset')
    ap.add_argument('--preset', '-p', type=int, required=True, help='Preset number 1-512')
    ap.add_argument('--channel', '-c', type=int, help='MIDI channel 1-16')
    ap.add_argument('--port', help='MIDI port name substring')
    ap.add_argument('--dry-run', action='store_true')
    args = ap.parse_args()

    try:
        bank, program = preset_to_bank_program(args.preset)
        with MiniFreak(channel=args.channel, port=args.port, dry_run=args.dry_run) as mf:
            mf.select_preset(args.preset)
        print(f'Selected preset {args.preset} (bank select {bank}, program change {program})')
    except Exception as e:
        print(f'Error: {e}', file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
