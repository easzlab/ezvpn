package config

import "time"

// BufferSize is the max size of a single message transferred via a tunnel. This
// should be sufficiently large to prevent fragmentation.
const BufferSize = 1500

// AgentCancelWait is the timeout for agents to cancel
const AgentCancelWait = 3 * time.Second

// AgentRetryInterval is the timeout for a failed agent.Start()
const AgentRetryInterval = 15 * time.Second

// WsHandshakeTimeout is the timeout for agents connecting to tunnel-servers
const WsHandshakeTimeout = 10 * time.Second

// WsCloseTimeout is the timeout of a WebSocket close message.
const WsCloseTimeout = 3 * time.Second

// NetDialTimeout is the timeout for agents connecting to the destination
const NetDialTimeout = 5 * time.Second

// GoroutinePoolSize is the size for initializing an ants.Pool.
const GoroutinePoolSize = 10000

// SmuxSessionKeepAliveInterval is how often a mux session peer to send a NOP command to the remote
const SmuxSessionKeepAliveInterval = 5 * time.Second

// SmuxSessionKeepAliveTimeout is how long a mux session will be closed if no data has arrived
const SmuxSessionKeepAliveTimeout = 10 * time.Second
