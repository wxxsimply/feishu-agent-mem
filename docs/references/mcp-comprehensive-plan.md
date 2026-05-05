# MCP 完整实现与集成计划

> 基于当前实现状态，制定完整的 MCP 生态系统计划，包括 mcporter 集成、多传输模式支持、客户端 SDK 等。

---

## 一、现状评估

### 1.1 已完成的工作（✓）

**协议层**
- ✓ JSON-RPC 2.0 基础协议实现
- ✓ 标准错误码（-32700, -32601, -32602, -32603, -32003, -32002）
- ✓ 初始化握手流程（initialize + notifications/initialized）
- ✓ 并发控制（semaphore，最大 20 并发）
- ✓ 优雅关闭（context propagation + wait group）
- ✓ 大消息支持（1MB buffer）

**能力层**
- ✓ Tools：8 个工具（search, topic, decision, extract_decision, classify_topic, detect_crosstopic, check_conflict, timeline）
- ✓ Resources：2 个资源（docs://design, docs://prompts）
- ✓ Prompts：4 个提示词模板
- ✓ LLM 集成（豆包大模型 via Volcengine Ark）

**架构层**
- ✓ stdio 传输模式（标准输入/输出）
- ✓ 与 Memory Graph 集成
- ✓ 与 Git Storage 集成
- ✓ 与 Bitable 集成（框架已搭，待完善）

**测试层**
- ✓ 协议格式测试
- ✓ 真实 LLM 集成测试

### 1.2 仍需完善的工作（⚠️）

**协议完善**
- ⚠️ roots/list 方法
- ⚠️ logging/message 通知
- ⚠️ window/showMessage 通知
- ⚠️ window/showDocument 通知
- ⚠️ 进度通知（notifications/progress）
- ⚠️ 分页支持（cursor 参数）
- ⚠️ 资源模板（resources/templates/list）

**架构增强**
- ⚠️ HTTP + SSE 传输模式
- ⚠️ WebSocket 传输模式
- ⚠️ 传输模式抽象（统一的 Transport 接口）
- ⚠️ 多实例支持与分布式一致性

**功能扩展**
- ⚠️ 写操作工具（创建/更新/删除决策）
- ⚠️ 飞书操作工具暴露
- ⚠️ 动态资源（从 Git/Bitable 加载）
- ⚠️ Prompt 模板参数化渲染

**可观测性**
- ⚠️ Prometheus metrics
- ⚠️ 结构化日志
- ⚠️ 健康检查端点
- ⚠️ 追踪（Tracing）

**测试覆盖**
- ⚠️ 端到端协议测试（SetIO 管道）
- ⚠️ 错误场景测试
- ⚠️ 并发测试

---

## 二、总体架构设计

### 2.1 完整 MCP 生态系统

```
┌─────────────────────────────────────────────────────────────────┐
│                        MCP 客户端层                             │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐ │
│  │ Claude Desktop  │  │  自定义客户端    │  │ mcporter 集成   │ │
│  │  (stdio)        │  │  (HTTP/SSE)     │  │  (多模式)       │ │
│  └────────┬────────┘  └────────┬────────┘  └────────┬────────┘ │
└───────────┼────────────────────┼────────────────────┼──────────┘
            │                    │                    │
            └────────────────────┼────────────────────┘
                                 │
┌────────────────────────────────▼────────────────────────────────┐
│                    Transport 抽象层                            │
│  ┌──────────────────┐  ┌──────────────────┐  ┌───────────────┐ │
│  │ StdioTransport   │  │ HTTPTransport    │  │ WSTransport   │ │
│  │  (stdin/stdout)  │  │  (HTTP + SSE)   │  │  (WebSocket)  │ │
│  └────────┬─────────┘  └────────┬─────────┘  └───────┬───────┘ │
└───────────┼──────────────────────┼────────────────────┼─────────┘
            │                      │                    │
            └──────────────────────┼────────────────────┘
                                   │
┌──────────────────────────────────▼──────────────────────────────┐
│                   MCP Server (核心)                          │
│  ┌─────────────────────────────────────────────────────────┐  │
│  │ Protocol Layer (JSON-RPC 2.0)                           │  │
│  │ - Request Router                                        │  │
│  │ - Response Handler                                      │  │
│  │ - Error Handling                                        │  │
│  └───────────────────────────────┬─────────────────────────┘  │
│                                  │                           │
│  ┌───────────────────────────────▼─────────────────────────┐  │
│  │ Capability Layer                                         │  │
│  │ - Tools Registry         ┌─────────────────────────┐   │  │
│  │ - Resources Manager      │  LLM Integration        │   │  │
│  │ - Prompts Registry       │  (豆包大模型)          │   │  │
│  │ - Roots Handler          └─────────────────────────┘   │  │
│  └───────────────────────────────┬─────────────────────────┘  │
│                                  │                           │
│  ┌───────────────────────────────▼─────────────────────────┐  │
│  │ Integration Layer                                        │  │
│  │ ┌──────────────┐ ┌──────────────┐ ┌──────────────────┐  │  │
│  │ │Memory Graph  │ │ Git Storage  │ │ Bitable Storage  │  │  │
│  │ └──────────────┘ └──────────────┘ └──────────────────┘  │  │
│  └───────────────────────────────┬─────────────────────────┘  │
└──────────────────────────────────┼────────────────────────────┘
                                   │
┌──────────────────────────────────▼────────────────────────────┐
│                   飞书适配器 (Lark Adapter)                  │
│  ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────┐ ┌──────────┐ │
│  │ IM      │ │ Calendar│ │ Wiki    │ │ Task    │ │ VC       │ │
│  └─────────┘ └─────────┘ └─────────┘ └─────────┘ └──────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

### 2.2 核心接口抽象

```go
// Transport 传输层抽象
type Transport interface {
    Start() error
    Stop() error
    In() io.Reader
    Out() io.Writer
}

