package server

import (
	"crypto/tls"
	"crypto/x509"
	"log"
	"net/http"
	"os"

	"github.com/easzlab/ezvpn/config"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// Start starts tunneling server with given configuration.
func Start() error {
	e := echo.New()
	e.HideBanner = true

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/register/:key", GetRegister)
	e.GET("/session/:key", GetSession)

	s := http.Server{
		Addr:    config.SERVER.ControlAddress,
		Handler: e,
	}

	if config.SERVER.EnableTLS {
		// load CA certificate file
		caCertFile, err := os.ReadFile(config.SERVER.CaFile)
		if err != nil {
			log.Fatalf("error reading CA certificate: %v", err)
		}
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(caCertFile)

		s.TLSConfig = &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  certPool,
			MinVersion: tls.VersionTLS12,
		}
		log.Println("ezvpn server is running on: " + config.SERVER.ControlAddress)
		return s.ListenAndServeTLS(config.SERVER.CertFile, config.SERVER.KeyFile)
	} else {
		log.Println("ezvpn server is running on: " + config.SERVER.ControlAddress)
		return s.ListenAndServe()
	}
}
