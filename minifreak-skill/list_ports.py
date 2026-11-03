#!/usr/bin/env python3
"""List MIDI ports and show which ones would be picked for the MiniFreak."""

import mido

from minifreak import find_minifreak_ports


def main():
    print('Inputs:')
    for n in mido.get_input_names():
        print(f'  {n}')
    print('Outputs:')
    for n in mido.get_output_names():
        print(f'  {n}')
    i, o = find_minifreak_ports()
    print(f'\nMiniFreak in:  {i or "(not found)"}')
    print(f'MiniFreak out: {o or "(not found)"}')


if __name__ == '__main__':
    main()
