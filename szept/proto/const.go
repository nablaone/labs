package proto

import "time"

const (
	Version = 1

	MaxEnvelope    = 65536
	MaxPlaintext   = MaxEnvelope - 5 - 16       // 0x01 | n:u32 | ... | tag
	MaxGroupPlain  = MaxEnvelope - 13 - 16 - 64 // 0x02 | hint | n | sig ... | tag
	MaxSkip        = 1000
	MessageTTL     = 7 * 24 * time.Hour
	BlobTTL        = 7 * 24 * time.Hour
	MaxBlob        = 100 << 20
	FileChunk      = 64 << 10
	InviteTTL      = 7 * 24 * time.Hour
	MaxOpenInvites = 5
	KillCodeWords  = 6
	ConfirmWords   = 3

	TypePairwise byte = 0x01
	TypeGroup    byte = 0x02
)

// PadBuckets are the plaintext sizes after padding (§4.3).
var (
	PadBuckets      = []int{256, 1024, 4096, 16384, MaxPlaintext}
	GroupPadBuckets = []int{256, 1024, 4096, 16384, MaxGroupPlain}
)
