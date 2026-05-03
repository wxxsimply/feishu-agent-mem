# MCP 服务器改进计划

> 基于 `docs/mcp-architecture.md` 的架构设计，对比 `internal/mcp/server.go` 当前实现，识别缺口并制定分阶段改进方案。

---

## 上下文

当前 MCP 服务器 (`internal/mcp/server.go`) 是一个原型级实现：手写 JSON-RPC 2.0 over stdio，暴露 8 个工具和 2 个资源，满足基本交互需求。但与生产级 MCP 服务器相比，在协议完备性、错误处理、并发安全、测试覆盖和架构整合等方面存在显著缺口。本计划将改进按优先级分 4 个阶段推进。

---

## 阶段一：关键修复（高优先级，高影响）

### 1.1 修复 `notifications/initialized` 未处理

**问题**: MCP 协议要求客户端在 initialize 之后发送 `notifications/initialized`。当前 switch 没有此分支，会返回 `"unknown method"` 错误。

**改动**: `internal/mcp/server.go:handleRequest` switch 中添加：
```go
case "notifications/initialized":
    // no-op, 初始化完成
```

### 1.2 修复 `bufio.Scanner` 64KB 行限制

**问题**: `extract_decision` 等工具可能接收大段文本，`bufio.Scanner` 默认 64KB 会触发 `ErrTooLong`。

**改动**: `internal/mcp/server.go:Start()` 中初始化 scanner 时设置更大 buffer：
```go
scanner := bufio.NewScanner(s.in)
scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB
```

### 1.3 修复 goruoutine 泄漏与并发控制

**问题**: `go s.handleRequest(req)` 无限制，大量请求会耗尽 goroutine；没有 context 传播。

**改动**:
- 添加 worker pool 或 semaphore 控制并发上限（如 20）
- 给 `MCPServer` 添加 `ctx context.Context` 和 `cancel` 字段
- `handleCallTool` 中将 context 传播给 LLM 调用链

### 1.4 实现优雅关闭

**问题**: `Stop()` 是空操作，`Start()` 没有信号处理。

**改动**:
- `Start()` 中监听 context.Done() 退出循环
- `Stop()` 调用 cancel，等待 in-flight 请求完成
- `cmd/mcp-server/main.go` 添加 `os/signal` 处理

### 1.5 修复 `req.ID` 类型

**问题**: `Request.ID` 定义为 `int`，但 JSON-RPC 2.0 允许 `string | number | null`。

**改动**: 改为 `any` 或 `json.Number`，确保字符串 ID 的客户端能正常工作。

### 1.6 正确的 MCP 错误代码

**问题**: 所有错误码硬编码为 `-32000`。

**改动**:
| 场景 | 正确代码 |
|------|---------|
| JSON 解析失败 | `-32700` Parse Error |
| 未知方法 | `-32601` Method Not Found |
| 无效参数 | `-32602` Invalid Params |
| 内部错误 | `-32603` Internal Error |
| 工具不存在 | `-32003` Tool Not Found |
| 资源不存在 | `-32002` Resource Not Found |

`Error` 结构体添加可选的 `Data` 字段。

### 1.7 修复 JSON marshal 错误被忽略

**问题**: `sendResponse` 和 `sendError` 中 `json.Marshal` 错误被 `_` 忽略。

**改动**: 检查 marshal 错误，fallback 写原始错误文本到 stdout。

---

## 阶段二：协议完备性（中优先级）

### 2.1 请求参数验证

**问题**: `handleCallTool` 未验证 `name` 和 `arguments` 是否存在，必需参数缺失时不返回 Invalid Params 错误。

**改动**:
- 验证 `params["name"]` 是否为非空 string
- 验证 `params["arguments"]` 是否为 object
- 对声明了 `required` 的工具，校验必需参数是否存在
- 缺失时返回 `-32602` Invalid Params

### 2.2 预初始化拒绝

**问题**: 在 `initialize` 方法调用之前，MCP 规范要求拒绝除 `initialize` 外的所有请求。

**改动**: 添加 `initialized bool` 状态，`handleRequest` 中检查，未初始化时返回错误。

### 2.3 实现 `resources/templates/list`

**问题**: 资源模板未实现，限制 URI 模式的灵活性。

**改动**: 返回静态资源模板列表（如有需要，如 `docs://{id}` 模式）。

