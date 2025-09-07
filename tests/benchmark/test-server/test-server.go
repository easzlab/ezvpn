package main

import (
	"context"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// TestServer 测试目标服务器
type TestServer struct {
	httpPort   int
	tcpPort    int
	httpServer *http.Server
	tcpServer  net.Listener
	stats      *ServerStats
	ctx        context.Context
	cancel     context.CancelFunc
}

// ServerStats 服务器统计信息
type ServerStats struct {
	httpConnections int64
	tcpConnections  int64
	bytesReceived   int64
	bytesSent       int64
	requestCount    int64
	startTime       time.Time
	mu              sync.RWMutex
}

func (s *ServerStats) AddHTTPConnection() {
	atomic.AddInt64(&s.httpConnections, 1)
}

func (s *ServerStats) AddTCPConnection() {
	atomic.AddInt64(&s.tcpConnections, 1)
}

func (s *ServerStats) AddBytesReceived(bytes int64) {
	atomic.AddInt64(&s.bytesReceived, bytes)
}

func (s *ServerStats) AddBytesSent(bytes int64) {
	atomic.AddInt64(&s.bytesSent, bytes)
}

func (s *ServerStats) AddRequest() {
	atomic.AddInt64(&s.requestCount, 1)
}

func (s *ServerStats) GetStats() (int64, int64, int64, int64, int64, time.Duration) {
	return atomic.LoadInt64(&s.httpConnections),
		atomic.LoadInt64(&s.tcpConnections),
		atomic.LoadInt64(&s.bytesReceived),
		atomic.LoadInt64(&s.bytesSent),
		atomic.LoadInt64(&s.requestCount),
		time.Since(s.startTime)
}

// NewTestServer 创建新的测试服务器
func NewTestServer(httpPort, tcpPort int) *TestServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &TestServer{
		httpPort: httpPort,
		tcpPort:  tcpPort,
		stats: &ServerStats{
			startTime: time.Now(),
		},
		ctx:    ctx,
		cancel: cancel,
	}
}

// StartHTTPServer 启动HTTP测试服务器
func (ts *TestServer) StartHTTPServer() error {
	mux := http.NewServeMux()

	// 健康检查端点
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		ts.stats.AddHTTPConnection()
		ts.stats.AddRequest()
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// 回显端点 - 返回接收到的数据
	mux.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		ts.stats.AddHTTPConnection()
		ts.stats.AddRequest()

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ts.stats.AddBytesReceived(int64(len(body)))
		ts.stats.AddBytesSent(int64(len(body)))

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(body)
	})

	// 下载端点 - 返回指定大小的随机数据
	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		ts.stats.AddHTTPConnection()
		ts.stats.AddRequest()

		sizeStr := r.URL.Query().Get("size")
		size, err := strconv.Atoi(sizeStr)
		if err != nil || size <= 0 {
			size = 1024 // 默认1KB
		}

		// 限制最大下载大小为100MB
		if size > 100*1024*1024 {
			size = 100 * 1024 * 1024
		}

		data := make([]byte, size)
		rand.Read(data)

		ts.stats.AddBytesSent(int64(size))

		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(size))
		w.Write(data)
	})

	// 上传端点 - 接收数据但不返回
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		ts.stats.AddHTTPConnection()
		ts.stats.AddRequest()

		written, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		ts.stats.AddBytesReceived(written)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Received %d bytes", written)))
	})

	// 延迟测试端点
	mux.HandleFunc("/delay", func(w http.ResponseWriter, r *http.Request) {
		ts.stats.AddHTTPConnection()
		ts.stats.AddRequest()

		delayStr := r.URL.Query().Get("ms")
		delayMs, err := strconv.Atoi(delayStr)
		if err != nil || delayMs < 0 {
			delayMs = 0
		}

		// 限制最大延迟为10秒
		if delayMs > 10000 {
			delayMs = 10000
		}

		time.Sleep(time.Duration(delayMs) * time.Millisecond)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("Delayed %d ms", delayMs)))
	})

	// 统计信息端点
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		httpConns, tcpConns, bytesRecv, bytesSent, requests, uptime := ts.stats.GetStats()

		stats := fmt.Sprintf(`{
	"http_connections": %d,
	"tcp_connections": %d,
	"bytes_received": %d,
	"bytes_sent": %d,
	"total_requests": %d,
	"uptime_seconds": %.2f,
	"requests_per_second": %.2f
}`, httpConns, tcpConns, bytesRecv, bytesSent, requests,
			uptime.Seconds(), float64(requests)/uptime.Seconds())

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(stats))
	})

	ts.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", ts.httpPort),
		Handler: mux,
	}

	log.Printf("Starting HTTP server on port %d", ts.httpPort)
	return ts.httpServer.ListenAndServe()
}

