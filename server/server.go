package server

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"os"

	"github.com/easzlab/ezvpn/config"
	"github.com/easzlab/ezvpn/logger"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.uber.org/zap"
)

// Start starts tunneling server with given configuration.
func Start() error {
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogProtocol: true,
		LogMethod:   true,
		LogLatency:  true,
		LogRemoteIP: true,
		LogURI:      true,
		LogStatus:   true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			logger.Echo.Info("request",
				zap.String("proto", v.Protocol),
				zap.String("URI", v.URI),
				zap.Int("status", v.Status),
				zap.String("latency", v.Latency.String()),
				zap.String("remote", v.RemoteIP),
			)

			return nil
		},
	}))
	e.Use(middleware.Recover())

	e.GET("/register/:key", GetRegister)

	s := http.Server{
		Addr:    config.SERVER.ControlAddress,
		Handler: e,
	}

	logger.Server.Debug("running ezvpn server",
		zap.String("reason", ""),
		zap.String("remote", ""),
		zap.String("version", config.FullVersion()),
		zap.String("address", config.SERVER.ControlAddress))

	if config.SERVER.EnableTLS {
		// load CA certificate file
		caCertFile, err := os.ReadFile(config.SERVER.CaFile)
		if err != nil {
			logger.Server.Fatal("failed to load CA certificate", zap.Error(err))
		}
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(caCertFile)

		s.TLSConfig = &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  certPool,
			MinVersion: tls.VersionTLS12,
		}
		return s.ListenAndServeTLS(config.SERVER.CertFile, config.SERVER.KeyFile)
	} else {
		return s.ListenAndServe()
	}
}
