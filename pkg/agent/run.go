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
	// 检查程序进程是否已经存在
	if err := checkProcessExists(a.config.LockFile); err == nil {
		return fmt.Errorf("another instance of the program is already running")
	}

	// 创建锁文件
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

// checkProcessExists 检查程序进程是否已经存在, 存在则返回nil, 不存在则返回错误
func checkProcessExists(lockFile string) error {
	// 尝试打开锁文件
	file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer file.Close()

	// 读取文件内容
	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	// 解析文件内容
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return err
	}

	// 检查进程是否存在
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("error finding process: %s", err.Error())
	}

	if runtime.GOOS != "windows" {
		// 类unix系统发送信号；windows 平台未实现，只能依靠上一步检查pid判断
		return process.Signal(syscall.Signal(0))
	}

	return nil
}

func createLockFile(lockFile string) error {
	// 获取当前进程的PID
	pid := os.Getpid()

	// 创建或打开锁文件
	file, err := os.OpenFile(lockFile, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	if err != nil {
		return fmt.Errorf("error creating lock file: %s", err.Error())
	}
	defer file.Close()

	// 写入当前进程的PID到锁文件
	_, err = file.WriteString(strconv.Itoa(pid))
	if err != nil {
		return fmt.Errorf("error writing PID to lock file: %s", err.Error())
	}

	return nil
}
