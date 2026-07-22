package mcproto

import "testing"

func TestASCIIBit(t *testing.T) {
	cases := []struct {
		word uint16
		want int
	}{
		{0x31, 1}, // ASCII '1'
		{0x30, 0}, // ASCII '0'
		{1, 1},    // plain numeric
		{0, 0},
		{0x9999, 0}, // garbage reads as 0, never as a false "on"
	}
	for _, c := range cases {
		if got := ASCIIBit(c.word); got != c.want {
			t.Errorf("ASCIIBit(0x%04X) = %d, want %d", c.word, got, c.want)
		}
	}
}

func TestASCIIDigit(t *testing.T) {
	cases := []struct {
		word uint16
		want int
	}{
		{0x30, 0}, // ASCII '0' — observed on the real PLC (D5019 = no result)
		{0x31, 1}, // ASCII '1' — match
		{0x32, 2}, // ASCII '2' — mismatch
		{2, 2},    // plain numeric passes through
		{0xFFFF, 0},
	}
	for _, c := range cases {
		if got := ASCIIDigit(c.word); got != c.want {
			t.Errorf("ASCIIDigit(0x%04X) = %d, want %d", c.word, got, c.want)
		}
	}
}

func TestStringRoundTrip(t *testing.T) {
	// 14-character serial in 7 words, like D8041~D8047.
	serial := "24072012345601"
	words := EncodeString(serial, 7)
	if got := DecodeString(words); got != serial {
		t.Errorf("round trip: got %q, want %q", got, serial)
	}
}

func TestEncodeStringPacking(t *testing.T) {
	words := EncodeString("ABC", 2)
	// 'A'=0x41 low byte, 'B'=0x42 high byte; then 'C' alone, NUL-padded.
	if words[0] != 0x4241 || words[1] != 0x0043 {
		t.Errorf("got [0x%04X 0x%04X], want [0x4241 0x0043]", words[0], words[1])
	}
}

func TestDecodeStringTrimsPadding(t *testing.T) {
	// A 40-char field (D8020~D8039) holding a short code plus NUL padding.
	words := EncodeString("PN-1234", 20)
	if got := DecodeString(words); got != "PN-1234" {
		t.Errorf("got %q, want %q", got, "PN-1234")
	}
}
