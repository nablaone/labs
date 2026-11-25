package proto

import (
	"crypto/sha256"
	"errors"

	"golang.org/x/crypto/hkdf"
)

func hkdfRaw(ikm, salt, info []byte, n int) []byte {
	out := make([]byte, n)
	if _, err := hkdf.New(sha256.New, ikm, salt, info).Read(out); err != nil {
		panic(err)
	}
	return out
}

var (
	ErrReplay  = errors.New("message key already used")
	ErrTooFar  = errors.New("too many skipped messages")
	ErrDecrypt = errors.New("decryption failed")
)

// SendChain is a sending hash ratchet (§4.1).
type SendChain struct {
	CK []byte
	N  uint32
}

// Next returns the message key for message N and advances the chain,
// forgetting the old chain key.
func (c *SendChain) Next() (mk []byte, n uint32) {
	mk, n = hmacByte(c.CK, 0x01), c.N
	c.CK = hmacByte(c.CK, 0x02)
	c.N++
	return
}

// RecvChain is a receiving hash ratchet with skipped-key storage.
type RecvChain struct {
	CK      []byte
	N       uint32
	Skipped map[uint32][]byte
}

// key returns the message key for n and the chain state that would result.
// Nothing is committed; call commit only after successful decryption, so that
// a forged counter cannot desynchronise the chain.
func (c *RecvChain) key(n uint32) (mk []byte, commit func(), err error) {
	if n < c.N {
		k, ok := c.Skipped[n]
		if !ok {
			return nil, nil, ErrReplay
		}
		return k, func() { delete(c.Skipped, n) }, nil
	}
	if n-c.N > MaxSkip {
		return nil, nil, ErrTooFar
	}
	ck := c.CK
	skipped := map[uint32][]byte{}
	for i := c.N; i < n; i++ {
		skipped[i] = hmacByte(ck, 0x01)
		ck = hmacByte(ck, 0x02)
	}
	mk = hmacByte(ck, 0x01)
	next := hmacByte(ck, 0x02)
	return mk, func() {
		if c.Skipped == nil {
			c.Skipped = map[uint32][]byte{}
		}
		for i, k := range skipped {
			c.Skipped[i] = k
		}
		c.CK, c.N = next, n+1
	}, nil
}

// messageKeyNonce derives the AEAD key and XChaCha nonce from a message key.
func messageKeyNonce(mk []byte) (key, nonce []byte) {
	kn := KDF(mk, "msg", 56)
	return kn[:32], kn[32:]
}
