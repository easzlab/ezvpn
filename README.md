# EZVPN

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.19-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)]()

[README](README.md) | [中文文档](README_cn.md)

EZVPN is a secure VPN solution built on top of SOCKS5 protocol, providing encrypted tunneling based on WebSocket and mTLS mechanism.

## 🚀 Features

- **Secure Tunneling**: WebSocket-based tunneling with mTLS encryption
- **SOCKS5 Proxy**: Built-in SOCKS5 proxy server support
- **Multiplexing**: Efficient connection multiplexing using smux
- **Cross-Platform**: Supports macOS, Linux, and Windows
- **Built-in Certificates**: Default certificates for quick setup
- **Hot Reload**: Dynamic configuration reloading
- **High Performance**: Goroutine pool for concurrent connection handling

## 📋 Architecture

EZVPN consists of two main components:

### Agent (eva)
- Acts as a local SOCKS proxy client
- Establishes WebSocket connection to the server
- Tunnels local traffic through encrypted connection
- Supports automatic retry with exponential backoff

### Server (evs)
- Accepts WebSocket connections from agents
- Provides SOCKS5 proxy services
- Manages multiple concurrent agent connections
- Supports both inline and external SOCKS servers

```
┌─────────────┐        socks5       ┌─────────────┐   ws+mTLS    ┌─────────────┐
│   Client    │ ◄─────────────────► │   Agent     │ ◄──────────► │   Server    │
│ Application │                     │   (eva)     │              │    (evs)    │
└─────────────┘                     └─────────────┘              └─────────────┘
                                                                        │
                                                                        ▼
                                                                 ┌─────────────┐
                                                                 │ Destination │
                                                                 │   Server    │
                                                                 └─────────────┘
```

## 🛠️ Installation

### Prerequisites

- Go 1.24 or higher
- Make (for building)

### Build from Source

```bash
# Clone the repository
git clone https://github.com/easzlab/ezvpn.git
cd ezvpn

# Build binaries
make build

# The binaries will be available in the build/ directory
ls build/
# eva  evs
```

### Quick Start with Built-in Certificates

The project includes built-in certificates for quick testing and development.

## 🚀 Usage

### Starting the Server

```bash
# Start server with default settings
./build/evs

# Start server with custom configuration
./build/evs --listen=":8443" --withsocks=true --loglvl=info

# Start server with external SOCKS server
./build/evs --withsocks=false --socks="127.0.0.1:1080"
```

### Starting the Agent

```bash
# Start agent (requires auth key)
./build/eva --auth="your-auth-key" --server="127.0.0.1:8443"

# Start agent with custom local address
./build/eva --auth="your-auth-key" --listen="127.0.0.1:6116"

# Disable TLS (for testing only)
./build/eva --auth="your-auth-key" --tls=false
```

### Configuration

#### Server Configuration

Create an `agents.yaml` file to configure allowed agents:

```yaml
agents:
  - name: "agent-1"
    auth_key: "your-secret-auth-key-1"
    approved_cn: "ezvpn-agent"
  - name: "agent-2"
    auth_key: "your-secret-auth-key-2"
    approved_cn: "ezvpn-agent"
```

#### Command Line Options

**Server (evs) Options:**
```
  -a, --agentsfile string   allowed agents file (default "agents.yaml")
  -l, --listen string       server listen address (default "127.0.0.1:8443")
      --socks string        socks server address (default "socks.sock")
      --withsocks           enable inline socks (default true)
      --tls                 enable tls (default true)
      --ca string           ca file (default "ca.pem")
      --cert string         cert file (default "evs.pem")
      --key string          key file (default "evs-key.pem")
      --logfile string      log file (default "evs.log")
      --loglvl string       log level (default "debug")
      --pprof               enable pprof
  -v, --version             show version
```