// StdioTransport 标准输入输出传输
type StdioTransport struct { /* 已实现 */ }

// HTTPTransport HTTP + SSE 传输
type HTTPTransport struct {
    Server *http.Server
    SSE    *SSEBroker
}

// WSTransport WebSocket 传输
type WSTransport struct {
    Upgrader websocket.Upgrader
    Conns    map[string]*websocket.Conn
}

// Tool 工具注册接口
type Tool interface {
    Name() string
    Description() string
    InputSchema() map[string]any
    Execute(ctx context.Context, args map[string]any) (Content, error)
}

// Resource 资源注册接口
type Resource interface {
    URI() string
    Name() string
    MimeType() string
    Read(ctx context.Context) (ResourceContent, error)
}

// Prompt 提示词注册接口
type Prompt interface {
    Name() string
    Description() string
    Arguments() []PromptArgument
    Render(ctx context.Context, args map[string]string) (string, error)
}
```

---

## 三、阶段一：传输模式扩展（高优先级）

### 3.1 目标

- 支持 stdio、HTTP/SSE、WebSocket 三种传输模式
- 统一的 Transport 接口抽象
- mcporter 集成支持

### 3.2 实现计划

#### 3.2.1 重构传输层抽象

**文件结构**
```
internal/mcp/
├── transport/
│   ├── transport.go       # Transport 接口定义
│   ├── stdio.go           # StdioTransport 实现
│   ├── http.go            # HTTPTransport 实现
│   └── websocket.go       # WSTransport 实现
├── server.go              # 重构 MCPServer 使用 Transport
└── ...
```

**核心接口** (`internal/mcp/transport/transport.go`)
```go
package transport

import (
    "context"
    "io"
)

// Transport 定义 MCP 传输层接口
type Transport interface {
    // Name 返回传输模式名称
    Name() string
    
    // Start 启动传输层
    Start(ctx context.Context) error
    
    // Stop 停止传输层
    Stop(ctx context.Context) error
    
    // In 返回输入流
    In() io.Reader
    
    // Out 返回输出流
    Out() io.Writer
    
    // Ready 返回传输层是否就绪
    Ready() bool
}

// TransportConfig 传输层配置
type TransportConfig struct {
    Mode string // "stdio", "http", "websocket"
    
    // HTTP 模式配置
    HTTP struct {
        Addr string
        Path string
    }
    
    // WebSocket 模式配置
    WebSocket struct {
        Addr string
        Path string
    }
}

// NewTransport 创建传输层实例
func NewTransport(cfg TransportConfig) (Transport, error) {
    switch cfg.Mode {
    case "stdio":
        return NewStdioTransport()
    case "http":
        return NewHTTPTransport(cfg.HTTP)
    case "websocket":
        return NewWebSocketTransport(cfg.WebSocket)
    default:
        return nil, fmt.Errorf("unsupported transport mode: %s", cfg.Mode)
    }
}
```

#### 3.2.2 HTTP + SSE 传输模式

**设计要点**
- HTTP POST 用于接收请求
- SSE (Server-Sent Events) 用于推送响应和通知
- 支持多客户端连接

**实现** (`internal/mcp/transport/http.go`)
```go
package transport

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "sync"
    "time"
)

// HTTPTransport HTTP + SSE 传输模式
type HTTPTransport struct {
    cfg      HTTPConfig
    server   *http.Server
    sseBroker *SSEBroker
    inPipe   *io.PipeReader
    outPipe  *io.PipeWriter
    ready    bool
    mu       sync.RWMutex
}

// HTTPConfig HTTP 传输配置
type HTTPConfig struct {
    Addr string // 监听地址，如 ":8080"
    Path string // 路径前缀，如 "/mcp"
}

// NewHTTPTransport 创建 HTTP 传输层
func NewHTTPTransport(cfg HTTPConfig) (*HTTPTransport, error) {
    if cfg.Addr == "" {
        cfg.Addr = ":8080"
    }
    if cfg.Path == "" {
        cfg.Path = "/mcp"
    }
    
    inReader, inWriter := io.Pipe()
    outReader, outWriter := io.Pipe()
    
    t := &HTTPTransport{
        cfg:      cfg,
        sseBroker: NewSSEBroker(),
        inPipe:   inReader,
        outPipe:  outWriter,
    }
    
    // 设置 HTTP 路由
    mux := http.NewServeMux()
    mux.HandleFunc(cfg.Path+"/request", t.handleRequest)
    mux.HandleFunc(cfg.Path+"/events", t.handleSSE)
    mux.HandleFunc(cfg.Path+"/health", t.handleHealth)
    
    t.server = &http.Server{
        Addr:    cfg.Addr,
        Handler: mux,
    }
    
    // 启动 goroutine 读取 outPipe 并通过 SSE 推送
    go t.pumpOutToSSE(outReader)
    
    return t, nil
}

