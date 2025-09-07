package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
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

// BenchmarkConfig 性能测试配置
type BenchmarkConfig struct {
	ProxyAddr      string        // SOCKS5代理地址
	TargetAddr     string        // 目标服务器地址
	Concurrency    int           // 并发连接数
	Duration       time.Duration // 测试持续时间
	RequestCount   int           // 请求总数（如果设置，则忽略Duration）
	DataSize       int           // 数据包大小
	TestType       string        // 测试类型：http, tcp, throughput, latency
	KeepAlive      bool          // 是否保持连接
	ReportInterval time.Duration // 报告间隔
}

// BenchmarkResult 性能测试结果
type BenchmarkResult struct {
	TotalRequests     int64         // 总请求数
	SuccessRequests   int64         // 成功请求数
	FailedRequests    int64         // 失败请求数
	TotalBytes        int64         // 总字节数
	TotalDuration     time.Duration // 总耗时
	MinLatency        time.Duration // 最小延迟
	MaxLatency        time.Duration // 最大延迟
	AvgLatency        time.Duration // 平均延迟
	P95Latency        time.Duration // 95%延迟
	P99Latency        time.Duration // 99%延迟
	RequestsPerSecond float64       // 每秒请求数
	BytesPerSecond    float64       // 每秒字节数
	ErrorRate         float64       // 错误率
}

// LatencyTracker 延迟跟踪器
type LatencyTracker struct {
	latencies []time.Duration
	mu        sync.Mutex
}

func (lt *LatencyTracker) AddLatency(latency time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()
	lt.latencies = append(lt.latencies, latency)
}

func (lt *LatencyTracker) GetPercentiles() (min, max, avg, p95, p99 time.Duration) {
	lt.mu.Lock()
	defer lt.mu.Unlock()

	if len(lt.latencies) == 0 {
		return 0, 0, 0, 0, 0
	}

	// 简单排序
	for i := 0; i < len(lt.latencies); i++ {
		for j := i + 1; j < len(lt.latencies); j++ {
			if lt.latencies[i] > lt.latencies[j] {
				lt.latencies[i], lt.latencies[j] = lt.latencies[j], lt.latencies[i]
			}
		}
	}

	min = lt.latencies[0]
	max = lt.latencies[len(lt.latencies)-1]

	var total time.Duration
	for _, lat := range lt.latencies {
		total += lat
	}
	avg = total / time.Duration(len(lt.latencies))

	p95Index := int(float64(len(lt.latencies)) * 0.95)
	if p95Index >= len(lt.latencies) {
		p95Index = len(lt.latencies) - 1
	}
	p95 = lt.latencies[p95Index]

	p99Index := int(float64(len(lt.latencies)) * 0.99)
	if p99Index >= len(lt.latencies) {
		p99Index = len(lt.latencies) - 1
	}
	p99 = lt.latencies[p99Index]

	return min, max, avg, p95, p99
}

// SOCKS5Client SOCKS5客户端
type SOCKS5Client struct {
	proxyAddr string
	timeout   time.Duration
}

func NewSOCKS5Client(proxyAddr string, timeout time.Duration) *SOCKS5Client {
	return &SOCKS5Client{
		proxyAddr: proxyAddr,
		timeout:   timeout,
	}
}

// Connect 通过SOCKS5代理连接到目标地址
func (c *SOCKS5Client) Connect(targetAddr string) (net.Conn, error) {
	// 连接到SOCKS5代理
	conn, err := net.DialTimeout("tcp", c.proxyAddr, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to proxy: %w", err)
	}

	// SOCKS5握手
	if err := c.socks5Handshake(conn, targetAddr); err != nil {
		conn.Close()
		return nil, fmt.Errorf("SOCKS5 handshake failed: %w", err)
	}

	return conn, nil
}

