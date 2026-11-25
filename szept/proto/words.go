package proto

import (
	"crypto/rand"
	"crypto/sha256"
	_ "embed"
	"errors"
	"strings"
)

//go:embed wordlist_en.txt
var wordlistRaw string

// Wordlist is the BIP39 English wordlist (2048 words).
var Wordlist = strings.Fields(wordlistRaw)

var wordIndex = func() map[string]int {
	m := make(map[string]int, len(Wordlist))
	for i, w := range Wordlist {
		m[w] = i
	}
	return m
}()

// bitsToWords turns the first 11*count bits of b (big-endian) into words.
func bitsToWords(b []byte, count int) []string {
	words := make([]string, count)
	for i := 0; i < count; i++ {
		idx := 0
		for j := 0; j < 11; j++ {
			bit := i*11 + j
			idx = idx<<1 | int(b[bit/8]>>(7-bit%8)&1)
		}
		words[i] = Wordlist[idx]
	}
	return words
}

// NormalizeKillCode lowercases and collapses whitespace. Every word must be in
// the wordlist.
func NormalizeKillCode(code string) (string, error) {
	f := strings.Fields(strings.ToLower(code))
	if len(f) != KillCodeWords {
		return "", errors.New("kill code must have 6 words")
	}
	for _, w := range f {
		if _, ok := wordIndex[w]; !ok {
			return "", errors.New("unknown word in kill code")
		}
	}
	return strings.Join(f, " "), nil
}

// KillHash is SHA-256 of the normalized kill code.
func KillHash(code string) ([]byte, error) {
	n, err := NormalizeKillCode(code)
	if err != nil {
		return nil, err
	}
	h := sha256.Sum256([]byte(n))
	return h[:], nil
}

// NewKillCode returns 6 random words (66 bits).
func NewKillCode() string {
	b := make([]byte, 9)
	rand.Read(b)
	return strings.Join(bitsToWords(b, KillCodeWords), " ")
}
