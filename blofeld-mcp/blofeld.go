package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gitlab.com/gomidi/midi/v2"
	"gitlab.com/gomidi/midi/v2/drivers"
)

const PatchSize = 383 // Expected SDATA payload size for a Blofeld patch

func dumpBytes(data []byte, file string) {

	/*
		date := time.Now().Format("20060102150405")

		f, err := os.Create("dump/" + date + "_" + file)
		if err != nil {
			log.Fatalf("failed to create dump file %s: %v", file, err)
		}
		defer f.Close()
	*/
	f := os.Stderr

	fmt.Fprintf(f, "Dumping %d bytes to %s:\n", len(data), file)

	for i, b := range data {
		fmt.Fprintf(f, "%d 0x%02X\n", i, b)
	}
}

type Oscillator struct {
	Octave     byte `json:"octave"`
	Pitch      byte `json:"pitch"` // semitone
	BendRange  byte `json:"bend_range"`
	Keytrack   byte `json:"keytrack"`
	Detune     byte `json:"detune"`
	Shape      byte `json:"shape"`
	PW         byte `json:"pw"`
	PWM        byte `json:"pwm"`
	PWMSource  byte `json:"pwm_source"`
	FM         byte `json:"fm"`
	FMSource   byte `json:"fm_source"`
	LimitWT    byte `json:"limit_wt"`
	Brilliance byte `json:"brilliance"`
}

type Filter struct {
	Type       byte `json:"type"`
	Cutoff     byte `json:"cutoff"`
	Res        byte `json:"res"`
	Drive      byte `json:"drive"`
	DriveCurve byte `json:"drive_curve"`
	EnvAmt     byte `json:"env_amt"`
	EnvVel     byte `json:"env_vel"`
	Keytrack   byte `json:"keytrack"`
	ModSource  byte `json:"mod_source"`
	ModAmount  byte `json:"mod_amount"`
	FMSource   byte `json:"fm_source"`
	FMAmount   byte `json:"fm_amount"`
	Pan        byte `json:"pan"`
	PanSource  byte `json:"pan_source"`
	PanAmount  byte `json:"pan_amount"`
}

type Envelope struct {
	Mode        byte `json:"mode"`
	Attack      byte `json:"attack"`
	AttackLevel byte `json:"attack_level"`
	Decay       byte `json:"decay"`
	Sustain     byte `json:"sustain"`
	Decay2      byte `json:"decay2"`
	Sustain2    byte `json:"sustain2"`
	Release     byte `json:"release"`
}

type LFO struct {
	Shape      byte `json:"shape"`
	Speed      byte `json:"speed"`
	Sync       byte `json:"sync"`
	Clocked    byte `json:"clocked"`
	StartPhase byte `json:"start_phase"`
	Delay      byte `json:"delay"`
	Fade       byte `json:"fade"`
	Keytrack   byte `json:"keytrack"`
}

type Effect struct {
	Type   byte     `json:"type"`
	Mix    byte     `json:"mix"`
	Param1 byte     `json:"param1"`
	Param2 byte     `json:"param2"`
	Params [14]byte `json:"params"`
}

type ModulationMatrix struct {
	Source byte `json:"source"`
	Amount byte `json:"amount"`
	Dest   byte `json:"dest"`
}

type Modifier struct {
	SourceA  byte `json:"source_a"`
	SourceB  byte `json:"source_b"`
	Operator byte `json:"operator"`
	Constant byte `json:"constant"`
}