// socks5Handshake 执行SOCKS5握手
func (c *SOCKS5Client) socks5Handshake(conn net.Conn, targetAddr string) error {
	// 设置超时
	conn.SetDeadline(time.Now().Add(c.timeout))
	defer conn.SetDeadline(time.Time{})

	// 1. 发送认证方法
	authReq := []byte{0x05, 0x01, 0x00} // VER=5, NMETHODS=1, METHOD=0(无认证)
	if _, err := conn.Write(authReq); err != nil {
		return err
	}

	// 2. 读取认证响应
	authResp := make([]byte, 2)
	if _, err := io.ReadFull(conn, authResp); err != nil {
		return err
	}

	if authResp[0] != 0x05 || authResp[1] != 0x00 {
		return fmt.Errorf("authentication failed")
	}

	// 3. 发送连接请求
	host, portStr, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return err
	}

	port, err := strconv.Atoi(portStr)
	if err != nil {
		return err
	}

	// 构建连接请求
	req := []byte{0x05, 0x01, 0x00} // VER=5, CMD=1(CONNECT), RSV=0

	// 添加地址类型和地址
	if ip := net.ParseIP(host); ip != nil {
		if ip4 := ip.To4(); ip4 != nil {
			// IPv4
			req = append(req, 0x01)
			req = append(req, ip4...)
		} else {
			// IPv6
			req = append(req, 0x04)
			req = append(req, ip.To16()...)
		}
	} else {
		// 域名
		req = append(req, 0x03)
		req = append(req, byte(len(host)))
		req = append(req, []byte(host)...)
	}

	// 添加端口
	portBytes := make([]byte, 2)
	binary.BigEndian.PutUint16(portBytes, uint16(port))
	req = append(req, portBytes...)

	if _, err := conn.Write(req); err != nil {
		return err
	}

	// 4. 读取连接响应
	resp := make([]byte, 4)
	if _, err := io.ReadFull(conn, resp); err != nil {
		return err
	}

	if resp[0] != 0x05 || resp[1] != 0x00 {
		return fmt.Errorf("connection failed, status: %d", resp[1])
	}

	// 读取绑定地址（根据地址类型）
	switch resp[3] {
	case 0x01: // IPv4
		addr := make([]byte, 6) // 4字节IP + 2字节端口
		io.ReadFull(conn, addr)
	case 0x03: // 域名
		lenByte := make([]byte, 1)
		io.ReadFull(conn, lenByte)
		addr := make([]byte, int(lenByte[0])+2) // 域名长度 + 2字节端口
		io.ReadFull(conn, addr)
	case 0x04: // IPv6
		addr := make([]byte, 18) // 16字节IP + 2字节端口
		io.ReadFull(conn, addr)
	}

	return nil
}

// HTTPBenchmark HTTP性能测试
func HTTPBenchmark(config *BenchmarkConfig) *BenchmarkResult {
	log.Printf("Starting HTTP benchmark: concurrency=%d, duration=%v, data_size=%d",
		config.Concurrency, config.Duration, config.DataSize)

	var (
		totalRequests   int64
		successRequests int64
		failedRequests  int64
		totalBytes      int64
		latencyTracker  = &LatencyTracker{}
		wg              sync.WaitGroup
		ctx, cancel     = context.WithTimeout(context.Background(), config.Duration)
	)
	defer cancel()

	// 创建HTTP客户端
	transport := &http.Transport{
		Dial: func(network, addr string) (net.Conn, error) {
			client := NewSOCKS5Client(config.ProxyAddr, 10*time.Second)
			return client.Connect(addr)
		},
		MaxIdleConns:        config.Concurrency,
		MaxIdleConnsPerHost: config.Concurrency,
		IdleConnTimeout:     30 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// 准备测试数据
	testData := make([]byte, config.DataSize)
	rand.Read(testData)

	startTime := time.Now()

	// 启动并发测试
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				reqStart := time.Now()
				atomic.AddInt64(&totalRequests, 1)

				// 发送HTTP请求
				targetURL := fmt.Sprintf("http://%s/echo", config.TargetAddr)
				resp, err := httpClient.Post(targetURL, "application/octet-stream", bytes.NewReader(testData))

				latency := time.Since(reqStart)
				latencyTracker.AddLatency(latency)

				if err != nil {
					atomic.AddInt64(&failedRequests, 1)
					continue
				}

				body, err := io.ReadAll(resp.Body)
				resp.Body.Close()

				if err != nil || resp.StatusCode != 200 {
					atomic.AddInt64(&failedRequests, 1)
					continue
				}

				atomic.AddInt64(&successRequests, 1)
				atomic.AddInt64(&totalBytes, int64(len(body)))
			}
		}()
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	// 计算结果
	min, max, avg, p95, p99 := latencyTracker.GetPercentiles()

	result := &BenchmarkResult{
		TotalRequests:     atomic.LoadInt64(&totalRequests),
		SuccessRequests:   atomic.LoadInt64(&successRequests),
		FailedRequests:    atomic.LoadInt64(&failedRequests),
		TotalBytes:        atomic.LoadInt64(&totalBytes),
		TotalDuration:     totalDuration,
		MinLatency:        min,
		MaxLatency:        max,
		AvgLatency:        avg,
		P95Latency:        p95,
		P99Latency:        p99,
		RequestsPerSecond: float64(atomic.LoadInt64(&successRequests)) / totalDuration.Seconds(),
		BytesPerSecond:    float64(atomic.LoadInt64(&totalBytes)) / totalDuration.Seconds(),
		ErrorRate:         float64(atomic.LoadInt64(&failedRequests)) / float64(atomic.LoadInt64(&totalRequests)) * 100,
	}

	return result
}

