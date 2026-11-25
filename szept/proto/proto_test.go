package proto

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"testing"
)

func pair(t *testing.T) (a, b *Keys) {
	t.Helper()
	pa, err := NewPairing("https://a.example", "Alice", nil)
	if err != nil {
		t.Fatal(err)
	}
	pb, _ := NewPairing("https://b.example", "Bob", make([]byte, 16))
	// Round-trip through the QR encoding, like a real scan.
	qa, _ := pa.Offer.Encode()
	qb, _ := pb.Offer.Encode()
	oa, err := DecodeOffer(qa)
	if err != nil {
		t.Fatal(err)
	}
	ob, err := DecodeOffer(qb)
	if err != nil {
		t.Fatal(err)
	}
	if a, err = pa.Complete(ob); err != nil {
		t.Fatal(err)
	}
	if b, err = pb.Complete(oa); err != nil {
		t.Fatal(err)
	}
	return a, b
}

func TestPairingSymmetric(t *testing.T) {
	a, b := pair(t)
	if !bytes.Equal(a.Root, b.Root) || a.IsLo == b.IsLo {
		t.Fatal("roots differ or both same side")
	}
	if !bytes.Equal(a.SendCK, b.RecvCK) || !bytes.Equal(a.SendMbox, b.RecvMbox) || !bytes.Equal(a.SendToken, b.RecvToken) {
		t.Fatal("directions do not mirror")
	}
	if bytes.Equal(a.SendCK, a.RecvCK) || bytes.Equal(a.SendMbox, a.RecvMbox) {
		t.Fatal("directions must differ")
	}
	if len(a.Confirm) != 3 || a.Confirm[0] != b.Confirm[0] || a.Confirm[2] != b.Confirm[2] {
		t.Fatal("confirm words differ")
	}
}

func TestOwnQR(t *testing.T) {
	p, _ := NewPairing("https://a", "", nil)
	if _, err := p.Complete(&p.Offer); err == nil {
		t.Fatal("expected error when scanning own code")
	}
}

func TestPairwise(t *testing.T) {
	a, b := pair(t)
	sc := &SendChain{CK: a.SendCK}
	rc := &RecvChain{CK: b.RecvCK}

	var envs [][]byte
	for i := 0; i < 5; i++ {
		e, err := SealPairwise(sc, a.SendMbox, []byte{byte(i), 'x'})
		if err != nil {
			t.Fatal(err)
		}
		if len(e) != 5+256+16 {
			t.Fatalf("envelope not padded: %d", len(e))
		}
		envs = append(envs, e)
	}
	// Out of order: 3, 0, 4, 1, 2
	for _, i := range []int{3, 0, 4, 1, 2} {
		m, err := OpenPairwise(rc, b.RecvMbox, envs[i])
		if err != nil || m[0] != byte(i) {
			t.Fatalf("msg %d: %v", i, err)
		}
	}
	if len(rc.Skipped) != 0 {
		t.Fatal("skipped keys not consumed")
	}
	if _, err := OpenPairwise(rc, b.RecvMbox, envs[2]); err != ErrReplay {
		t.Fatalf("replay: %v", err)
	}
	// Tampering, wrong mailbox, and forged far-ahead counter must not
	// corrupt state.
	e, _ := SealPairwise(sc, a.SendMbox, []byte("ok"))
	bad := bytes.Clone(e)
	bad[20] ^= 1
	if _, err := OpenPairwise(rc, b.RecvMbox, bad); err == nil {
		t.Fatal("tampered accepted")
	}
	if _, err := OpenPairwise(rc, b.SendMbox, e); err == nil {
		t.Fatal("wrong mailbox accepted")
	}
	forged := bytes.Clone(e)
	forged[4] = 200
	if _, err := OpenPairwise(rc, b.RecvMbox, forged); err == nil {
		t.Fatal("forged counter accepted")
	}
	if m, err := OpenPairwise(rc, b.RecvMbox, e); err != nil || string(m) != "ok" {
		t.Fatalf("state corrupted by bad input: %v", err)
	}
	far := &SendChain{CK: a.SendCK, N: rc.N + MaxSkip + 1}
	e, _ = SealPairwise(far, a.SendMbox, []byte("x"))
	if _, err := OpenPairwise(rc, b.RecvMbox, e); err != ErrTooFar {
		t.Fatalf("skip limit: %v", err)
	}
}