type Patch struct {
	Oscillators    [3]Oscillator `json:"oscillators"`
	Osc2Sync       byte          `json:"osc2_sync"`
	OscPitchSource byte          `json:"osc_pitch_source"`
	OscPitchAmount byte          `json:"osc_pitch_amount"`

	Filters [2]Filter `json:"filters"`

	// Mixer
	MixOsc1         byte `json:"mix_osc1"`
	MixOsc1Balance  byte `json:"mix_osc1_balance"`
	MixOsc2         byte `json:"mix_osc2"`
	MixOsc2Balance  byte `json:"mix_osc2_balance"`
	MixOsc3         byte `json:"mix_osc3"`
	MixOsc3Balance  byte `json:"mix_osc3_balance"`
	MixNoise        byte `json:"mix_noise"`
	MixNoiseBalance byte `json:"mix_noise_balance"`
	MixNoiseColor   byte `json:"mix_noise_color"`
	MixRing         byte `json:"mix_ring"`
	MixRingBalance  byte `json:"mix_ring_balance"`

	FilterRouting byte `json:"filter_routing"`
	GlideMode     byte `json:"glide_mode"`
	GlideRate     byte `json:"glide_rate"`
	Unison        byte `json:"unison"`
	UnisonDetune  byte `json:"unison_detune"`

	Envelopes [3]Envelope `json:"envelopes"`

	LFOs [3]LFO `json:"lfos"`

	// Mod Matrix (16 slots × (source, amount, dest))
	ModMatrix [16]ModulationMatrix `json:"mod_matrix"`
	Modifiers [4]Modifier          `json:"modifiers"`

	// Arpeggiator section
	ArpMode          byte     `json:"arp_mode"`
	ArpPattern       byte     `json:"arp_pattern"`
	ArpClock         byte     `json:"arp_clock"`
	ArpLength        byte     `json:"arp_length"`
	ArpRange         byte     `json:"arp_range"`
	ArpDirection     byte     `json:"arp_direction"`
	ArpSort          byte     `json:"arp_sort"`
	ArpVelocityMode  byte     `json:"arp_velocity_mode"`
	ArpTimingFactor  byte     `json:"arp_timing_factor"`
	ArpPatternReset  byte     `json:"arp_pattern_reset"`
	ArpPatternLength byte     `json:"arp_pattern_length"`
	ArpTempo         byte     `json:"arp_tempo"`
	ArpPatternSteps  [16]byte `json:"arp_pattern_steps"`
	ArpPatternTiming [16]byte `json:"arp_pattern_timing"`

	// FX
	Effects [2]Effect `json:"effects"`

	// Amp
	AmpVolume    byte `json:"amp_volume"`
	AmpVelocity  byte `json:"amp_velocity"`
	AmpModSource byte `json:"amp_mod_source"`
	AmpModAmount byte `json:"amp_mod_amount"`
	AmpPan       byte `json:"amp_pan"`
	AmpDrive     byte `json:"amp_drive"`

	// Master tuning + globals
	MasterTune byte `json:"master_tune"`

	// Patch name – 16 ASCII chars (363–378)
	Name string `json:"name"`

	// Category + Subcategory
	Category    byte `json:"category"`
	SubCategory byte `json:"subcategory"`
}

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

// Index mappings into the 383-byte SDATA payload (see Blofeld spec 3.1).
var oscFieldMapping = []struct {
	octave     int
	pitch      int
	bendRange  int
	keytrack   int
	detune     int
	fmSource   int
	fm         int
	shape      int
	pw         int
	pwmSource  int
	pwm        int
	limitWT    int
	brilliance int
}{
	{octave: 1, pitch: 2, bendRange: 4, keytrack: 5, detune: 3, fmSource: 6, fm: 7, shape: 8, pw: 9, pwmSource: 10, pwm: 11, limitWT: 14, brilliance: 16},
	{octave: 17, pitch: 18, bendRange: 20, keytrack: 21, detune: 19, fmSource: 22, fm: 23, shape: 24, pw: 25, pwmSource: 26, pwm: 27, limitWT: 30, brilliance: 32},
	{octave: 33, pitch: 34, bendRange: 36, keytrack: 37, detune: 35, fmSource: 38, fm: 39, shape: 40, pw: 41, pwmSource: 42, pwm: 43, limitWT: -1, brilliance: 48},
}

var filterFieldMapping = []struct {
	typ        int
	cutoff     int
	res        int
	drive      int
	driveCurve int
	envAmt     int
	envVel     int
	keytrack   int
	modSource  int
	modAmount  int
	fmSource   int
	fmAmount   int
	pan        int
	panSource  int
	panAmount  int
}{
	{typ: 77, cutoff: 78, res: 80, drive: 81, driveCurve: 82, envAmt: 87, envVel: 88, keytrack: 86, modSource: 89, modAmount: 90, fmSource: 91, fmAmount: 92, pan: 93, panSource: 94, panAmount: 95},
	{typ: 97, cutoff: 98, res: 100, drive: 101, driveCurve: 102, envAmt: 107, envVel: 108, keytrack: 106, modSource: 109, modAmount: 110, fmSource: 111, fmAmount: 112, pan: 113, panSource: 114, panAmount: 115},
}

var envelopeFieldMapping = []struct {
	mode        int
	attack      int
	attackLevel int
	decay       int
	sustain     int
	decay2      int
	sustain2    int
	release     int
}{
	{mode: 196, attack: 199, attackLevel: 200, decay: 201, sustain: 202, decay2: 203, sustain2: 204, release: 205}, // Filter envelope
	{mode: 208, attack: 211, attackLevel: 212, decay: 213, sustain: 214, decay2: 215, sustain2: 216, release: 217}, // Amp envelope
	{mode: 220, attack: 223, attackLevel: 224, decay: 225, sustain: 226, decay2: 227, sustain2: 228, release: 229}, // Envelope 3
}

