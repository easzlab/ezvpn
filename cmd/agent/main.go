package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"time"

	"github.com/easzlab/ezvpn/agent"
	"github.com/easzlab/ezvpn/config"
)

func main() {
	a := agent.Agent{}
	flag.StringVar(&a.AuthKey, "auth", "xxx", "Specify the authentication key")
	flag.BoolVar(&a.EnableTLS, "tls", true, "To enable tls between agent and server or not")
	flag.BoolVar(&a.EnablePprof, "pprof", false, "To enable pprof or not")
	flag.StringVar(&a.CaFile, "ca", "./ca.pem", "Specify the trusted ca file")
	flag.StringVar(&a.CertFile, "cert", "./agent.pem", "Specify the agent cert file")
	flag.StringVar(&a.KeyFile, "key", "./agent-key.pem", "Specify the agent key file")
	flag.StringVar(&a.LocalAddress, "local", ":16116", "Specify the local address")
	flag.StringVar(&a.ServerAddress, "server", "127.0.0.1:8443", "Specify the server address")
	flag.Parse()

	if a.EnablePprof {
		go http.ListenAndServe("0.0.0.0:6061", nil)
	}

	if err := run(a); err != nil {
		log.Printf("error: %s", err)
		os.Exit(1)
	}
}

func run(a agent.Agent) error {
	ctx := withSignalCancel(context.Background())
	err := a.Start(ctx)

	if errors.Is(err, context.Canceled) {
		log.Printf("the agent proc is canceled, waiting for it to stop...")
		time.Sleep(config.AgentCancelWait)
		return nil
	}

	return err
}

func withSignalCancel(ctx context.Context) context.Context {
	newCtx, cancel := context.WithCancel(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()
	return newCtx
}