// StartTCPServer 启动TCP测试服务器
func (ts *TestServer) StartTCPServer() error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", ts.tcpPort))
	if err != nil {
		return err
	}

	ts.tcpServer = listener
	log.Printf("Starting TCP server on port %d", ts.tcpPort)

	go func() {
		for {
			select {
			case <-ts.ctx.Done():
				return
			default:
			}

			conn, err := listener.Accept()
			if err != nil {
				if ts.ctx.Err() != nil {
					return
				}
				log.Printf("TCP accept error: %v", err)
				continue
			}

			go ts.handleTCPConnection(conn)
		}
	}()

	return nil
}

// handleTCPConnection 处理TCP连接
func (ts *TestServer) handleTCPConnection(conn net.Conn) {
	defer conn.Close()
	ts.stats.AddTCPConnection()

	buffer := make([]byte, 4096)
	for {
		select {
		case <-ts.ctx.Done():
			return
		default:
		}

		conn.SetReadDeadline(time.Now().Add(30 * time.Second))
		n, err := conn.Read(buffer)
		if err != nil {
			if err != io.EOF {
				log.Printf("TCP read error: %v", err)
			}
			return
		}

		if n > 0 {
			ts.stats.AddBytesReceived(int64(n))

			// 回显数据
			written, err := conn.Write(buffer[:n])
			if err != nil {
				log.Printf("TCP write error: %v", err)
				return
			}
			ts.stats.AddBytesSent(int64(written))
		}
	}
}

// Stop 停止服务器
func (ts *TestServer) Stop() error {
	ts.cancel()

	var err error
	if ts.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := ts.httpServer.Shutdown(ctx); shutdownErr != nil {
			err = shutdownErr
		}
	}

	if ts.tcpServer != nil {
		if closeErr := ts.tcpServer.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}

	return err
}

// PrintStats 打印统计信息
func (ts *TestServer) PrintStats() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ts.ctx.Done():
			return
		case <-ticker.C:
			httpConns, tcpConns, bytesRecv, bytesSent, requests, uptime := ts.stats.GetStats()

			log.Printf("Stats - HTTP Conns: %d, TCP Conns: %d, Bytes Recv: %d, Bytes Sent: %d, Requests: %d, Uptime: %.1fs, RPS: %.2f",
				httpConns, tcpConns, bytesRecv, bytesSent, requests, uptime.Seconds(), float64(requests)/uptime.Seconds())
		}
	}
}

func main() {
	var (
		httpPort = flag.Int("http-port", 8080, "HTTP server port")
		tcpPort  = flag.Int("tcp-port", 8081, "TCP server port")
		stats    = flag.Bool("stats", true, "Print periodic stats")
	)
	flag.Parse()

	server := NewTestServer(*httpPort, *tcpPort)

	// 启动统计信息打印
	if *stats {
		go server.PrintStats()
	}

	// 启动TCP服务器
	if err := server.StartTCPServer(); err != nil {
		log.Fatalf("Failed to start TCP server: %v", err)
	}

	// 启动HTTP服务器（阻塞）
	log.Printf("Test server starting on HTTP port %d and TCP port %d", *httpPort, *tcpPort)
	if err := server.StartHTTPServer(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
