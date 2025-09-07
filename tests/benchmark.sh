#!/bin/bash

# ezvpn benchmark test script

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
TEST_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$TEST_DIR")"
BINARY_DIR="$PROJECT_ROOT/build"
SERVER_PORT=18443
AGENT_LOCAL_PORT=16116
TARGET_HTTP_PORT=8000
TARGET_TCP_PORT=8001
TEST_DURATION="15s"
PROXY_ADDR="127.0.0.1:$AGENT_LOCAL_PORT"
TARGET_HTTP_ADDR="127.0.0.1:$TARGET_HTTP_PORT"
TARGET_TCP_ADDR="127.0.0.1:$TARGET_TCP_PORT"
RESULTS_DIR="$PROJECT_ROOT/build/benchmark-results-$(date +%Y%m%d-%H%M%S)"

# Process PIDs
EZVPN_SERVER_PID=""
EZVPN_AGENT_PID=""
TEST_SERVER_PID=""

# Logging function
log() {
    echo -e "${BLUE}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# Cleanup function
cleanup() {
    log "Cleaning up test environment..."

    cd "$PROJECT_ROOT"
    
    # Kill processes
    if [[ -n "$EZVPN_AGENT_PID" ]]; then
        kill $EZVPN_AGENT_PID 2>/dev/null || true
        wait $EZVPN_AGENT_PID 2>/dev/null || true
        log "Agent process stopped"
    fi
    
    if [[ -n "$EZVPN_SERVER_PID" ]]; then
        kill $EZVPN_SERVER_PID 2>/dev/null || true
        wait $EZVPN_SERVER_PID 2>/dev/null || true
        log "Server process stopped"
    fi

    if [[ -n "$TEST_SERVER_PID" ]]; then
        kill $TEST_SERVER_PID 2>/dev/null || true
        wait $TEST_SERVER_PID 2>/dev/null || true
        log "Test server stopped"
    fi
    
    log_success "Cleanup completed"
}

# Set trap for cleanup
trap cleanup EXIT INT TERM

# Setup test environment
setup_environment() {
    log "Setting up test environment..."

    cd "$PROJECT_ROOT"

    # Build binaries
    make build
    go build -o "$PROJECT_ROOT/build/test-server" tests/benchmark/test-server/test-server.go
    go build -o "$PROJECT_ROOT/build/proxy-test" tests/benchmark/proxy-test/proxy-test.go
    
    log_success "Environment setup completed"
}

# Start server
start_server() {
    log "Starting ezvpn server..."
    
    cd "$BINARY_DIR"
    
    # Start server in background with built-in certificates
    ./evs \
        --tls=true \
        --listen=":$SERVER_PORT" \
        --withsocks=true \
        --loglvl=debug > "server.out" 2>&1 &
    
    EZVPN_SERVER_PID=$!
    
    # Wait for server to start
    sleep 3
    
    # Check if server is running
    if ! kill -0 $EZVPN_SERVER_PID 2>/dev/null; then
        log_error "EZVPN Server failed to start"
        cat "server.out"
        exit 1
    fi
    
    log_success "EZVPN Server started successfully (PID: $EZVPN_SERVER_PID)"

    # Start test server
    ./test-server -http-port=$TARGET_HTTP_PORT -tcp-port=$TARGET_TCP_PORT > test-server.log 2>&1 &
    TEST_SERVER_PID=$!

    sleep 2
    if ! kill -0 $TEST_SERVER_PID 2>/dev/null; then
        log_error "Test server failed to start"
        cat "test-server.log"
        exit 1
    fi

    log_success "Test server started successfully (PID: $TEST_SERVER_PID)"
}

# Start agent
start_agent() {
    log "Starting ezvpn agent..."
    
    cd "$BINARY_DIR"
    
    # Start agent in background with built-in certificates
    ./eva \
        --auth="3b02fe02-2552-78ae-ed40-d4df03fc21c9" \
        --tls=true \
        --server="127.0.0.1:$SERVER_PORT" \
        --listen=":$AGENT_LOCAL_PORT" \
        --loglvl=debug > "agent.out" 2>&1 &
    
    EZVPN_AGENT_PID=$!
    
    # Wait for agent to start
    sleep 3
    
    # Check if agent is running
    if ! kill -0 $EZVPN_AGENT_PID 2>/dev/null; then
        log_error "Agent failed to start"
        cat "agent.out"
        exit 1
    fi
    
    log_success "Agent started successfully (PID: $EZVPN_AGENT_PID)"
}

# Test SOCKS proxy functionality
check_proxy() {
    log "Testing SOCKS proxy functionality..."
    
    # Run proxy-test
    timeout 5 ./proxy-test \
        -proxy="$PROXY_ADDR" \
        -target="http://127.0.0.1:$TARGET_HTTP_PORT" \
        -type="http" -concurrency=1 -duration=1s > "proxy-test.out" 2>&1
    
    # Check if proxy-test is successful
    if [ $? -eq 0 ]; then
        log_success "SOCKS5 proxy connection successful"
    else
        log_error "SOCKS5 proxy connection failed"
        cat "proxy-test.out"
        exit 1
    fi
}

# Run single test
run_test() {
    local test_name=$1
    local test_type=$2
    local target_addr=$3
    local concurrency=$4
    local data_size=$5
    local extra_args=$6
    
    log "Running test: $test_name"
    
    local output_file="$RESULTS_DIR/${test_name}.txt"
    
    ./proxy-test \
        -proxy="$PROXY_ADDR" \
        -target="$target_addr" \
        -type="$test_type" \
        -concurrency=$concurrency \
        -duration=$TEST_DURATION \
        -data-size=$data_size \
        $extra_args \
        > "$output_file" 2>&1
    
    if [ $? -eq 0 ]; then
        log_success "Test $test_name Successed"
        # Retrieve summary
        echo "=== $test_name ===" >> "$RESULTS_DIR/summary.txt"
        grep -E "(Requests/sec|Throughput|Error Rate|Avg Latency)" "$output_file" >> "$RESULTS_DIR/summary.txt"
        echo "" >> "$RESULTS_DIR/summary.txt"
    else
        log_error "Test $test_name Failed"
        cat "$output_file"
    fi
}

# Run benchmark suite
run_benchmark_suite() {
    log "Starting performance test suite..."
    
    mkdir -p "$RESULTS_DIR"
    
    # Print test configuration
    cat > "$RESULTS_DIR/config.txt" << EOF
Test Configuration:
Proxy address: $PROXY_ADDR
Target HTTP address: $TARGET_HTTP_ADDR  
Target TCP address: $TARGET_TCP_ADDR
Time Duration: $TEST_DURATION
Start time: $(date)
EOF
    
    log "=== HTTP Performance Test ==="
    run_test "http-concurrency-10" "http" "$TARGET_HTTP_ADDR" 10 1024
    #run_test "http-concurrency-50" "http" "$TARGET_HTTP_ADDR" 50 1024
    run_test "http-concurrency-100" "http" "$TARGET_HTTP_ADDR" 100 1024
    
    log "=== TCP Performance Test ==="
    run_test "tcp-concurrency-50-1kb" "tcp" "$TARGET_TCP_ADDR" 50 1024 "-keep-alive=true"
    #run_test "tcp-concurrency-50-10kb" "tcp" "$TARGET_TCP_ADDR" 50 10240 "-keep-alive=true"
    run_test "tcp-concurrency-50-100kb" "tcp" "$TARGET_TCP_ADDR" 50 102400 "-keep-alive=true"
    
    log "=== Throughput Test ==="
    run_test "throughput-1mb" "throughput" "$TARGET_HTTP_ADDR" 20 1048576
    run_test "throughput-10mb" "throughput" "$TARGET_HTTP_ADDR" 20 10485760
    
    log "=== High Concurrency Test ==="
    # Raise limit so that connections won't be killed by kernel. net.core.somaxconn
    # (or kern.ipc.somaxconn on BSDs and mac) should also be raised if you test for
    # a very high number of concurrent access.
    ulimit -n 10000

    run_test "http-high-concurrency-200" "http" "$TARGET_HTTP_ADDR" 200 1024
    run_test "http-high-concurrency-500" "http" "$TARGET_HTTP_ADDR" 500 1024
    
    log_success "All tests completed, results saved in $RESULTS_DIR/"
}

# Generate test report
generate_report() {
    log "Generating test report..."
    
    local report_file="$RESULTS_DIR/report.md"
    
    cat > "$report_file" << EOF
# SOCKS5 Proxy Performance Test Report

## Test Environment

- Test time: $(date)
- Proxy address: $PROXY_ADDR
- Target server: $TARGET_HTTP_ADDR (HTTP), $TARGET_TCP_ADDR (TCP)
- Test duration: $TEST_DURATION
- Operating system: $(uname -s)
- Go version: $(go version)

## Test Results Summary

\`\`\`
$(cat "$RESULTS_DIR/summary.txt")
\`\`\`

## Detailed Results

Detailed test results can be found in individual test files:

EOF

    # Add links to individual test files
    for file in "$RESULTS_DIR"/*.txt; do
        if [ -f "$file" ] && [ "$(basename "$file")" != "summary.txt" ] && [ "$(basename "$file")" != "config.txt" ]; then
            echo "- [$(basename "$file" .txt)]($(basename "$file"))" >> "$report_file"
        fi
    done
    
    log_success "Test report generated: $report_file"
}

# Main function
main() {
    echo "=========================================="
    echo "    SOCKS5 Proxy Performance Test Tool"
    echo "=========================================="
    echo ""
    
    log "Test configuration:"
    log "  Proxy address: $PROXY_ADDR"
    log "  Target HTTP address: $TARGET_HTTP_ADDR"
    log "  Target TCP address: $TARGET_TCP_ADDR"
    log "  Test duration: $TEST_DURATION"
    echo ""
    
    setup_environment
    start_server
    start_agent
    check_proxy
    run_benchmark_suite
    generate_report
    
    echo ""
    log_success "Performance test completed!"
    log "Results directory: $RESULTS_DIR"
    log "Test report: $RESULTS_DIR/report.md"
    log "Results summary: $RESULTS_DIR/summary.txt"
}

# Run main function
main