// TCPBenchmark TCP性能测试
func TCPBenchmark(config *BenchmarkConfig) *BenchmarkResult {
	log.Printf("Starting TCP benchmark: concurrency=%d, duration=%v, data_size=%d",
		config.Concurrency, config.Duration, config.DataSize)

	var (
		totalRequests   int64
		successRequests int64
		failedRequests  int64
		totalBytes      int64
		latencyTracker  = &LatencyTracker{}
		wg              sync.WaitGroup
		ctx, cancel     = context.WithTimeout(context.Background(), config.Duration)
	)
	defer cancel()

	// 准备测试数据
	testData := make([]byte, config.DataSize)
	rand.Read(testData)

	startTime := time.Now()

	// 启动并发测试
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			var conn net.Conn
			var err error

			for {
				select {
				case <-ctx.Done():
					if conn != nil {
						conn.Close()
					}
					return
				default:
				}

				// 如果不保持连接或连接不存在，创建新连接
				if !config.KeepAlive || conn == nil {
					if conn != nil {
						conn.Close()
					}

					client := NewSOCKS5Client(config.ProxyAddr, 10*time.Second)
					conn, err = client.Connect(config.TargetAddr)
					if err != nil {
						atomic.AddInt64(&failedRequests, 1)
						time.Sleep(100 * time.Millisecond)
						continue
					}
				}

				reqStart := time.Now()
				atomic.AddInt64(&totalRequests, 1)

				// 发送数据
				_, err = conn.Write(testData)
				if err != nil {
					atomic.AddInt64(&failedRequests, 1)
					conn.Close()
					conn = nil
					continue
				}

				// 读取响应
				response := make([]byte, len(testData))
				_, err = io.ReadFull(conn, response)

				latency := time.Since(reqStart)
				latencyTracker.AddLatency(latency)

				if err != nil {
					atomic.AddInt64(&failedRequests, 1)
					conn.Close()
					conn = nil
					continue
				}

				atomic.AddInt64(&successRequests, 1)
				atomic.AddInt64(&totalBytes, int64(len(response)))

				if !config.KeepAlive {
					conn.Close()
					conn = nil
				}
			}
		}()
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	// 计算结果
	min, max, avg, p95, p99 := latencyTracker.GetPercentiles()

	result := &BenchmarkResult{
		TotalRequests:     atomic.LoadInt64(&totalRequests),
		SuccessRequests:   atomic.LoadInt64(&successRequests),
		FailedRequests:    atomic.LoadInt64(&failedRequests),
		TotalBytes:        atomic.LoadInt64(&totalBytes),
		TotalDuration:     totalDuration,
		MinLatency:        min,
		MaxLatency:        max,
		AvgLatency:        avg,
		P95Latency:        p95,
		P99Latency:        p99,
		RequestsPerSecond: float64(atomic.LoadInt64(&successRequests)) / totalDuration.Seconds(),
		BytesPerSecond:    float64(atomic.LoadInt64(&totalBytes)) / totalDuration.Seconds(),
		ErrorRate:         float64(atomic.LoadInt64(&failedRequests)) / float64(atomic.LoadInt64(&totalRequests)) * 100,
	}

	return result
}

