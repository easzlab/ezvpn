package socks

import (
	"bytes"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSendReply_Success_IPv4(t *testing.T) {
	var buf bytes.Buffer
	
	addr := &AddrSpec{
		IP:   net.ParseIP("192.168.1.1"),
		Port: 8080,
	}

	err := SendReply(&buf, successReply, addr)
	require.NoError(t, err)

	expected := []byte{
		0x05,                    // SOCKS version
		0x00,                    // Success reply
		0x00,                    // Reserved
		0x01,                    // IPv4 address type
		192, 168, 1, 1,         // IPv4 address
		0x1f, 0x90,             // Port 8080
	}

	assert.Equal(t, expected, buf.Bytes())
}

func TestSendReply_Success_IPv6(t *testing.T) {
	var buf bytes.Buffer
	
	addr := &AddrSpec{
		IP:   net.ParseIP("2001:db8::1"),
		Port: 443,
	}

	err := SendReply(&buf, successReply, addr)
	require.NoError(t, err)

	expected := []byte{
		0x05,                           // SOCKS version
		0x00,                           // Success reply
		0x00,                           // Reserved
		0x04,                           // IPv6 address type
		0x20, 0x01, 0x0d, 0xb8,        // IPv6 address 2001:db8::1
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x01,
		0x01, 0xbb,                     // Port 443
	}

	assert.Equal(t, expected, buf.Bytes())
}

func TestSendReply_Success_FQDN(t *testing.T) {
	var buf bytes.Buffer
	
	addr := &AddrSpec{
		FQDN: "example.com",
		Port: 80,
	}

	err := SendReply(&buf, successReply, addr)
	require.NoError(t, err)

	expected := []byte{
		0x05,                    // SOCKS version
		0x00,                    // Success reply
		0x00,                    // Reserved
		0x03,                    // FQDN address type
		11,                      // FQDN length
	}
	expected = append(expected, []byte("example.com")...)
	expected = append(expected, []byte{0x00, 0x50}...) // Port 80

	assert.Equal(t, expected, buf.Bytes())
}

func TestSendReply_NilAddress(t *testing.T) {
	var buf bytes.Buffer

	err := SendReply(&buf, hostUnreachable, nil)
	require.NoError(t, err)

	expected := []byte{
		0x05,                    // SOCKS version
		0x04,                    // Host unreachable
		0x00,                    // Reserved
		0x01,                    // IPv4 address type (default)
		0x00, 0x00, 0x00, 0x00, // 0.0.0.0
		0x00, 0x00,             // Port 0
	}

	assert.Equal(t, expected, buf.Bytes())
}

func TestSendReply_ErrorReply(t *testing.T) {
	var buf bytes.Buffer
	
	addr := &AddrSpec{
		IP:   net.ParseIP("127.0.0.1"),
		Port: 1080,
	}

	err := SendReply(&buf, commandNotSupported, addr)
	require.NoError(t, err)

	expected := []byte{
		0x05,                    // SOCKS version
		0x07,                    // Command not supported
		0x00,                    // Reserved
		0x01,                    // IPv4 address type
		127, 0, 0, 1,           // 127.0.0.1
		0x04, 0x38,             // Port 1080
	}

	assert.Equal(t, expected, buf.Bytes())
}

func TestSendReply_InvalidAddress(t *testing.T) {
	var buf bytes.Buffer
	
	// Create address with neither IP nor FQDN
	addr := &AddrSpec{
		Port: 8080,
	}

	err := SendReply(&buf, successReply, addr)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to format address")
}

func TestSendReply_IPv4To16Conversion(t *testing.T) {
	var buf bytes.Buffer
	
	// Test that IPv4 addresses are properly handled
	addr := &AddrSpec{
		IP:   net.ParseIP("192.168.1.1").To4(), // Ensure it's IPv4
		Port: 8080,
	}

	err := SendReply(&buf, successReply, addr)
	require.NoError(t, err)

	// Should use IPv4 format, not IPv6
	assert.Equal(t, byte(0x01), buf.Bytes()[3]) // IPv4 address type
	assert.Equal(t, 10, len(buf.Bytes()))       // IPv4 reply length
}