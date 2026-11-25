package proto

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
)

// File encryption (§7): ChaCha20-Poly1305 STREAM, the same construction as the
// age payload. fkey is random per file, so the nonce is just
// 11-byte big-endian chunk counter || last-chunk flag.

const fileTagSize = 16

func streamNonce(i uint64, last bool) []byte {
	n := make([]byte, 12)
	for j := 10; j >= 3; j-- { // counter fits in 8 bytes; top 3 stay zero
		n[j] = byte(i)
		i >>= 8
	}
	if last {
		n[11] = 1
	}
	return n
}

// NewFileKey returns a random 32-byte file key.
func NewFileKey() []byte {
	k := make([]byte, 32)
	rand.Read(k)
	return k
}

// EncryptFile streams r to w and returns SHA-256 of the ciphertext.
// An empty file is one empty final chunk.
func EncryptFile(fkey []byte, r io.Reader, w io.Writer) ([]byte, error) {
	a, err := chacha20poly1305.New(fkey)
	if err != nil {
		return nil, err
	}
	h := sha256.New()
	out := io.MultiWriter(w, h)
	buf := make([]byte, FileChunk)
	next := make([]byte, FileChunk)
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
		return nil, err
	}
	for i := uint64(0); ; i++ {
		last := n < FileChunk
		var m int
		if !last {
			// Peek: a full chunk is only "last" if nothing follows.
			m, err = io.ReadFull(r, next)
			if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
				return nil, err
			}
			last = m == 0
		}
		if _, err := out.Write(a.Seal(nil, streamNonce(i, last), buf[:n], nil)); err != nil {
			return nil, err
		}
		if last {
			return h.Sum(nil), nil
		}
		buf, next, n = next, buf, m
	}
}

// DecryptFile reverses EncryptFile. The caller verifies the ciphertext hash
// separately (or uses a TeeReader).
func DecryptFile(fkey []byte, r io.Reader, w io.Writer) error {
	a, err := chacha20poly1305.New(fkey)
	if err != nil {
		return err
	}
	const ctChunk = FileChunk + fileTagSize
	buf := make([]byte, ctChunk)
	next := make([]byte, ctChunk)
	n, err := io.ReadFull(r, buf)
	if err != nil && err != io.ErrUnexpectedEOF {
		return errors.New("truncated file")
	}
	for i := uint64(0); ; i++ {
		last := n < ctChunk
		var m int
		if !last {
			m, err = io.ReadFull(r, next)
			if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
				return err
			}
			last = m == 0
		}
		p, err := a.Open(nil, streamNonce(i, last), buf[:n], nil)
		if err != nil {
			return ErrDecrypt
		}
		if _, err := w.Write(p); err != nil {
			return err
		}
		if last {
			return nil
		}
		buf, next, n = next, buf, m
	}
}
