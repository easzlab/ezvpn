package socks

import (
	"context"
	"fmt"
	"io"
	"net"

	"github.com/easzlab/ezvpn/config"
	"github.com/easzlab/ezvpn/logger"
	"go.uber.org/zap"
)

// CMD CONNECT
func (s *Server) handleConnect(ctx context.Context, conn net.Conn, req *Request) error {
	cli := conn.RemoteAddr().String()

	// Try to connect the destination
	target, err := net.DialTimeout("tcp", req.DestAddr.Address(), config.NetDialTimeout)
	if err != nil {
		SendReply(conn, hostUnreachable, nil)
		errcon := fmt.Errorf("connect to target failed: %v", err)
		logger.Server.Warn("target unreachable",
			zap.String("reason", errcon.Error()),
			zap.String("remote", cli),
			zap.String("target", req.DestAddr.String()))
		return errcon
	}
	defer target.Close()

	// Send success
	local, ok := target.LocalAddr().(*net.TCPAddr)
	if !ok {
		msg := fmt.Sprintf("expect *net.TCPAddr, not %t", target.LocalAddr())
		logger.Server.Warn("unknown addr type",
			zap.String("reason", msg),
			zap.String("remote", cli),
			zap.String("target", req.DestAddr.String()))
	}
	bind := AddrSpec{IP: local.IP, Port: local.Port}
	if err := SendReply(conn, successReply, &bind); err != nil {
		logger.Server.Warn("failed to send reply",
			zap.String("reason", err.Error()),
			zap.String("remote", cli),
			zap.String("target", req.DestAddr.String()))
		return fmt.Errorf("failed to send reply: %v", err)
	}

	// Start proxying
	logger.Server.Debug("start proxying",
		zap.String("reason", ""),
		zap.String("remote", cli),
		zap.String("target", req.DestAddr.String()))

	errCh := make(chan error, 2)
	go proxy(target, conn, errCh)
	go proxy(conn, target, errCh)

	// Wait
	for i := 0; i < 2; i++ {
		err := <-errCh
		if err != nil {
			// return from this function closes target (and conn).
			return err
		}
	}
	return nil
}

// proxy is used to suffle data from src to destination, and sends errors
// down a dedicated channel
func proxy(dst net.Conn, src net.Conn, errCh chan error) {
	_, err := io.Copy(dst, src)
	dst.Close()
	errCh <- err
}
