package logger

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewLoggerSuccess tests successful creation of logger.
func TestNewLoggerSuccess(t *testing.T) {
	logFile := filepath.Join(os.TempDir(), "test.log") // Use system temporary directory.
	logLevel := "info"

	logger, err := NewLogger(logFile, logLevel)
	require.NoError(t, err)
	assert.NotNil(t, logger)

	// Clean up resources and test-generated log files after testing.
	defer logger.Sync()
	defer os.Remove(logFile)
}

// TestNewLoggerInvalidLevel tests invalid log level handling.
func TestNewLoggerInvalidLevel(t *testing.T) {
	logFile := filepath.Join(os.TempDir(), "test.log")
	logLevel := "invalid"

	_, err := NewLogger(logFile, logLevel)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to unmarshal level")
}

// TestNewLoggerInvalidPath tests invalid log file path handling.
func TestNewLoggerInvalidPath(t *testing.T) {
	logFile := "/path/to/nowhere/test.log" // Use a directory path that definitely doesn't exist.
	logLevel := "info"

	_, err := NewLogger(logFile, logLevel)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to open log file")
}

// TestNewLoggerDifferentLevels tests logger creation with different valid log levels.
func TestNewLoggerDifferentLevels(t *testing.T) {
	testCases := []struct {
		name     string
		logLevel string
	}{
		{"debug level", "debug"},
		{"info level", "info"},
		{"warn level", "warn"},
		{"error level", "error"},
		{"fatal level", "fatal"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logFile := filepath.Join(os.TempDir(), "test_"+tc.logLevel+".log")

			logger, err := NewLogger(logFile, tc.logLevel)
			require.NoError(t, err)
			assert.NotNil(t, logger)

			// Clean up
			defer logger.Sync()
			defer os.Remove(logFile)
		})
	}
}

// TestLoggerActualLogging tests that the logger can actually write log messages.
func TestLoggerActualLogging(t *testing.T) {
	logFile := filepath.Join(os.TempDir(), "test_logging.log")
	logLevel := "info"

	logger, err := NewLogger(logFile, logLevel)
	require.NoError(t, err)
	require.NotNil(t, logger)

	// Write some test log messages
	logger.Info("test info message")
	logger.Error("test error message")
	logger.Warn("test warning message")

	// Sync to ensure messages are written
	logger.Sync()

	// Check if log file exists and has content
	info, err := os.Stat(logFile)
	require.NoError(t, err)
	assert.True(t, info.Size() > 0, "log file should contain log messages")

	// Clean up
	defer os.Remove(logFile)
}
