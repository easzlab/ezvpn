# EZVPN

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.19-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)
[![Build Status](https://img.shields.io/badge/build-passing-brightgreen.svg)]()

EZVPN 是一个基于 SOCKS5 协议构建的安全 VPN 解决方案，通过 WebSocket 和 mTLS 提供加密隧道。

## 🚀 特性

- **安全隧道**: 基于 WebSocket 的隧道，支持 mTLS 加密
- **SOCKS5 代理**: 内置 SOCKS5 代理服务器支持
- **多路复用**: 使用 smux 进行高效连接多路复用
- **跨平台**: 支持 macOS、Linux 和 Windows
- **内置证书**: 提供默认证书以便快速设置
- **热重载**: 动态配置重新加载
- **高性能**: 使用协程池处理并发连接

## 📋 架构

EZVPN 由两个主要组件组成：

### 代理端 (eva)
- 作为本地 SOCKS 代理客户端
- 建立到服务器的 WebSocket 连接
- 通过加密连接隧道传输本地流量
- 支持指数退避的自动重试

### 服务端 (evs)
- 接受来自代理端的 WebSocket 连接
- 提供 SOCKS5 代理服务
- 管理多个并发代理连接
- 支持内联和外部 SOCKS 服务器

```
┌─────────────┐        socks5       ┌─────────────┐   ws+mTLS    ┌─────────────┐
│   客户端     │ ◄─────────────────► │   代理端     │ ◄──────────► │   服务端     │
│   应用程序   │                     │   (eva)     │              │    (evs)    │
└─────────────┘                     └─────────────┘              └─────────────┘
                                                                        │
                                                                        ▼
                                                                 ┌─────────────┐
                                                                 │   目标       │
                                                                 │   服务器     │
                                                                 └─────────────┘
```

## 🛠️ 安装

### 前置要求

- Go 1.24 或更高版本
- Make（用于构建）

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/easzlab/ezvpn.git
cd ezvpn

# 构建二进制文件
make build

# 二进制文件将在 build/ 目录中可用
ls build/
# eva  evs
```

### 使用内置证书快速开始

项目包含内置证书，便于快速测试和开发。

## 🚀 使用方法

### 启动服务端

```bash
# 使用默认设置启动服务端
./build/evs

# 使用自定义配置启动服务端
./build/evs --listen=":8443" --withsocks=true --loglvl=info

# 使用外部 SOCKS 服务器启动服务端
./build/evs --withsocks=false --socks="127.0.0.1:1080"
```

### 启动代理端

```bash
# 启动代理端（需要认证密钥）
./build/eva --auth="your-auth-key" --server="127.0.0.1:8443"

# 使用自定义本地地址启动代理端
./build/eva --auth="your-auth-key" --listen="127.0.0.1:6116"

# 禁用 TLS（仅用于测试）
./build/eva --auth="your-auth-key" --tls=false
```

### 配置

#### 服务端配置

创建 `agents.yaml` 文件来配置允许的代理端：

```yaml
agents:
  - name: "agent-1"
    auth_key: "your-secret-auth-key-1"
    approved_cn: "ezvpn-agent"
  - name: "agent-2"
    auth_key: "your-secret-auth-key-2"
    approved_cn: "ezvpn-agent"
```

#### 命令行选项

**服务端 (evs) 选项：**
```
  -a, --agentsfile string   允许的代理端文件 (默认 "agents.yaml")
  -l, --listen string       服务端监听地址 (默认 "127.0.0.1:8443")
      --socks string        socks 服务器地址 (默认 "socks.sock")
      --withsocks           启用内联 socks (默认 true)
      --tls                 启用 tls (默认 true)
      --ca string           ca 文件 (默认 "ca.pem")
      --cert string         证书文件 (默认 "evs.pem")
      --key string          密钥文件 (默认 "evs-key.pem")
      --logfile string      日志文件 (默认 "evs.log")
      --loglvl string       日志级别 (默认 "debug")
      --pprof               启用 pprof
  -v, --version             显示版本
```

**代理端 (eva) 选项：**
```
  -k, --auth string         认证密钥 (必需)
  -s, --server string       服务器地址 (默认 "127.0.0.1:8443")
  -l, --listen string       本地地址 (默认 "127.0.0.1:6116")
      --tls                 启用 tls (默认 true)
      --ca string           ca 文件 (默认 "ca.pem")
      --cert string         证书文件 (默认 "eva.pem")
      --key string          密钥文件 (默认 "eva-key.pem")
      --lock string         锁文件 (默认 "eva.lock")
      --logfile string      日志文件 (默认 "eva.log")
      --loglvl string       日志级别 (默认 "debug")
      --pprof               启用 pprof
  -v, --version             显示版本
```

## 🧪 测试

### 单元测试

```bash
# 运行所有测试
make test

# 运行带覆盖率的测试
make test-cov
```

### 端到端测试

```bash
# 运行 e2e 测试
./tests/e2e_test.sh
```

## 🔧 开发

### 项目结构

```
ezvpn/
├── cmd/                    # 命令行应用程序
│   ├── agent/             # 代理端 (eva) 主程序
│   └── server/            # 服务端 (evs) 主程序
├── pkg/                   # 公共包
│   ├── agent/             # 代理端实现
│   ├── server/            # 服务端实现
│   └── utils/             # 工具函数
├── internal/              # 私有包
│   └── config/            # 配置管理
├── tests/                 # 测试文件
│   └── e2e_test.sh       # 端到端测试脚本
├── docs/                  # 文档
├── Makefile              # 构建自动化
└── README.md             # 本文件
```

## 🔒 安全

### mTLS 身份验证

EZVPN 使用双向 TLS (mTLS) 进行安全身份验证：

1. **证书颁发机构 (CA)**：验证客户端和服务端证书
2. **服务端证书**：向客户端验证服务端身份
3. **客户端证书**：向服务端验证客户端身份
4. **证书验证**：双方都根据 CA 验证证书

### 内置证书

对于开发和测试，EZVPN 包含内置证书。**请勿在生产环境中使用这些证书。**

### 自定义证书

对于生产使用，生成您自己的证书：

```bash
# 生成 CA 私钥
openssl genrsa -out ca-key.pem 4096

# 生成 CA 证书
openssl req -new -x509 -days 365 -key ca-key.pem -out ca.pem

# 生成服务端私钥和证书
openssl genrsa -out evs-key.pem 4096
openssl req -new -key evs-key.pem -out evs.csr
openssl x509 -req -days 365 -in evs.csr -CA ca.pem -CAkey ca-key.pem -out evs.pem

# 生成客户端私钥和证书
openssl genrsa -out eva-key.pem 4096
openssl req -new -key eva-key.pem -out eva.csr
openssl x509 -req -days 365 -in eva.csr -CA ca.pem -CAkey ca-key.pem -out eva.pem
```

## 📊 性能

### 基准测试

- **并发连接**：支持数千个并发连接
- **吞吐量**：通过多路复用流实现高速数据传输
- **内存使用**：使用协程池进行高效内存管理
- **延迟**：通过 WebSocket 连接实现低延迟隧道

### 监控

启用 pprof 进行性能监控：

```bash
# 启用 pprof 的服务端
./build/evs --pprof

# 启用 pprof 的代理端
./build/eva --pprof --auth="your-key"

# 访问 pprof 端点
curl http://localhost:6062/debug/pprof/  # 服务端
curl http://localhost:6061/debug/pprof/  # 代理端
```

## 🤝 贡献

1. Fork 仓库
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交您的更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 打开 Pull Request

### 开发指南

- 遵循 Go 最佳实践和约定
- 为新功能添加测试
- 根据需要更新文档
- 确保所有测试在提交 PR 前通过

## 📝 许可证

本项目基于 MIT 许可证 - 详情请参阅 [LICENSE](LICENSE) 文件。

## 🙏 致谢

- [gorilla/websocket](https://github.com/gorilla/websocket) - WebSocket 实现
- [xtaci/smux](https://github.com/xtaci/smux) - 流多路复用
- [panjf2000/ants](https://github.com/panjf2000/ants) - 协程池
- [labstack/echo](https://github.com/labstack/echo) - HTTP 框架
- [spf13/cobra](https://github.com/spf13/cobra) - CLI 框架

## 📞 支持

- 📧 邮箱：support@easzlab.com
- 🐛 问题反馈：[GitHub Issues](https://github.com/easzlab/ezvpn/issues)
- 📖 文档：[Wiki](https://github.com/easzlab/ezvpn/wiki)

---

**EZVPN** - 安全、快速、可靠的 VPN 解决方案