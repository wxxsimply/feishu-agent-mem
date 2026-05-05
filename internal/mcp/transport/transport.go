package transport

import (
	"io"
)

// Transport MCP 传输接口
type Transport interface {
	Start() error
	Stop() error
	In() io.Reader
	Out() io.Writer
}

// TransportType 传输类型
type TransportType string

const (
	TransportTypeStdio    TransportType = "stdio"
	TransportTypeHTTP     TransportType = "http"
	TransportTypeWebSocket TransportType = "websocket"
)

// Config 传输配置
type Config struct {
	Type TransportType

	// HTTP/WebSocket 模式配置
	Addr string
	Path string
}
