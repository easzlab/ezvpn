package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/easzlab/ezvpn/internal/config"
	"github.com/easzlab/ezvpn/pkg/utils/logger"
	"github.com/gorilla/websocket"
	"github.com/panjf2000/ants/v2"
	"github.com/xtaci/smux"
	"go.uber.org/zap"
)

// AgentConfig is the configuration of agent.
type AgentConfig struct {
	AuthKey       string // auth key when registering to the server
	ServerAddress string // server address, e.g. 127.0.0.1:443

	CaFile       string // ca file
	CertFile     string // cert file used in mtls
	KeyFile      string // key file used in mtls
	LocalAddress string // local address to listen on for local socks proxy service
	LockFile     string // lock file to prevent multiple agent instances
	LogFile      string
	LogLevel     string
	EnableTLS    bool // enable mtls
}

// Agent tunnels local socks stream to the server.
type Agent struct {
	config    *AgentConfig
	pool      *ants.Pool
	logger    *zap.Logger
	dialer    *websocket.Dialer
	cleanOnce sync.Once
}

func NewAgent(cfg *AgentConfig) (*Agent, error) {
	// Create ants pool
	pool, err := ants.NewPool(config.GoroutinePoolSize, ants.WithNonblocking(true))
	if err != nil {
		return nil, fmt.Errorf("error init ants pool: %w", err)
	}

	// Create logger
	logger, err := logger.NewLogger(cfg.LogFile, cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Create websocket dialer
	dialer := websocket.DefaultDialer
	if cfg.EnableTLS {
		cert, err := os.ReadFile(cfg.CaFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read ca file: %w", err)
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(cert)

		certificate, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load x509 key pair: %w", err)
		}

		dialer.TLSClientConfig = &tls.Config{
			InsecureSkipVerify:       true,
			MinVersion:               tls.VersionTLS13,
			RootCAs:                  caCertPool,
			PreferServerCipherSuites: true,
			Certificates:             []tls.Certificate{certificate},
		}
	}
	dialer.HandshakeTimeout = config.WsHandshakeTimeout

	return &Agent{
		config: cfg,
		pool:   pool,
		logger: logger,
		dialer: dialer,
	}, nil
}

// CleanUp stops the agent, and do cleanup work
func (a *Agent) CleanUp() {
	a.cleanOnce.Do(func() {
		a.logger.Info("starting clean up")

		// 1. Stop accepting new tasks in the pool
		a.pool.Release()

		// 2. make sure the logger entries are flushed to disk
		a.logger.Sync()
	})
}

// Start starts the agent with retry mechanism
func (a *Agent) Start(ctx context.Context) error {
	var lastErr error

	// Retry mechanism
	for attempt := 1; attempt <= config.AgentMaxRetry; attempt++ {
		lastErr = a.register(ctx)
		if lastErr == nil || errors.Is(lastErr, context.Canceled) {
			return lastErr
		}

		a.logger.Warn("register failed",
			zap.Error(lastErr),
			zap.String("retry", fmt.Sprintf("%d/%d", attempt, config.AgentMaxRetry)),
		)

		// Exponential backoff
		backoff := time.Duration(attempt) * time.Second
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}
	}

	return fmt.Errorf("register failed after %d attempts: %w", config.AgentMaxRetry, lastErr)
}

// register creates a vpn tunnel, and listens on a local socks port to serve
func (a *Agent) register(ctx context.Context) error {
	// 1. Connection to the ezvpn server
	var url string
	if a.config.EnableTLS {
		url = "wss://" + a.config.ServerAddress + "/register/" + a.config.AuthKey
	} else {
		url = "ws://" + a.config.ServerAddress + "/register/" + a.config.AuthKey
	}

	header := http.Header{}
	header.Add("Agent", "ezvpn-agent@easzlab")
	ws, resp, err := a.dialer.DialContext(ctx, url, header)
	if err != nil {
		statusCode := 0
		if resp != nil {
			statusCode = resp.StatusCode
		}
		a.logger.Warn("websocket connection failed",
			zap.String("url", url),
			zap.Error(err),
			zap.Int("status_code", statusCode))
		return err
	}
	defer closeWebsocket(ws)

	// 2. Setup client side of smux
	cfg := smux.DefaultConfig()
	cfg.KeepAliveInterval = config.SmuxSessionKeepAliveInterval
	cfg.KeepAliveTimeout = config.SmuxSessionKeepAliveTimeout

	session, err := smux.Client(ws.UnderlyingConn(), cfg)
	if err != nil {
		return fmt.Errorf("failed to create smux client: %w", err)
	}
	defer session.Close()

	// 3. Listen on local port
	lc := net.ListenConfig{}
	ln, err := lc.Listen(ctx, "tcp", a.config.LocalAddress)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", a.config.LocalAddress, err)
	}
	defer ln.Close()

	a.logger.Info("listening for connections",
		zap.String("address", a.config.LocalAddress))

	errCh := make(chan error, 2)

	// Start health check goroutine
	go a.healthCheckConn(ctx, session, errCh)

	// 4. Accept and handle local connections
	go a.acceptConnections(ctx, ln, session, errCh)

	return <-errCh
}

