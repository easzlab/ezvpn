# EZVPN - Easy VPN Over Secured Socks Proxy

# Project information
PROJECT_NAME := ezvpn
MODULE_NAME := github.com/easzlab/ezvpn
BUILD_TIME := $(shell date +%Y-%m-%d\ %H:%M:%S)
BUILD_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_BRANCH := $(shell git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "unknown")

# Build configuration
BUILD_DIR := build

# Linker flags for build information
LDFLAGS := -ldflags "-X '$(MODULE_NAME)/internal/config.BuildTime=$(BUILD_TIME)' \
                     -X '$(MODULE_NAME)/internal/config.BuildCommit=$(BUILD_COMMIT)' \
					 -X '$(MODULE_NAME)/internal/config.BuildBranch=$(BUILD_BRANCH)' \
                     -s -w -extldflags -static"

# Default target
.PHONY: all
all: clean build

.PHONY: help
help: ## show help
	@echo "Available targets: "
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z0-9_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

.PHONY: build
build: ## build the binary
	@echo "Building $(PROJECT_NAME) agent(eva) and server(evs)..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build $(LDFLAGS) -o build/eva cmd/agent/main.go
	CGO_ENABLED=0 go build $(LDFLAGS) -o build/evs cmd/server/main.go
	@echo "✓ Build completed."

.PHONY: build-dev
build-dev: ## build the binary with debug info
	@echo "Building $(PROJECT_NAME) for development..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 go build -race -o build/eva cmd/agent/main.go
	CGO_ENABLED=0 go build -race -o build/evs cmd/server/main.go
	@echo "✓ Development build completed."

.PHONY: build-linux
build-linux: ## build the binary for Linux
	@echo "Building for Linux..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o build/eva-linux-amd64 cmd/agent/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o build/evs-linux-arm64 cmd/server/main.go
	@echo "✓ Linux build completed"

.PHONY: build-windows
build-windows: ## build the binary for Windows
	@echo "Building for Windows..."
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 CGO=0 go build -ldflags -H=windowsgui -o build/eva-windows-amd64.exe cmd/agent/main.go
	@echo "✓ Windows build completed"

.PHONY: build-darwin
build-darwin: ## build the binary for macOS
	@echo "Building for macOS..."
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o build/eva-darwin-amd64 cmd/agent/main.go
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o build/evs-darwin-arm64 cmd/server/main.go
	@echo "✓ macOS builds completed"

.PHONY: build-all
build-all: build-linux build-windows build-darwin ## build the binary for all platforms
	@echo "✓ All platform builds completed"

.PHONY: test
test: ## run tests
	@echo "Running tests..."
	@go test -v ./...
	@echo "✓ Tests completed"

.PHONY: test-cov
test-cov: ## run tests with coverage
	@echo "Running tests with coverage..."
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report generated: coverage.html"

.PHONY: e2e
e2e: ## run end-to-end tests
	@echo "Running end-to-end tests..."
	@bash tests/e2e_test.sh
	@echo "✓ End-to-end tests completed"

.PHONY: bench
bench: ## run benchmarks
	@echo "Running benchmarks..."
	@bash tests/benchmark.sh
	@echo "✓ Benchmarks completed"

.PHONY: deps
deps: ## download dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@echo "✓ Dependencies downloaded"

.PHONY: clean
clean: ## clean build artifacts
	@echo "Cleaning build artifacts..."
	@rm -rf $(BUILD_DIR)
	@rm -f coverage.out coverage.html
	@echo "✓ Clean completed"

.PHONY: update
update: ## update dependencies
	@echo "Updating dependencies..."
	@go get -u ./...
	@go mod tidy
	@echo "✓ Dependencies updated"