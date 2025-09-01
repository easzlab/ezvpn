package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

func (a *Agent) Run() error {
	// Check if the program process already exists
	if err := checkProcessExists(a.config.LockFile); err == nil {
		return fmt.Errorf("another instance of the program is already running")
	}

	// Create lock file
	if err := createLockFile(a.config.LockFile); err != nil {
		return err
	}
	defer func() {
		os.Remove(a.config.LockFile)
	}()

	// Set signal handling
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nReceived interrupt signal, stopping agent...")
		cancel()
	}()

	err := a.Start(ctx)
	a.CleanUp()
	return err
}

// checkProcessExists checks if the program process already exists, returns nil if exists, returns error if not exists
func checkProcessExists(lockFile string) error {
	// Try to open the lock file
	file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read file content
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	// Parse file content
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return err
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("error finding process: %s", err.Error())
	}

	if runtime.GOOS != "windows" {
		// Unix-like systems send signal; Windows platform is not implemented, can only rely on the previous step to check pid
		return process.Signal(syscall.Signal(0))
	}

	return nil
}

func createLockFile(lockFile string) error {
	// Get the PID of the current process
	pid := os.Getpid()

	// Create or open the lock file
	file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("error creating lock file: %s", err.Error())
	}
	defer file.Close()

	// Write the current process PID to the lock file
	_, err = file.WriteString(strconv.Itoa(pid))
	if err != nil {
		return fmt.Errorf("error writing PID to lock file: %s", err.Error())
	}

	return nil
}
