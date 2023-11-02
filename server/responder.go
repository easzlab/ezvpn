package server

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/easzlab/ezvpn/config"
	"github.com/easzlab/ezvpn/logger"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/panjf2000/ants/v2"
	"github.com/xtaci/smux"
	"go.uber.org/zap"
)

// Upgrader is the websocket upgrader.
var upgrader = websocket.Upgrader{}

// wg and ants.Pool is used to manage goroutines.
var wg sync.WaitGroup
var pool *ants.Pool

// init initializes ants.Pool.
func init() {
	var err error
	pool, err = ants.NewPool(config.GoroutinePoolSize)
	if err != nil {
		logger.Server.Fatal("error init ants pool", zap.String("reason", err.Error()))
	}
}

// Error responds to client with an error. The error is logged and translated
// to proper HTTP status response.
func Error(c echo.Context, status int, err error) error {
	logger.Server.Warn("request error",
		zap.String("reason", err.Error()),
		zap.String("remote", c.RealIP()),
		zap.Int("status", status))

	return c.JSON(status, map[string]string{"error": err.Error()})
}

// WebSocket starts websocket session. The handler is invoked in a goroutine.
func WebSocket(c echo.Context, handler func(ws *websocket.Conn) error) error {
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return Error(c, http.StatusBadRequest, err)
	}

	go func() {
		var closeMessage []byte

		if err := handler(ws); err != nil {
			closeMessage = websocket.FormatCloseMessage(
				websocket.ClosePolicyViolation, "error: "+err.Error(),
			)
		} else {
			closeMessage = websocket.FormatCloseMessage(
				websocket.CloseNormalClosure, "",
			)
		}

		ws.WriteControl(
			websocket.CloseMessage, closeMessage, time.Now().Add(config.WsCloseTimeout),
		)
		ws.Close()
	}()

	return nil
}

// GetRegister
func GetRegister(c echo.Context) error {
	key := c.Param("key")

	for _, agent := range config.AGENTS.Agents {
		// check if the key is valid
		if key == agent.AuthKey {
			if c.Request().TLS == nil { // TLS is disabled
				logger.Server.Debug("agent registered",
					zap.String("reason", ""),
					zap.String("remote", c.RealIP()),
					zap.String("name", agent.Name))
				return WebSocket(c, func(ws *websocket.Conn) error {
					return startSmuxSessionFromWS(ws)
				})
			} else { // TLS is enabled
				if len(c.Request().TLS.PeerCertificates) > 0 {
					cn := c.Request().TLS.PeerCertificates[0].Subject.CommonName
					for _, approved_cn := range agent.ApprovedCNs {
						if cn == approved_cn {
							logger.Server.Debug("agent registered",
								zap.String("reason", ""),
								zap.String("remote", c.RealIP()),
								zap.String("name", agent.Name))
							return WebSocket(c, func(ws *websocket.Conn) error {
								return startSmuxSessionFromWS(ws)
							})
						}
					}
				}
			}
		}
	}
	err := fmt.Errorf("failed to register: invalid auth key(%s) or cert CN", key)
	return Error(c, http.StatusUnauthorized, err)
}

// startSmuxSessionFromWS starts smux session from underlying websocket, and tunnel the accepted stream to the socks server
func startSmuxSessionFromWS(ws *websocket.Conn) error {
	// Setup server side of smux
	cfg := smux.DefaultConfig()
	cfg.KeepAliveInterval = config.SmuxSessionKeepAliveInterval
	cfg.KeepAliveTimeout = config.SmuxSessionKeepAliveTimeout

	session, err := smux.Server(ws.UnderlyingConn(), cfg)
	if err != nil {
		logger.Server.Warn("failed to setup smux session",
			zap.String("reason", err.Error()),
			zap.String("remote", ws.RemoteAddr().String()))
		return err
	}
	defer session.Close()

	for {
		// Accept a stream
		stream, err := session.AcceptStream()
		if err != nil {
			logger.Server.Warn("failed to accept smux stream",
				zap.String("reason", err.Error()),
				zap.String("remote", ws.RemoteAddr().String()))
			return err
		}
		defer stream.Close()

		go tunnel(stream)
	}
}

// tunnel tunnels stream to the socks server
func tunnel(stream *smux.Stream) error {

	defer stream.Close()

	// connection to the socks5 server.
	conn, err := net.DialTimeout("tcp", config.SERVER.SocksServer, config.NetDialTimeout)
	if err != nil {
		logger.Server.Warn("failed to connect the socks server",
			zap.String("reason", err.Error()),
			zap.String("remote", stream.RemoteAddr().String()),
			zap.Int("id", int(stream.ID())))
		return err
	}
	defer conn.Close()

	errCh := make(chan error, 2)

	// uplink: Agent --smux stream--> Server --conn--> (socks server)
	uplink := func() {
		buf := make([]byte, config.BufferSize)
		proxy(conn, stream, buf, errCh)
		wg.Done()
	}

	// Downlink: Agent <--smux stream-- Server <--conn--(socks server)
	downlink := func() {
		buf := make([]byte, config.BufferSize)
		proxy(stream, conn, buf, errCh)
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
				logger.Server.Debug("tunnel finished",
					zap.String("reason", err.Error()),
					zap.String("remote", stream.RemoteAddr().String()),
					zap.Int("id", int(stream.ID())))
				return nil
			}

			if errors.Is(err, net.ErrClosed) {
				logger.Server.Debug("tunnel closed",
					zap.String("reason", err.Error()),
					zap.String("remote", stream.RemoteAddr().String()),
					zap.Int("id", int(stream.ID())))
				return nil
			}

			logger.Server.Warn("tunnel killed",
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