// ThroughputBenchmark 吞吐量测试
func ThroughputBenchmark(config *BenchmarkConfig) *BenchmarkResult {
	log.Printf("Starting throughput benchmark: concurrency=%d, duration=%v, data_size=%d",
		config.Concurrency, config.Duration, config.DataSize)

	var (
		totalBytes      int64
		totalRequests   int64
		successRequests int64
		failedRequests  int64
		wg              sync.WaitGroup
		ctx, cancel     = context.WithTimeout(context.Background(), config.Duration)
	)
	defer cancel()

	startTime := time.Now()

	// 启动并发下载测试
	for i := 0; i < config.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// 创建HTTP客户端
			transport := &http.Transport{
				Dial: func(network, addr string) (net.Conn, error) {
					client := NewSOCKS5Client(config.ProxyAddr, 10*time.Second)
					return client.Connect(addr)
				},
			}

			httpClient := &http.Client{
				Transport: transport,
				Timeout:   30 * time.Second,
			}

			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				atomic.AddInt64(&totalRequests, 1)

				// 下载指定大小的数据
				targetURL := fmt.Sprintf("http://%s/download?size=%d", config.TargetAddr, config.DataSize)
				resp, err := httpClient.Get(targetURL)
				if err != nil {
					atomic.AddInt64(&failedRequests, 1)
					continue
				}

				written, err := io.Copy(io.Discard, resp.Body)
				resp.Body.Close()

				if err != nil || resp.StatusCode != 200 {
					atomic.AddInt64(&failedRequests, 1)
					continue
				}

				atomic.AddInt64(&successRequests, 1)
				atomic.AddInt64(&totalBytes, written)
			}
		}()
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	result := &BenchmarkResult{
		TotalRequests:     atomic.LoadInt64(&totalRequests),
		SuccessRequests:   atomic.LoadInt64(&successRequests),
		FailedRequests:    atomic.LoadInt64(&failedRequests),
		TotalBytes:        atomic.LoadInt64(&totalBytes),
		TotalDuration:     totalDuration,
		RequestsPerSecond: float64(atomic.LoadInt64(&successRequests)) / totalDuration.Seconds(),
		BytesPerSecond:    float64(atomic.LoadInt64(&totalBytes)) / totalDuration.Seconds(),
		ErrorRate:         float64(atomic.LoadInt64(&failedRequests)) / float64(atomic.LoadInt64(&totalRequests)) * 100,
	}

	return result
}

// PrintResult 打印测试结果
func PrintResult(testType string, result *BenchmarkResult) {
	fmt.Printf("\n=== %s Benchmark Results ===\n", testType)
	fmt.Printf("Total Requests:      %d\n", result.TotalRequests)
	fmt.Printf("Success Requests:    %d\n", result.SuccessRequests)
	fmt.Printf("Failed Requests:     %d\n", result.FailedRequests)
	fmt.Printf("Total Duration:      %v\n", result.TotalDuration)
	fmt.Printf("Requests/sec:        %.2f\n", result.RequestsPerSecond)
	fmt.Printf("Total Bytes:         %d (%.2f MB)\n", result.TotalBytes, float64(result.TotalBytes)/(1024*1024))
	fmt.Printf("Throughput:          %.2f MB/s\n", result.BytesPerSecond/(1024*1024))
	fmt.Printf("Error Rate:          %.2f%%\n", result.ErrorRate)

	if result.MinLatency > 0 {
		fmt.Printf("Min Latency:         %v\n", result.MinLatency)
		fmt.Printf("Max Latency:         %v\n", result.MaxLatency)
		fmt.Printf("Avg Latency:         %v\n", result.AvgLatency)
		fmt.Printf("95%% Latency:         %v\n", result.P95Latency)
		fmt.Printf("99%% Latency:         %v\n", result.P99Latency)
	}
	fmt.Println()
}

