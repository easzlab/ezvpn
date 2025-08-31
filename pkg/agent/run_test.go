package agent

import (
	"os"
	"runtime"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckProcessExists(t *testing.T) {
	tests := []struct {
		name     string
		setup    func() (string, func()) // Returns lockFile path and cleanup function
		wantErr  bool
		errCheck func(error) bool
	}{
		{
			name: "no lock file exists",
			setup: func() (string, func()) {
				lockFile := "/tmp/test-no-lock.pid"
				os.Remove(lockFile) // Ensure it doesn't exist
				return lockFile, func() { os.Remove(lockFile) }
			},
			wantErr: true,
		},
		{
			name: "empty lock file",
			setup: func() (string, func()) {
				lockFile := "/tmp/test-empty-lock.pid"
				f, _ := os.Create(lockFile)
				f.Close()
				return lockFile, func() { os.Remove(lockFile) }
			},
			wantErr: true,
		},
		{
			name: "invalid pid in lock file",
			setup: func() (string, func()) {
				lockFile := "/tmp/test-invalid-pid.pid"
				f, _ := os.Create(lockFile)
				f.WriteString("invalid")
				f.Close()
				return lockFile, func() { os.Remove(lockFile) }
			},
			wantErr: true,
		},
		{
			name: "non-existent pid in lock file",
			setup: func() (string, func()) {
				lockFile := "/tmp/test-nonexistent-pid.pid"
				f, _ := os.Create(lockFile)
				f.WriteString("99999") // Assuming this PID doesn't exist
				f.Close()
				return lockFile, func() { os.Remove(lockFile) }
			},
			wantErr: true,
		},
		{
			name: "current process pid in lock file",
			setup: func() (string, func()) {
				lockFile := "/tmp/test-current-pid.pid"
				f, _ := os.Create(lockFile)
				f.WriteString(strconv.Itoa(os.Getpid()))
				f.Close()
				return lockFile, func() { os.Remove(lockFile) }
			},
			wantErr: false, // Current process exists
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lockFile, cleanup := tt.setup()
			defer cleanup()

			err := checkProcessExists(lockFile)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				if runtime.GOOS != "windows" {
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestCreateLockFile(t *testing.T) {
	tests := []struct {
		name     string
		lockFile string
		wantErr  bool
		cleanup  bool
	}{
		{
			name:     "create lock file successfully",
			lockFile: "/tmp/test-create-lock.pid",
			wantErr:  false,
			cleanup:  true,
		},
		{
			name:     "invalid directory path",
			lockFile: "/invalid/path/test.pid",
			wantErr:  true,
			cleanup:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cleanup {
				defer os.Remove(tt.lockFile)
			}

			err := createLockFile(tt.lockFile)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Check file exists and contains current PID
				content, err := os.ReadFile(tt.lockFile)
				require.NoError(t, err)
				
				pid, err := strconv.Atoi(string(content))
				require.NoError(t, err)
				assert.Equal(t, os.Getpid(), pid)
			}
		})
	}
}

func TestCreateLockFile_Overwrite(t *testing.T) {
	lockFile := "/tmp/test-overwrite-lock.pid"
	defer os.Remove(lockFile)

	// Create initial lock file with different content
	f, err := os.Create(lockFile)
	require.NoError(t, err)
	f.WriteString("12345")
	f.Close()

	// Create lock file should overwrite
	err = createLockFile(lockFile)
	require.NoError(t, err)

	// Check content is current PID
	content, err := os.ReadFile(lockFile)
	require.NoError(t, err)
	
	pid, err := strconv.Atoi(string(content))
	require.NoError(t, err)
	assert.Equal(t, os.Getpid(), pid)
}