var lfoFieldMapping = []struct {
	shape      int
	speed      int
	sync       int
	clocked    int
	startPhase int
	delay      int
	fade       int
	keytrack   int
}{
	{shape: 160, speed: 161, sync: 163, clocked: 164, startPhase: 165, delay: 166, fade: 167, keytrack: 170},
	{shape: 172, speed: 173, sync: 175, clocked: 176, startPhase: 177, delay: 178, fade: 179, keytrack: 182},
	{shape: 184, speed: 185, sync: 187, clocked: 188, startPhase: 189, delay: 190, fade: 191, keytrack: 194},
}

var effectFieldMapping = []struct {
	typ         int
	mix         int
	param1      int
	param2      int
	paramsStart int
}{
	{typ: 128, mix: 129, param1: 130, param2: 131, paramsStart: 130},
	{typ: 144, mix: 145, param1: 146, param2: 147, paramsStart: 146},
}

const (
	oscSyncIdx        = 49
	oscPitchSourceIdx = 50
	oscPitchAmountIdx = 51
	glideModeIdx      = 56
	glideRateIdx      = 57
	unisonIdx         = 58
	unisonDetuneIdx   = 59

	mixOsc1Idx       = 61
	mixOsc1BalIdx    = 62
	mixOsc2Idx       = 63
	mixOsc2BalIdx    = 64
	mixOsc3Idx       = 65
	mixOsc3BalIdx    = 66
	mixNoiseIdx      = 67
	mixNoiseBalIdx   = 68
	mixNoiseColorIdx = 69
	mixRingIdx       = 71
	mixRingBalIdx    = 72

	filterRoutingIdx = 117

	ampVolumeIdx    = 121
	ampVelocityIdx  = 122
	ampModSourceIdx = 123
	ampModAmountIdx = 124

	masterTuneIdx = 52 // Reserved slot; kept to preserve round-tripping of the struct field

	modMatrixStartIdx = 261
	modMatrixStride   = 3
	modifierStartIdx  = 245
	modifierStride    = 4

	arpModeIdx               = 311
	arpPatternIdx            = 312
	arpClockIdx              = 314
	arpLengthIdx             = 315
	arpRangeIdx              = 316
	arpDirectionIdx          = 317
	arpSortIdx               = 318
	arpVelocityModeIdx       = 319
	arpTimingFactorIdx       = 320
	arpPatternResetIdx       = 322
	arpPatternLengthIdx      = 323
	arpTempoIdx              = 326
	arpPatternStepsStartIdx  = 327
	arpPatternTimingStartIdx = 343
)

// RandomizeOscillators mutates only oscillator-related parameters using a PRNG.
func (p *Patch) RandomizeOscillators() {

	randByte := func() byte {
		return byte(random.Intn(128)) // constrain to 0–127 MIDI range
	}

	// Limit shape to the first 5 oscillator types (0–4), avoiding custom samples/wavetables.
	randShape := func() byte {
		return byte(random.Intn(5))
	}

	for i := range p.Oscillators {
		p.Oscillators[i].Shape = randShape()
		p.Oscillators[i].Octave = randByte()
		p.Oscillators[i].Pitch = randByte()
		p.Oscillators[i].BendRange = randByte()
		p.Oscillators[i].Keytrack = randByte()
		p.Oscillators[i].Detune = randByte()
		p.Oscillators[i].PW = randByte()
		p.Oscillators[i].PWM = randByte()
		p.Oscillators[i].PWMSource = randByte()
		p.Oscillators[i].FM = randByte()
		p.Oscillators[i].FMSource = randByte()
		p.Oscillators[i].LimitWT = randByte()
		p.Oscillators[i].Brilliance = randByte()
	}
}

