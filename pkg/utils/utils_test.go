package utils

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureFile(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		content  string
		wantErr  bool
		setup    func(string) // Setup function to prepare test conditions
		cleanup  func(string) // Cleanup function
	}{
		{
			name:     "create new file",
			filePath: "/tmp/test-ensure-new.txt",
			content:  "test content",
			wantErr:  false,
			setup: func(path string) {
				os.Remove(path) // Ensure file doesn't exist
			},
			cleanup: func(path string) {
				os.Remove(path)
			},
		},
		{
			name:     "file already exists",
			filePath: "/tmp/test-ensure-exists.txt",
			content:  "new content",
			wantErr:  false,
			setup: func(path string) {
				// Create file with different content
				WriteFile(path, "existing content")
			},
			cleanup: func(path string) {
				os.Remove(path)
			},
		},
		{
			name:     "invalid file path",
			filePath: "/invalid/path/test.txt",
			content:  "test content",
			wantErr:  true,
			setup:    func(path string) {},
			cleanup:  func(path string) {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup(tt.filePath)
			defer tt.cleanup(tt.filePath)

			err := EnsureFile(tt.filePath, tt.content)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				
				// Check file exists
				_, err := os.Stat(tt.filePath)
				assert.NoError(t, err, "File should exist after EnsureFile")
				
				// For new file creation, check content
				if tt.name == "create new file" {
					content, err := os.ReadFile(tt.filePath)
					require.NoError(t, err)
					assert.Equal(t, tt.content, string(content))
				}
				
				// For existing file, content should not change
				if tt.name == "file already exists" {
					content, err := os.ReadFile(tt.filePath)
					require.NoError(t, err)
					assert.Equal(t, "existing content", string(content))
				}
			}
		})
	}
}

func TestWriteFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		text     string
		wantErr  bool
		cleanup  bool
	}{
		{
			name:     "write to new file",
			filename: "/tmp/test-write-new.txt",
			text:     "hello world",
			wantErr:  false,
			cleanup:  true,
		},
		{
			name:     "overwrite existing file",
			filename: "/tmp/test-write-overwrite.txt",
			text:     "new content",
			wantErr:  false,
			cleanup:  true,
		},
		{
			name:     "write empty content",
			filename: "/tmp/test-write-empty.txt",
			text:     "",
			wantErr:  false,
			cleanup:  true,
		},
		{
			name:     "invalid file path",
			filename: "/invalid/path/test.txt",
			text:     "test content",
			wantErr:  true,
			cleanup:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.cleanup {
				defer os.Remove(tt.filename)
			}
			
			// For overwrite test, create existing file first
			if tt.name == "overwrite existing file" {
				WriteFile(tt.filename, "old content")
			}

			err := WriteFile(tt.filename, tt.text)
			
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				
				// Check file exists and has correct content
				content, err := os.ReadFile(tt.filename)
				require.NoError(t, err)
				assert.Equal(t, tt.text, string(content))
				
				// Check file info
				info, err := os.Stat(tt.filename)
				require.NoError(t, err)
				assert.Equal(t, int64(len(tt.text)), info.Size())
			}
		})
	}
}

func TestWriteFile_Permissions(t *testing.T) {
	filename := "/tmp/test-permissions.txt"
	defer os.Remove(filename)
	
	err := WriteFile(filename, "test content")
	require.NoError(t, err)
	
	// Check file permissions
	info, err := os.Stat(filename)
	require.NoError(t, err)
	
	// Should have 0644 permissions
	assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
}