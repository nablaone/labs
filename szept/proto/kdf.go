// Package proto implements the szept v1 cryptographic protocol (PROTOCOL.md).
// It is shared by the server (kill codes, sizes) and the reference test client,
// and generates the test vectors the Android client must match.
package proto

import (
	"crypto/hmac"
	"crypto/sha256"
	"io"

	"golang.org/x/crypto/hkdf"
)

const labelPrefix = "szept/v1/"

// KDF(key, label, L) = HKDF-SHA256(ikm=key, salt="", info="szept/v1/"||label, L)
func KDF(key []byte, label string, n int) []byte {
	r := hkdf.New(sha256.New, key, nil, []byte(labelPrefix+label))
	out := make([]byte, n)
	if _, err := io.ReadFull(r, out); err != nil {
		panic(err) // only possible if n > 255*32
	}
	return out
}

// hmacByte is H(key, byte) from §1.
func hmacByte(key []byte, b byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write([]byte{b})
	return m.Sum(nil)
}

func hmacSHA256(key, data []byte) []byte {
	m := hmac.New(sha256.New, key)
	m.Write(data)
	return m.Sum(nil)
}