func TestPadding(t *testing.T) {
	for _, n := range []int{0, 255, 256, 1000, MaxPlaintext - 1} {
		p, err := Pad(bytes.Repeat([]byte{0}, n), PadBuckets)
		if err != nil {
			t.Fatal(n, err)
		}
		u, err := Unpad(p)
		if err != nil || len(u) != n {
			t.Fatalf("%d: %v %d", n, err, len(u))
		}
	}
	if _, err := Pad(make([]byte, MaxPlaintext), PadBuckets); err == nil {
		t.Fatal("oversize accepted")
	}
}

func TestGroup(t *testing.T) {
	gk, gid := make([]byte, 32), make([]byte, 16)
	rand.Read(gk)
	rand.Read(gid)
	pub, priv, _ := ed25519.GenerateKey(nil)
	ck := make([]byte, 32)
	rand.Read(ck)
	sc := &SendChain{CK: ck}
	rc := &RecvChain{CK: ck}

	e, err := SealGroup(sc, gk, gid, 1, priv, []byte("hi all"))
	if err != nil {
		t.Fatal(err)
	}
	hint, _, _ := GroupHeader(e)
	if !bytes.Equal(hint, GroupHint(gk, pub)) {
		t.Fatal("hint")
	}
	if m, err := OpenGroup(rc, gid, 1, pub, e); err != nil || string(m) != "hi all" {
		t.Fatal(err)
	}
	// A member who knows the chain key but not S cannot impersonate.
	_, mallory, _ := ed25519.GenerateKey(nil)
	e, _ = SealGroup(&SendChain{CK: rc.CK, N: rc.N}, gk, gid, 1, mallory, []byte("fake"))
	if _, err := OpenGroup(rc, gid, 1, pub, e); err == nil {
		t.Fatal("impersonation accepted")
	}
	// Max-size group message fits in an envelope.
	e, err = SealGroup(sc, gk, gid, 1, priv, make([]byte, MaxGroupPlain-1))
	if err != nil || len(e) != MaxEnvelope {
		t.Fatalf("max group: %v %d", err, len(e))
	}
}

func TestFile(t *testing.T) {
	for _, n := range []int{0, 1, FileChunk - 1, FileChunk, FileChunk + 1, 3*FileChunk + 7} {
		data := make([]byte, n)
		rand.Read(data)
		k := NewFileKey()
		var ct bytes.Buffer
		h, err := EncryptFile(k, bytes.NewReader(data), &ct)
		if err != nil {
			t.Fatal(err)
		}
		if sum := sha256.Sum256(ct.Bytes()); !bytes.Equal(h, sum[:]) {
			t.Fatal("hash")
		}
		var pt bytes.Buffer
		if err := DecryptFile(k, bytes.NewReader(ct.Bytes()), &pt); err != nil || !bytes.Equal(pt.Bytes(), data) {
			t.Fatalf("size %d: %v", n, err)
		}
		// Truncation at a chunk boundary must be detected.
		if n > FileChunk {
			cut := ct.Bytes()[:FileChunk+fileTagSize]
			if err := DecryptFile(k, bytes.NewReader(cut), &bytes.Buffer{}); err == nil {
				t.Fatalf("size %d: truncation not detected", n)
			}
		}
	}
}

func TestKillCode(t *testing.T) {
	c := NewKillCode()
	h1, err := KillHash(c)
	if err != nil {
		t.Fatal(err)
	}
	h2, err := KillHash("  " + bytes.NewBufferString(c).String() + "\n")
	if err != nil || !bytes.Equal(h1, h2) {
		t.Fatal("normalization")
	}
	if _, err := KillHash("abandon abandon abandon"); err == nil {
		t.Fatal("short code accepted")
	}
	if _, err := KillHash("abandon abandon abandon abandon abandon szept"); err == nil {
		t.Fatal("unknown word accepted")
	}
}
