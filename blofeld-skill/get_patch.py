#!/usr/bin/env python3
"""
Get Patch Tool - Retrieve a patch from Waldorf Blofeld synthesizer.

Usage:
    python get_patch.py --bank A --program 1
"""

import argparse
import sys

from blofeld import Blofeld, patch_to_json


def main():
    parser = argparse.ArgumentParser(description='Get patch from Blofeld synthesizer')
    parser.add_argument('--bank', '-b', required=True, help='Bank (A-H)')
    parser.add_argument('--program', '-p', type=int, required=True, help='Program number (1-128)')
    args = parser.parse_args()

    try:
        with Blofeld() as blo:
            patch = blo.request_patch(args.bank, args.program)
            print(patch_to_json(patch))
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
