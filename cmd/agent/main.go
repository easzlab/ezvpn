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
	rootCmd.Flags().BoolVarP(&enablePprof, "pprof", "", false, "enable pprof")
	rootCmd.Flags().BoolVarP(&enableTLS, "tls", "", true, "enable tls")
	rootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "show version")
	rootCmd.Flags().StringVarP(&authKey, "auth", "k", "", "auth key")
	rootCmd.Flags().StringVarP(&serverAddress, "server", "s", "127.0.0.1:8443", "server address")
	rootCmd.Flags().StringVarP(&caFile, "ca", "", "ca.pem", "ca file")
	rootCmd.Flags().StringVarP(&certFile, "cert", "", "eva.pem", "cert file")
	rootCmd.Flags().StringVarP(&keyFile, "key", "", "eva-key.pem", "key file")
	rootCmd.Flags().StringVarP(&localAddress, "listen", "l", "127.0.0.1:6116", "local address")
	rootCmd.Flags().StringVarP(&lockFile, "lock", "", "eva.lock", "lock file")
	rootCmd.Flags().StringVarP(&logFile, "logfile", "", "eva.log", "log file")
	rootCmd.Flags().StringVarP(&logLevel, "loglvl", "", "debug", "log level")

	rootCmd.MarkFlagRequired("auth-key")
}

var rootCmd = &cobra.Command{
	Use:   "eva",
	Short: "ezvpn is a vpn over secure socks protocol, eva is the agent side",
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
