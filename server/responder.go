package server

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/easzlab/ezvpn/config"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
	"github.com/panjf2000/ants/v2"
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
		log.Fatal(err)
	}
}

// Error responds to client with an error. The error is logged and translated
// to proper HTTP status response.
func Error(c echo.Context, status int, err error) error {
	c.Logger().Error(err)
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
			c.Logger().Error(err)

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

func GetRegister(c echo.Context) error {
	key := c.Param("key")
	if key != "" {
		for _, agent := range config.AGENTS.Agents {
			// check if the key is valid
			if key == agent.AuthKey {
				// check CN if mTLS is enabled
				if c.Request().TLS != nil && len(c.Request().TLS.PeerCertificates) > 0 {
					cn := c.Request().TLS.PeerCertificates[0].Subject.CommonName
					for _, approved_cn := range agent.ApprovedCNs {
						if cn == approved_cn {
							log.Printf("agent %s@%s registered", agent.Name, c.RealIP())
							return WebSocket(c, func(ws *websocket.Conn) error {
								return readFromWS(ws)
							})
						}
					}
				}
			}
		}
	}
	err := fmt.Errorf("failed to register: invalid auth key: %s", key)
	return Error(c, http.StatusUnauthorized, err)
}

func GetSession(c echo.Context) error {
	key := c.Param("key")
	if key != "" {
		for _, agent := range config.AGENTS.Agents {
			// check if the key is valid
			if key == agent.AuthKey {
				// check CN if mTLS is enabled
				if c.Request().TLS != nil && len(c.Request().TLS.PeerCertificates) > 0 {
					cn := c.Request().TLS.PeerCertificates[0].Subject.CommonName
					for _, approved_cn := range agent.ApprovedCNs {
						if cn == approved_cn {
							log.Printf("agent %s@%s registered", agent.Name, c.RealIP())
							return WebSocket(c, func(ws *websocket.Conn) error {
								return tunnel(ws)
							})
						}
					}
				}
			}
		}
	}
	err := fmt.Errorf("failed to establish session: invalid auth key: %s", key)
	return Error(c, http.StatusUnauthorized, err)
}

// readFromWS reads from ws continuously, the agent side will send the ws ping message regularly
func readFromWS(ws *websocket.Conn) error {
	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			return err
		}
	}
}

func tunnel(ws *websocket.Conn) error {
	// connection to the socks5 server.
	conn, err := net.DialTimeout("tcp", config.SERVER.SocksServer, config.NetDialTimeout)
	if err != nil {
		return err
	}
	defer conn.Close()

	errCh := make(chan error, 2)

	// uplink: Agent --ws--> Server --conn--> (socks server)
	uplink := func() {
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

	// Downlink: Agent <--ws-- Server <--conn--(socks server)
	downlink := func() {
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

	wg.Add(1)
	pool.Submit(uplink)

	wg.Add(1)
	pool.Submit(downlink)

	// wait for uplink and downlink to finish or emit error
	for i := 0; i < 2; i++ {
		err := <-errCh
		if err != nil {
			if errors.Is(err, io.EOF) {
				log.Println("Destination closed. Finishing session")
				return nil
			}

			if errors.Is(err, net.ErrClosed) {
				log.Println("Canceled. Finishing session")
				return nil
			}

			if websocket.IsCloseError(err, websocket.CloseNormalClosure) {
				log.Println("Tunnel closed. Finishing session")
				return nil
			}

			log.Printf("Error %q. Killing session", err)

			return err
		}
	}

	return nil

}