// RunBenchmarkSuite 运行完整的性能测试套件
func RunBenchmarkSuite(proxyAddr, targetAddr string) {
	fmt.Printf("Running SOCKS5 Proxy Benchmark Suite\n")
	fmt.Printf("Proxy: %s, Target: %s\n\n", proxyAddr, targetAddr)

	// 测试配置
	configs := []BenchmarkConfig{
		// HTTP测试 - 不同并发数
		{ProxyAddr: proxyAddr, TargetAddr: targetAddr, Concurrency: 10, Duration: 10 * time.Second, DataSize: 1024, TestType: "HTTP"},
		{ProxyAddr: proxyAddr, TargetAddr: targetAddr, Concurrency: 50, Duration: 10 * time.Second, DataSize: 1024, TestType: "HTTP"},
		{ProxyAddr: proxyAddr, TargetAddr: targetAddr, Concurrency: 100, Duration: 10 * time.Second, DataSize: 1024, TestType: "HTTP"},

		// TCP测试 - 不同数据包大小
		{ProxyAddr: proxyAddr, TargetAddr: targetAddr, Concurrency: 50, Duration: 10 * time.Second, DataSize: 1024, TestType: "TCP", KeepAlive: true},
		{ProxyAddr: proxyAddr, TargetAddr: targetAddr, Concurrency: 50, Duration: 10 * time.Second, DataSize: 10240, TestType: "TCP", KeepAlive: true},
		{ProxyAddr: proxyAddr, TargetAddr: targetAddr, Concurrency: 50, Duration: 10 * time.Second, DataSize: 102400, TestType: "TCP", KeepAlive: true},

		// 吞吐量测试
		{ProxyAddr: proxyAddr, TargetAddr: targetAddr, Concurrency: 20, Duration: 10 * time.Second, DataSize: 1024 * 1024, TestType: "Throughput"},
	}

	for i, config := range configs {
		fmt.Printf("Running test %d/%d: %s (concurrency=%d, data_size=%d)\n",
			i+1, len(configs), config.TestType, config.Concurrency, config.DataSize)

		var result *BenchmarkResult

		switch config.TestType {
		case "HTTP":
			result = HTTPBenchmark(&config)
		case "TCP":
			result = TCPBenchmark(&config)
		case "Throughput":
			result = ThroughputBenchmark(&config)
		}

		PrintResult(fmt.Sprintf("%s (C=%d, Size=%d)", config.TestType, config.Concurrency, config.DataSize), result)

		// 测试间隔
		time.Sleep(5 * time.Second)
	}
}

func main() {
	var (
		proxyAddr   = flag.String("proxy", "127.0.0.1:1080", "SOCKS5 proxy address")
		targetAddr  = flag.String("target", "127.0.0.1:8080", "Target server address")
		testType    = flag.String("type", "suite", "Test type: http, tcp, throughput, suite")
		concurrency = flag.Int("concurrency", 50, "Number of concurrent connections")
		duration    = flag.Duration("duration", 30*time.Second, "Test duration")
		dataSize    = flag.Int("data-size", 1024, "Data size in bytes")
		keepAlive   = flag.Bool("keep-alive", false, "Keep connections alive (TCP only)")
	)
	flag.Parse()

	if *testType == "suite" {
		RunBenchmarkSuite(*proxyAddr, *targetAddr)
		return
	}

	config := &BenchmarkConfig{
		ProxyAddr:   *proxyAddr,
		TargetAddr:  *targetAddr,
		Concurrency: *concurrency,
		Duration:    *duration,
		DataSize:    *dataSize,
		TestType:    *testType,
		KeepAlive:   *keepAlive,
	}

	var result *BenchmarkResult

	switch *testType {
	case "http":
		result = HTTPBenchmark(config)
	case "tcp":
		result = TCPBenchmark(config)
	case "throughput":
		result = ThroughputBenchmark(config)
	default:
		log.Fatalf("Unknown test type: %s", *testType)
	}

	PrintResult(*testType, result)
}
