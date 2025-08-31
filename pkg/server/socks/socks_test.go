package socks

import (
	"testing"

	"github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewSocksServer(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	pool, err := ants.NewPool(100)
	require.NoError(t, err)
	defer pool.Release()

	addr := "/tmp/test.sock"
	ss := NewSocksServer(addr, logger, pool)

	assert.NotNil(t, ss)
	assert.Equal(t, addr, ss.addr)
	assert.Equal(t, logger, ss.logger)
	assert.Equal(t, pool, ss.pool)
}

func TestSocksServer_SocksService(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	pool, err := ants.NewPool(100)
	require.NoError(t, err)
	defer pool.Release()

	ss := NewSocksServer("/tmp/test.sock", logger, pool)

	// Test with nil connection should handle gracefully
	err = ss.SocksService(nil)
	assert.Error(t, err) // Should error due to nil connection
}

func TestSocksConstants(t *testing.T) {
	// Test SOCKS5 protocol constants
	assert.Equal(t, uint8(0x05), socks5Version)
	assert.Equal(t, uint8(0x00), noAuthRequired)
	assert.Equal(t, uint8(0xff), noAcceptableMethods)
	assert.Equal(t, uint8(0x01), connectCommand)
	assert.Equal(t, uint8(0x01), ipv4Address)
	assert.Equal(t, uint8(0x03), fqdnAddress)
	assert.Equal(t, uint8(0x04), ipv6Address)
	assert.Equal(t, uint8(0x00), successReply)
	assert.Equal(t, uint8(0x04), hostUnreachable)
	assert.Equal(t, uint8(0x7), commandNotSupported)
	assert.Equal(t, uint8(0x08), addrTypeNotSupported)
}