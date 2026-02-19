// Package zap provides a ZAP transport adapter for ZT connections.
//
// This allows ZAP RPC calls to flow through the ZT fabric,
// providing zero-trust networking with Cap'n Proto RPC.
package zap

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/hanzozt/sdk-golang/zt"
)

// Transport wraps a ZT connection to provide ZAP framing.
// It implements a bidirectional byte stream with 4-byte length-prefixed
// message framing suitable for Cap'n Proto RPC.
type Transport struct {
	ctx     zt.Context
	conn    net.Conn
	service string
	mu      sync.Mutex
	closed  bool
}

// Dial creates a new ZAP transport by dialing a ZT service.
// The URL format is "zt://service-name".
func Dial(ctx context.Context, ztCtx zt.Context, serviceURL string) (*Transport, error) {
	service := strings.TrimPrefix(serviceURL, "zt://")
	if service == "" {
		return nil, fmt.Errorf("zap/zt: empty service name in URL: %s", serviceURL)
	}

	conn, err := ztCtx.Dial(service)
	if err != nil {
		return nil, fmt.Errorf("zap/zt: dial %s: %w", service, err)
	}

	return &Transport{
		ctx:     ztCtx,
		conn:    conn,
		service: service,
	}, nil
}

// FromConn wraps an existing ZT connection as a ZAP transport.
func FromConn(conn net.Conn, service string) *Transport {
	return &Transport{
		conn:    conn,
		service: service,
	}
}

// Send writes a length-prefixed message to the transport.
func (t *Transport) Send(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return fmt.Errorf("zap/zt: transport closed")
	}

	// 4-byte big-endian length prefix
	length := uint32(len(data))
	header := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}

	if _, err := t.conn.Write(header); err != nil {
		return fmt.Errorf("zap/zt: write header: %w", err)
	}
	if _, err := t.conn.Write(data); err != nil {
		return fmt.Errorf("zap/zt: write data: %w", err)
	}

	return nil
}

// Recv reads a length-prefixed message from the transport.
func (t *Transport) Recv() ([]byte, error) {
	if t.closed {
		return nil, fmt.Errorf("zap/zt: transport closed")
	}

	// Read 4-byte length header
	header := make([]byte, 4)
	if _, err := readFull(t.conn, header); err != nil {
		return nil, fmt.Errorf("zap/zt: read header: %w", err)
	}

	length := uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])
	if length > 16*1024*1024 { // 16 MB max
		return nil, fmt.Errorf("zap/zt: message too large: %d bytes", length)
	}

	data := make([]byte, length)
	if _, err := readFull(t.conn, data); err != nil {
		return nil, fmt.Errorf("zap/zt: read data: %w", err)
	}

	return data, nil
}

// Close closes the transport.
func (t *Transport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	if t.conn != nil {
		return t.conn.Close()
	}
	return nil
}

// IsConnected returns whether the transport is still open.
func (t *Transport) IsConnected() bool {
	return !t.closed
}

// ServiceName returns the ZT service this transport connects to.
func (t *Transport) ServiceName() string {
	return t.service
}

// SetDeadline sets the read/write deadline on the underlying connection.
func (t *Transport) SetDeadline(d time.Time) error {
	if t.conn != nil {
		return t.conn.SetDeadline(d)
	}
	return nil
}

func readFull(conn net.Conn, buf []byte) (int, error) {
	total := 0
	for total < len(buf) {
		n, err := conn.Read(buf[total:])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
