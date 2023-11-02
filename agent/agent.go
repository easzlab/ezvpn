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

	"github.com/easzlab/ezvpn/config"
	"github.com/easzlab/ezvpn/logger"
	"github.com/gorilla/websocket"
	"github.com/panjf2000/ants/v2"
	"github.com/xtaci/smux"
	"go.uber.org/zap"
)

// dialer is the websocket dialer used to connect to the server.
var dialer = websocket.DefaultDialer

// wg and ants.Pool is used to manage goroutines.
var wg sync.WaitGroup
var pool *ants.Pool

// init initializes ants.Pool.
func init() {
	var err error
	pool, err = ants.NewPool(config.GoroutinePoolSize)
	if err != nil {
		logger.Agent.Fatal("error init ants pool", zap.String("reason", err.Error()))
	}
}

// Agent tunnels local socks stream to the server.
type Agent struct {
	AuthKey       string
	ServerAddress string
	EnableTLS     bool
	EnablePprof   bool
	ShowVersion   bool
	CaFile        string
	CertFile      string
	KeyFile       string
	LocalAddress  string
	LockFile      string
	LogFile       string
	LogLevel      string
}

// Start starts
func (agent *Agent) Start(ctx context.Context) error {
	if agent.EnableTLS {
		cert, err := os.ReadFile(agent.CaFile)
		if err != nil {
			return err
		}

		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(cert)

		certificate, err := tls.LoadX509KeyPair(agent.CertFile, agent.KeyFile)
		if err != nil {
			return err
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

	errCh := make(chan error)

	go func(c chan error) {
		// TODO: 用指数退避算法重试
		retry := time.NewTicker(config.AgentRetryInterval)
		defer retry.Stop()
		for {
			err := agent.register(ctx)
			if err == nil || !isRecoverable(err) {
				c <- err
				return
			}
			logger.Agent.Warn("agent recovering...", zap.String("reason", err.Error()))
			<-retry.C
		}
	}(errCh)

	return <-errCh
}

// Returns true if agent error is recoverable (by restarting agent).
func isRecoverable(err error) bool {
	return !errors.Is(err, context.Canceled)
}

// hookCancel launches a goroutine for handling task cancellation. handler is
// called upon cancellation. The caller must invoke returned function after the
// cancelable task is finished.
func hookCancel(ctx context.Context, handler func()) func() {
	end := make(chan struct{})
	unhook := func() {
		close(end)
	}
	go func() {
		select {
		case <-ctx.Done():
			handler()
		case <-end:
		}
	}()
	return unhook
}

// register create a vpn tunnel, and listen a local socks port to serve
func (agent *Agent) register(ctx context.Context) error {

	// 1.Connection to the ezvpn server.
	var url string
	if agent.EnableTLS {
		url = "wss://" + agent.ServerAddress + "/register/" + agent.AuthKey
	} else {
		url = "ws://" + agent.ServerAddress + "/register/" + agent.AuthKey
	}

	header := http.Header{}
	header.Add("Agent", "ezvpn-agent@easzlab")
	ws, resp, err := dialer.DialContext(ctx, url, header)
	if err != nil {
		if err == websocket.ErrBadHandshake {
			logger.Agent.Warn("handshake failed", zap.String("reason", err.Error()), zap.Int("status", resp.StatusCode))
		}
		return err
	}
	defer closeWebsocket(ws)

	unhookCancel := hookCancel(ctx, func() {
		closeWebsocket(ws)
	})
	defer unhookCancel()

	// 2. Listen on local port.
	ln, err := net.Listen("tcp", agent.LocalAddress)
	if err != nil {
		return err
	}
	defer ln.Close()

	logger.Agent.Info("listen successfully",
		zap.String("reason", ""),
		zap.String("address", agent.LocalAddress))

	// 3. Setup client side of smux
	cfg := smux.DefaultConfig()
	cfg.KeepAliveInterval = config.SmuxSessionKeepAliveInterval
	cfg.KeepAliveTimeout = config.SmuxSessionKeepAliveTimeout

	session, err := smux.Client(ws.UnderlyingConn(), cfg)
	if err != nil {
		return err
	}
	defer session.Close()

	errCh := make(chan error, 2)

	// health check of the smux session
	go func(c chan error) {
		retry := time.NewTicker(config.SmuxSessionKeepAliveInterval)
		defer retry.Stop()
		for {
			if session.IsClosed() {
				err := fmt.Errorf("error: broken session with the server")
				c <- err
				return
			}
			<-retry.C
		}
	}(errCh)

	// 4. tunnel accepted local connection
	go func(c chan error) {
		for {
			conn, err := ln.Accept()
			if err != nil {
				c <- err
				return
			}
			defer conn.Close()

			go func() {
				stream, err := session.OpenStream()
				if err != nil {
					logger.Agent.Warn("error open a new stream",
						zap.String("reason", err.Error()),
						zap.String("remote", ""))
					return
				}
				defer stream.Close()
				if err := agent.tunnel(ctx, conn, stream); err != nil {
					logger.Agent.Warn("tunneling error",
						zap.String("reason", err.Error()),
						zap.String("remote", stream.RemoteAddr().String()),
						zap.Int("id", int(stream.ID())))
				}
			}()
		}
	}(errCh)

	return <-errCh
}

// tunnel proxies a local connection(socks protocol) to a remote server via smux stream.
// The tunnel can be canceled via context, it looks like this:
// (socks client) <--conn--> Agent <--smux stream--> Server <--conn--> (socks server) <--> Destination
func (agent *Agent) tunnel(ctx context.Context, conn net.Conn, stream *smux.Stream) error {

	defer conn.Close()
	defer stream.Close()

	logger.Agent.Info("start tunneling",
		zap.String("reason", ""),
		zap.String("remote", stream.RemoteAddr().String()),
		zap.Int("id", int(stream.ID())))

	// Cancelable.
	unhookCancel := hookCancel(ctx, func() {
		conn.Close()
		stream.Close()
	})
	defer unhookCancel()

	errCh := make(chan error, 2)

	// uplink: (socks client) --conn--> Agent --smux stream--> Server
	uplink := func() {
		buf := make([]byte, config.BufferSize)
		proxy(stream, conn, buf, errCh)
		wg.Done()
	}

	// Downlink: (socks client) <--conn-- Agent <--smux stream-- Server
	downlink := func() {
		buf := make([]byte, config.BufferSize)
		proxy(conn, stream, buf, errCh)
		wg.Done()
	}

	wg.Add(1)
	pool.Submit(uplink)

	wg.Add(1)
	pool.Submit(downlink)

	// wait for uplink and downlink to finish or emit error
	for i := 0; i < 2; i++ {
		err := <-errCh
		if err != nil {
			if errors.Is(err, io.EOF) {
				logger.Agent.Info("tunnel finished",
					zap.String("reason", err.Error()),
					zap.String("remote", stream.RemoteAddr().String()),
					zap.Int("id", int(stream.ID())))
				return nil
			}

			if errors.Is(err, net.ErrClosed) {
				logger.Agent.Info("tunnel closed",
					zap.String("reason", err.Error()),
					zap.String("remote", stream.RemoteAddr().String()),
					zap.Int("id", int(stream.ID())))
				return nil
			}

			if errors.Is(err, io.ErrClosedPipe) {
				logger.Agent.Info("tunnel closed",
					zap.String("reason", err.Error()),
					zap.String("remote", stream.RemoteAddr().String()),
					zap.Int("id", int(stream.ID())))
				return nil
			}

			logger.Agent.Warn("tunnel killed",
				zap.String("reason", err.Error()),
				zap.String("remote", stream.RemoteAddr().String()),
				zap.Int("id", int(stream.ID())))

			return err
		}
	}

	return nil
}

// proxy is used to suffle data from src to destination, and sends errors
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