// handleRequest 处理客户端 POST 请求
func (t *HTTPTransport) handleRequest(w http.ResponseWriter, r *http.Request) {
    if r.Method != http.MethodPost {
        http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    // 读取请求体并写入 inPipe
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    defer r.Body.Close()
    
    // 确保以换行结尾
    if len(body) == 0 || body[len(body)-1] != '\n' {
        body = append(body, '\n')
    }
    
    // 写入 inPipe
    if _, err := t.inPipe.Write(body); err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    w.WriteHeader(http.StatusAccepted)
}

// handleSSE 处理 SSE 连接
func (t *HTTPTransport) handleSSE(w http.ResponseWriter, r *http.Request) {
    t.sseBroker.ServeHTTP(w, r)
}

// handleHealth 健康检查
func (t *HTTPTransport) handleHealth(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]any{
        "status": "ok",
        "transport": "http",
        "ready": t.Ready(),
    })
}

// pumpOutToSSE 从 outPipe 读取并通过 SSE 推送
func (t *HTTPTransport) pumpOutToSSE(reader io.Reader) {
    scanner := bufio.NewScanner(reader)
    for scanner.Scan() {
        line := scanner.Text()
        t.sseBroker.Broadcast("message", line)
    }
}

// Start 启动 HTTP 服务器
func (t *HTTPTransport) Start(ctx context.Context) error {
    t.mu.Lock()
    t.ready = true
    t.mu.Unlock()
    
    go func() {
        fmt.Fprintf(os.Stderr, "[HTTP] server listening on %s\n", t.cfg.Addr)
        if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            fmt.Fprintf(os.Stderr, "[HTTP] server error: %v\n", err)
        }
    }()
    
    return nil
}

// Stop 停止 HTTP 服务器
func (t *HTTPTransport) Stop(ctx context.Context) error {
    t.mu.Lock()
    t.ready = false
    t.mu.Unlock()
    
    t.sseBroker.Close()
    return t.server.Shutdown(ctx)
}

// In 返回输入流
func (t *HTTPTransport) In() io.Reader {
    return t.inPipe
}

// Out 返回输出流
func (t *HTTPTransport) Out() io.Writer {
    return t.outPipe
}

// Ready 返回是否就绪
func (t *HTTPTransport) Ready() bool {
    t.mu.RLock()
    defer t.mu.RUnlock()
    return t.ready
}

// Name 返回传输模式名称
func (t *HTTPTransport) Name() string {
    return "http"
}

// SSEBroker SSE 消息代理
type SSEBroker struct {
    clients map[chan string]bool
    mu      sync.RWMutex
    closing bool
}

func NewSSEBroker() *SSEBroker {
    return &SSEBroker{
        clients: make(map[chan string]bool),
    }
}

func (b *SSEBroker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 设置 SSE headers
    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    // 创建客户端 channel
    clientChan := make(chan string, 10)
    
    // 注册客户端
    b.mu.Lock()
    if b.closing {
        b.mu.Unlock()
        return
    }
    b.clients[clientChan] = true
    b.mu.Unlock()
    
    // 取消时清理
    defer func() {
        b.mu.Lock()
        delete(b.clients, clientChan)
        b.mu.Unlock()
        close(clientChan)
    }()
    
    // 推送消息
    for msg := range clientChan {
        fmt.Fprintf(w, "data: %s\n\n", msg)
        if flusher, ok := w.(http.Flusher); ok {
            flusher.Flush()
        }
    }
}

func (b *SSEBroker) Broadcast(event, data string) {
    b.mu.RLock()
    defer b.mu.RUnlock()
    
    if b.closing {
        return
    }
    
    for clientChan := range b.clients {
        select {
        case clientChan <- data:
        default:
        }
    }
}

func (b *SSEBroker) Close() {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    b.closing = true
    for clientChan := range b.clients {
        close(clientChan)
    }
    b.clients = nil
}
```

#### 3.2.3 WebSocket 传输模式

**实现** (`internal/mcp/transport/websocket.go`)
```go
package transport

import (
    "context"
    "io"
    "net/http"
    "sync"
    
    "github.com/gorilla/websocket"
)

// WSTransport WebSocket 传输模式
type WSTransport struct {
    cfg      WebSocketConfig
    server   *http.Server
    upgrader websocket.Upgrader
    conns    map[string]*websocket.Conn
    inPipe   *io.PipeReader
    outPipe  *io.PipeWriter
    ready    bool
    mu       sync.RWMutex
}

// WebSocketConfig WebSocket 传输配置
type WebSocketConfig struct {
    Addr string
    Path string
}

