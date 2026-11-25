package proto

import (
	"bytes"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/fxamacker/cbor/v2"
)

const QRPrefix = "szept1:"

// Offer is the QR payload (§3.1).
type Offer struct {
	V int    `cbor:"v"`
	E []byte `cbor:"E"`
	R []byte `cbor:"r"`
	S string `cbor:"s"`
	N string `cbor:"n,omitempty"`
	I []byte `cbor:"i,omitempty"`
}

var detEnc = func() cbor.EncMode {
	m, err := cbor.CoreDetEncOptions().EncMode()
	if err != nil {
		panic(err)
	}
	return m
}()

// Marshal encodes v as deterministic CBOR.
func Marshal(v any) ([]byte, error) { return detEnc.Marshal(v) }

// Unmarshal decodes CBOR.
func Unmarshal(b []byte, v any) error { return cbor.Unmarshal(b, v) }

func (o *Offer) Encode() (string, error) {
	b, err := Marshal(o)
	if err != nil {
		return "", err
	}
	return QRPrefix + base64.RawURLEncoding.EncodeToString(b), nil
}

func DecodeOffer(s string) (*Offer, error) {
	if !strings.HasPrefix(s, QRPrefix) {
		return nil, errors.New("not a szept v1 QR code")
	}
	b, err := base64.RawURLEncoding.DecodeString(s[len(QRPrefix):])
	if err != nil {
		return nil, err
	}
	var o Offer
	if err := Unmarshal(b, &o); err != nil {
		return nil, err
	}
	switch {
	case o.V != Version:
		return nil, errors.New("unsupported version")
	case len(o.E) != 32, len(o.R) != 32:
		return nil, errors.New("bad key sizes")
	case o.S == "":
		return nil, errors.New("missing server")
	case len(o.N) > 64:
		return nil, errors.New("name too long")
	case o.I != nil && len(o.I) != 16:
		return nil, errors.New("bad invite size")
	}
	return &o, nil
}

// Pairing is one side of an in-person pairing in progress.
type Pairing struct {
	priv  *ecdh.PrivateKey
	Offer Offer
}

// NewPairing creates fresh pairing material.
func NewPairing(server, name string, invite []byte) (*Pairing, error) {
	priv, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	r := make([]byte, 32)
	rand.Read(r)
	return newPairing(priv, r, server, name, invite), nil
}

// NewPairingFromSecrets is deterministic; used for test vectors.
func NewPairingFromSecrets(e, r []byte, server, name string, invite []byte) (*Pairing, error) {
	priv, err := ecdh.X25519().NewPrivateKey(e)
	if err != nil {
		return nil, err
	}
	return newPairing(priv, r, server, name, invite), nil
}

func newPairing(priv *ecdh.PrivateKey, r []byte, server, name string, invite []byte) *Pairing {
	return &Pairing{priv: priv, Offer: Offer{
		V: Version, E: priv.PublicKey().Bytes(), R: r, S: server, N: name, I: invite,
	}}
}

// Keys is everything a contact needs, derived from root (§3.3).
// "Send" fields are for messages this device sends; "Recv" for the peer's.
type Keys struct {
	Root      []byte
	IsLo      bool
	SendCK    []byte // ck0 of our sending direction
	RecvCK    []byte
	SendMbox  []byte // peer's mailbox (on peer's server)
	RecvMbox  []byte // our mailbox (on our server)
	SendToken []byte
	RecvToken []byte // we register SHA-256 of this with our server
	Confirm   []string
}

// Complete derives the shared keys from the peer's offer. The caller must then
// discard the Pairing (§3.2 step 4).
func (p *Pairing) Complete(peer *Offer) (*Keys, error) {
	peerPub, err := ecdh.X25519().NewPublicKey(peer.E)
	if err != nil {
		return nil, err
	}
	cmp := bytes.Compare(p.Offer.E, peer.E)
	if cmp == 0 {
		return nil, errors.New("scanned own QR code")
	}
	dh, err := p.priv.ECDH(peerPub)
	if err != nil {
		return nil, err
	}
	lo, hi := &p.Offer, peer
	if cmp > 0 {
		lo, hi = peer, &p.Offer
	}
	return DeriveKeys(dh, lo, hi, cmp < 0), nil
}

// DeriveKeys implements §3.3.
func DeriveKeys(dh []byte, lo, hi *Offer, weAreLo bool) *Keys {
	ikm := concat(dh, lo.R, hi.R)
	info := concat(lo.E, hi.E)
	root := hkdfRaw(ikm, []byte(labelPrefix+"pair"), info, 32)

	send, recv := "lo2hi", "hi2lo"
	if !weAreLo {
		send, recv = recv, send
	}
	return &Keys{
		Root:      root,
		IsLo:      weAreLo,
		SendCK:    KDF(root, "chain/"+send, 32),
		RecvCK:    KDF(root, "chain/"+recv, 32),
		SendMbox:  KDF(root, "mbox/"+send, 16),
		RecvMbox:  KDF(root, "mbox/"+recv, 16),
		SendToken: KDF(root, "send/"+send, 32),
		RecvToken: KDF(root, "send/"+recv, 32),
		Confirm:   bitsToWords(KDF(root, "confirm", 5), ConfirmWords),
	}
}

func concat(parts ...[]byte) []byte {
	var b []byte
	for _, p := range parts {
		b = append(b, p...)
	}
	return b
}
