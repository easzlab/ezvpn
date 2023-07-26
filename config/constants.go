package config

import "time"

// BufferSize is the max size of a single message transferred via a tunnel. This
// should be sufficiently large to prevent fragmentation.
const BufferSize = 1500

// SessionIDSize is the number of random bytes encoded in a session ID.
const SessionIDSize = 16

// AgentCancelWait is the timeout for agents to cancel
const AgentCancelWait = 3 * time.Second

// AgentRetryInterval is the timeout for a failed agent.Start()
const AgentRetryInterval = 15 * time.Second

// WsHandshakeTimeout is the timeout for agents connecting to tunnel-servers
const WsHandshakeTimeout = 10 * time.Second

// WsCloseTimeout is the timeout of a WebSocket close message.
const WsCloseTimeout = 3 * time.Second

// WsKeepliveInterval is used for checking websocket connection loss.
const WsKeepliveInterval = 3 * time.Second

// NetDialTimeout is the timeout for agents connecting to the destination
const NetDialTimeout = 5 * time.Second

// AcceptRetryWait is the time to wait before retrying a failed Accept().
const AcceptRetryWait = 100 * time.Millisecond

// UdpSessionTimeout is the timeout used to kill idle UDP session.
const UdpSessionTimeout = 300 * time.Second

// GoroutinePoolSize is the size for initializing an ants.Pool.
const GoroutinePoolSize = 10000
