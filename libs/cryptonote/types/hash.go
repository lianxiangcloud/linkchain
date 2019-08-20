package types

import (
	"encoding/hex"
	"fmt"
	"strings"

)

const (
	HASH_SIZE      = 32
	HASH_DATA_AREA = 136
	HashLength     = HASH_SIZE
)

type Hash [HASH_SIZE]byte
type Hash8 [8]byte

var NullHash = Hash{}

// HexToHash sets byte representation of s to hash.
// If b is larger than len(h), b will be cropped from the left.
func HexToHash(s string) Hash { return BytesToHash(FromHex(s)) }

// SetBytes sets the hash to the value of b.
// If b is larger than len(h), b will be cropped from the left.
func (h *Hash) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-HashLength:]
	}

	copy(h[HashLength-len(b):], b)
}

// BytesToHash sets b to hash.
// If b is larger than len(h), b will be cropped from the left.
func BytesToHash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}
func HexEncode(b []byte) string {
	enc := make([]byte, len(b)*2+2)
	copy(enc, "0x")
	hex.Encode(enc[2:], b)
	return string(enc)
}
// Hex converts a hash to a hex string.
func (h Hash) Hex() string { return strings.ToLower(HexEncode(h[:])) }

// TerminalString implements log.TerminalStringer, formatting a string for console
// output during logging.
func (h Hash) TerminalString() string {
	return fmt.Sprintf("%x…%x", h[:3], h[29:])
}

// String implements the stringer interface and is used also by the logger when
// doing full logging into a file.
func (h Hash) String() string {
	return h.Hex()
}

//===========================
// HexToHash sets byte representation of s to hash.
// If b is larger than len(h), b will be cropped from the left.
func HexToHash8(s string) Hash8 { return BytesToHash8(FromHex(s)) }

// SetBytes sets the hash to the value of b.
// If b is larger than len(h), b will be cropped from the left.
func (h *Hash8) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-HashLength:]
	}

	copy(h[HashLength-len(b):], b)
}

// BytesToHash sets b to hash.
// If b is larger than len(h), b will be cropped from the left.
func BytesToHash8(b []byte) Hash8 {
	var h Hash8
	h.SetBytes(b)
	return h
}

// Hex converts a hash to a hex string.
func (h Hash8) Hex() string { return strings.ToLower(HexEncode(h[:])) }

// TerminalString implements log.TerminalStringer, formatting a string for console
// output during logging.
// func (h Hash8) TerminalString() string {
// 	return fmt.Sprintf("%x…%x", h[:3], h[29:])
// }

// String implements the stringer interface and is used also by the logger when
// doing full logging into a file.
func (h Hash8) String() string {
	return h.Hex()
}
