#!/bin/bash

# ezvpn e2e test script
# This script tests the complete workflow of ezvpn agent and server

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

# Network configuration
SERVER_PORT=18443
AGENT_LOCAL_PORT=16116

# Process PIDs
SERVER_PID=""
AGENT_PID=""

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
    if [[ -n "$AGENT_PID" ]]; then
        kill $AGENT_PID 2>/dev/null || true
        wait $AGENT_PID 2>/dev/null || true
        log "Agent process stopped"
    fi
    
    if [[ -n "$SERVER_PID" ]]; then
        kill $SERVER_PID 2>/dev/null || true
        wait $SERVER_PID 2>/dev/null || true
        log "Server process stopped"
    fi
    
    # Remove temp directory
    make clean
    
    log_success "Cleanup completed"
}

# Set trap for cleanup
trap cleanup EXIT INT TERM

# Setup test environment
setup_environment() {
    log "Setting up test environment..."

    cd "$PROJECT_ROOT"
    
    # Clean up previous build
    make clean

    # Build binaries
    make build
    
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
    
    SERVER_PID=$!
    
    # Wait for server to start
    sleep 3
    
    # Check if server is running
    if ! kill -0 $SERVER_PID 2>/dev/null; then
        log_error "Server failed to start"
        cat "server.out"
        exit 1
    fi
    
    log_success "Server started successfully (PID: $SERVER_PID)"
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
    
    AGENT_PID=$!
    
    # Wait for agent to start
    sleep 3
    
    # Check if agent is running
    if ! kill -0 $AGENT_PID 2>/dev/null; then
        log_error "Agent failed to start"
        cat "agent.out"
        exit 1
    fi
    
    log_success "Agent started successfully (PID: $AGENT_PID)"
}

# Test SOCKS proxy functionality
test_socks_proxy() {
    log "Testing SOCKS proxy functionality..."
    
    # Test HTTP request through SOCKS proxy
    if command -v curl >/dev/null 2>&1; then
        log "Testing HTTP request through SOCKS proxy..."
        
        # Try to make a request through the SOCKS proxy
        if curl -s -x "socks5h://127.0.0.1:$AGENT_LOCAL_PORT" \
           --connect-timeout 5 \
           -o /dev/null \
           "v4.ident.me" 2>/dev/null; then
            log_success "HTTP request through SOCKS proxy successful"
        else
            log_warn "HTTP request through SOCKS proxy failed (this might be expected in isolated environments)"
        fi
    else
        log_warn "curl not available, skipping HTTP test"
    fi
    
    # Test basic SOCKS connection
    log "Testing basic SOCKS connection..."
    
    # Use netcat to test SOCKS connection if available
    if command -v nc >/dev/null 2>&1; then
        # Test connection to the SOCKS proxy
        if nc -z 127.0.0.1 $AGENT_LOCAL_PORT; then
            log_success "SOCKS proxy is accepting connections"
        else
            log_error "SOCKS proxy is not accepting connections"
            return 1
        fi
    else
        log_warn "netcat not available, skipping connection test"
    fi
}

# Test WebSocket tunnel
test_websocket_tunnel() {
    cd "$BINARY_DIR"

    log "Testing WebSocket tunnel..."
    
    # Check server logs for successful agent registration
    if grep -q "agent registered" "evs.log" 2>/dev/null; then
        log_success "Agent successfully registered with server"
    else
        log_error "Agent registration not found in server logs"
        return 1
    fi
    
    # Check agent logs for successful connection
    if grep -q "listening for connections" "eva.log" 2>/dev/null; then
        log_success "Agent is listening for connections"
    else
        log_warn "Agent connection status unclear from logs"
    fi
}

# Run comprehensive tests
run_tests() {
    log "Running e2e tests..."
    
    # Test 1: WebSocket tunnel establishment
    test_websocket_tunnel
    
    # Test 2: SOCKS proxy functionality
    test_socks_proxy
    
    # Test 3: Process health check
    log "Checking process health..."
    if kill -0 $SERVER_PID 2>/dev/null && kill -0 $AGENT_PID 2>/dev/null; then
        log_success "Both server and agent processes are healthy"
    else
        log_error "One or more processes are not healthy"
        return 1
    fi
}

# Main execution
main() {
    log "Starting ezvpn e2e tests ..."
    
    # Check prerequisites
    if ! command -v go >/dev/null 2>&1; then
        log_error "go is required but not installed"
        exit 1
    fi
    
    # Run test steps
    setup_environment
    start_server
    start_agent
    
    # Wait a bit for everything to stabilize
    sleep 2
    
    # Run tests
    if run_tests; then
        log_success "All e2e tests passed!"
        exit 0
    else
        log_error "Some e2e tests failed!"
        exit 1
    fi
}

# Show usage
usage() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help     Show this help message"
    echo "  -v, --verbose  Enable verbose output"
    echo ""
    echo "This script runs end-to-end tests for ezvpn agent and server using built-in certificates."
}

# Parse command line arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            usage
            exit 0
            ;;
        -v|--verbose)
            set -x
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

# Run main function
main "$@"