func ParseSDATA(data []byte) (*Patch, error) {
	if len(data) != PatchSize {
		return nil, errors.New("invalid SDATA length")
	}

	p := &Patch{}

	// Map oscillator fields per Blofeld spec indexes (3.1 SDATA table).
	for i, m := range oscFieldMapping {
		if i >= len(p.Oscillators) {
			break
		}
		if m.octave >= 0 {
			p.Oscillators[i].Octave = data[m.octave]
		}
		p.Oscillators[i].Pitch = data[m.pitch]
		if m.bendRange >= 0 {
			p.Oscillators[i].BendRange = data[m.bendRange]
		}
		if m.keytrack >= 0 {
			p.Oscillators[i].Keytrack = data[m.keytrack]
		}
		p.Oscillators[i].Detune = data[m.detune]
		p.Oscillators[i].FMSource = data[m.fmSource]
		p.Oscillators[i].FM = data[m.fm]
		p.Oscillators[i].Shape = data[m.shape]
		p.Oscillators[i].PW = data[m.pw]
		if m.pwmSource >= 0 {
			p.Oscillators[i].PWMSource = data[m.pwmSource]
		}
		p.Oscillators[i].PWM = data[m.pwm]
		if m.limitWT >= 0 {
			p.Oscillators[i].LimitWT = data[m.limitWT]
		}
		if m.brilliance >= 0 {
			p.Oscillators[i].Brilliance = data[m.brilliance]
		}
	}

	p.Osc2Sync = data[oscSyncIdx]
	p.OscPitchSource = data[oscPitchSourceIdx]
	p.OscPitchAmount = data[oscPitchAmountIdx]

	// Mixer levels
	p.MixOsc1 = data[mixOsc1Idx]
	p.MixOsc1Balance = data[mixOsc1BalIdx]
	p.MixOsc2 = data[mixOsc2Idx]
	p.MixOsc2Balance = data[mixOsc2BalIdx]
	p.MixOsc3 = data[mixOsc3Idx]
	p.MixOsc3Balance = data[mixOsc3BalIdx]
	p.MixNoise = data[mixNoiseIdx]
	p.MixNoiseBalance = data[mixNoiseBalIdx]
	p.MixNoiseColor = data[mixNoiseColorIdx]
	p.MixRing = data[mixRingIdx]
	p.MixRingBalance = data[mixRingBalIdx]

	// Filters
	for i, m := range filterFieldMapping {
		if i >= len(p.Filters) {
			break
		}
		p.Filters[i].Type = data[m.typ]
		p.Filters[i].Cutoff = data[m.cutoff]
		p.Filters[i].Res = data[m.res]
		p.Filters[i].Drive = data[m.drive]
		p.Filters[i].DriveCurve = data[m.driveCurve]
		p.Filters[i].EnvAmt = data[m.envAmt]
		p.Filters[i].EnvVel = data[m.envVel]
		p.Filters[i].Keytrack = data[m.keytrack]
		p.Filters[i].ModSource = data[m.modSource]
		p.Filters[i].ModAmount = data[m.modAmount]
		p.Filters[i].FMSource = data[m.fmSource]
		p.Filters[i].FMAmount = data[m.fmAmount]
		p.Filters[i].Pan = data[m.pan]
		p.Filters[i].PanSource = data[m.panSource]
		p.Filters[i].PanAmount = data[m.panAmount]
	}

	p.FilterRouting = data[filterRoutingIdx]
	p.GlideMode = data[glideModeIdx]
	p.GlideRate = data[glideRateIdx]
	p.Unison = data[unisonIdx]
	p.UnisonDetune = data[unisonDetuneIdx]

	// Envelopes
	for i, m := range envelopeFieldMapping {
		if i >= len(p.Envelopes) {
			break
		}
		p.Envelopes[i].Mode = data[m.mode]
		p.Envelopes[i].Attack = data[m.attack]
		p.Envelopes[i].AttackLevel = data[m.attackLevel]
		p.Envelopes[i].Decay = data[m.decay]
		p.Envelopes[i].Sustain = data[m.sustain]
		p.Envelopes[i].Decay2 = data[m.decay2]
		p.Envelopes[i].Sustain2 = data[m.sustain2]
		p.Envelopes[i].Release = data[m.release]
	}

	// LFOs
	for i, m := range lfoFieldMapping {
		if i >= len(p.LFOs) {
			break
		}
		p.LFOs[i].Shape = data[m.shape]
		p.LFOs[i].Speed = data[m.speed]
		p.LFOs[i].Sync = data[m.sync]
		p.LFOs[i].Clocked = data[m.clocked]
		p.LFOs[i].StartPhase = data[m.startPhase]
		p.LFOs[i].Delay = data[m.delay]
		p.LFOs[i].Fade = data[m.fade]
		p.LFOs[i].Keytrack = data[m.keytrack]
	}

	// Effects
	for i, m := range effectFieldMapping {
		if i >= len(p.Effects) {
			break
		}
		p.Effects[i].Type = data[m.typ]
		p.Effects[i].Mix = data[m.mix]
		p.Effects[i].Param1 = data[m.param1]
		p.Effects[i].Param2 = data[m.param2]
		// Params 1..14
		for j := 0; j < len(p.Effects[i].Params); j++ {
			idx := m.paramsStart + j
			if idx >= len(data) {
				break
			}
			p.Effects[i].Params[j] = data[idx]
		}
	}

	// Modulation matrix
	for i := 0; i < len(p.ModMatrix); i++ {
		base := modMatrixStartIdx + i*modMatrixStride
		if base+2 >= len(data) {
			break
		}
		p.ModMatrix[i].Source = data[base]
		p.ModMatrix[i].Dest = data[base+1]
		p.ModMatrix[i].Amount = data[base+2]
	}

	// Modifiers
	for i := 0; i < len(p.Modifiers); i++ {
		base := modifierStartIdx + i*modifierStride
		if base+3 >= len(data) {
			break
		}
		p.Modifiers[i].SourceA = data[base]
		p.Modifiers[i].SourceB = data[base+1]
		p.Modifiers[i].Operator = data[base+2]
		p.Modifiers[i].Constant = data[base+3]
	}

	// Arpeggiator block
	p.ArpMode = data[arpModeIdx]
	p.ArpPattern = data[arpPatternIdx]
	p.ArpClock = data[arpClockIdx]
	p.ArpLength = data[arpLengthIdx]
	p.ArpRange = data[arpRangeIdx]
	p.ArpDirection = data[arpDirectionIdx]
	p.ArpSort = data[arpSortIdx]
	p.ArpVelocityMode = data[arpVelocityModeIdx]
	p.ArpTimingFactor = data[arpTimingFactorIdx]
	p.ArpPatternReset = data[arpPatternResetIdx]
	p.ArpPatternLength = data[arpPatternLengthIdx]
	p.ArpTempo = data[arpTempoIdx]
	for i := 0; i < len(p.ArpPatternSteps); i++ {
		idx := arpPatternStepsStartIdx + i
		if idx < len(data) {
			p.ArpPatternSteps[i] = data[idx]
		}
	}
	for i := 0; i < len(p.ArpPatternTiming); i++ {
		idx := arpPatternTimingStartIdx + i
		if idx < len(data) {
			p.ArpPatternTiming[i] = data[idx]
		}
	}

	// Keep some metadata handy.
	name := make([]byte, 16)
	copy(name, data[363:363+16])
	p.Name = string(bytes.TrimRight(name, "\x00"))
	p.Category = data[379]
	p.SubCategory = data[380]

	// Amp and misc fields
	p.AmpVolume = data[ampVolumeIdx]
	p.AmpVelocity = data[ampVelocityIdx]
	p.AmpModSource = data[ampModSourceIdx]
	p.AmpModAmount = data[ampModAmountIdx]
	p.AmpDrive = p.AmpModAmount
	p.MasterTune = data[masterTuneIdx]

	return p, nil
}

