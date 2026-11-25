package proto

import (
	"crypto/ed25519"
	"encoding/binary"
	"errors"

	"golang.org/x/crypto/chacha20poly1305"
)

// Pad implements §4.3.
func Pad(p []byte, buckets []int) ([]byte, error) {
	for _, b := range buckets {
		if len(p)+1 <= b {
			out := make([]byte, b)
			copy(out, p)
			out[len(p)] = 0x80
			return out, nil
		}
	}
	return nil, errors.New("plaintext too large")
}

func Unpad(p []byte) ([]byte, error) {
	i := len(p) - 1
	for i >= 0 && p[i] == 0 {
		i--
	}
	if i < 0 || p[i] != 0x80 {
		return nil, errors.New("bad padding")
	}
	return p[:i], nil
}

func u32(n uint32) []byte { return binary.BigEndian.AppendUint32(nil, n) }

func aeadSeal(mk, plaintext, aad []byte) []byte {
	key, nonce := messageKeyNonce(mk)
	a, _ := chacha20poly1305.NewX(key)
	return a.Seal(nil, nonce, plaintext, aad)
}

func aeadOpen(mk, ct, aad []byte) ([]byte, error) {
	key, nonce := messageKeyNonce(mk)
	a, _ := chacha20poly1305.NewX(key)
	p, err := a.Open(nil, nonce, ct, aad)
	if err != nil {
		return nil, ErrDecrypt
	}
	return p, nil
}

// SealPairwise encrypts a CBOR message for the peer (§4.2).
// mbox is the receiver's mailbox ID (Keys.SendMbox).
func SealPairwise(c *SendChain, mbox, message []byte) ([]byte, error) {
	padded, err := Pad(message, PadBuckets)
	if err != nil {
		return nil, err
	}
	mk, n := c.Next()
	hdr := append([]byte{TypePairwise}, u32(n)...)
	aad := concat([]byte{TypePairwise}, mbox, u32(n))
	return append(hdr, aeadSeal(mk, padded, aad)...), nil
}

// OpenPairwise decrypts an envelope from our mailbox (Keys.RecvMbox).
func OpenPairwise(c *RecvChain, mbox, env []byte) ([]byte, error) {
	if len(env) < 5+16 || env[0] != TypePairwise {
		return nil, errors.New("not a pairwise envelope")
	}
	n := binary.BigEndian.Uint32(env[1:5])
	mk, commit, err := c.key(n)
	if err != nil {
		return nil, err
	}
	aad := concat([]byte{TypePairwise}, mbox, u32(n))
	padded, err := aeadOpen(mk, env[5:], aad)
	if err != nil {
		return nil, err
	}
	msg, err := Unpad(padded)
	if err != nil {
		return nil, err
	}
	commit()
	return msg, nil
}

// GroupHint is the 8-byte sender hint (§5.3).
func GroupHint(gk []byte, spub ed25519.PublicKey) []byte {
	return hmacSHA256(gk, spub)[:8]
}

func groupSigInput(gid []byte, epoch, n uint32, padded []byte) []byte {
	return concat([]byte(labelPrefix+"gsig"), gid, u32(epoch), u32(n), padded)
}

// SealGroup encrypts one group message; the result is posted to every
// member's group mailbox.
func SealGroup(c *SendChain, gk, gid []byte, epoch uint32, s ed25519.PrivateKey, message []byte) ([]byte, error) {
	padded, err := Pad(message, GroupPadBuckets)
	if err != nil {
		return nil, err
	}
	mk, n := c.Next()
	hint := GroupHint(gk, s.Public().(ed25519.PublicKey))
	sig := ed25519.Sign(s, groupSigInput(gid, epoch, n, padded))
	hdr := concat([]byte{TypeGroup}, hint, u32(n))
	return append(hdr, aeadSeal(mk, append(sig, padded...), hdr)...), nil
}

// GroupHeader parses the hint and counter so the receiver can find the sender.
func GroupHeader(env []byte) (hint []byte, n uint32, err error) {
	if len(env) < 13+16+ed25519.SignatureSize || env[0] != TypeGroup {
		return nil, 0, errors.New("not a group envelope")
	}
	return env[1:9], binary.BigEndian.Uint32(env[9:13]), nil
}

// OpenGroup decrypts and verifies a group envelope from a known sender.
func OpenGroup(c *RecvChain, gid []byte, epoch uint32, spub ed25519.PublicKey, env []byte) ([]byte, error) {
	_, n, err := GroupHeader(env)
	if err != nil {
		return nil, err
	}
	mk, commit, err := c.key(n)
	if err != nil {
		return nil, err
	}
	p, err := aeadOpen(mk, env[13:], env[:13])
	if err != nil {
		return nil, err
	}
	if len(p) < ed25519.SignatureSize {
		return nil, ErrDecrypt
	}
	sig, padded := p[:ed25519.SignatureSize], p[ed25519.SignatureSize:]
	if !ed25519.Verify(spub, groupSigInput(gid, epoch, n, padded), sig) {
		return nil, errors.New("bad signature")
	}
	msg, err := Unpad(padded)
	if err != nil {
		return nil, err
	}
	commit()
	return msg, nil
}
