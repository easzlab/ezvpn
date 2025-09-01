package utils

import (
	"bufio"
	"os"
)

// EnsureFile checks if a file exists, creates it if it doesn't exist, and writes content to it;
// if the file already exists, no modifications are made;
// suitable for initialization configuration operations
func EnsureFile(filePath, content string) error {
	// Use OpenFile to open the file in write-only mode, create it if it doesn't exist,
	// if the file already exists, OpenFile won't clear the file or write new content
	f, err := os.OpenFile(filePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if err != nil {
		// If an error occurs and it's not because the file already exists, return the error
		if !os.IsExist(err) {
			return err
		}
		// File already exists, no need to create or write, return nil
		return nil
	}
	defer f.Close()

	// Write content to the file
	writer := bufio.NewWriter(f)

	if _, err := writer.WriteString(content); err != nil {
		return err
	}

	return writer.Flush()
}

// WriteFile: write text to file with name 'filename'
func WriteFile(filename string, text string) error {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := bufio.NewWriter(file)

	if _, err := writer.WriteString(text); err != nil {
		return err
	}

	return writer.Flush()
}
