#!/usr/bin/env python3
"""
Send Patch Tool - Send a patch to Waldorf Blofeld synthesizer.

Usage:
    python send_patch.py --bank A --program 1 --patch-json '{"name": "Test", ...}'
    echo '{"name": "Test", ...}' | python send_patch.py --bank A --program 1
"""

import argparse
import sys

from blofeld import Blofeld, patch_from_json


def main():
    parser = argparse.ArgumentParser(description='Send patch to Blofeld synthesizer')
    parser.add_argument('--bank', '-b', required=True, help='Bank (A-H)')
    parser.add_argument('--program', '-p', type=int, required=True, help='Program number (1-128)')
    parser.add_argument('--patch-json', '-j', help='Patch JSON (or read from stdin)')
    args = parser.parse_args()

    # Get patch JSON from argument or stdin
    if args.patch_json:
        patch_json = args.patch_json
    else:
        patch_json = sys.stdin.read()

    if not patch_json.strip():
        print("Error: No patch JSON provided", file=sys.stderr)
        sys.exit(1)

    try:
        patch = patch_from_json(patch_json)
        with Blofeld() as blo:
            blo.send_patch(args.bank, args.program, patch)
            print(f"Patch '{patch.name}' sent to bank {args.bank} program {args.program}")
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
