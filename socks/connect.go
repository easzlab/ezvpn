package socks

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"

	"github.com/easzlab/ezvpn/config"
)

// CMD CONNECT
func (s *Server) handleConnect(ctx context.Context, conn net.Conn, req *Request) error {
	cli := conn.RemoteAddr().String()

	// Try to connect the destination
	target, err := net.DialTimeout("tcp", req.DestAddr.Address(), config.NetDialTimeout)
	if err != nil {
		SendReply(conn, hostUnreachable, nil)
		errcon := fmt.Errorf("connect to target failed: %v", err)
		log.Printf("target unreachable, error: %s, client: %s, target: %s", errcon.Error(), cli, req.DestAddr.String())
		return errcon
	}
	defer target.Close()

	// Send success
	local, ok := target.LocalAddr().(*net.TCPAddr)
	if !ok {
		msg := fmt.Sprintf("expect *net.TCPAddr, not %t", target.LocalAddr())
		log.Printf("unknown type, error: %s, client: %s, target: %s", msg, cli, req.DestAddr.String())
	}
	bind := AddrSpec{IP: local.IP, Port: local.Port}
	if err := SendReply(conn, successReply, &bind); err != nil {
		log.Printf("failed to send reply, error: %s, client: %s, target: %s", err.Error(), cli, req.DestAddr.String())
		return fmt.Errorf("failed to send reply: %v", err)
	}

	// Start proxying
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
