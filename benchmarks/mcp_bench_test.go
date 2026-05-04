package benchmarks

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// MCPTool 表示一个 MCP 工具
type MCPTool struct {
	Name      string
	Args      map[string]any
	NeedsLLM  bool
	SetupData bool // 是否需要先在系统中创建数据
}

var mcpTools = []MCPTool{
	{Name: "search", Args: map[string]any{"query": "PostgreSQL", "limit": float64(5)}},
	{Name: "search", Args: map[string]any{"query": "数据库", "topic": "数据库架构"}},
	{Name: "search", Args: map[string]any{"query": ""}},
	{Name: "topic", Args: map[string]any{"topic": "数据库架构"}},
	{Name: "decision", Args: map[string]any{"sdr_id": ""}},
	{Name: "extract_decision", Args: map[string]any{"content": "我们决定使用PostgreSQL作为主数据库"}, NeedsLLM: true},
	{Name: "classify_topic", Args: map[string]any{"decision": "使用PostgreSQL", "topics": []any{"数据库架构", "缓存方案", "前端架构"}}, NeedsLLM: true},
	{Name: "detect_crosstopic", Args: map[string]any{"title": "数据库选型", "decision": "使用PostgreSQL", "candidate_topics": []any{"数据库架构", "缓存方案"}}, NeedsLLM: true},
	{Name: "check_conflict", Args: map[string]any{"decision_a": "使用PostgreSQL", "decision_b": "使用MySQL"}, NeedsLLM: true},
	{Name: "timeline", Args: map[string]any{}},
}

type mcpResponse struct {
	ID      int              `json:"id"`
	Result  *json.RawMessage `json:"result,omitempty"`
	Error   *json.RawMessage `json:"error,omitempty"`
	Latency time.Duration
}

func buildRequest(id int, method string, params map[string]any) string {
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if method == "tools/call" {
		req["params"] = params
	}
	data, _ := json.Marshal(req)
	return string(data)
}

func runMCPBench(b *testing.B, toolName string, args map[string]any) {
	// 启动 MCP 服务器（stdio 模式）
	cmd := exec.Command("../bin/mcp-server")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr

	if err := cmd.Start(); err != nil {
		b.Fatalf("start mcp-server: %v", err)
	}
	defer cmd.Process.Kill()

	// 等待启动
	time.Sleep(500 * time.Millisecond)

	reader := bufio.NewReader(stdout)

	// 初始化
	initReq := buildRequest(1, "initialize", nil)
	fmt.Fprintf(stdin, "%s\n", initReq)
	readResponse(reader)

	// 预热
	warmupReq := buildRequest(2, "tools/list", nil)
	fmt.Fprintf(stdin, "%s\n", warmupReq)
	readResponse(reader)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		params := map[string]any{
			"name":      toolName,
			"arguments": args,
		}
		req := buildRequest(3, "tools/call", params)
		start := time.Now()
		fmt.Fprintf(stdin, "%s\n", req)
		_, err := reader.ReadString('\n')
		if err != nil {
			b.Fatalf("read response: %v", err)
		}
		elapsed := time.Since(start)
		_ = elapsed
	}
}

func readResponse(reader *bufio.Reader) (string, error) {
	return reader.ReadString('\n')
}

func BenchmarkMCP_Initialize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		cmd := exec.Command("../bin/mcp-server")
		stdin, _ := cmd.StdinPipe()
		stdout, _ := cmd.StdoutPipe()
		cmd.Stderr = io.Discard

		if err := cmd.Start(); err != nil {
			b.Fatalf("start: %v", err)
		}

		time.Sleep(200 * time.Millisecond)
		reader := bufio.NewReader(stdout)

		initReq := buildRequest(1, "initialize", nil)
		fmt.Fprintf(stdin, "%s\n", initReq)
		readResponse(reader)

		cmd.Process.Kill()
		cmd.Wait()
	}
}

func BenchmarkMCP_Search(b *testing.B) {
	runMCPBench(b, "search", map[string]any{"query": "PostgreSQL"})
}

func BenchmarkMCP_Topic(b *testing.B) {
	runMCPBench(b, "topic", map[string]any{"topic": "数据库架构"})
}

func BenchmarkMCP_Decision(b *testing.B) {
	runMCPBench(b, "decision", map[string]any{"sdr_id": ""})
}

func BenchmarkMCP_Timeline(b *testing.B) {
	runMCPBench(b, "timeline", map[string]any{})
}

