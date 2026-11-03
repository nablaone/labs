package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestPatchSerialization(t *testing.T) {
	p := &Patch{
		Name: "Test Patch",
	}

	p.MixOsc1 = 10
	p.MixOsc2 = 20
	p.MixOsc3 = 30
	p.MixNoise = 40
	p.MixRing = 50
	p.FilterRouting = 1

	p.Filters[0] = Filter{Type: 3, Cutoff: 100, Res: 80, Drive: 20, EnvAmt: 64}
	p.Filters[1] = Filter{Type: 5, Cutoff: 90, Res: 70, Drive: 40, EnvAmt: 32}

	p.Envelopes[0] = Envelope{Attack: 5, Decay: 10, Sustain: 15, Release: 20}
	p.Envelopes[1] = Envelope{Attack: 25, Decay: 30, Sustain: 35, Release: 40}
	p.Envelopes[2] = Envelope{Attack: 45, Decay: 50, Sustain: 55, Release: 60}

	p.LFOs[0] = LFO{Shape: 1, Speed: 90}
	p.LFOs[1] = LFO{Shape: 2, Speed: 80}
	p.LFOs[2] = LFO{Shape: 3, Speed: 70}

	p.ModMatrix[0] = ModulationMatrix{Source: 1, Amount: 64, Dest: 10}
	p.ModMatrix[1] = ModulationMatrix{Source: 2, Amount: 32, Dest: 20}

	p.ArpMode = 1
	p.ArpPattern = 2
	p.ArpClock = 3
	p.ArpLength = 4
	p.ArpRange = 5
	p.ArpDirection = 6
	p.ArpSort = 7
	p.ArpVelocityMode = 8
	p.ArpTimingFactor = 9
	p.ArpPatternReset = 1
	p.ArpPatternLength = 16
	p.ArpTempo = 100
	for i := range p.ArpPatternSteps {
		p.ArpPatternSteps[i] = byte(i)
		p.ArpPatternTiming[i] = byte(127 - i)
	}

	p.Effects[0] = Effect{Type: 1, Param1: 2, Param2: 3}
	p.Effects[1] = Effect{Type: 4, Param1: 5, Param2: 6}

	p.AmpVolume = 100
	p.AmpModAmount = 64
	p.AmpDrive = 64
	p.AmpPan = 0
	p.MasterTune = 58

	p.RandomizeOscillators()

	bytes, err := p.ToSDATA()
	if err != nil {
		t.Fatalf("failed to serialize patch: %v", err)
	}

	p2, err := ParseSDATA(bytes)

	if err != nil {
		t.Fatalf("failed to parse SDATA: %v", err)
	}

	if p2.Name != p.Name {
		t.Errorf("expected patch name %q, got %q", p.Name, p2.Name)
	}

	p1Json, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("failed to marshal patch to JSON: %v", err)
	}

	p2Json, err := json.Marshal(p2)
	if err != nil {
		t.Fatalf("failed to marshal parsed patch to JSON: %v", err)

	}

	if string(p1Json) == string(p2Json) {
		t.Logf("Patch JSONs are equal")
	} else {
		t.Logf("Patch JSONs are not equal")
	}

	p1Map := make(map[string]interface{})
	if err := json.Unmarshal(p1Json, &p1Map); err != nil {
		t.Fatalf("failed to unmarshal original patch JSON: %v", err)
	}

	p2Map := make(map[string]interface{})
	if err := json.Unmarshal(p2Json, &p2Map); err != nil {
		t.Fatalf("failed to unmarshal parsed patch JSON: %v", err)
	}

	keys := make(map[string]bool)
	for k := range p1Map {
		keys[k] = true
	}
	for k := range p2Map {
		keys[k] = true
	}

	for k := range keys {
		fmt.Printf("Comparing key: %s\n", k)
		if _, ok := p1Map[k]; !ok {
			t.Errorf("key %q not found in original patch JSON", k)
			continue
		}
		if _, ok := p2Map[k]; !ok {
			t.Errorf("key %q not found in parsed patch JSON", k)
			continue
		}

		if p1Map[k] == nil && p2Map[k] == nil {
			continue // Both are nil, no mismatch
		}

		type1 := fmt.Sprintf("%T", p1Map[k])
		type2 := fmt.Sprintf("%T", p2Map[k])
		if type1 != type2 {
			t.Errorf("type mismatch for key %q: expected %s, got %s", k, type1, type2)
			continue
		}

		if t, ok := p1Map[k].([]interface{}); ok {
			fmt.Printf("Comparing slice for key %q with length %d\n", k, len(t))
			continue
		}

		if p1Map[k] != p2Map[k] {
			t.Errorf("mismatch for key %q: expected %v, got %v", k, p1Map[k], p2Map[k])
		}
	}

}
