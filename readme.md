# EZVPN

## 流程

1. 客户端注册
通过mTLS，客户端使用token，服务端验证通过，建立wss连接（控制连接）

2. 客户端监听端口
控制连接建立成功后，客户端监听tcp端口，监听实际socks5流量请求

3. 客户端实际流量处理
客户端新流量连接建立后，新建wss数据连接，连接建立成功后；数据转发如下：
(socks client) <--conn--> Agent <--ws--> Server <--conn--> (socks server) <--> Destination

