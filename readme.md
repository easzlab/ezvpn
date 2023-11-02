# EZVPN

## 流程

1. 客户端注册
通过mTLS，客户端使用token，服务端验证通过，建立wss连接（控制连接）

2. 客户端监听端口
控制连接建立成功后，客户端监听tcp端口，监听实际socks5流量请求

3. 客户端实际流量处理
客户端新流量连接建立后，新建wss数据连接，连接建立成功后；数据转发如下：
(socks client) <--conn--> Agent <--ws--> Server <--conn--> (socks server) <--> Destination

## 编译

1. macOS

```
# 编译 ezvpn-agent
GOOS=darwin GOARCH=amd64 CGO=0 go build -o ezvpn-agent cmd/agent/main.go
# 编译 ezvpn-server
GOOS=darwin GOARCH=amd64 CGO=0 go build -o ezvpn-server cmd/server/main.go
```

2. linux

```
# 编译 ezvpn-agent
GOOS=linux GOARCH=amd64 CGO=0 go build -o ezvpn-agent cmd/agent/main.go
# 编译 ezvpn-server
GOOS=linux GOARCH=amd64 CGO=0 go build -o ezvpn-server cmd/server/main.go
# 静态编译
CGO_ENABLED=0 go build -ldflags='-w -s -extldflags -static' -o ezvpn-server cmd/server/main.go
```

3. windows

```
# 编译 ezvpn-agent.exe，支持后台运行
GOOS=windows GOARCH=amd64 CGO=0 go build -ldflags -H=windowsgui -o ezvpn-agent.exe cmd/agent/main.go
# 编译 ezvpn-server.exe，支持后台运行
GOOS=windows GOARCH=amd64 CGO=0 go build -ldflags -H=windowsgui -o ezvpn-server.exe cmd/server/main.go
```