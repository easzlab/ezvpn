package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/easzlab/ezvpn/config"
	"github.com/easzlab/ezvpn/server"
	"github.com/easzlab/ezvpn/socks"
)

func main() {
	s := config.Server{}
	flag.BoolVar(&s.EnableTLS, "tls", true, "To enable tls between agent and server or not")
	flag.BoolVar(&s.EnablePprof, "pprof", false, "To enable pprof or not")
	flag.StringVar(&s.ControlAddress, "listen", ":8443", "Specify the control address")
	flag.StringVar(&s.ConfigFile, "config", "./config/allowed-agents.yml", "Specify the config file")
	flag.StringVar(&s.CaFile, "ca", "./ca.pem", "Specify the trusted ca file")
	flag.StringVar(&s.CertFile, "cert", "./server.pem", "Specify the server cert file")
	flag.StringVar(&s.KeyFile, "key", "./server-key.pem", "Specify the server key file")
	flag.StringVar(&s.SocksServer, "socks5", "0.0.0.0:6116", "Specify the socks server address")
	flag.Parse()

	// load configuration
	config.SERVER = s
	config.SERVER.HotReload()

	if s.EnablePprof {
		go http.ListenAndServe("0.0.0.0:6060", nil)
	}

	// run socks server
	socksServer := socks.Server{ListenAddr: s.SocksServer}
	go socksServer.Run()

	// run ezvpn server
	if err := server.Start(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
