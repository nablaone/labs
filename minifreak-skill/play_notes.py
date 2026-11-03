#!/usr/bin/env python3
"""
Play Notes Tool - Play notes on Arturia MiniFreak.

Usage:
    python play_notes.py --test           # Play C4, E4, G4 test notes
    python play_notes.py --minor7         # Play C minor 7 chord
    python play_notes.py --industrial     # Play dissonant cluster F3, F#3, B3
    python play_notes.py --notes "C4 E4 G4 Bb4"  # Play custom melody
"""

import argparse
import re
import sys
import time

from minifreak import MiniFreak


def parse_note(token: str) -> tuple[int, bool]:
    """
    Parse a note token into MIDI note number.
    Returns (note_number, is_rest).
    Supports: C4, D#5, Bb3, etc. and R/rest for rests.
    """
    t = token.strip()
    if not t:
        raise ValueError("Empty token")

    if t.lower() in ('r', 'rest'):
        return 0, True

    if len(t) < 2:
        raise ValueError(f"Too short: {t}")

    base = t[0].upper()
    accidental = 0
    rest = t[1:]

    if rest and rest[0] == '#':
        accidental = 1
        rest = rest[1:]
    elif rest and rest[0].lower() == 'b':
        accidental = -1
        rest = rest[1:]

    if not rest:
        raise ValueError(f"Missing octave: {t}")

    try:
        octave = int(rest)
    except ValueError:
        raise ValueError(f"Invalid octave: {rest}")

    semitones = {'C': 0, 'D': 2, 'E': 4, 'F': 5, 'G': 7, 'A': 9, 'B': 11}
    if base not in semitones:
        raise ValueError(f"Invalid note letter: {base}")

    semitone = semitones[base] + accidental
    note = 12 * (octave + 1) + semitone

    if note < 0 or note > 127:
        raise ValueError(f"MIDI note out of range: {note}")

    return note, False


def play_test_notes(mf: MiniFreak) -> None:
    """Play C4, E4, G4 test notes."""
    notes = [60, 64, 67]  # C4, E4, G4
    for n in notes:
        mf.note_on(n, 100)
        time.sleep(0.2)
        mf.note_off(n)


def play_minor7_chord(mf: MiniFreak) -> None:
    """Play C minor 7 chord for 10 seconds."""
    root = 60  # C4
    chord = [root, root + 3, root + 7, root + 10]  # C, Eb, G, Bb

    for n in chord:
        mf.note_on(n, 100)

    time.sleep(10.0)

    for n in chord:
        mf.note_off(n)


def play_industrial(mf: MiniFreak) -> None:
    """Play dissonant cluster F3, F#3, B3 for 10 seconds."""
    cluster = [53, 54, 59]  # F3, F#3, B3 — tritone + semitone cluster

    for n in cluster:
        mf.note_on(n, 110)

    time.sleep(10.0)

    for n in cluster:
        mf.note_off(n)


def play_notes_from_text(mf: MiniFreak, notes_text: str) -> None:
    """Play a melody from text notation."""
    tokens = re.split(r'[\s,;|]+', notes_text)
    tokens = [t for t in tokens if t]

    if not tokens:
        raise ValueError("No notes provided")

    for tok in tokens:
        note, is_rest = parse_note(tok)

        if is_rest:
            time.sleep(0.36)
            continue

        mf.note_on(note, 100)
        time.sleep(0.3)
        mf.note_off(note)
        time.sleep(0.06)


def main():
    parser = argparse.ArgumentParser(description='Play notes on MiniFreak')
    group = parser.add_mutually_exclusive_group(required=True)
    group.add_argument('--test', action='store_true', help='Play C4, E4, G4 test notes')
    group.add_argument('--minor7', action='store_true', help='Play C minor 7 chord')
    group.add_argument('--industrial', action='store_true', help='Play dissonant cluster F3, F#3, B3')
    group.add_argument('--notes', '-n', help='Notes to play (e.g., "C4 E4 G4")')
    parser.add_argument('--channel', '-c', type=int, help='MIDI channel 1-16 (default 1 / $MINIFREAK_CHANNEL)')
    parser.add_argument('--port', help='MIDI port name substring')
    parser.add_argument('--dry-run', action='store_true', help='Print messages instead of sending')
    args = parser.parse_args()

    try:
        with MiniFreak(channel=args.channel, port=args.port, dry_run=args.dry_run) as mf:
            if args.test:
                play_test_notes(mf)
                print("Played test notes: C4, E4, G4")
            elif args.minor7:
                play_minor7_chord(mf)
                print("Played C minor 7 chord")
            elif args.industrial:
                play_industrial(mf)
                print("Played industrial cluster: F3, F#3, B3")
            elif args.notes:
                play_notes_from_text(mf, args.notes)
                print(f"Played notes: {args.notes}")
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)


if __name__ == '__main__':
    main()