func (p *Patch) ToSDATA() ([]byte, error) {
	data := make([]byte, PatchSize)

	// Map oscillator fields back to SDATA indexes (3.1 SDATA table).
	for i, m := range oscFieldMapping {
		if i >= len(p.Oscillators) {
			break
		}
		osc := p.Oscillators[i]
		if m.octave >= 0 {
			data[m.octave] = osc.Octave
		}
		data[m.pitch] = osc.Pitch
		if m.bendRange >= 0 {
			data[m.bendRange] = osc.BendRange
		}
		if m.keytrack >= 0 {
			data[m.keytrack] = osc.Keytrack
		}
		data[m.detune] = osc.Detune
		data[m.fmSource] = osc.FMSource
		data[m.fm] = osc.FM
		data[m.shape] = osc.Shape
		data[m.pw] = osc.PW
		if m.pwmSource >= 0 {
			data[m.pwmSource] = osc.PWMSource
		}
		data[m.pwm] = osc.PWM
		if m.limitWT >= 0 {
			data[m.limitWT] = osc.LimitWT
		}
		if m.brilliance >= 0 {
			data[m.brilliance] = osc.Brilliance
		}
	}

	data[oscSyncIdx] = p.Osc2Sync
	data[oscPitchSourceIdx] = p.OscPitchSource
	data[oscPitchAmountIdx] = p.OscPitchAmount

	// Mixer levels
	data[mixOsc1Idx] = p.MixOsc1
	data[mixOsc1BalIdx] = p.MixOsc1Balance
	data[mixOsc2Idx] = p.MixOsc2
	data[mixOsc2BalIdx] = p.MixOsc2Balance
	data[mixOsc3Idx] = p.MixOsc3
	data[mixOsc3BalIdx] = p.MixOsc3Balance
	data[mixNoiseIdx] = p.MixNoise
	data[mixNoiseBalIdx] = p.MixNoiseBalance
	data[mixNoiseColorIdx] = p.MixNoiseColor
	data[mixRingIdx] = p.MixRing
	data[mixRingBalIdx] = p.MixRingBalance

	// Filters
	for i, m := range filterFieldMapping {
		if i >= len(p.Filters) {
			break
		}
		f := p.Filters[i]
		data[m.typ] = f.Type
		data[m.cutoff] = f.Cutoff
		data[m.res] = f.Res
		data[m.drive] = f.Drive
		data[m.driveCurve] = f.DriveCurve
		data[m.envAmt] = f.EnvAmt
		data[m.envVel] = f.EnvVel
		data[m.keytrack] = f.Keytrack
		data[m.modSource] = f.ModSource
		data[m.modAmount] = f.ModAmount
		data[m.fmSource] = f.FMSource
		data[m.fmAmount] = f.FMAmount
		data[m.pan] = f.Pan
		data[m.panSource] = f.PanSource
		data[m.panAmount] = f.PanAmount
	}

	data[filterRoutingIdx] = p.FilterRouting
	data[glideModeIdx] = p.GlideMode
	data[glideRateIdx] = p.GlideRate
	data[unisonIdx] = p.Unison
	data[unisonDetuneIdx] = p.UnisonDetune

	// Envelopes
	for i, m := range envelopeFieldMapping {
		if i >= len(p.Envelopes) {
			break
		}
		env := p.Envelopes[i]
		data[m.mode] = env.Mode
		data[m.attack] = env.Attack
		data[m.attackLevel] = env.AttackLevel
		data[m.decay] = env.Decay
		data[m.sustain] = env.Sustain
		data[m.decay2] = env.Decay2
		data[m.sustain2] = env.Sustain2
		data[m.release] = env.Release
	}

	// LFOs
	for i, m := range lfoFieldMapping {
		if i >= len(p.LFOs) {
			break
		}
		lfo := p.LFOs[i]
		data[m.shape] = lfo.Shape
		data[m.speed] = lfo.Speed
		data[m.sync] = lfo.Sync
		data[m.clocked] = lfo.Clocked
		data[m.startPhase] = lfo.StartPhase
		data[m.delay] = lfo.Delay
		data[m.fade] = lfo.Fade
		data[m.keytrack] = lfo.Keytrack
	}

	// Effects
	for i, m := range effectFieldMapping {
		if i >= len(p.Effects) {
			break
		}
		eff := p.Effects[i]
		data[m.typ] = eff.Type
		data[m.mix] = eff.Mix
		data[m.param1] = eff.Param1
		data[m.param2] = eff.Param2
		for j := 0; j < len(eff.Params); j++ {
			idx := m.paramsStart + j
			if idx >= len(data) {
				break
			}
			data[idx] = eff.Params[j]
		}
	}

	// Modulation matrix
	for i, mod := range p.ModMatrix {
		base := modMatrixStartIdx + i*modMatrixStride
		if base+2 >= len(data) {
			break
		}
		data[base] = mod.Source
		data[base+1] = mod.Dest
		data[base+2] = mod.Amount
	}

	// Modifiers
	for i, mod := range p.Modifiers {
		base := modifierStartIdx + i*modifierStride
		if base+3 >= len(data) {
			break
		}
		data[base] = mod.SourceA
		data[base+1] = mod.SourceB
		data[base+2] = mod.Operator
		data[base+3] = mod.Constant
	}

	// Arpeggiator block
	data[arpModeIdx] = p.ArpMode
	data[arpPatternIdx] = p.ArpPattern
	data[arpClockIdx] = p.ArpClock
	data[arpLengthIdx] = p.ArpLength
	data[arpRangeIdx] = p.ArpRange
	data[arpDirectionIdx] = p.ArpDirection
	data[arpSortIdx] = p.ArpSort
	data[arpVelocityModeIdx] = p.ArpVelocityMode
	data[arpTimingFactorIdx] = p.ArpTimingFactor
	data[arpPatternResetIdx] = p.ArpPatternReset
	data[arpPatternLengthIdx] = p.ArpPatternLength
	data[arpTempoIdx] = p.ArpTempo
	for i := 0; i < len(p.ArpPatternSteps); i++ {
		idx := arpPatternStepsStartIdx + i
		if idx < len(data) {
			data[idx] = p.ArpPatternSteps[i]
		}
	}
	for i := 0; i < len(p.ArpPatternTiming); i++ {
		idx := arpPatternTimingStartIdx + i
		if idx < len(data) {
			data[idx] = p.ArpPatternTiming[i]
		}
	}

	// Patch name and metadata preserved/overwritten as before.
	if p.Name != "" {
		name := []byte(p.Name)
		if len(name) > 16 {
			name = name[:16]
		}
		for len(name) < 16 {
			name = append(name, 0)
		}
		copy(data[363:363+16], name)
	}

	if p.Category != 0 {
		data[379] = p.Category
	}
	if p.SubCategory != 0 {
		data[380] = p.SubCategory
	}

	// Amp and misc fields
	data[ampVolumeIdx] = p.AmpVolume
	data[ampVelocityIdx] = p.AmpVelocity
	data[ampModSourceIdx] = p.AmpModSource
	data[ampModAmountIdx] = p.AmpModAmount
	data[masterTuneIdx] = p.MasterTune

	return data, nil
}