// NewWebSocketTransport 创建 WebSocket 传输层
func NewWebSocketTransport(cfg WebSocketConfig) (*WSTransport, error) {
    if cfg.Addr == "" {
        cfg.Addr = ":8081"
    }
    if cfg.Path == "" {
        cfg.Path = "/mcp/ws"
    }
    
    inReader, inWriter := io.Pipe()
    outReader, outWriter := io.Pipe()
    
    t := &WSTransport{
        cfg:      cfg,
        upgrader: websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }},
        conns:    make(map[string]*websocket.Conn),
        inPipe:   inReader,
        outPipe:  outWriter,
    }
    
    mux := http.NewServeMux()
    mux.HandleFunc(cfg.Path, t.handleWebSocket)
    mux.HandleFunc(cfg.Path+"/health", t.handleHealth)
    
    t.server = &http.Server{
        Addr:    cfg.Addr,
        Handler: mux,
    }
    
    go t.pumpOutToWS(outReader)
    
    return t, nil
}

// handleWebSocket 处理 WebSocket 连接
func (t *WSTransport) handleWebSocket(w http.ResponseWriter, r *http.Request) {
    conn, err := t.upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }
    
    connID := generateConnID()
    
    t.mu.Lock()
    t.conns[connID] = conn
    t.mu.Unlock()
    
    defer func() {
        t.mu.Lock()
        delete(t.conns, connID)
        t.mu.Unlock()
        conn.Close()
    }()
    
    // 读取客户端消息
    for {
        _, msg, err := conn.ReadMessage()
        if err != nil {
            break
        }
        
        // 确保以换行结尾
        if len(msg) == 0 || msg[len(msg)-1] != '\n' {
            msg = append(msg, '\n')
        }
        
        // 写入 inPipe
        t.inPipe.Write(msg)
    }
}

// pumpOutToWS 从 outPipe 读取并通过 WebSocket 推送
func (t *WSTransport) pumpOutToWS(reader io.Reader) {
    scanner := bufio.NewScanner(reader)
    for scanner.Scan() {
        line := scanner.Text()
        
        t.mu.RLock()
        for _, conn := range t.conns {
            conn.WriteMessage(websocket.TextMessage, []byte(line))
        }
        t.mu.RUnlock()
    }
}

// ... Start/Stop/In/Out/Ready/Name 方法类似 HTTPTransport
```

#### 3.2.4 重构 MCPServer

**修改** (`internal/mcp/server.go`)
```go
// MCPServer MCP 服务器
type MCPServer struct {
    memoryGraph  MemoryGraphInterface
    gitStorage   GitStorageInterface
    bitableStore BitableStoreInterface
    llmAgent     *llm.MemoryAgent
    
    // 传输层
    transport transport.Transport
    
    // 状态
    mu          sync.Mutex
    ctx         context.Context
    cancel      context.CancelFunc
    wg          sync.WaitGroup
    sema        chan struct{}
    initialized bool
}

// NewMCPServer 创建 MCP Server
func NewMCPServer(
    mg MemoryGraphInterface,
    gs GitStorageInterface,
    bs BitableStoreInterface,
) *MCPServer {
    ctx, cancel := context.WithCancel(context.Background())
    
    // 默认使用 stdio 传输
    defaultTransport, _ := transport.NewStdioTransport()
    
    return &MCPServer{
        memoryGraph:  mg,
        gitStorage:   gs,
        bitableStore: bs,
        llmAgent:     llm.NewMemoryAgent(),
        transport:    defaultTransport,
        ctx:          ctx,
        cancel:       cancel,
        sema:         make(chan struct{}, maxConcurrency),
    }
}

// SetTransport 设置传输层
func (s *MCPServer) SetTransport(t transport.Transport) {
    s.transport = t
}

// Start 启动 MCP Server
func (s *MCPServer) Start() error {
    fmt.Fprintf(os.Stderr, "[MCP] starting server with transport: %s\n", s.transport.Name())
    
    // 启动传输层
    if err := s.transport.Start(s.ctx); err != nil {
        return err
    }
    
    // 使用 transport 的 In/Out
    scanner := bufio.NewScanner(s.transport.In())
    scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
    
    for scanner.Scan() {
        if s.ctx.Err() != nil {
            break
        }
        
        line := scanner.Text()
        if line == "" {
            continue
        }
        
        var req Request
        if err := json.Unmarshal([]byte(line), &req); err != nil {
            s.sendError(nil, ErrCodeParseError, "parse error: "+err.Error())
            continue
        }
        
        select {
        case s.sema <- struct{}{}:
        case <-s.ctx.Done():
            break
        }
        s.wg.Add(1)
        go func(r Request) {
            defer func() {
                <-s.sema
                s.wg.Done()
            }()
            s.handleRequest(r)
        }(req)
    }
    
    s.wg.Wait()
    return scanner.Err()
}

// Stop 停止 MCP Server
func (s *MCPServer) Stop() error {
    s.cancel()
    if s.transport != nil {
        return s.transport.Stop(context.Background())
    }
    return nil
}

// sendResponse 发送响应（使用 transport.Out）
func (s *MCPServer) sendResponse(id any, result any) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    resp := Response{
        JSONRPC: "2.0",
        ID:      id,
        Result:  result,
    }
    data, err := json.Marshal(resp)
    if err != nil {
        fmt.Fprintf(os.Stderr, "[MCP] sendResponse marshal error: %v\n", err)
        fmt.Fprintf(s.transport.Out(), `{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"internal error"}}`+"\n")
        return
    }
    fmt.Fprintf(s.transport.Out(), "%s\n", data)
    fmt.Fprintf(os.Stderr, "[MCP] <-- response id=%v size=%d\n", id, len(data))
}