**Agent (eva) Options:**
```
  -k, --auth string         auth key (required)
  -s, --server string       server address (default "127.0.0.1:8443")
  -l, --listen string       local address (default "127.0.0.1:6116")
      --tls                 enable tls (default true)
      --ca string           ca file (default "ca.pem")
      --cert string         cert file (default "eva.pem")
      --key string          key file (default "eva-key.pem")
      --lock string         lock file (default "eva.lock")
      --logfile string      log file (default "eva.log")
      --loglvl string       log level (default "debug")
      --pprof               enable pprof
  -v, --version             show version
```

## 🧪 Testing

### Unit Tests

```bash
# Run all tests
make test

# Run tests with coverage
make test-cov
```

### End-to-End Tests

```bash
# Run e2e tests
./tests/e2e_test.sh
```

## 🔧 Development

### Project Structure

```
ezvpn/
├── cmd/                    # Command line applications
│   ├── agent/             # Agent (eva) main
│   └── server/            # Server (evs) main
├── pkg/                   # Public packages
│   ├── agent/             # Agent implementation
│   ├── server/            # Server implementation
│   └── utils/             # Utility functions
├── internal/              # Private packages
│   └── config/            # Configuration management
├── tests/                 # Test files
│   └── e2e_test.sh       # End-to-end test script
├── docs/                  # Documentation
├── Makefile              # Build automation
└── README.md             # This file
```

## 🔒 Security

### mTLS Authentication

EZVPN uses mutual TLS (mTLS) for secure authentication:

1. **Certificate Authority (CA)**: Validates both client and server certificates
2. **Server Certificate**: Authenticates the server to clients
3. **Client Certificate**: Authenticates clients to the server
4. **Certificate Validation**: Both sides validate certificates against the CA

### Built-in Certificates

For development and testing, EZVPN includes built-in certificates. **Do not use these in production environments.**

### Custom Certificates

For production use, generate your own certificates:

```bash
# Generate CA private key
openssl genrsa -out ca-key.pem 4096

# Generate CA certificate
openssl req -new -x509 -days 365 -key ca-key.pem -out ca.pem

# Generate server private key and certificate
openssl genrsa -out evs-key.pem 4096
openssl req -new -key evs-key.pem -out evs.csr
openssl x509 -req -days 365 -in evs.csr -CA ca.pem -CAkey ca-key.pem -out evs.pem

# Generate client private key and certificate
openssl genrsa -out eva-key.pem 4096
openssl req -new -key eva-key.pem -out eva.csr
openssl x509 -req -days 365 -in eva.csr -CA ca.pem -CAkey ca-key.pem -out eva.pem
```

## 📊 Performance

### Benchmarks

- **Concurrent Connections**: Supports thousands of concurrent connections
- **Throughput**: High-speed data transfer through multiplexed streams
- **Memory Usage**: Efficient memory management with goroutine pools
- **Latency**: Low-latency tunneling with WebSocket connections

### Monitoring

Enable pprof for performance monitoring:

```bash
# Server with pprof
./build/evs --pprof

# Agent with pprof
./build/eva --pprof --auth="your-key"

# Access pprof endpoints
curl http://localhost:6062/debug/pprof/  # Server
curl http://localhost:6061/debug/pprof/  # Agent
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

### Development Guidelines

- Follow Go best practices and conventions
- Add tests for new functionality
- Update documentation as needed
- Ensure all tests pass before submitting PR

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket implementation
- [xtaci/smux](https://github.com/xtaci/smux) - Stream multiplexing
- [panjf2000/ants](https://github.com/panjf2000/ants) - Goroutine pool
- [labstack/echo](https://github.com/labstack/echo) - HTTP framework
- [spf13/cobra](https://github.com/spf13/cobra) - CLI framework

## 📞 Support

- 📧 Email: support@easzlab.com
- 🐛 Issues: [GitHub Issues](https://github.com/easzlab/ezvpn/issues)
- 📖 Documentation: [Wiki](https://github.com/easzlab/ezvpn/wiki)

---

**EZVPN** - Secure, Fast, and Reliable VPN Solution