func (p *Patch) ToSNDD(deviceID byte, bank byte, program byte) ([]byte, error) {
	sdata, err := p.ToSDATA()

	dumpBytes(sdata, "sent_sdata.txt")

	if err != nil {
		return nil, err
	}

	out := []byte{0xF0, 0x3E, 0x13, deviceID, 0x10, bank, program}
	out = append(out, sdata...)

	var chk byte
	for _, b := range sdata {
		chk = (chk + b) & 0x7F
	}
	out = append(out, chk, 0xF7)

	dumpBytes(out, "sent_sndd.txt")

	return out, nil
}

type Blofeld struct {
	devID     byte
	out       drivers.Out
	opened    bool
	inPortIdx int
	portIdx   int
	Channel   uint8
}

func NewBlofeld() *Blofeld {
	return &Blofeld{
		Channel: 4, // Default to channel 5 (0-based value 4)
	}
}

func (b *Blofeld) ensureOpened() error {
	const (

		// Blofeld SysEx device ID often matches the MIDI channel (5 -> 0x04).
		blofeldDeviceID byte = 0x00
		nameHint             = "blofeld"
	)

	if !b.opened {
		portIdx, err := findOutPort(nameHint)
		if err != nil {
			log.Fatalf("could not find Blofeld MIDI out port: %v", err)
		}

		inPortIdx, err := findInPort(nameHint)
		if err != nil {
			log.Fatalf("could not find Blofeld MIDI in port: %v", err)
		}

		if err := b.open(blofeldDeviceID, portIdx); err != nil {
			return fmt.Errorf("failed to open Blofeld MIDI out port: %w", err)
		}

		b.inPortIdx = inPortIdx
		b.portIdx = portIdx
	}
	return nil
}