// sendError 发送错误（使用 transport.Out）
func (s *MCPServer) sendError(id any, code int, errMsg string) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    resp := Response{
        JSONRPC: "2.0",
        ID:      id,
        Error:   &Error{Code: code, Message: errMsg},
    }
    data, err := json.Marshal(resp)
    if err != nil {
        fmt.Fprintf(os.Stderr, "[MCP] sendError marshal error: %v\n", err)
        fmt.Fprintf(s.transport.Out(), `{"jsonrpc":"2.0","id":null,"error":{"code":-32603,"message":"internal error"}}`+"\n")
        return
    }
    fmt.Fprintf(s.transport.Out(), "%s\n", data)
    fmt.Fprintf(os.Stderr, "[MCP] <-- error id=%v code=%d message=%q\n", id, code, errMsg)
}
```

### 3.3 Docker 集成与 mcporter 配置

#### 3.3.1 更新 cmd/mcp-server/main.go

```go
package main

import (
    "flag"
    "log"
    "os"
    "os/signal"
    "syscall"

    "feishu-mem/internal/config"
    "feishu-mem/internal/core"
    "feishu-mem/internal/mcp"
    "feishu-mem/internal/mcp/transport"
    "feishu-mem/internal/storage/bitable"
    "feishu-mem/internal/storage/git"
)

func main() {
    // 命令行参数
    transportMode := flag.String("transport", "stdio", "Transport mode: stdio, http, websocket")
    httpAddr := flag.String("http-addr", ":8080", "HTTP listen address")
    wsAddr := flag.String("ws-addr", ":8081", "WebSocket listen address")
    flag.Parse()
    
    // 加载配置
    larkadapter.LoadEnv()
    settings := config.DefaultSettings()
    if cfgPath := os.Getenv("CONFIG_PATH"); cfgPath != "" {
        if s, err := config.LoadSettings(cfgPath); err == nil {
            settings = s
        }
    } else if s, err := config.LoadSettings("config/openclaw.yaml"); err == nil {
        settings = s
    }
    
    // 初始化存储
    var gitStorage *git.GitStorage
    if settings.Git.WorkDir != "" {
        var err error
        gitStorage, err = git.NewGitStorage(git.Config{
            WorkDir:  settings.Git.WorkDir,
            Remote:   settings.Git.Remote,
            AutoPush: false,
            Branch:   settings.Git.Branch,
        })
        if err != nil {
            log.Printf("Warning: Git storage init failed: %v", err)
        }
    }
    
    memoryGraph := core.NewMemoryGraph()
    if gitStorage != nil && settings.Memory.PreloadOnStart {
        if err := memoryGraph.LoadFromGit(gitStorage, settings.Project.Name); err != nil {
            log.Printf("Warning: failed to load from Git: %v", err)
        }
    }
    
    var bitableStore mcp.BitableStoreInterface
    if settings.Bitable.BaseToken != "" {
        bitableStore = bitable.NewBitableStore(bitable.Config{
            BaseToken: settings.Bitable.BaseToken,
            Tables: bitable.TablesConfig{
                Decision: settings.Bitable.Tables.Decision,
                Topic:    settings.Bitable.Tables.Topic,
                Phase:    settings.Bitable.Tables.Phase,
                Relation: settings.Bitable.Tables.Relation,
            },
        }, nil)
    }
    
    // 创建 MCP 服务器
    server := mcp.NewMCPServer(memoryGraph, gitStorage, bitableStore)
    
    // 配置传输层
    transportCfg := transport.TransportConfig{
        Mode: *transportMode,
        HTTP: transport.HTTPConfig{
            Addr: *httpAddr,
            Path: "/mcp",
        },
        WebSocket: transport.WebSocketConfig{
            Addr: *wsAddr,
            Path: "/mcp/ws",
        },
    }
    
    t, err := transport.NewTransport(transportCfg)
    if err != nil {
        log.Fatalf("Failed to create transport: %v", err)
    }
    server.SetTransport(t)
    
    log.Printf("Feishu Memory MCP Server starting with transport: %s", *transportMode)
    log.Printf("  Project: %s", settings.Project.Name)
    log.Printf("  Decisions loaded: %d", memoryGraph.Count())
    
    // 启动服务器
    go func() {
        if err := server.Start(); err != nil {
            log.Fatalf("MCP server error: %v", err)
        }
    }()
    
    // 等待退出信号
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    sig := <-sigChan
    log.Printf("Received signal: %v, shutting down...", sig)
    server.Stop()
    log.Println("MCP server stopped")
}
```

#### 3.3.2 mcporter 配置文件

创建 `config/mcporter.yaml`:
```yaml
# mcporter 配置 - 在 Docker 容器内部署

mcpServers:
  - name: feishu-mem
    command:
      path: /root/openclaw-workspace/feishu-agent-mem/bin/mcp-server
      args:
        - --transport=http
        - --http-addr=:8080
    env:
      CONFIG_PATH: /root/openclaw-workspace/feishu-agent-mem/config/openclaw.yaml
      ARK_API_KEY: ${ARK_API_KEY}
    transport:
      type: http
      url: http://localhost:8080/mcp
      
  - name: feishu-mem-stdio
    command:
      path: /root/openclaw-workspace/feishu-agent-mem/bin/mcp-server
      args:
        - --transport=stdio
    env:
      CONFIG_PATH: /root/openclaw-workspace/feishu-agent-mem/config/openclaw.yaml
      ARK_API_KEY: ${ARK_API_KEY}
    transport:
      type: stdio
