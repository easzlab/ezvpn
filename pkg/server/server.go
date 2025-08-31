package server

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/panjf2000/ants/v2"
	"github.com/spf13/viper"
	"go.uber.org/zap"

	"github.com/easzlab/ezvpn/internal/config"
	"github.com/easzlab/ezvpn/pkg/server/socks"
	"github.com/easzlab/ezvpn/pkg/utils/logger"
)

type ServerConfig struct {
	AgentsFile    string
	ListenAddress string
	CaFile        string
	CertFile      string
	KeyFile       string
	LogFile       string
	LogLevel      string
	SocksServer   string

	EnableTLS         bool
	EnableInlineSocks bool
}

type AllowedAgents struct {
	Agents []struct {
		Name       string `mapstructure:"name"`
		AuthKey    string `mapstructure:"auth_key"`
		ApprovedCN string `mapstructure:"approved_cn"`
	}
}

type Server struct {
	agents *AllowedAgents
	config *ServerConfig
	logger *zap.Logger
	pool   *ants.Pool

	cleanOnce sync.Once
}

func NewServer(cfg *ServerConfig) (*Server, error) {
	// Create logger
	logger, err := logger.NewLogger(cfg.LogFile, cfg.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	// Create ants pool
	pool, err := ants.NewPool(3*config.GoroutinePoolSize, ants.WithNonblocking(true))
	if err != nil {
		return nil, fmt.Errorf("error init ants pool: %w", err)
	}

	agents := AllowedAgents{}
	viper.SetConfigFile(cfg.AgentsFile)

	if err = viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read agents file: %w", err)
	}

	if err = viper.Unmarshal(&agents); err != nil {
		return nil, fmt.Errorf("failed to unmarshal agents file: %w", err)
	}

	viper.WatchConfig()

	viper.OnConfigChange(func(e fsnotify.Event) {
		logger.Info("agents file changed, reloading", zap.String("name", e.Name))

		tmp_agents := AllowedAgents{}
		if err := viper.Unmarshal(&tmp_agents); err != nil {
			logger.Warn("failed to unmarshal new agents file", zap.Error(err))
		} else {
			if err := check(&tmp_agents); err != nil {
				logger.Warn("failed to recognize new agents file", zap.Error(err))
			} else {
				agents = tmp_agents
			}
		}
	})

	return &Server{
		agents: &agents,
		config: cfg,
		logger: logger,
		pool:   pool,
	}, nil
}

// basic check for the allowed agents
func check(a *AllowedAgents) error {
	if a == nil {
		return fmt.Errorf("nil pointer")
	}
	for _, agent := range a.Agents {
		if agent.Name == "" {
			return fmt.Errorf("empty name")
		}
		if agent.AuthKey == "" {
			return fmt.Errorf("empty auth key")
		}
		if agent.ApprovedCN == "" {
			return fmt.Errorf("empty approved cn")
		}
	}
	return nil
}

// CleanUp do cleanup work when the server is stopped
func (s *Server) CleanUp() {
	s.cleanOnce.Do(func() {
		s.logger.Info("starting clean up")

		// 1. Stop accepting new tasks in the pool
		s.pool.Release()

		// 2. make sure the logger entries are flushed to disk
		s.logger.Sync()
	})
}

func (s *Server) Start() error {
	e := echo.New()
	e.HideBanner = true

	// 使用自定义日志中间件
	e.Use(ZapLoggerMiddleware(s.logger))
	e.Use(middleware.Recover())

	e.GET("/register/:key", s.GetRegister)

	server := http.Server{
		Addr:    s.config.ListenAddress,
		Handler: e,
	}

	s.logger.Debug("running ezvpn server",
		zap.String("address", s.config.ListenAddress),
		zap.String("version", config.FullVersion()),
	)

	var serverErr error
	if s.config.EnableTLS {
		caCertFile, err := os.ReadFile(s.config.CaFile)
		if err != nil {
			s.logger.Fatal("failed to load CA certificate", zap.Error(err))
			return fmt.Errorf("failed to load CA certificate: %w", err)
		}
		certPool := x509.NewCertPool()
		certPool.AppendCertsFromPEM(caCertFile)

		server.TLSConfig = &tls.Config{
			ClientAuth: tls.RequireAndVerifyClientCert,
			ClientCAs:  certPool,
			MinVersion: tls.VersionTLS12,
		}
		serverErr = server.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile)
	} else {
		serverErr = server.ListenAndServe()
	}
	s.CleanUp()
	return serverErr
}

func (s *Server) StartSocksServer() error {
	ss := socks.NewSocksServer(s.config.SocksServer, s.logger, s.pool)
	return ss.Run()
}
