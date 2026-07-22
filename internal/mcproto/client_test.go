package mcproto

import (
	"io"
	"net"
	"testing"
	"time"
)

// fakePLC is a minimal in-memory MC 3E binary server: it keeps a register
// map and answers batch read/write requests, letting us test the real
// Client over a real TCP socket without hardware.
type fakePLC struct {
	ln   net.Listener
	regs map[uint32]uint16
}

func startFakePLC(t *testing.T) *fakePLC {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &fakePLC{ln: ln, regs: map[uint32]uint16{}}
	go f.serve()
	t.Cleanup(func() { ln.Close() })
	return f
}

func (f *fakePLC) serve() {
	for {
		conn, err := f.ln.Accept()
		if err != nil {
			return
		}
		go f.handle(conn)
	}
}

func (f *fakePLC) handle(conn net.Conn) {
	defer conn.Close()
	for {
		var hdr [9]byte
		if _, err := io.ReadFull(conn, hdr[:]); err != nil {
			return
		}
		body := make([]byte, int(hdr[7])|int(hdr[8])<<8)
		if _, err := io.ReadFull(conn, body); err != nil {
			return
		}
		cmd := uint16(body[2]) | uint16(body[3])<<8
		head := uint32(body[6]) | uint32(body[7])<<8 | uint32(body[8])<<16
		count := int(body[10]) | int(body[11])<<8

		var data []byte
		switch cmd {
		case 0x0401: // batch read
			for i := 0; i < count; i++ {
				w := f.regs[head+uint32(i)]
				data = append(data, byte(w), byte(w>>8))
			}
		case 0x1401: // batch write
			for i := 0; i < count; i++ {
				f.regs[head+uint32(i)] = uint16(body[12+2*i]) | uint16(body[13+2*i])<<8
			}
		}

		payloadLen := 2 + len(data)
		resp := []byte{0xD0, 0x00, 0x00, 0xFF, 0xFF, 0x03, 0x00,
			byte(payloadLen), byte(payloadLen >> 8),
			0x00, 0x00, // end code: success
		}
		resp = append(resp, data...)
		if _, err := conn.Write(resp); err != nil {
			return
		}
	}
}

func TestClientReadWriteAgainstFakePLC(t *testing.T) {
	plc := startFakePLC(t)
	plc.regs[5000] = 0x0031 // flicker '1'
	plc.regs[5010] = 0x0031
	plc.regs[5011] = 0x0030

	c, err := Dial(plc.ln.Addr().String(), 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	words, err := c.ReadD(5000, 12)
	if err != nil {
		t.Fatal(err)
	}
	if words[0] != 0x0031 || words[10] != 0x0031 || words[11] != 0x0030 {
		t.Errorf("read: got %04X %04X %04X", words[0], words[10], words[11])
	}

	// Write the 14-char serial to D8041~D8047 and read it back.
	serial := EncodeString("24072012345601", 7)
	if err := c.WriteD(8041, serial); err != nil {
		t.Fatal(err)
	}
	back, err := c.ReadD(8041, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got := DecodeString(back); got != "24072012345601" {
		t.Errorf("serial round trip: got %q", got)
	}
}