```

#### 3.3.3 Docker 部署脚本

创建 `scripts/deploy-mcp-docker.sh`:
```bash
#!/bin/bash
# MCP 服务器 Docker 部署脚本

DOCKER_CONTAINER="openclaw-zh"
PROJECT_DIR="/root/openclaw-workspace/feishu-agent-mem"

echo "=== 部署 MCP 服务器到 Docker 容器 ==="

# 1. 编译二进制
echo "[1/5] 编译二进制..."
docker exec $DOCKER_CONTAINER bash -c "cd $PROJECT_DIR && export PATH=/usr/local/go/bin:\$PATH && go build -o bin/mcp-server ./cmd/mcp-server/main.go"

# 2. 停止旧进程
echo "[2/5] 停止旧进程..."
docker exec $DOCKER_CONTAINER bash -c "pkill -f mcp-server 2>/dev/null || true"

# 3. 部署配置
echo "[3/5] 部署配置..."
docker cp config/mcporter.yaml $DOCKER_CONTAINER:$PROJECT_DIR/config/

# 4. 启动服务器（HTTP 模式，后台运行）
echo "[4/5] 启动 MCP 服务器（HTTP 模式）..."
docker exec -d $DOCKER_CONTAINER bash -c "cd $PROJECT_DIR && export PATH=/usr/local/go/bin:\$PATH && export ARK_API_KEY=$ARK_API_KEY && ./bin/mcp-server --transport=http --http-addr=:8080 > logs/mcp-server.log 2>&1 &"

# 5. 等待并验证
echo "[5/5] 等待服务启动..."
sleep 2

# 健康检查
docker exec $DOCKER_CONTAINER bash -c "curl -s http://localhost:8080/mcp/health"

echo ""
echo "=== 部署完成 ==="
echo "MCP 服务器运行在 http://localhost:8080/mcp"
echo "日志: docker exec -it $DOCKER_CONTAINER tail -f $PROJECT_DIR/logs/mcp-server.log"
```

---

## 四、阶段二：MCP 客户端 SDK（中优先级）

### 4.1 目标

创建 Go 语言 MCP 客户端 SDK，支持：
- stdio、HTTP/SSE、WebSocket 三种传输模式
- 完整的协议握手
- 工具调用、资源读取、Prompt 获取
- 错误处理与重试

### 4.2 客户端 SDK 实现

**文件结构**
```
pkg/mcpclient/
├── client.go          # 客户端核心
├── transport/
│   ├── transport.go   # 传输层接口
│   ├── stdio.go       # stdio 实现
│   ├── http.go        # HTTP/SSE 实现
│   └── websocket.go   # WebSocket 实现
├── tools.go           # 工具调用 API
├── resources.go       # 资源读取 API
├── prompts.go         # Prompt 获取 API
└── errors.go          # 错误定义
```

**核心客户端** (`pkg/mcpclient/client.go`)
```go
package mcpclient

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "sync/atomic"
)

// Client MCP 客户端
type Client struct {
    transport Transport
    
    mu          sync.Mutex
    initialized bool
    capabilities *Capabilities
    serverInfo   *ServerInfo
    
    pendingRequests map[int64]chan *ResponseOrError
    requestID       int64
    
    ctx    context.Context
    cancel context.CancelFunc
}

// NewClient 创建 MCP 客户端
func NewClient(t Transport) *Client {
    ctx, cancel := context.WithCancel(context.Background())
    return &Client{
        transport: t,
        pendingRequests: make(map[int64]chan *ResponseOrError),
        ctx: ctx,
        cancel: cancel,
    }
}

// Initialize 初始化握手
func (c *Client) Initialize(ctx context.Context, params InitializeParams) (*InitializeResult, error) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    if c.initialized {
        return &InitializeResult{
            ProtocolVersion: c.capabilities.ProtocolVersion,
            Capabilities:    c.capabilities,
            ServerInfo:      c.serverInfo,
        }, nil
    }
    
    // 启动响应接收循环
    go c.receiveLoop()
    
    // 发送 initialize 请求
    req := Request{
        JSONRPC: "2.0",
        ID:      c.nextRequestID(),
        Method:  "initialize",
        Params:  params,
    }
    
    resp, err := c.sendRequestAndWait(ctx, req)
    if err != nil {
        return nil, err
    }
    
    if resp.Error != nil {
        return nil, resp.Error
    }
    
    var result InitializeResult
    if err := mapToStruct(resp.Result, &result); err != nil {
        return nil, err
    }
    
    // 发送 initialized 通知
    c.sendNotification("notifications/initialized", nil)
    
    c.initialized = true
    c.capabilities = &result.Capabilities
    c.serverInfo = &result.ServerInfo
    
    return &result, nil
}

