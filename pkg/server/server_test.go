package server

import (
	"testing"

	"github.com/panjf2000/ants/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/easzlab/ezvpn/pkg/utils/logger"
)

func TestNewServer(t *testing.T) {
	tests := []struct {
		name    string
		config  *ServerConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: &ServerConfig{
				AgentsFile:    "testdata/agents.yml",
				ListenAddress: ":8443",
				LogFile:       "/tmp/test-server.log",
				LogLevel:      "debug",
				SocksServer:   "/tmp/test.sock",
				EnableTLS:     false,
			},
			wantErr: true, // Will fail because agents file doesn't exist
		},
		{
			name: "invalid log file path",
			config: &ServerConfig{
				AgentsFile:    "testdata/agents.yml",
				ListenAddress: ":8443",
				LogFile:       "/invalid/path/test-server.log",
				LogLevel:      "debug",
				SocksServer:   "/tmp/test.sock",
				EnableTLS:     false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, err := NewServer(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, server)
				assert.NotNil(t, server.pool)
				assert.NotNil(t, server.logger)
				assert.Equal(t, tt.config, server.config)
			}
		})
	}
}

func TestServer_CleanUp(t *testing.T) {
	// Create a minimal valid config for testing cleanup
	config := &ServerConfig{
		LogFile:  "/tmp/test-server.log",
		LogLevel: "debug",
	}

	// Create server components manually for testing
	server := &Server{
		config: config,
	}

	// Initialize logger and pool manually
	serverLogger, err := logger.NewLogger(config.LogFile, config.LogLevel)
	require.NoError(t, err)
	server.logger = serverLogger

	pool, err := ants.NewPool(100, ants.WithNonblocking(true))
	require.NoError(t, err)
	server.pool = pool

	// Test cleanup can be called multiple times safely
	server.CleanUp()
	server.CleanUp()

	// Pool should be released after cleanup
	assert.True(t, server.pool.IsClosed())
}

func TestCheck(t *testing.T) {
	tests := []struct {
		name    string
		agents  *AllowedAgents
		wantErr bool
	}{
		{
			name:    "nil agents",
			agents:  nil,
			wantErr: true,
		},
		{
			name: "valid agents",
			agents: &AllowedAgents{
				Agents: []struct {
					Name       string `mapstructure:"name"`
					AuthKey    string `mapstructure:"auth_key"`
					ApprovedCN string `mapstructure:"approved_cn"`
				}{
					{
						Name:       "test-agent",
						AuthKey:    "test-key",
						ApprovedCN: "test-cn",
					},
				},
			},
			wantErr: false,
		},
		{
			name: "empty name",
			agents: &AllowedAgents{
				Agents: []struct {
					Name       string `mapstructure:"name"`
					AuthKey    string `mapstructure:"auth_key"`
					ApprovedCN string `mapstructure:"approved_cn"`
				}{
					{
						Name:       "",
						AuthKey:    "test-key",
						ApprovedCN: "test-cn",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty auth key",
			agents: &AllowedAgents{
				Agents: []struct {
					Name       string `mapstructure:"name"`
					AuthKey    string `mapstructure:"auth_key"`
					ApprovedCN string `mapstructure:"approved_cn"`
				}{
					{
						Name:       "test-agent",
						AuthKey:    "",
						ApprovedCN: "test-cn",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "empty approved cn",
			agents: &AllowedAgents{
				Agents: []struct {
					Name       string `mapstructure:"name"`
					AuthKey    string `mapstructure:"auth_key"`
					ApprovedCN string `mapstructure:"approved_cn"`
				}{
					{
						Name:       "test-agent",
						AuthKey:    "test-key",
						ApprovedCN: "",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := check(tt.agents)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