// BenchmarkMCP_ConcurrentSearch 并发搜索基准
func BenchmarkMCP_ConcurrentSearch(b *testing.B) {
	cmd := exec.Command("../bin/mcp-server")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		b.Fatalf("start: %v", err)
	}
	defer cmd.Process.Kill()

	time.Sleep(500 * time.Millisecond)
	reader := bufio.NewReader(stdout)

	initReq := buildRequest(1, "initialize", nil)
	fmt.Fprintf(stdin, "%s\n", initReq)
	readResponse(reader)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		id := 0
		for pb.Next() {
			id++
			params := map[string]any{
				"name": "search",
				"arguments": map[string]any{
					"query": "PostgreSQL",
				},
			}
			req := buildRequest(100+id, "tools/call", params)
			fmt.Fprintf(stdin, "%s\n", req)
			readResponse(reader)
		}
	})
}

// BenchmarkMCP_ToolMix 混合工具调用基准
func BenchmarkMCP_ToolMix(b *testing.B) {
	cmd := exec.Command("../bin/mcp-server")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		b.Fatalf("start: %v", err)
	}
	defer cmd.Process.Kill()

	time.Sleep(500 * time.Millisecond)
	reader := bufio.NewReader(stdout)

	initReq := buildRequest(1, "initialize", nil)
	fmt.Fprintf(stdin, "%s\n", initReq)
	readResponse(reader)

	tools := []MCPTool{
		{Name: "search", Args: map[string]any{"query": "PostgreSQL"}},
		{Name: "search", Args: map[string]any{"query": "Redis"}},
		{Name: "topic", Args: map[string]any{"topic": "数据库架构"}},
		{Name: "timeline", Args: map[string]any{}},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tool := tools[i%len(tools)]
		params := map[string]any{
			"name":      tool.Name,
			"arguments": tool.Args,
		}
		req := buildRequest(10+i, "tools/call", params)
		fmt.Fprintf(stdin, "%s\n", req)
		readResponse(reader)
	}
}

// ============================================================
// 端到端延迟基准测试
// ============================================================

func TestEndToEndLatency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping end-to-end benchmark in short mode")
	}

	// 测试 MCP 服务器的 JSON-RPC 往返延迟
	cmd := exec.Command("../bin/mcp-server")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		t.Fatalf("start mcp-server: %v", err)
	}
	defer cmd.Process.Kill()

	time.Sleep(500 * time.Millisecond)
	reader := bufio.NewReader(stdout)

	// 初始化
	fmt.Fprintf(stdin, "%s\n", buildRequest(1, "initialize", nil))
	readResponse(reader)

	var results []struct {
		tool    string
		latency time.Duration
	}

	for _, tool := range mcpTools {
		if tool.NeedsLLM {
			continue // 跳过 LLM 工具，它们依赖外部 API
		}

		params := map[string]any{
			"name":      tool.Name,
			"arguments": tool.Args,
		}
		req := buildRequest(2, "tools/call", params)

		start := time.Now()
		fmt.Fprintf(stdin, "%s\n", req)
		readResponse(reader)
		elapsed := time.Since(start)

		results = append(results, struct {
			tool    string
			latency time.Duration
		}{tool.Name, elapsed})
	}

	t.Logf("MCP 端到端延迟结果 (%d 工具):", len(results))
	total := time.Duration(0)
	for _, r := range results {
		total += r.latency
		t.Logf("  %-20s %v", r.tool, r.latency)
	}
	if len(results) > 0 {
		avg := total / time.Duration(len(results))
		t.Logf("  平均延迟: %v", avg)
	}
}

// ============================================================
// JSON-RPC 吞吐量基准
// ============================================================

func BenchmarkMCP_JSONRPCThroughput(b *testing.B) {
	// 测量初始化 + tools/list 的吞吐量
	cmd := exec.Command("../bin/mcp-server")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		b.Fatalf("start: %v", err)
	}
	defer cmd.Process.Kill()

	time.Sleep(500 * time.Millisecond)
	reader := bufio.NewReader(stdout)

	var mu sync.Mutex
	var latencies []time.Duration

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := buildRequest(1+i, "tools/list", nil)
		start := time.Now()
		fmt.Fprintf(stdin, "%s\n", req)
		readResponse(reader)
		elapsed := time.Since(start)
		mu.Lock()
		latencies = append(latencies, elapsed)
		mu.Unlock()
	}

	if len(latencies) > 0 {
		total := time.Duration(0)
		for _, l := range latencies {
			total += l
		}
		avg := total / time.Duration(len(latencies))
		b.ReportMetric(float64(avg.Microseconds()), "avg-us/op")
	}
}

// ============================================================
// 辅助：检查 MCP 服务器是否可达
// ============================================================

func TestMCPBinaryExists(t *testing.T) {
	cmd := exec.Command("ls", "-la", "../bin/mcp-server")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Skipf("MCP binary not found (run 'go build -o bin/mcp-server ./cmd/mcp-server' first): %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "mcp-server") {
		t.Skip("mcp-server binary not found at ../bin/mcp-server")
	}
}
