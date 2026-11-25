package proto

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"os"
	"reflect"
	"testing"
)

// Test vectors for other implementations (the Kotlin client).
// Regenerate with: go test ./proto -run TestVectors -update

var update = flag.Bool("update", false, "rewrite testdata/vectors.json")

func seq(start byte, n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = start + byte(i)
	}
	return b
}

type hexb []byte

func (h hexb) MarshalJSON() ([]byte, error) { return json.Marshal(hex.EncodeToString(h)) }
func (h *hexb) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	v, err := hex.DecodeString(s)
	*h = v
	return err
}

type vectors struct {
	Pairing struct {
		AE, AR, BE, BR   hexb
		AQR, BQR         string
		Root             hexb
		ALo              bool
		ASendCK, ARecvCK hexb
		ASendMbox        hexb
		ARecvMbox        hexb
		ASendToken       hexb
		ARecvToken       hexb
		Confirm          []string
	}
	Ratchet struct {
		CK0       hexb
		MK        []hexb // mk[0..2]
		CK3       hexb
		MsgKey0   hexb
		MsgNonce0 hexb
	}
	Pairwise struct {
		Plaintext hexb
		Envelope  hexb // A's first message to B
	}
	Group struct {
		GK, GID, CK0, SSeed hexb
		Epoch               uint32
		Hint                hexb
		Plaintext           hexb
		Envelope            hexb
	}
	KillCode struct {
		Input string
		Hash  hexb
	}
	// File: plaintext byte i is byte(i); only SHA-256 of the ciphertext is kept.
	File struct {
		Key   hexb
		Cases []fileCase
	}
}

type fileCase struct {
	Size         int
	CipherSize   int
	CipherSHA256 hexb
}

func filePlain(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i)
	}
	return b
}

func buildVectors(t *testing.T) *vectors {
	var v vectors
	p := &v.Pairing
	p.AE, p.AR, p.BE, p.BR = seq(0x10, 32), seq(0x30, 32), seq(0x50, 32), seq(0x70, 32)
	pa, err := NewPairingFromSecrets(p.AE, p.AR, "https://a.example", "Alice", nil)
	if err != nil {
		t.Fatal(err)
	}
	pb, _ := NewPairingFromSecrets(p.BE, p.BR, "https://b.example", "Bob", seq(0x90, 16))
	p.AQR, _ = pa.Offer.Encode()
	p.BQR, _ = pb.Offer.Encode()
	ka, _ := pa.Complete(&pb.Offer)
	p.Root, p.ALo = ka.Root, ka.IsLo
	p.ASendCK, p.ARecvCK = ka.SendCK, ka.RecvCK
	p.ASendMbox, p.ARecvMbox = ka.SendMbox, ka.RecvMbox
	p.ASendToken, p.ARecvToken = ka.SendToken, ka.RecvToken
	p.Confirm = ka.Confirm

	r := &v.Ratchet
	r.CK0 = ka.SendCK
	c := &SendChain{CK: ka.SendCK}
	for i := 0; i < 3; i++ {
		mk, _ := c.Next()
		r.MK = append(r.MK, mk)
	}
	r.CK3 = c.CK
	r.MsgKey0, r.MsgNonce0 = messageKeyNonce(r.MK[0])

	v.Pairwise.Plaintext, _ = Marshal(map[string]any{"t": "text", "id": seq(0xa0, 16), "ts": uint64(1700000000000), "body": "cześć"})
	v.Pairwise.Envelope, _ = SealPairwise(&SendChain{CK: ka.SendCK}, ka.SendMbox, v.Pairwise.Plaintext)

	g := &v.Group
	g.GK, g.GID, g.CK0, g.SSeed, g.Epoch = seq(0xb0, 32), seq(0xd0, 16), seq(0xe0, 32), seq(0x01, 32), 3
	s := ed25519.NewKeyFromSeed(g.SSeed)
	g.Hint = GroupHint(g.GK, s.Public().(ed25519.PublicKey))
	g.Plaintext, _ = Marshal(map[string]any{"t": "text", "id": seq(0xc0, 16), "ts": uint64(1700000000000), "gid": []byte(g.GID), "body": "hej"})
	g.Envelope, _ = SealGroup(&SendChain{CK: g.CK0}, g.GK, g.GID, g.Epoch, s, g.Plaintext)

	v.KillCode.Input = "  Abandon ability able ABOUT above   absent "
	v.KillCode.Hash, _ = KillHash(v.KillCode.Input)

	v.File.Key = seq(0x40, 32)
	for _, n := range []int{0, 1, FileChunk - 1, FileChunk, FileChunk + 1, 3*FileChunk + 100} {
		var ct bytes.Buffer
		sum, err := EncryptFile(v.File.Key, bytes.NewReader(filePlain(n)), &ct)
		if err != nil {
			t.Fatal(err)
		}
		v.File.Cases = append(v.File.Cases, fileCase{n, ct.Len(), sum})
	}
	return &v
}

func TestVectors(t *testing.T) {
	v := buildVectors(t)
	const path = "testdata/vectors.json"
	if *update {
		os.MkdirAll("testdata", 0o755)
		b, _ := json.MarshalIndent(v, "", "  ")
		if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (run with -update)", err)
	}
	var want vectors
	if err := json.Unmarshal(b, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*v, want) {
		t.Fatal("output differs from testdata/vectors.json: protocol change?")
	}
	// The vectors must also decrypt.
	kb, _ := DeriveKeysFromVector(v)
	m, err := OpenPairwise(&RecvChain{CK: kb.RecvCK}, kb.RecvMbox, v.Pairwise.Envelope)
	if err != nil || string(m) != string(v.Pairwise.Plaintext) {
		t.Fatal("pairwise vector does not open from B's side", err)
	}
}

// DeriveKeysFromVector derives B's keys from the vector secrets.
func DeriveKeysFromVector(v *vectors) (*Keys, error) {
	p := &v.Pairing
	pa, _ := NewPairingFromSecrets(p.AE, p.AR, "https://a.example", "Alice", nil)
	pb, _ := NewPairingFromSecrets(p.BE, p.BR, "https://b.example", "Bob", seq(0x90, 16))
	return pb.Complete(&pa.Offer)
}
