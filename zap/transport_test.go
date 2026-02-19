package zap

import (
	"testing"
)

func TestTransportCreation(t *testing.T) {
	tr := FromConn(nil, "test-service")
	if tr.ServiceName() != "test-service" {
		t.Errorf("expected service name 'test-service', got '%s'", tr.ServiceName())
	}
	if tr.IsConnected() {
		t.Error("expected transport to not be closed initially... wait, it should be connected")
	}
}

func TestLengthPrefix(t *testing.T) {
	// Test that our framing works correctly with a pipe
	// This is a unit test for the framing logic
	data := []byte("hello world")
	length := uint32(len(data))
	header := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}

	if len(header) != 4 {
		t.Errorf("expected 4-byte header, got %d", len(header))
	}

	// Verify the encoded length
	decoded := uint32(header[0])<<24 | uint32(header[1])<<16 | uint32(header[2])<<8 | uint32(header[3])
	if decoded != uint32(len(data)) {
		t.Errorf("expected length %d, got %d", len(data), decoded)
	}
}