func findOutPort(nameFragment string) (int, error) {
	outs := midi.GetOutPorts()
	if len(outs) == 0 {
		return -1, fmt.Errorf("no MIDI outputs available")
	}

	lower := strings.ToLower(nameFragment)
	for _, out := range outs {
		if strings.Contains(strings.ToLower(out.String()), lower) {
			return out.Number(), nil
		}
	}

	return -1, fmt.Errorf("no MIDI output contains %q", nameFragment)
}

func findInPort(nameFragment string) (int, error) {
	ins := midi.GetInPorts()
	if len(ins) == 0 {
		return -1, fmt.Errorf("no MIDI inputs available")
	}

	lower := strings.ToLower(nameFragment)
	for _, in := range ins {
		if strings.Contains(strings.ToLower(in.String()), lower) {
			return in.Number(), nil
		}
	}

	return -1, fmt.Errorf("no MIDI input contains %q", nameFragment)
}

func (b *Blofeld) Close() {
	if b.opened {
		_ = b.out.Close()
		drivers.Close()
		b.opened = false
		log.Println("Closed Blofeld MIDI output")
	}
}

func (b *Blofeld) open(devID byte, portIndex int) error {
	outs, err := drivers.Outs()
	if err != nil {
		return err
	}

	if portIndex < 0 || portIndex >= len(outs) {
		return fmt.Errorf("output port index %d out of range", portIndex)
	}

	out := outs[portIndex]
	if err := out.Open(); err != nil {
		return err
	}

	log.Println("Opened Blofeld MIDI output port", devID, out.String())

	b.devID = devID
	b.out = out
	b.opened = true
	return nil
}

// Send transmits a MIDI message to the Blofeld output port.
func (b *Blofeld) Send(msg midi.Message) error {

	if err := b.ensureOpened(); err != nil {
		return err
	}

	return b.out.Send(msg.Bytes())
}

// SendSysEx transmits a raw SysEx payload.
func (b *Blofeld) SendSysEx(data []byte) error {
	return b.Send(midi.Message(data))
}

// RequestPatchDump asks Blofeld for a single program and waits for SNDD.
func (b *Blofeld) RequestPatchDump(bank string, program int) (*Patch, byte, error) {

	if err := b.ensureOpened(); err != nil {
		return nil, 0, err
	}

	inPort := midi.GetInPorts()[b.inPortIdx]

	bankByte, err := bankToByte(bank)
	if err != nil {
		return nil, 0, err
	}

	if program < 1 || program > 128 {
		return nil, 0, fmt.Errorf("program must be in range 1–128, got %d", program)
	}
	progByte := byte(program - 1) // Blofeld expects 0–127

	msgCh := make(chan midi.Message, 1)

	stop, err := midi.ListenTo(inPort, func(msg midi.Message, _ int32) {
		if len(msg) > 0 && msg[0] == 0xF0 {
			select {
			case msgCh <- msg:
			default:
			}
		}
	}, midi.UseSysEx(), midi.SysExBufferSize(2048))
	if err != nil {
		return nil, 0, fmt.Errorf("failed to listen for patch dump: %w", err)
	}
	defer stop()

	log.Printf("Requesting patch dump from device ID 0x%02X", b.devID)
	req := []byte{0xF0, 0x3E, 0x13, b.devID, 0x00, bankByte, progByte, 0xF7}
	log.Println("Sending SysEx request")
	if err := b.SendSysEx(req); err != nil {
		return nil, 0, fmt.Errorf("failed to request patch dump: %w", err)
	}

	select {
	case msg := <-msgCh:
		log.Println("Received SysEx message")
		patch, respDevID, parseErr := parseSNDD(msg)
		if parseErr != nil {
			return nil, 0, parseErr
		}
		return patch, respDevID, nil
	case <-time.After(5 * time.Second):
		log.Println("Timed out waiting for patch dump")
	}

	return nil, 0, errors.New("timed out waiting for patch dump")
}