// ListTools 列出工具
func (c *Client) ListTools(ctx context.Context) (*ListToolsResult, error) {
    req := Request{
        JSONRPC: "2.0",
        ID:      c.nextRequestID(),
        Method:  "tools/list",
        Params:  map[string]any{},
    }
    
    resp, err := c.sendRequestAndWait(ctx, req)
    if err != nil {
        return nil, err
    }
    
    if resp.Error != nil {
        return nil, resp.Error
    }
    
    var result ListToolsResult
    if err := mapToStruct(resp.Result, &result); err != nil {
        return nil, err
    }
    
    return &result, nil
}

// CallTool 调用工具
func (c *Client) CallTool(ctx context.Context, params CallToolParams) (*CallToolResult, error) {
    req := Request{
        JSONRPC: "2.0",
        ID:      c.nextRequestID(),
        Method:  "tools/call",
        Params:  params,
    }
    
    resp, err := c.sendRequestAndWait(ctx, req)
    if err != nil {
        return nil, err
    }
    
    if resp.Error != nil {
        return nil, resp.Error
    }
    
    var result CallToolResult
    if err := mapToStruct(resp.Result, &result); err != nil {
        return nil, err
    }
    
    return &result, nil
}

// ... ListResources, ReadResource, ListPrompts, GetPrompt

// receiveLoop 接收响应循环
func (c *Client) receiveLoop() {
    scanner := bufio.NewScanner(c.transport.In())
    scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
    
    for scanner.Scan() {
        select {
        case <-c.ctx.Done():
            return
        default:
        }
        
        line := scanner.Text()
        if line == "" {
            continue
        }
        
        var resp Response
        if err := json.Unmarshal([]byte(line), &resp); err != nil {
            continue
        }
        
        // 找到等待的请求
        id, ok := resp.ID.(float64)
        if !ok {
            continue
        }
        
        c.mu.Lock()
        ch, ok := c.pendingRequests[int64(id)]
        if ok {
            delete(c.pendingRequests, int64(id))
        }
        c.mu.Unlock()
        
        if ok {
            ch <- &ResponseOrError{Response: &resp}
            close(ch)
        }
    }
}

// sendRequestAndWait 发送请求并等待响应
func (c *Client) sendRequestAndWait(ctx context.Context, req Request) (*Response, error) {
    ch := make(chan *ResponseOrError, 1)
    
    id, _ := req.ID.(int64)
    c.mu.Lock()
    c.pendingRequests[id] = ch
    c.mu.Unlock()
    
    // 发送请求
    data, err := json.Marshal(req)
    if err != nil {
        return nil, err
    }
    
    if _, err := fmt.Fprintf(c.transport.Out(), "%s\n", data); err != nil {
        return nil, err
    }
    
    // 等待响应或超时
    select {
    case result := <-ch:
        if result.Error != nil {
            return nil, result.Error
        }
        return result.Response, nil
    case <-ctx.Done():
        c.mu.Lock()
        delete(c.pendingRequests, id)
        c.mu.Unlock()
        return nil, ctx.Err()
    }
}

// Close 关闭客户端
func (c *Client) Close() error {
    c.cancel()
    return c.transport.Close()
}

func (c *Client) nextRequestID() int64 {
    return atomic.AddInt64(&c.requestID, 1)
}

// Helper: map to struct
func mapToStruct(m any, out any) error {
    data, err := json.Marshal(m)
    if err != nil {
        return err
    }
    return json.Unmarshal(data, out)
}
```

### 4.3 客户端使用示例

```go
package main

import (
    "context"
    "fmt"
    "time"
    
    "feishu-mem/pkg/mcpclient"
    "feishu-mem/pkg/mcpclient/transport"
)

func main() {
    // 1. 创建 HTTP 传输
    t, _ := transport.NewHTTPTransport(transport.HTTPConfig{
        URL: "http://localhost:8080/mcp",
    })
    
    // 2. 创建客户端
    client := mcpclient.NewClient(t)
    defer client.Close()
    
    // 3. 初始化
    ctx := context.Background()
    initResult, _ := client.Initialize(ctx, mcpclient.InitializeParams{
        ProtocolVersion: "2024-11-05",
        Capabilities:    mcpclient.ClientCapabilities{},
        ClientInfo: mcpclient.ClientInfo{
            Name:    "feishu-mem-client",
            Version: "1.0.0",
        },
    })
    
    fmt.Printf("Connected to %s %s\n", initResult.ServerInfo.Name, initResult.ServerInfo.Version)
    
    // 4. 列出工具
    tools, _ := client.ListTools(ctx)
    fmt.Printf("Available tools: %d\n", len(tools.Tools))
    
    // 5. 调用搜索工具
    searchResult, _ := client.CallTool(ctx, mcpclient.CallToolParams{
        Name: "search",
        Arguments: map[string]any{
            "query": "数据库",
            "limit": 10,
        },
    })
    
    for _, content := range searchResult.Content {
        fmt.Println(content.Text)
    }
}
```

---

## 五、阶段三：协议完善与功能扩展（中优先级）

### 5.1 完善 MCP 协议方法

#### 5.1.1 实现 roots/list

```go
func (s *MCPServer) handleListRoots(req Request) {
    // 列出可用的根目录（Git 仓库、Bitable 等）
    roots := []map[string]any{
        {
            "uri":  "git://" + s.gitStorage.WorkDir,
            "name": "Git Storage",
        },
        {
            "uri":  "bitable://" + s.bitableStore.BaseToken,
            "name": "Bitable Storage",
        },
    }
    
    s.sendResponse(req.ID, map[string]any{"roots": roots})
}
```

#### 5.1.2 实现 logging/message 通知

```go
func (s *MCPServer) sendLog(level string, message string) {
    // 发送日志通知（仅支持，不期待响应）
    notif := Request{
        JSONRPC: "2.0",
        Method:  "notifications/logging/message",
        Params: map[string]any{
            "level": level,
            "data": map[string]any{
                "text": message,
            },
        },
    }
    
    data, _ := json.Marshal(notif)
    fmt.Fprintf(s.transport.Out(), "%s\n", data)
}
```

### 5.2 新增写操作工具

| 工具名 | 描述 |
|-------|------|
| `create_decision` | 创建新决策 |
| `update_decision` | 更新已有决策 |
| `delete_decision` | 删除决策 |
| `upsert_topic` | 创建或更新议题 |
| `link_decisions` | 建立决策之间的关系 |

---

## 六、阶段四：可观测性与测试（低优先级）

### 6.1 Prometheus Metrics

**新增** `internal/mcp/metrics.go`:
```go
package mcp

