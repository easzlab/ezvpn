package socks

import (
	"bytes"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddrSpec_String(t *testing.T) {
	tests := []struct {
		name     string
		addr     *AddrSpec
		expected string
	}{
		{
			name: "IPv4 address",
			addr: &AddrSpec{
				IP:   net.ParseIP("192.168.1.1"),
				Port: 8080,
			},
			expected: "192.168.1.1:8080",
		},
		{
			name: "FQDN with IP",
			addr: &AddrSpec{
				FQDN: "example.com",
				IP:   net.ParseIP("1.2.3.4"),
				Port: 443,
			},
			expected: "example.com (1.2.3.4):443",
		},
		{
			name: "IPv6 address",
			addr: &AddrSpec{
				IP:   net.ParseIP("2001:db8::1"),
				Port: 80,
			},
			expected: "2001:db8::1:80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addr.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAddrSpec_Address(t *testing.T) {
	tests := []struct {
		name     string
		addr     *AddrSpec
		expected string
	}{
		{
			name: "IPv4 address",
			addr: &AddrSpec{
				IP:   net.ParseIP("192.168.1.1"),
				Port: 8080,
			},
			expected: "192.168.1.1:8080",
		},
		{
			name: "FQDN fallback",
			addr: &AddrSpec{
				FQDN: "example.com",
				Port: 443,
			},
			expected: "example.com:443",
		},
		{
			name: "IPv6 address",
			addr: &AddrSpec{
				IP:   net.ParseIP("2001:db8::1"),
				Port: 80,
			},
			expected: "[2001:db8::1]:80",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.addr.Address()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRequest_ParseRequest_IPv4(t *testing.T) {
	// Create SOCKS5 CONNECT request for 192.168.1.1:8080
	// Header: version(0x05) + command(0x01) + reserved(0x00)
	// Address: type(0x01) + ipv4(192.168.1.1) + port(8080)
	requestData := []byte{
		0x05, 0x01, 0x00,           // Header
		0x01,                       // IPv4 address type
		192, 168, 1, 1,            // IPv4 address
		0x1f, 0x90,                // Port 8080 (0x1f90)
	}

	conn := newMockConn(requestData)
	req := &Request{}

	err := req.ParseRequest(conn)
	require.NoError(t, err)

	assert.Equal(t, uint8(0x05), req.Version)
	assert.Equal(t, uint8(0x01), req.Command)
	assert.NotNil(t, req.DestAddr)
	assert.Equal(t, "192.168.1.1", req.DestAddr.IP.String())
	assert.Equal(t, 8080, req.DestAddr.Port)
}

func TestRequest_ParseRequest_FQDN(t *testing.T) {
	// Create SOCKS5 CONNECT request for example.com:443
	// Header: version(0x05) + command(0x01) + reserved(0x00)
	// Address: type(0x03) + len(11) + fqdn(example.com) + port(443)
	requestData := []byte{
		0x05, 0x01, 0x00,           // Header
		0x03,                       // FQDN address type
		11,                         // FQDN length
	}
	requestData = append(requestData, []byte("example.com")...)
	requestData = append(requestData, []byte{0x01, 0xbb}...) // Port 443 (0x01bb)

	conn := newMockConn(requestData)
	req := &Request{}

	err := req.ParseRequest(conn)
	require.NoError(t, err)

	assert.Equal(t, uint8(0x05), req.Version)
	assert.Equal(t, uint8(0x01), req.Command)
	assert.NotNil(t, req.DestAddr)
	assert.Equal(t, "example.com", req.DestAddr.FQDN)
	assert.Equal(t, 443, req.DestAddr.Port)
}

func TestRequest_ParseRequest_IPv6(t *testing.T) {
	// Create SOCKS5 CONNECT request for [2001:db8::1]:80
	// Header: version(0x05) + command(0x01) + reserved(0x00)
	// Address: type(0x04) + ipv6(2001:db8::1) + port(80)
	requestData := []byte{
		0x05, 0x01, 0x00,           // Header
		0x04,                       // IPv6 address type
		0x20, 0x01, 0x0d, 0xb8,     // IPv6 address 2001:db8::1
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x01,
		0x00, 0x50,                 // Port 80 (0x0050)
	}

	conn := newMockConn(requestData)
	req := &Request{}

	err := req.ParseRequest(conn)
	require.NoError(t, err)

	assert.Equal(t, uint8(0x05), req.Version)
	assert.Equal(t, uint8(0x01), req.Command)
	assert.NotNil(t, req.DestAddr)
	assert.Equal(t, "2001:db8::1", req.DestAddr.IP.String())
	assert.Equal(t, 80, req.DestAddr.Port)
}

func TestRequest_ParseRequest_InvalidVersion(t *testing.T) {
	// Create request with invalid version
	requestData := []byte{0x04, 0x01, 0x00} // SOCKS4 version

	conn := newMockConn(requestData)
	req := &Request{}

	err := req.ParseRequest(conn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported proxy version")
}

func TestRequest_ParseRequest_InvalidAddressType(t *testing.T) {
	// Create request with invalid address type
	requestData := []byte{
		0x05, 0x01, 0x00,           // Header
		0x05,                       // Invalid address type
	}

	conn := newMockConn(requestData)
	req := &Request{}

	err := req.ParseRequest(conn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unrecognized address type")
}

func TestReadAddrSpec_InsufficientData(t *testing.T) {
	// Test with insufficient data for IPv4
	data := []byte{0x01, 192, 168} // IPv4 type but incomplete address

	reader := bytes.NewReader(data)
	_, err := readAddrSpec(reader)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "read ipv4 add error")
}