package socks

import (
	"bufio"
	"fmt"
	"io"
	"net"

	"github.com/easzlab/ezvpn/logger"
	"go.uber.org/zap"
)

func (s *Server) HandleAuth(conn net.Conn) error {
	cli := conn.RemoteAddr().String()

	// auth-check
	reply, err := authReply(conn)
	if err != nil {
		logger.Server.Warn("auth failed ", zap.String("reason", err.Error()), zap.String("remote", cli))
		return err
	}

	if n, e := conn.Write([]byte{0x05, reply}); e != nil || n != 2 {
		senderr := fmt.Errorf("failed to send method selection reply: %v", e)
		logger.Server.Warn("auth failed ", zap.String("reason", senderr.Error()), zap.String("remote", cli))
		return senderr
	}

	return nil
}

func authReply(conn net.Conn) (uint8, error) {
	bufConn := bufio.NewReader(conn)
	reply := noAcceptableMethods

	// Read the version byte
	version := []byte{0}
	if _, err := bufConn.Read(version); err != nil {
		return reply, fmt.Errorf("failed to get version byte: %v", err)
	}

	// Ensure socks5 version
	if version[0] != socks5Version {
		return reply, fmt.Errorf("unsupported socks version: %v", version)
	}

	// Read the NMETHODS byte
	numMethods := []byte{0}
	if _, err := bufConn.Read(numMethods); err != nil {
		return reply, fmt.Errorf("failed to get nmethods byte: %v", err)
	}
	nMethods := int(numMethods[0])

	// Read the METHODS bytes
	methods := make([]byte, nMethods)
	if _, err := io.ReadAtLeast(bufConn, methods, nMethods); err != nil {
		return reply, fmt.Errorf("failed to get methods bytes: %v", err)
	}

	// Only Support NoAuth method right now
	for _, m := range methods {
		if m == noAuthRequired {
			reply = noAuthRequired
			break
		}
	}

	if reply == noAcceptableMethods {
		return reply, fmt.Errorf("no acceptable methods")
	}

	return reply, nil
}