// healthCheck monitors the smux session health
func (a *Agent) healthCheckConn(ctx context.Context, session *smux.Session, errCh chan error) {
	ticker := time.NewTicker(config.SmuxSessionKeepAliveInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		case <-ticker.C:
			if session.IsClosed() {
				errCh <- fmt.Errorf("smux session closed")
				return
			}
		}
	}
}

// acceptConnections accepts and handles incoming connections
func (a *Agent) acceptConnections(ctx context.Context, ln net.Listener, session *smux.Session, errCh chan error) {
	for {
		select {
		case <-ctx.Done():
			errCh <- ctx.Err()
			return
		default:
		}

		conn, err := ln.Accept()
		if err != nil {
			errCh <- fmt.Errorf("failed to accept connection: %w", err)
			return
		}

		// Handle connection in a separate goroutine
		a.pool.Submit(func() {
			a.handleConnection(ctx, conn, session)
		})
	}
}

// handleConnection handles a single connection
func (a *Agent) handleConnection(ctx context.Context, conn net.Conn, session *smux.Session) {
	defer conn.Close()

	stream, err := session.OpenStream()
	if err != nil {
		a.logger.Warn("open stream failed",
			zap.Error(err),
			zap.String("remote", conn.RemoteAddr().String()))
		return
	}
	defer stream.Close()

	a.tunnel(ctx, conn, stream)
}

// tunnel proxies a local connection(socks protocol) to a remote server via smux stream.
// The tunnel can be canceled via context, it looks like this:
// (socks client) <--conn--> Agent <--smux stream--> Server <--conn--> (socks server) <--> Destination
func (a *Agent) tunnel(ctx context.Context, conn net.Conn, stream *smux.Stream) error {
	a.logger.Debug("start tunneling",
		zap.String("local", conn.RemoteAddr().String()),
		zap.String("remote", stream.RemoteAddr().String()),
		zap.Uint32("id", stream.ID()))

	// Monitor context cancellation and close connections
	go func() {
		<-ctx.Done()
		conn.Close()
		stream.Close()
	}()

	errCh := make(chan error, 2)

	// uplink: (socks client) --conn--> Agent --smux stream--> Server
	uplink := func() {
		buf := make([]byte, config.BufferSize)
		proxy(stream, conn, buf, errCh)
	}

	// Downlink: (socks client) <--conn-- Agent <--smux stream-- Server
	downlink := func() {
		buf := make([]byte, config.BufferSize)
		proxy(conn, stream, buf, errCh)
	}

	if err := a.pool.Submit(uplink); err != nil {
		return fmt.Errorf("failed to submit uplink task: %w", err)
	}

	if err := a.pool.Submit(downlink); err != nil {
		return fmt.Errorf("failed to submit downlink task: %w", err)
	}

	// wait for uplink and downlink to finish or emit error
	for i := 0; i < 2; i++ {
		err := <-errCh
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
				a.logger.Debug("tunnel closed",
					zap.String("remote", stream.RemoteAddr().String()),
					zap.Int("id", int(stream.ID())))
				return nil
			}

			a.logger.Warn("tunnel killed",
				zap.String("remote", stream.RemoteAddr().String()),
				zap.Error(err))

			return err
		}
	}

	return nil
}

// proxy is used to shuffle data from src to destination, and sends errors
// down a dedicated channel
func proxy(dst net.Conn, src net.Conn, buf []byte, errCh chan error) {
	_, err := io.CopyBuffer(dst, src, buf)
	dst.Close()
	errCh <- err
}

// closeWebsocket attempts to close a websocket session normally. It is ok to
// call this function on a connection that has already been closed by the peer.
func closeWebsocket(ws *websocket.Conn) {
	ws.WriteControl(
		websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(config.WsCloseTimeout),
	)
	ws.Close()
}
