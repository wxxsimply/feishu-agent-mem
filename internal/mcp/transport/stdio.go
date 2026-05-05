package transport

import (
	"io"
	"os"
)

// StdioTransport 标准输入输出传输
type StdioTransport struct {
	in  io.Reader
	out io.Writer
}

// NewStdioTransport 创建 stdio 传输
func NewStdioTransport() *StdioTransport {
	return &StdioTransport{
		in:  os.Stdin,
		out: os.Stdout,
	}
}

// Start 启动传输
func (t *StdioTransport) Start() error {
	return nil
}

// Stop 停止传输
func (t *StdioTransport) Stop() error {
	return nil
}

// In 返回输入流
func (t *StdioTransport) In() io.Reader {
	return t.in
}

// Out 返回输出流
func (t *StdioTransport) Out() io.Writer {
	return t.out
}
