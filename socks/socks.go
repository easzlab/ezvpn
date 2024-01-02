package socks

import (
	"context"
	"fmt"
	"net"
	"os"

	"github.com/easzlab/ezvpn/config"
	"github.com/easzlab/ezvpn/logger"
	"go.uber.org/zap"
)

const (
	socks5Version = uint8(0x05)
	// Authentication methods
	noAuthRequired      = uint8(0x00)
	noAcceptableMethods = uint8(0xff)
	// Requests CMD
	connectCommand   = uint8(0x01)
	bindCommand      = uint8(0x02)
	associateCommand = uint8(0x03)
	// Requests Addressing
	ipv4Address = uint8(0x01)
	fqdnAddress = uint8(0x03)
	ipv6Address = uint8(0x04)
	// Replies REP
	successReply         = uint8(0x00)
	ruleBlocked          = uint8(0x02)
	hostUnreachable      = uint8(0x04)
	commandNotSupported  = uint8(0x7)
	addrTypeNotSupported = uint8(0x08)
)

type Server struct {
	ListenAddr string
}

func (s *Server) Run() error {
	var l net.Listener
	var err error

	if config.SERVER.EnableInlineSocks {
		// use unix socket, first to remove old socket
		if err = os.RemoveAll(s.ListenAddr); err != nil {
			logger.Server.Warn("failed to remove old socket",
				zap.String("reason", err.Error()),
				zap.String("address", s.ListenAddr))
			return err
		}
		l, err = net.Listen("unix", s.ListenAddr)
	} else {
		// use tcp
		l, err = net.Listen("tcp", s.ListenAddr)
	}

	if err != nil {
		logger.Server.Warn("failed to listen",
			zap.String("reason", err.Error()),
			zap.String("address", s.ListenAddr))
		return err
	}

	defer l.Close()

	logger.Server.Debug("running socks server",
		zap.String("reason", ""),
		zap.String("version", config.FullVersion()),
		zap.String("address", s.ListenAddr))

	for {
		c, err := l.Accept()
		if err != nil {
			logger.Server.Warn("failed to accept connection",
				zap.String("reason", err.Error()))
			return err
		}
		// A goroutine for each client connection
		go s.SocksService(c)
	}
}

func (s *Server) SocksService(conn net.Conn) error {
	defer conn.Close()

	// Handle Authenticate handshake
	if err := s.HandleAuth(conn); err != nil {
		return err
	}

	// Handle client requests
	return s.HandleRequest(conn)
}

func (s *Server) HandleRequest(conn net.Conn) error {
	// Parse requests
	r := &Request{}
	cli := conn.RemoteAddr().String()

	if err := r.ParseRequest(conn); err != nil {
		logger.Server.Warn("failed to parse request",
			zap.String("reason", err.Error()),
			zap.String("remote", cli),
			zap.String("target", ""))
		SendReply(conn, addrTypeNotSupported, nil)
		return err
	}

	ctx := context.Background()

	// Switch on the command, only CONNECT command supported
	switch r.Command {
	case connectCommand:
		return s.handleConnect(ctx, conn, r)
	default:
		SendReply(conn, commandNotSupported, nil)
		logger.Server.Warn("unsupported command",
			zap.String("reason", ""),
			zap.String("remote", cli),
			zap.String("target", r.DestAddr.String()))
		return fmt.Errorf("unsupported command: %v", r.Command)
	}
}
