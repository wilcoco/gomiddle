package mcproto

import (
	"fmt"
	"net"
	"sync"
	"time"
)

// Client is a TCP connection to one Mitsubishi PLC. Methods are safe for
// concurrent use; requests are serialized because the PLC answers one
// request at a time on a connection.
type Client struct {
	addr    string
	timeout time.Duration

	mu   sync.Mutex
	conn net.Conn
}

func Dial(addr string, timeout time.Duration) (*Client, error) {
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return nil, fmt.Errorf("mcproto: dial %s: %w", addr, err)
	}
	return &Client{addr: addr, timeout: timeout, conn: conn}, nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil
	}
	err := c.conn.Close()
	c.conn = nil
	return err
}

// ReadD batch-reads count words starting at D<head> (e.g. head=5000 → D5000).
func (c *Client) ReadD(head uint32, count uint16) ([]uint16, error) {
	data, err := c.roundTrip(buildReadRequest(DeviceD, head, count))
	if err != nil {
		return nil, err
	}
	if len(data) != int(count)*2 {
		return nil, fmt.Errorf("mcproto: expected %d data bytes, got %d", count*2, len(data))
	}
	words := make([]uint16, count)
	for i := range words {
		words[i] = uint16(data[2*i]) | uint16(data[2*i+1])<<8
	}
	return words, nil
}

// WriteD batch-writes values to consecutive registers starting at D<head>.
func (c *Client) WriteD(head uint32, values []uint16) error {
	_, err := c.roundTrip(buildWriteRequest(DeviceD, head, values))
	return err
}

func (c *Client) roundTrip(req []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return nil, fmt.Errorf("mcproto: connection to %s is closed", c.addr)
	}
	if err := c.conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, err
	}
	if _, err := c.conn.Write(req); err != nil {
		return nil, fmt.Errorf("mcproto: write to %s: %w", c.addr, err)
	}
	return readResponse(c.conn)
}
