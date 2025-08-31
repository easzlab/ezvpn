package socks

import (
	"fmt"
	"io"
	"net"

	"github.com/easzlab/ezvpn/internal/config"
	"go.uber.org/zap"
)

// CMD CONNECT
func (ss *SocksServer) handleConnect(conn net.Conn, req *Request) error {
	cli := conn.RemoteAddr().String()

	// Try to connect the destination
	target, err := net.DialTimeout("tcp", req.DestAddr.Address(), config.NetDialTimeout)
	if err != nil {
		SendReply(conn, hostUnreachable, nil)
		ss.logger.Warn("target unreachable",
			zap.Error(err),
			zap.String("remote", cli),
			zap.String("target", req.DestAddr.String()))
		return err
	}
	defer target.Close()

	// Send success
	local, ok := target.LocalAddr().(*net.TCPAddr)
	if !ok {
		err := fmt.Errorf("expect *net.TCPAddr, not %t", target.LocalAddr())
		ss.logger.Warn("cannot bind a valid local address",
			zap.String("remote", cli),
			zap.String("target", req.DestAddr.String()),
			zap.Error(err))
		return err
	}

	bind := AddrSpec{IP: local.IP, Port: local.Port}
	if err := SendReply(conn, successReply, &bind); err != nil {
		ss.logger.Warn("failed to send reply",
			zap.Error(err),
			zap.String("remote", cli),
			zap.String("target", req.DestAddr.String()))
		return fmt.Errorf("failed to send reply: %v", err)
	}

	// Start proxying
	ss.logger.Debug("start proxying",
		zap.String("remote", cli),
		zap.String("target", req.DestAddr.String()))

	errCh := make(chan error, 2)

	errDownlink := ss.pool.Submit(func() {
		proxy(target, conn, errCh)
	})

	errUplink := ss.pool.Submit(func() {
		proxy(conn, target, errCh)
	})

	if errDownlink != nil || errUplink != nil {
		errSubmit := fmt.Errorf("failed to submit proxy task: %w|%w", errDownlink, errUplink)
		ss.logger.Warn("failed to submit proxy task",
			zap.Error(errSubmit),
			zap.String("remote", cli),
			zap.String("target", req.DestAddr.String()))
		return errSubmit
	}

	// Wait
	for i := 0; i < 2; i++ {
		err := <-errCh
		if err != nil {
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