func parseSNDD(msg midi.Message) (*Patch, byte, error) {

	dumpBytes(msg, "received_sysex.txt")

	if len(msg) != PatchSize+9 {
		return nil, 0, fmt.Errorf("unexpected dump size %d (want %d)", len(msg), PatchSize+9)
	}

	if msg[0] != 0xF0 || msg[len(msg)-1] != 0xF7 {
		return nil, 0, errors.New("message is not a SysEx frame")
	}

	if msg[1] != 0x3E || msg[2] != 0x13 {
		return nil, 0, errors.New("not a Waldorf Blofeld SysEx")
	}

	if msg[4] != 0x10 {
		return nil, 0, fmt.Errorf("unexpected message type 0x%02X (expected SNDD 0x10)", msg[4])
	}

	sdata := msg[7 : 7+PatchSize]
	checksum := msg[7+PatchSize]

	var chk byte
	for _, b := range sdata {
		chk = (chk + b) & 0x7F
	}

	if checksum != 0x7F && chk != checksum {
		return nil, 0, fmt.Errorf("checksum mismatch: expected 0x%02X got 0x%02X", chk, checksum)
	}

	dumpBytes(sdata, "received_sdata.txt")

	patch, err := ParseSDATA(sdata)
	return patch, msg[3], err
}

// normalizePatchName converts a patch name into a filesystem-safe string.
func normalizePatchName(name string) string {
	// Replace spaces and special characters with underscores
	reg := regexp.MustCompile(`[^a-zA-Z0-9-_]+`)
	normalized := reg.ReplaceAllString(name, "_")
	// Remove leading/trailing underscores
	normalized = strings.Trim(normalized, "_")
	// Convert to lowercase
	normalized = strings.ToLower(normalized)
	// If empty after normalization, use a default name
	if normalized == "" {
		normalized = "unnamed"
	}
	return normalized
}

// savePatchToFile stores the patch as JSON in the patches directory.
func savePatchToFile(p *Patch) error {
	// Get the directory where the binary is located
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	execDir := filepath.Dir(execPath)

	// Ensure patches directory exists relative to binary location
	patchesDir := filepath.Join(execDir, "patches")
	if err := os.MkdirAll(patchesDir, 0755); err != nil {
		return fmt.Errorf("failed to create patches directory: %w", err)
	}

	// Create filename with datetime prefix, normalized patch name, and .json suffix
	timestamp := time.Now().Format("20060102_150405")
	normalizedName := normalizePatchName(p.Name)
	filename := fmt.Sprintf("%s_%s.json", timestamp, normalizedName)
	filePath := filepath.Join(patchesDir, filename)

	// Marshal patch to JSON with indentation
	jsonData, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal patch to JSON: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filePath, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write patch file: %w", err)
	}

	log.Printf("Saved patch to %s", filePath)
	return nil
}

// SendPatch transmits a patch to the given bank/program.
func (b *Blofeld) SendPatch(bank string, program int, p *Patch) error {

	// Save patch to file before sending
	if err := savePatchToFile(p); err != nil {
		log.Printf("Warning: failed to save patch to file: %v", err)
		// Continue with sending even if save fails
	}

	if err := b.ensureOpened(); err != nil {
		return err
	}

	bankByte, err := bankToByte(bank)
	if err != nil {
		return err
	}
	if program < 1 || program > 128 {
		return fmt.Errorf("program must be in range 1–128, got %d", program)
	}
	progByte := byte(program - 1)

	payload, err := p.ToSNDD(b.devID, bankByte, progByte)
	if err != nil {
		return fmt.Errorf("failed to build SNDD payload: %w", err)
	}

	if err := b.SendSysEx(payload); err != nil {
		return fmt.Errorf("failed to send patch to bank %s program %d: %w", bank, program, err)
	}
	return nil
}
