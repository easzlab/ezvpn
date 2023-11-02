package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/easzlab/ezvpn/agent"
	"github.com/easzlab/ezvpn/config"
	"github.com/easzlab/ezvpn/logger"
	"go.uber.org/zap"
)

func main() {
	a := agent.Agent{}
	flag.StringVar(&a.AuthKey, "auth", "xxx", "Specify the authentication key")
	flag.BoolVar(&a.EnableTLS, "tls", true, "enable tls between agent and server")
	flag.BoolVar(&a.EnablePprof, "pprof", false, "enable pprof")
	flag.BoolVar(&a.ShowVersion, "version", false, "show version of the agent")
	flag.StringVar(&a.CaFile, "ca", "ca.pem", "Specify the trusted ca file")
	flag.StringVar(&a.CertFile, "cert", "agent.pem", "Specify the agent cert file")
	flag.StringVar(&a.KeyFile, "key", "agent-key.pem", "Specify the agent key file")
	flag.StringVar(&a.LockFile, "lock", "agent.lock", "Specify the agent lock file")
	flag.StringVar(&a.LogFile, "logfile", "agent.log", "Specify the agent log file")
	flag.StringVar(&a.LogLevel, "loglvl", "debug", "Specify the agent log level")
	flag.StringVar(&a.LocalAddress, "local", ":16116", "Specify the local address")
	flag.StringVar(&a.ServerAddress, "server", "127.0.0.1:8443", "Specify the server address")
	flag.Parse()

	if a.ShowVersion {
		fmt.Println(config.FullVersion())
		os.Exit(0)
	}

	logger.InitAgentLogger(a.LogFile, a.LogLevel)

	if a.EnablePprof {
		go http.ListenAndServe("0.0.0.0:6061", nil)
	}

	if err := run(a); err != nil {
		logger.Agent.Warn("agent run error", zap.String("reason", err.Error()))
		os.Exit(1)
	}
}

func run(a agent.Agent) error {
	// 检查程序进程是否已经存在
	if err := checkProcessExists(a.LockFile); err == nil {
		return fmt.Errorf("another instance of the program is already running")
	}

	// 创建锁文件
	if err := createLockFile(a.LockFile); err != nil {
		return err
	}

	defer func() {
		// 程序退出时删除锁文件
		os.Remove(a.LockFile)
		os.Remove(a.LogFile)
	}()

	ctx := withSignalCancel(context.Background())
	err := a.Start(ctx)

	if errors.Is(err, context.Canceled) {
		logger.Agent.Warn("agent canceled, waiting for it to stop...", zap.String("reason", "user canceled"))
		time.Sleep(config.AgentCancelWait)
		return nil
	}

	return err
}

func withSignalCancel(ctx context.Context) context.Context {
	newCtx, cancel := context.WithCancel(ctx)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	go func() {
		<-sigCh
		cancel()
	}()
	return newCtx
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
