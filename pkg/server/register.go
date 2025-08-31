package server

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/xtaci/smux"
	"go.uber.org/zap"

	"github.com/easzlab/ezvpn/internal/config"
)

// GetRegister
func (s *Server) GetRegister(c echo.Context) error {
	key := c.Param("key")

	for _, agent := range s.agents.Agents {
		// check if the key is valid
		if key == agent.AuthKey {
			if c.Request().TLS == nil { // TLS is disabled
				s.logger.Debug("agent registered",
					zap.String("client", c.RealIP()),
					zap.String("name", agent.Name))
				return s.startWebSocket(c, func(ws *websocket.Conn) error {
					return s.startSmuxSessionFromWS(ws)
				})
			} else { // TLS is enabled
				if len(c.Request().TLS.PeerCertificates) > 0 {
					cn := c.Request().TLS.PeerCertificates[0].Subject.CommonName
					if cn == agent.ApprovedCN {
						s.logger.Debug("agent registered",
							zap.String("client", c.RealIP()),
							zap.String("name", agent.Name))
						return s.startWebSocket(c, func(ws *websocket.Conn) error {
							return s.startSmuxSessionFromWS(ws)
						})
					}
				}
			}
		}
	}
	err := fmt.Errorf("failed to register: invalid auth key(%s) or cert CN", key)
	return handleError(c, s.logger, http.StatusUnauthorized, err)
}

// startWebSocket starts websocket session. The handler is invoked in a goroutine.
func (s *Server) startWebSocket(c echo.Context, handler func(ws *websocket.Conn) error) error {
	upgrader := websocket.Upgrader{}
	ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return handleError(c, s.logger, http.StatusBadRequest, err)
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

// startSmuxSessionFromWS starts smux session from underlying websocket, and tunnel the accepted stream to the socks server
func (s *Server) startSmuxSessionFromWS(ws *websocket.Conn) error {
	// Setup server side of smux
	cfg := smux.DefaultConfig()
	cfg.KeepAliveInterval = config.SmuxSessionKeepAliveInterval
	cfg.KeepAliveTimeout = config.SmuxSessionKeepAliveTimeout

	session, err := smux.Server(ws.UnderlyingConn(), cfg)
	if err != nil {
		s.logger.Warn("failed to setup smux session",
			zap.String("client", ws.RemoteAddr().String()),
			zap.Error(err))
		return err
	}
	defer session.Close()

	for {
		// Accept a stream
		stream, err := session.AcceptStream()
		if err != nil {
			s.logger.Warn("failed to accept smux stream",
				zap.String("client", ws.RemoteAddr().String()),
				zap.Error(err))
			return err
		}

		s.pool.Submit(func() {
			s.tunnel(stream)
		})
	}
}

// tunnel tunnels stream to the socks server
func (s *Server) tunnel(stream *smux.Stream) error {
	defer stream.Close()

	var conn net.Conn
	var err error

	// connection to the socks5 server.
	if s.config.EnableInlineSocks {
		// use unix domain socket
		conn, err = net.Dial("unix", s.config.SocksServer)
	} else {
		// use tcp
		conn, err = net.DialTimeout("tcp", s.config.SocksServer, config.NetDialTimeout)
	}
	if err != nil {
		s.logger.Warn("failed to connect the socks server",
			zap.String("client", stream.RemoteAddr().String()),
			zap.Error(err),
			zap.Int("id", int(stream.ID())))
		return err
	}
	defer conn.Close()

	errCh := make(chan error, 2)

	// uplink: Agent --smux stream--> Server --conn--> (socks server)
	uplink := func() {
		buf := make([]byte, config.BufferSize)
		proxy(conn, stream, buf, errCh)
	}

	// Downlink: Agent <--smux stream-- Server <--conn--(socks server)
	downlink := func() {
		buf := make([]byte, config.BufferSize)
		proxy(stream, conn, buf, errCh)
	}

	if err := s.pool.Submit(uplink); err != nil {
		return fmt.Errorf("failed to submit uplink task: %w", err)
	}

	if err := s.pool.Submit(downlink); err != nil {
		return fmt.Errorf("failed to submit downlink task: %w", err)
	}

	// wait for uplink and downlink to finish or emit error
	for i := 0; i < 2; i++ {
		err := <-errCh
		if err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) || errors.Is(err, io.ErrClosedPipe) {
				s.logger.Debug("tunnel finished",
					zap.String("client", stream.RemoteAddr().String()),
					zap.Int("id", int(stream.ID())))
				return nil
			}

			s.logger.Warn("tunnel killed",
				zap.String("client", stream.RemoteAddr().String()),
				zap.Error(err))
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

// handleError responds to client with an error. The error is logged and translated
// to proper HTTP status response.
func handleError(c echo.Context, logger *zap.Logger, status int, err error) error {
	logger.Warn("request error",
		zap.String("client", c.RealIP()),
		zap.Error(err),
		zap.Int("status", status))

	return c.JSON(status, map[string]string{"error": err.Error()})
}
