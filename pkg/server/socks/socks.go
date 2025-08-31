package socks

import (
	"fmt"
	"net"
	"os"

	"github.com/easzlab/ezvpn/internal/config"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
)

const (
	socks5Version = uint8(0x05)
	// Authentication methods
	noAuthRequired      = uint8(0x00)
	noAcceptableMethods = uint8(0xff)
	// Requests CMD
	connectCommand = uint8(0x01)
	//bindCommand      = uint8(0x02)
	//associateCommand = uint8(0x03)
	// Requests Addressing
	ipv4Address = uint8(0x01)
	fqdnAddress = uint8(0x03)
	ipv6Address = uint8(0x04)
	// Replies REP
	successReply = uint8(0x00)
	//ruleBlocked          = uint8(0x02)
	hostUnreachable      = uint8(0x04)
	commandNotSupported  = uint8(0x7)
	addrTypeNotSupported = uint8(0x08)
)

type SocksServer struct {
	addr   string
	pool   *ants.Pool
	logger *zap.Logger
}

func NewSocksServer(addr string, logger *zap.Logger, pool *ants.Pool) *SocksServer {
	return &SocksServer{
		addr:   addr,
		pool:   pool,
		logger: logger,
	}
}

func (ss *SocksServer) Run() error {
	// use unix socket, first to remove old socket
	if err := os.RemoveAll(ss.addr); err != nil {
		ss.logger.Warn("failed to remove old socket",
			zap.String("address", ss.addr),
			zap.Error(err),
		)
		return err
	}
	l, err := net.Listen("unix", ss.addr)

	if err != nil {
		ss.logger.Warn("failed to listen",
			zap.String("address", ss.addr),
			zap.Error(err),
		)
		return err
	}

	defer l.Close()

	ss.logger.Debug("running socks server",
		zap.String("address", ss.addr),
		zap.String("version", config.FullVersion()),
	)

	for {
		c, err := l.Accept()
		if err != nil {
			ss.logger.Warn("failed to accept connection",
				zap.Error(err))
			return err
		}
		// A goroutine for each client connection
		ss.pool.Submit((func() {
			ss.SocksService(c)
		}))
	}
}

func (ss *SocksServer) SocksService(conn net.Conn) error {
	defer conn.Close()

	// Handle Authenticate handshake
	if err := ss.HandleAuth(conn); err != nil {
		ss.logger.Warn("failed to handle auth",
			zap.String("client", conn.RemoteAddr().String()),
			zap.Error(err),
		)
		return err
	}

	return ss.HandleRequest(conn)
}

// Handle client requests
func (ss *SocksServer) HandleRequest(conn net.Conn) error {
	// Parse requests
	r := &Request{}

	if err := r.ParseRequest(conn); err != nil {
		ss.logger.Warn("failed to parse request",
			zap.String("client", conn.RemoteAddr().String()),
			zap.Error(err),
		)
		SendReply(conn, addrTypeNotSupported, nil)
		return err
	}

	// Switch on the command, only CONNECT command supported
	switch r.Command {
	case connectCommand:
		return ss.handleConnect(conn, r)
	default:
		SendReply(conn, commandNotSupported, nil)
		ss.logger.Warn("unsupported command",
			zap.String("client", conn.RemoteAddr().String()),
			zap.String("target", r.DestAddr.String()))
		return fmt.Errorf("unsupported command: %v", r.Command)
	}
}
