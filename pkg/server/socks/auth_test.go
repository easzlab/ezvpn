package socks

import (
	"bytes"
	"net"
	"testing"
	"time"

	"github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// mockConn implements net.Conn for testing
type mockConn struct {
	readBuf  *bytes.Buffer
	writeBuf *bytes.Buffer
}

func newMockConn(readData []byte) *mockConn {
	return &mockConn{
		readBuf:  bytes.NewBuffer(readData),
		writeBuf: &bytes.Buffer{},
	}
}

func (m *mockConn) Read(b []byte) (n int, err error) {
	return m.readBuf.Read(b)
}

func (m *mockConn) Write(b []byte) (n int, err error) {
	return m.writeBuf.Write(b)
}

func (m *mockConn) Close() error                       { return nil }
func (m *mockConn) LocalAddr() net.Addr                { return nil }
func (m *mockConn) RemoteAddr() net.Addr               { return nil }
func (m *mockConn) SetDeadline(t time.Time) error      { return nil }
func (m *mockConn) SetReadDeadline(t time.Time) error  { return nil }
func (m *mockConn) SetWriteDeadline(t time.Time) error { return nil }

func TestSocksServer_HandleAuth_Success(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	pool, err := ants.NewPool(10)
	require.NoError(t, err)
	defer pool.Release()

	ss := NewSocksServer("/tmp/test.sock", logger, pool)

	// Create valid SOCKS5 auth request: version(0x05) + nmethods(0x01) + method(0x00)
	authData := []byte{0x05, 0x01, 0x00}
	conn := newMockConn(authData)

	err = ss.HandleAuth(conn)
	assert.NoError(t, err)

	// Check response: version(0x05) + method(0x00)
	expected := []byte{0x05, 0x00}
	assert.Equal(t, expected, conn.writeBuf.Bytes())
}

func TestSocksServer_HandleAuth_InvalidVersion(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	pool, err := ants.NewPool(10)
	require.NoError(t, err)
	defer pool.Release()

	ss := NewSocksServer("/tmp/test.sock", logger, pool)

	// Create invalid SOCKS version request: version(0x04) + nmethods(0x01) + method(0x00)
	authData := []byte{0x04, 0x01, 0x00}
	conn := newMockConn(authData)

	err = ss.HandleAuth(conn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported socks version")
}

func TestSocksServer_HandleAuth_NoAcceptableMethods(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	pool, err := ants.NewPool(10)
	require.NoError(t, err)
	defer pool.Release()

	ss := NewSocksServer("/tmp/test.sock", logger, pool)

	// Create request with unsupported auth method: version(0x05) + nmethods(0x01) + method(0x02)
	authData := []byte{0x05, 0x01, 0x02}
	conn := newMockConn(authData)

	err = ss.HandleAuth(conn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no acceptable methods")
}

func TestSocksServer_HandleAuth_MultipleMethodsWithValid(t *testing.T) {
	logger, err := zap.NewDevelopment()
	require.NoError(t, err)

	pool, err := ants.NewPool(10)
	require.NoError(t, err)
	defer pool.Release()

	ss := NewSocksServer("/tmp/test.sock", logger, pool)

	// Create request with multiple methods including valid one:
	// version(0x05) + nmethods(0x03) + methods(0x01, 0x00, 0x02)
	authData := []byte{0x05, 0x03, 0x01, 0x00, 0x02}
	conn := newMockConn(authData)

	err = ss.HandleAuth(conn)
	assert.NoError(t, err)

	// Should select the valid method (0x00)
	expected := []byte{0x05, 0x00}
	assert.Equal(t, expected, conn.writeBuf.Bytes())
}

func TestAuthReply_InsufficientData(t *testing.T) {
	// Test with insufficient data
	authData := []byte{0x05} // Only version, missing nmethods and methods
	conn := newMockConn(authData)

	reply, err := authReply(conn)
	assert.Error(t, err)
	assert.Equal(t, noAcceptableMethods, reply)
}
