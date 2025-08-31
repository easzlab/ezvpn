package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/easzlab/ezvpn/internal/config"
	"github.com/easzlab/ezvpn/pkg/server"
	"github.com/easzlab/ezvpn/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	enableInlineSocks bool
	enablePprof       bool
	enableTLS         bool
	showVersion       bool

	agentsFile    string
	listenAddress string
	socksServer   string
	caFile        string
	certFile      string
	keyFile       string
	logFile       string
	logLevel      string
)

func init() {
	rootCmd.Flags().BoolVarP(&enablePprof, "pprof", "", false, "enable pprof")
	rootCmd.Flags().BoolVarP(&enableTLS, "tls", "", true, "enable tls")
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "show version")
	rootCmd.Flags().BoolVarP(&enableInlineSocks, "withsocks", "", true, "enable inline socks")

	rootCmd.Flags().StringVarP(&agentsFile, "agentsfile", "a", "agents.yaml", "allowed agents file")
	rootCmd.Flags().StringVarP(&listenAddress, "listen", "l", "127.0.0.1:8443", "server listen address")
	rootCmd.Flags().StringVarP(&socksServer, "socks", "", "socks.sock", "socks server address: using a unix socket addr for inline socks, using a tcp addr for external socks")
	rootCmd.Flags().StringVarP(&caFile, "ca", "", "ca.pem", "ca file")
	rootCmd.Flags().StringVarP(&certFile, "cert", "", "evs.pem", "cert file")
	rootCmd.Flags().StringVarP(&keyFile, "key", "", "evs-key.pem", "key file")
	rootCmd.Flags().StringVarP(&logFile, "logfile", "", "evs.log", "log file")
	rootCmd.Flags().StringVarP(&logLevel, "loglvl", "", "debug", "log level")
}

var rootCmd = &cobra.Command{
	Use:   "evs",
	Short: "ezvpn is a vpn over secure socks protocol, evs is the server side",
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println(config.FullVersion())
			return nil
		}

		if enablePprof {
			go http.ListenAndServe("0.0.0.0:6062", nil)
		}

		if err := serverDefault(); err != nil {
			return err
		}

		cfg := server.ServerConfig{
			AgentsFile:        agentsFile,
			ListenAddress:     listenAddress,
			CaFile:            caFile,
			CertFile:          certFile,
			KeyFile:           keyFile,
			LogFile:           logFile,
			LogLevel:          logLevel,
			SocksServer:       socksServer,
			EnableTLS:         enableTLS,
			EnableInlineSocks: enableInlineSocks,
		}

		evs, err := server.NewServer(&cfg)
		if err != nil {
			return err
		}

		if enableInlineSocks {
			go evs.StartSocksServer()
		}

		return evs.Start()
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func main() {
	Execute()
}

func serverDefault() error {
	if err := utils.EnsureFile(caFile, config.DefaultCa); err != nil {
		return err
	}
	if err := utils.EnsureFile(certFile, config.DefaultServerCert); err != nil {
		return err
	}
	if err := utils.EnsureFile(keyFile, config.DefaultServerKey); err != nil {
		return err
	}
	if err := utils.EnsureFile(agentsFile, config.DefaultAgentsFile); err != nil {
		return err
	}

	return nil
}
