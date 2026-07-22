package mcproto

import (
	"bytes"
	"errors"
	"testing"
)

func TestBuildReadRequest(t *testing.T) {
	// Read 41 words from D5000. Expected bytes worked out by hand from the
	// 3E binary frame layout (see package comment).
	got := buildReadRequest(DeviceD, 5000, 41)
	want := []byte{
		0x50, 0x00, // subheader
		0x00,       // network
		0xFF,       // PC
		0xFF, 0x03, // dest module I/O
		0x00,       // dest station
		0x0C, 0x00, // data length = 12
		0x10, 0x00, // monitoring timer
		0x01, 0x04, // batch read
		0x00, 0x00, // word units
		0x88, 0x13, 0x00, // head address 5000 = 0x001388
		0xA8,       // device D
		0x29, 0x00, // 41 points
	}
	if !bytes.Equal(got, want) {
		t.Errorf("read request\n got % X\nwant % X", got, want)
	}
}

func TestBuildWriteRequest(t *testing.T) {
	// Write one word (0x0031) to D8000.
	got := buildWriteRequest(DeviceD, 8000, []uint16{0x0031})
	want := []byte{
		0x50, 0x00,
		0x00,
		0xFF,
		0xFF, 0x03,
		0x00,
		0x0E, 0x00, // data length = 12 + 2 bytes of data
		0x10, 0x00,
		0x01, 0x14, // batch write
		0x00, 0x00,
		0x40, 0x1F, 0x00, // head address 8000 = 0x001F40
		0xA8,
		0x01, 0x00, // 1 point
		0x31, 0x00, // the data word, little-endian
	}
	if !bytes.Equal(got, want) {
		t.Errorf("write request\n got % X\nwant % X", got, want)
	}
}

func TestReadResponseSuccess(t *testing.T) {
	// A successful read response carrying two words: 0x0031, 0x1234.
	resp := []byte{
		0xD0, 0x00, 0x00, 0xFF, 0xFF, 0x03, 0x00,
		0x06, 0x00, // data length: end code (2) + 4 data bytes
		0x00, 0x00, // end code: success
		0x31, 0x00, 0x34, 0x12,
	}
	data, err := readResponse(bytes.NewReader(resp))
	if err != nil {
		t.Fatal(err)
	}
	if want := []byte{0x31, 0x00, 0x34, 0x12}; !bytes.Equal(data, want) {
		t.Errorf("data: got % X, want % X", data, want)
	}
}

func TestReadResponseEndCodeError(t *testing.T) {
	// End code 0xC059 = "command/subcommand error" from the PLC.
	resp := []byte{
		0xD0, 0x00, 0x00, 0xFF, 0xFF, 0x03, 0x00,
		0x02, 0x00,
		0x59, 0xC0,
	}
	_, err := readResponse(bytes.NewReader(resp))
	var ec *EndCodeError
	if !errors.As(err, &ec) || ec.Code != 0xC059 {
		t.Errorf("expected EndCodeError 0xC059, got %v", err)
	}
}
