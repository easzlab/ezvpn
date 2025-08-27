package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"

	"github.com/easzlab/ezvpn/internal/config"
	"github.com/easzlab/ezvpn/pkg/agent"
	"github.com/easzlab/ezvpn/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	authKey       string
	serverAddress string
	caFile        string
	certFile      string
	keyFile       string
	localAddress  string
	lockFile      string
	logFile       string
	logLevel      string
	enableTLS     bool
	enablePprof   bool
	showVersion   bool
)

func init() {
	rootCmd.Flags().BoolVarP(&enablePprof, "enable-pprof", "", false, "enable pprof")
	rootCmd.Flags().BoolVarP(&enableTLS, "enable-tls", "", true, "enable tls")
	rootCmd.Flags().BoolVarP(&showVersion, "version", "", false, "show version")
	rootCmd.Flags().StringVarP(&authKey, "auth-key", "k", "", "auth key")
	rootCmd.Flags().StringVarP(&serverAddress, "server-addr", "s", "127.0.0.1:443", "server address")
	rootCmd.Flags().StringVarP(&caFile, "ca", "", "ca.pem", "ca file")
	rootCmd.Flags().StringVarP(&certFile, "cert", "", "eva.pem", "cert file")
	rootCmd.Flags().StringVarP(&keyFile, "key", "", "eva-key.pem", "key file")
	rootCmd.Flags().StringVarP(&localAddress, "local-addr", "l", "127.0.0.1:6116", "local address")
	rootCmd.Flags().StringVarP(&lockFile, "lock-file", "", "eva.lock", "lock file")
	rootCmd.Flags().StringVarP(&logFile, "log-file", "", "eva.log", "log file")
	rootCmd.Flags().StringVarP(&logLevel, "log-level", "", "debug", "log level")

	rootCmd.MarkFlagRequired("auth-key")
}

var rootCmd = &cobra.Command{
	Use:   "eva",
	Short: "eva is a vpn agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		if showVersion {
			fmt.Println(config.FullVersion())
			return nil
		}

		if enablePprof {
			go http.ListenAndServe("0.0.0.0:6061", nil)
		}

		if err := agentDefault(); err != nil {
			return err
		}

		cfg := agent.AgentConfig{
			AuthKey:       authKey,
			ServerAddress: serverAddress,
			CaFile:        caFile,
			CertFile:      certFile,
			KeyFile:       keyFile,
			LocalAddress:  localAddress,
			LockFile:      lockFile,
			LogFile:       logFile,
			LogLevel:      logLevel,
			EnableTLS:     enableTLS,
		}

		eva, err := agent.NewAgent(&cfg)
		if err != nil {
			return err
		}

		return eva.Run()
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

func agentDefault() error {
	if err := utils.EnsureFile(caFile, config.DefaultCa); err != nil {
		return err
	}
	if err := utils.EnsureFile(certFile, config.DefaultAgentCert); err != nil {
		return err
	}
	if err := utils.EnsureFile(keyFile, config.DefaultAgentKey); err != nil {
		return err
	}

	return nil
}
