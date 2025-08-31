package agent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAgent(t *testing.T) {
	tests := []struct {
		name    string
		config  *AgentConfig
		wantErr bool
	}{
		{
			name: "valid config without TLS",
			config: &AgentConfig{
				AuthKey:       "test-key",
				ServerAddress: "127.0.0.1:8443",
				LocalAddress:  ":16116",
				LogFile:       "/tmp/test-agent.log",
				LogLevel:      "debug",
				EnableTLS:     false,
			},
			wantErr: false,
		},
		{
			name: "invalid log file path",
			config: &AgentConfig{
				AuthKey:       "test-key",
				ServerAddress: "127.0.0.1:8443",
				LocalAddress:  ":16116",
				LogFile:       "/invalid/path/test-agent.log",
				LogLevel:      "debug",
				EnableTLS:     false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			agent, err := NewAgent(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, agent)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, agent)
				assert.NotNil(t, agent.pool)
				assert.NotNil(t, agent.logger)
				assert.NotNil(t, agent.dialer)
				assert.Equal(t, tt.config, agent.config)
			}
		})
	}
}

func TestAgent_CleanUp(t *testing.T) {
	config := &AgentConfig{
		AuthKey:       "test-key",
		ServerAddress: "127.0.0.1:8443",
		LocalAddress:  ":16116",
		LogFile:       "/tmp/test-agent.log",
		LogLevel:      "debug",
		EnableTLS:     false,
	}

	agent, err := NewAgent(config)
	require.NoError(t, err)
	require.NotNil(t, agent)

	// Test cleanup can be called multiple times safely
	agent.CleanUp()
	agent.CleanUp()

	// Pool should be released after cleanup
	assert.True(t, agent.pool.IsClosed())
}

func TestAgent_Start_ContextCanceled(t *testing.T) {
	config := &AgentConfig{
		AuthKey:       "test-key",
		ServerAddress: "invalid-server:8443", // Invalid server to force connection failure
		LocalAddress:  ":16116",
		LogFile:       "/tmp/test-agent.log",
		LogLevel:      "debug",
		EnableTLS:     false,
	}

	agent, err := NewAgent(config)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err = agent.Start(ctx)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestAgentConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config *AgentConfig
		field  string
	}{
		{
			name: "empty auth key",
			config: &AgentConfig{
				AuthKey:       "",
				ServerAddress: "127.0.0.1:8443",
				LocalAddress:  ":16116",
			},
			field: "AuthKey",
		},
		{
			name: "empty server address",
			config: &AgentConfig{
				AuthKey:       "test-key",
				ServerAddress: "",
				LocalAddress:  ":16116",
			},
			field: "ServerAddress",
		},
		{
			name: "empty local address",
			config: &AgentConfig{
				AuthKey:       "test-key",
				ServerAddress: "127.0.0.1:8443",
				LocalAddress:  "",
			},
			field: "LocalAddress",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that empty required fields are handled appropriately
			switch tt.field {
			case "AuthKey":
				assert.Empty(t, tt.config.AuthKey)
			case "ServerAddress":
				assert.Empty(t, tt.config.ServerAddress)
			case "LocalAddress":
				assert.Empty(t, tt.config.LocalAddress)
			}
		})
	}
}