### 2.4 `prompts/list` 和 `prompts/get` 实现

**问题**: 当前返回空对象，prompts 能力未暴露。

**改动**: 注册可用 prompt 模板（如 `extract_decision`、`classify_topic` 的提示词模板），支持参数化渲染。

### 2.5 进度通知支持

**问题**: 长时间运行的 LLM 工具不发送进度通知，AI 客户端无法感知进度。

**改动**:
- 可选：为 `extract_decision` 等 LLM 工具发送 `notifications/progress`
- LLM 调用前后发送开始/完成进度

### 2.6 添加分页支持

**问题**: `tools/list` 和 `resources/list` 忽略 `cursor` 参数。

**改动**: 接受 cursor 参数，当前列表小可返回空 cursor，但不应忽略输入。

---

## 阶段三：架构整合（中优先级）

### 3.1 消除两个工具注册表

**问题**: `internal/mcp/server.go` 和 `internal/llm/tools/mcp_tools.go` 各维护一套工具定义，互相独立。

**改动**:
- 统一工具定义到 `internal/mcp/` 或共享包
- `MCPServer.handleListTools` 从统一的注册表读取
- 消除 `mcp_tools.go` 中的空 handler（`handleSearch` 返回 `nil, nil`）

### 3.2 集成 `internal/search` 包

**问题**: `handleSearch` 直接调用 `memoryGraph.SearchByKeywords`，未使用 `internal/search` 包的缓存、排名、token 预算功能。

**改动**: `handleSearch` 改走 `SearchTool` 接口，利用其缓存和排名能力。

### 3.3 修复 `cmd/mcp-server/main.go` 的 bitable nil

**问题**: `bitableStore` 声明为 nil，Bitable 功能在该入口失效。

**改动**: 根据配置初始化 bitableStore，或至少 log 警告说明未配置。

### 3.4 隔离 stdio 和 HTTP 模式

**问题**: `settings.MCP.Port` 被记录但从不使用，MCP 只有 stdio 模式。

**改动**: 明确分离 stdio 模式（AI 客户端子进程）和 HTTP/TCP 模式（远程调用）。HTTP 模式可作为后续增强。

---

## 阶段四：测试与可观测性（持续）

### 4.1 端到端 MCP 协议测试

**问题**: 当前测试只构建请求结构体，不实际通过 `SetIO` 管道通信。

**改动**: 用 `io.Pipe` 构造 stdin/stdout，写完整 JSON-RPC 请求，解析响应，验证：
- initialize 握手
- tools/list 返回 8 个工具
- tools/call 各工具返回预期格式
- 错误场景（未知方法、缺失参数、格式错误 JSON）
- 并发请求

### 4.2 为 `sendResponse` / `sendError` 添加基础日志

**问题**: 没有请求/响应日志，调试困难。

**改动**: 在 `sendResponse` 和 `sendError` 中记录请求 ID、方法、耗时到 stderr（结构化 JSON 格式）。

### 4.3 添加错误场景测试

- 未知 method → `-32601`
- 缺少参数 → `-32602`
- 非法 JSON → `-32700`
- 未知 tool → `-32003`
- 未初始化时发请求

---

## 关键文件清单

| 文件 | 涉及改进 |
|------|---------|
| `internal/mcp/server.go` | 1.1-1.7, 2.1-2.6, 3.1-3.2, 4.2 |
| `cmd/mcp-server/main.go` | 1.4, 3.3 |
| `cmd/mem-service/main.go` | 3.4 |
| `internal/llm/tools/mcp_tools.go` | 3.1 |
| `internal/search/search_tool.go` | 3.2 |
| `test/mcp/mcp_protocol_test.go` | 4.1, 4.3 |

---

## 验证方式

1. **阶段一后**: 运行 `go test ./internal/mcp/` 确认基础协议可用
2. **阶段二后**: 用 `internal/mcp/server_test.go`（新增）做 SetIO 管道测试，验证协议交互
3. **阶段三后**: `go build ./cmd/mcp-server/` 通过，bitable 不再为 nil
4. **阶段四后**: `go test ./test/mcp/` 覆盖端到端流程和错误场景
5. **最终验证**: `cmd/mcp-server/main.go` 启动后与 Claude Desktop 或任意 MCP 客户端握手成功
