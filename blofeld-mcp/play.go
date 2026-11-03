package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gitlab.com/gomidi/midi/v2"
)

func singleTest(blo *Blofeld) {

	if err := playTestNotes(blo); err != nil {
		log.Fatalf("failed to play test notes: %v", err)
	}

	var Bank = "H"
	var Program = 128

	p, devID, err := blo.RequestPatchDump(Bank, Program)
	if err != nil {
		log.Fatalf("failed to read patch: %v", err)
	}
	log.Println("Patch name", p.Name)
	log.Printf("Read patch from Bank %s, Program %d (device 0x%02X): %+v\n", Bank, Program, devID, p)

	asJson, err := json.MarshalIndent(&p, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal patch to JSON: %v", err)
	}
	if asJsonErr := json.Unmarshal(asJson, &p); asJsonErr != nil {
		log.Fatalf("JSON roundtrip failed: %v", asJsonErr)
	}

	p.Oscillators[0].Shape = p.Oscillators[0].Shape + 1

	log.Printf("Patch as JSON:\n%s\n", asJson)

	if err := blo.SendPatch(Bank, Program, p); err != nil {
		log.Fatalf("failed to send patch: %v", err)
	}

	if err := playTestNotes(blo); err != nil {
		log.Fatalf("failed to play test notes: %v", err)
	}

	log.Println("Reading again.")
	p2, devID, err := blo.RequestPatchDump(Bank, Program)
	if err != nil {
		log.Fatalf("failed to read patch: %v", err)
	}
	log.Println("Patch name", p2.Name)

	asJson2, err := json.MarshalIndent(&p2, "", "  ")
	if err != nil {
		log.Fatalf("failed to marshal patch to JSON: %v", err)
	}
	log.Printf("Patch as JSON after sending modified version:\n%s\n", asJson2)

	fmt.Println("Done.")
}

func playTestNotes(blo *Blofeld) error {
	notes := []uint8{midi.C(4), midi.E(4), midi.G(4)}
	for _, n := range notes {
		if err := blo.Send(midi.NoteOn(blo.Channel, n, 100)); err != nil {
			return fmt.Errorf("note on failed for %d: %w", n, err)
		}
		time.Sleep(200 * time.Millisecond)
		if err := blo.Send(midi.NoteOff(blo.Channel, n)); err != nil {
			return fmt.Errorf("note off failed for %d: %w", n, err)
		}
	}
	return nil
}

func playMinor7Chord(blo *Blofeld) error {
	root := midi.C(4)
	chord := []uint8{root, root + 3, root + 7, root + 10}

	for _, n := range chord {
		if err := blo.Send(midi.NoteOn(blo.Channel, n, 100)); err != nil {
			return fmt.Errorf("note on failed for %d: %w", n, err)
		}
	}

	time.Sleep(10000 * time.Millisecond)

	for _, n := range chord {
		if err := blo.Send(midi.NoteOff(blo.Channel, n)); err != nil {
			return fmt.Errorf("note off failed for %d: %w", n, err)
		}
	}

	return nil
}

func playNotesFromText(blo *Blofeld, notesText string) error {
	tokens := strings.FieldsFunc(notesText, func(r rune) bool {
		return unicode.IsSpace(r) || r == ',' || r == ';' || r == '|'
	})
	if len(tokens) == 0 {
		return fmt.Errorf("no notes provided")
	}

	for _, tok := range tokens {
		n, isRest, err := parseNoteToken(tok)
		if err != nil {
			return fmt.Errorf("invalid note %q: %w", tok, err)
		}

		if isRest {
			time.Sleep(360 * time.Millisecond)
			continue
		}

		if err := blo.Send(midi.NoteOn(blo.Channel, n, 100)); err != nil {
			return fmt.Errorf("note on failed for %d: %w", n, err)
		}
		time.Sleep(300 * time.Millisecond)
		if err := blo.Send(midi.NoteOff(blo.Channel, n)); err != nil {
			return fmt.Errorf("note off failed for %d: %w", n, err)
		}
		time.Sleep(60 * time.Millisecond)
	}

	return nil
}

func parseNoteToken(tok string) (uint8, bool, error) {
	t := strings.TrimSpace(tok)
	if t == "" {
		return 0, false, fmt.Errorf("empty token")
	}

	if strings.EqualFold(t, "r") || strings.EqualFold(t, "rest") {
		return 0, true, nil
	}

	if len(t) < 2 {
		return 0, false, fmt.Errorf("too short")
	}

	base := strings.ToUpper(string(t[0]))
	accidental := 0
	rest := t[1:]

	if len(rest) > 0 {
		switch rest[0] {
		case '#':
			accidental = 1
			rest = rest[1:]
		case 'b', 'B':
			accidental = -1
			rest = rest[1:]
		}
	}

	if rest == "" {
		return 0, false, fmt.Errorf("missing octave")
	}

	octave, err := strconv.Atoi(rest)
	if err != nil {
		return 0, false, fmt.Errorf("invalid octave: %w", err)
	}

	var semitone int
	switch base {
	case "C":
		semitone = 0
	case "D":
		semitone = 2
	case "E":
		semitone = 4
	case "F":
		semitone = 5
	case "G":
		semitone = 7
	case "A":
		semitone = 9
	case "B":
		semitone = 11
	default:
		return 0, false, fmt.Errorf("invalid note letter %q", base)
	}

	semitone += accidental
	n := 12*(octave+1) + semitone

	if n < 0 || n > 127 {
		return 0, false, fmt.Errorf("MIDI note out of range: %d", n)
	}

	return uint8(n), false, nil
}