import "github.com/prometheus/client_golang/prometheus"

var (
    requestsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_requests_total",
            Help: "Total number of MCP requests",
        },
        []string{"method"},
    )
    
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "mcp_request_duration_seconds",
            Help:    "Duration of MCP requests",
            Buckets: prometheus.DefBuckets,
        },
        []string{"method"},
    )
    
    errorsTotal = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_errors_total",
            Help: "Total number of MCP errors",
        },
        []string{"method", "code"},
    )
)

func init() {
    prometheus.MustRegister(requestsTotal)
    prometheus.MustRegister(requestDuration)
    prometheus.MustRegister(errorsTotal)
}
```

### 6.2 端到端测试

**新增** `test/mcp/e2e_test.go`:
```go
package mcp_test

import (
    "context"
    "io"
    "testing"
    "time"
    
    "feishu-mem/internal/core"
    "feishu-mem/internal/mcp"
    "feishu-mem/pkg/mcpclient"
    "feishu-mem/pkg/mcpclient/transport"
)

// TestE2E_Stdio stdio 传输端到端测试
func TestE2E_Stdio(t *testing.T) {
    // 创建内存 pipes
    serverIn, clientOut := io.Pipe()
    clientIn, serverOut := io.Pipe()
    
    // 启动服务器
    memoryGraph := core.NewMemoryGraph()
    server := mcp.NewMCPServer(memoryGraph, nil, nil)
    server.SetIO(serverIn, serverOut)
    
    go func() {
        server.Start()
    }()
    defer server.Stop()
    
    // 创建客户端
    serverTransport := transport.NewPipeTransport(clientIn, clientOut)
    client := mcpclient.NewClient(serverTransport)
    defer client.Close()
    
    // 初始化
    ctx := context.Background()
    initResult, err := client.Initialize(ctx, mcpclient.InitializeParams{
        ProtocolVersion: "2024-11-05",
        ClientInfo: mcpclient.ClientInfo{
            Name:    "test-client",
            Version: "1.0.0",
        },
    })
    
    assert.NoError(t, err)
    assert.Equal(t, "Feishu Memory Agent", initResult.ServerInfo.Name)
    
    // 列出工具
    tools, err := client.ListTools(ctx)
    assert.NoError(t, err)
    assert.Equal(t, 8, len(tools.Tools))
    
    // 调用搜索
    result, err := client.CallTool(ctx, mcpclient.CallToolParams{
        Name: "search",
        Arguments: map[string]any{
            "query": "test",
            "limit": 10,
        },
    })
    
    assert.NoError(t, err)
    assert.NotEmpty(t, result.Content)
}
```

---

## 七、实施路线图

### 阶段一：传输模式扩展（1-2 周）
- [ ] 传输层抽象重构
- [ ] HTTP + SSE 传输实现
- [ ] WebSocket 传输实现
- [ ] MCPServer 集成传输层
- [ ] Docker 部署脚本
- [ ] mcporter 配置

### 阶段二：客户端 SDK（1 周）
- [ ] 客户端核心实现
- [ ] 多传输模式支持
- [ ] 工具/资源/Prompt API
- [ ] 客户端使用示例

### 阶段三：协议完善与功能扩展（1-2 周）
- [ ] roots/list 实现
- [ ] logging/message 通知
- [ ] window 通知方法
- [ ] 写操作工具
- [ ] 动态资源支持

### 阶段四：可观测性与测试（1 周）
- [ ] Prometheus metrics
- [ ] 结构化日志
- [ ] 健康检查
- [ ] 端到端测试
- [ ] 错误场景测试

---

## 八、验证清单

- [ ] mcporter 可以成功发现并连接 MCP 服务器
- [ ] Claude Desktop 通过 stdio 正常工作
- [ ] 自定义客户端通过 HTTP/SSE 正常工作
- [ ] 所有 8 个工具正常调用
- [ ] LLM 集成正常工作
- [ ] 错误处理与重试机制正常
- [ ] 并发场景下稳定
