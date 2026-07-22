package mcproto

import "strings"

// This PLC program stores data in D registers as ASCII characters:
//   - one-character signals: the word holds '0' (0x30) or '1' (0x31)
//   - strings: packed two characters per word, first char in the low byte,
//     unused positions padded with NUL (0x00)

// ASCIIBit decodes a one-character signal word. It accepts both the ASCII
// form ('0'/'1') and a plain numeric 0/1, since PLC programs sometimes mix
// the two during commissioning.
func ASCIIBit(w uint16) int {
	switch w {
	case 0x31, 1:
		return 1
	default:
		return 0
	}
}

// EncodeASCIIBit is the inverse: 1 → 0x31 ('1'), anything else → 0x30 ('0').
func EncodeASCIIBit(v int) uint16 {
	if v == 1 {
		return 0x31
	}
	return 0x30
}

// ASCIIDigit decodes a single ASCII digit word ('0'-'9' → 0-9). Values that
// are already plain numerics (0-9) pass through unchanged; anything else
// returns 0. Used for status codes like D5019 (0=none, 1=match, 2=mismatch),
// which the PLC stores as an ASCII character, not a raw integer.
func ASCIIDigit(w uint16) int {
	if w >= 0x30 && w <= 0x39 {
		return int(w - 0x30)
	}
	if w <= 9 {
		return int(w)
	}
	return 0
}

// DecodeString unpacks a packed-ASCII register block into a Go string,
// trimming NUL padding and surrounding spaces.
func DecodeString(words []uint16) string {
	b := make([]byte, 0, len(words)*2)
	for _, w := range words {
		b = append(b, byte(w), byte(w>>8))
	}
	return strings.TrimSpace(strings.Trim(string(b), "\x00"))
}

// EncodeString packs s into wordCount registers, two characters per word,
// NUL-padding the remainder. Characters beyond the capacity are dropped.
func EncodeString(s string, wordCount int) []uint16 {
	b := make([]byte, wordCount*2)
	copy(b, s)
	words := make([]uint16, wordCount)
	for i := range words {
		words[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
	}
	return words
}
