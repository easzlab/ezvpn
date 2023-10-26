package agent

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/easzlab/ezvpn/config"
	"github.com/gorilla/websocket"
	"github.com/panjf2000/ants/v2"
)

// dialer is the websocket dialer used to connect to a gateway server.
var dialer = websocket.DefaultDialer

// Agent tunnels remote port on a gateway server to local destination.
type Agent struct {
	AuthKey       string
	ServerAddress string
	EnableTLS     bool
	EnablePprof   bool
	CaFile        string
	CertFile      string
	KeyFile       string
	LocalAddress  string
	LockFile      string
}

// wg and ants.Pool is used to manage goroutines.
var wg sync.WaitGroup
var pool *ants.Pool

// init initializes ants.Pool.
func init() {
	var err error
	pool, err = ants.NewPool(config.GoroutinePoolSize)
	if err != nil {
		log.Fatal(err)
	}
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

	go (func(c chan error) {
		// TODO: 用指数退避算法重试
		retry := time.NewTicker(config.AgentRetryInterval)
		defer retry.Stop()
		for {
			err := agent.register(ctx)
			if err == nil || !isRecoverable(err) {
				c <- err
				return
			}
			log.Printf("Agent error %q - recovering...", err)
			<-retry.C
		}
	})(errCh)

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

// Reg
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
			log.Printf("handshake failed with status %d", resp.StatusCode)
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

	log.Printf("Listening on: %s", agent.LocalAddress)

	// Forcifully close connection if the server does not respond to ping.
	go Watch(ws, config.WsKeepliveInterval, ln.Close)

	go func() {
		for {
			// Server does not send message to this channel in the current
			// protocol, but it is required to drain the channel to check
			// for ping responses.
			if _, _, err := ws.NextReader(); err != nil {
				break
			}
		}
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			return err
		}

		go func() {
			if err := agent.tunnel(conn, ctx); err != nil {
				log.Printf("Tunneling error: %s", err)
			}
		}()
	}
}

// tunnel proxies a local connection(socks protocol) to a remote server via websocket.
// The tunnel can be canceled via context, it looks like this:
// (socks client) <--conn--> Agent <--ws--> Server <--conn--> (socks server) <--> Destination
func (agent *Agent) tunnel(conn net.Conn, ctx context.Context) error {

	defer conn.Close()

	log.Printf("Tunneling local connection from %s", conn.RemoteAddr())

	// Remote connection proxied through WebSocket.
	var url string
	if agent.EnableTLS {
		url = "wss://" + agent.ServerAddress + "/session/" + agent.AuthKey
	} else {
		url = "ws://" + agent.ServerAddress + "/session/" + agent.AuthKey
	}
	ws, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return err
	}
	defer closeWebsocket(ws)

	// Cancelable.
	unhookCancel := hookCancel(ctx, func() {
		conn.Close()
		closeWebsocket(ws)
	})
	defer unhookCancel()

	errCh := make(chan error, 2)

	// uplink: (socks client) --conn--> Agent --ws--> Server
	uplink := func() {
		func(c chan error) {
			buf := make([]byte, config.BufferSize)

			for {
				n, err := conn.Read(buf)
				if err != nil {
					c <- err
					return
				}

				if err := ws.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
					c <- err
					return
				}
			}
		}(errCh)

		wg.Done()
	}

	// Downlink: (socks client) <--conn-- Agent <--ws-- Server
	downlink := func() {
		func(c chan error) {
			buf := make([]byte, config.BufferSize)

			for {
				_, r, err := ws.NextReader()
				if err != nil {
					c <- err
					return
				}

				if _, err := io.CopyBuffer(conn, r, buf); err != nil {
					c <- err
					return
				}
			}
		}(errCh)

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
				log.Printf("Client %s closed normally. Closing session.", conn.RemoteAddr())
				return nil
			}

			if errors.Is(err, net.ErrClosed) {
				log.Printf("Canceled. Finishing session from %s", conn.RemoteAddr())
				return nil
			}

			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				log.Printf("Session closed normally. Closing client %s.", conn.RemoteAddr())
				return nil
			}

			log.Printf("Error %q. Killing session %s.", err, conn.RemoteAddr())

			return err
		}
	}

	return nil
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

// Watch watches for broken WebSocket connection. This function periodically
// sends ping message to the websocket peer and invokes `handler` on first
// timeout. The caller must continuously read something from `ws` to allow
// pong messages to be received.
func Watch(ws *websocket.Conn, timeout time.Duration, handler func() error) error {
	ticker := time.NewTicker(timeout)
	defer ticker.Stop()

	pong := make(chan bool)
	ws.SetPongHandler(func(_ string) error {
		pong <- true
		return nil
	})

	for tick := range ticker.C {
		if err := ws.WriteControl(websocket.PingMessage, []byte(""), tick.Add(timeout)); err != nil {
			break
		}

		select {
		case <-pong:
			continue

		case <-ticker.C:
		}
		break
	}

	return handler()
}
