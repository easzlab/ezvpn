package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/easzlab/ezvpn/config"
	"github.com/easzlab/ezvpn/logger"
	"github.com/easzlab/ezvpn/server"
	"github.com/easzlab/ezvpn/socks"
	"go.uber.org/zap"
)

func main() {
	s := config.Server{}
	flag.BoolVar(&s.EnableTLS, "tls", true, "enable tls between agent and server")
	flag.BoolVar(&s.EnablePprof, "pprof", false, "enable pprof")
	flag.BoolVar(&s.EnableInlineSocks, "withsocks", true, "enable the inline socks server")
	flag.BoolVar(&s.ShowVersion, "version", false, "version of the server")
	flag.StringVar(&s.ControlAddress, "listen", ":8443", "control address")
	flag.StringVar(&s.ConfigFile, "config", "config/allowed-agents.yml", "allowed-agents config file")
	flag.StringVar(&s.CaFile, "ca", "ca.pem", "trusted ca")
	flag.StringVar(&s.CertFile, "cert", "server.pem", "server cert file")
	flag.StringVar(&s.KeyFile, "key", "server-key.pem", "server key file")
	flag.StringVar(&s.LogFile, "logfile", "logs/server.log", "server log file")
	flag.StringVar(&s.LogLevel, "loglvl", "debug", "server log level")
	flag.StringVar(&s.SocksServer, "socksaddr", "socks.sock", "socks server，for inline socks using unix socket addr, for external socks using tcp addr")
	flag.Parse()

	if s.ShowVersion {
		fmt.Println(config.FullVersion())
		os.Exit(0)
	}

	logger.InitServerLogger(s.LogFile, s.LogLevel)
	logger.InitEchoLogger("logs/echo.log", s.LogLevel)

	// load configuration
	config.SERVER = s
	config.SERVER.HotReload()

	if s.EnablePprof {
		go http.ListenAndServe("0.0.0.0:6060", nil)
	}

	// run socks server
	if s.EnableInlineSocks {
		socksServer := socks.Server{ListenAddr: s.SocksServer}
		go socksServer.Run()
	}

	// run ezvpn server
	if err := server.Start(); err != nil {
		logger.Server.Error("server error", zap.String("reason", err.Error()))
		os.Exit(1)
	}
}
