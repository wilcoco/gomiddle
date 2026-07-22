// Package mcproto implements the Mitsubishi MC Protocol, frame 3E, in
// binary mode over TCP — the protocol spoken by the Q06UDV injection PLCs.
//
// A 3E binary request frame looks like this (all multi-byte fields are
// little-endian):
//
//	50 00        subheader (request)
//	00           network number
//	FF           PC number
//	FF 03        destination module I/O (0x03FF = the CPU itself)
//	00           destination station
//	LL LL        request data length (everything after these two bytes)
//	10 00        monitoring timer (16 × 250ms = 4s)
//	CC CC        command  (0401 = batch read, 1401 = batch write)
//	SS SS        subcommand (0000 = word units)
//	AA AA AA     head device address (3 bytes)
//	DD           device code (A8 = D register)
//	NN NN        number of device points
//	[data...]    write only: NN words, 2 bytes each
//
// The response echoes a similar header with subheader D0 00, then a 2-byte
// end code (0000 = success) followed by the data for reads.
package mcproto

import (
	"fmt"
	"io"
)

// DeviceD is the device code for D (data) registers in binary mode.
const DeviceD = 0xA8

func buildHeader(dataLen int) []byte {
	return []byte{
		0x50, 0x00, // subheader: 3E request
		0x00,       // network number
		0xFF,       // PC number
		0xFF, 0x03, // destination module I/O: CPU
		0x00, // destination station
		byte(dataLen), byte(dataLen >> 8),
	}
}

func buildReadRequest(device byte, head uint32, count uint16) []byte {
	body := []byte{
		0x10, 0x00, // monitoring timer
		0x01, 0x04, // command: batch read
		0x00, 0x00, // subcommand: word units
		byte(head), byte(head >> 8), byte(head >> 16),
		device,
		byte(count), byte(count >> 8),
	}
	return append(buildHeader(len(body)), body...)
}

func buildWriteRequest(device byte, head uint32, values []uint16) []byte {
	body := []byte{
		0x10, 0x00, // monitoring timer
		0x01, 0x14, // command: batch write
		0x00, 0x00, // subcommand: word units
		byte(head), byte(head >> 8), byte(head >> 16),
		device,
		byte(len(values)), byte(len(values) >> 8),
	}
	for _, v := range values {
		body = append(body, byte(v), byte(v>>8))
	}
	return append(buildHeader(len(body)), body...)
}

// EndCodeError is a PLC-side rejection (bad address, wrong subcommand, ...).
// The code values are documented in the Mitsubishi MELSEC communication manual.
type EndCodeError struct {
	Code uint16
}

func (e *EndCodeError) Error() string {
	return fmt.Sprintf("mcproto: PLC returned end code 0x%04X", e.Code)
}

// readResponse consumes one 3E binary response frame and returns its data
// portion (empty for writes). It returns *EndCodeError if the PLC reported
// a non-zero end code.
func readResponse(r io.Reader) ([]byte, error) {
	var hdr [9]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, fmt.Errorf("mcproto: reading response header: %w", err)
	}
	if hdr[0] != 0xD0 || hdr[1] != 0x00 {
		return nil, fmt.Errorf("mcproto: unexpected subheader % X", hdr[:2])
	}
	dataLen := int(hdr[7]) | int(hdr[8])<<8
	if dataLen < 2 {
		return nil, fmt.Errorf("mcproto: response data length %d too short", dataLen)
	}
	payload := make([]byte, dataLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, fmt.Errorf("mcproto: reading response payload: %w", err)
	}
	endCode := uint16(payload[0]) | uint16(payload[1])<<8
	if endCode != 0 {
		return nil, &EndCodeError{Code: endCode}
	}
	return payload[2:], nil
}
