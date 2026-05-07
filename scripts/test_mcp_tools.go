// +build ignore

package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"
)

type JSONRPCReq struct {
	JSONRPC string `json:"jsonrpc"`
	ID      int    `json:"id"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

type JSONRPCResp struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   any             `json:"error,omitempty"`
}

func main() {
	os.Chdir("/Users/halllo/openclaw-workspace/feishu-agent-mem")

	cmd := exec.Command("./bin/mem-service", "-mode", "mcp")
	stdin, _ := cmd.StdinPipe()
	stdout, _ := cmd.StdoutPipe()
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		log.Fatalf("Failed to start MCP server: %v", err)
	}
	defer cmd.Process.Kill()

	reader := bufio.NewReader(stdout)
	enc := json.NewEncoder(stdin)
	dec := json.NewDecoder(reader)

	nextID := 1
	call := func(method string, params any) (json.RawMessage, error) {
		id := nextID
		nextID++
		req := JSONRPCReq{
			JSONRPC: "2.0",
			ID:      id,
			Method:  method,
			Params:  params,
		}
		if err := enc.Encode(req); err != nil {
			return nil, fmt.Errorf("encode error: %w", err)
		}
		for {
			var raw json.RawMessage
			if err := dec.Decode(&raw); err != nil {
				return nil, fmt.Errorf("decode error: %w", err)
			}
			var notif struct{ Method string }
			if json.Unmarshal(raw, &notif) == nil && notif.Method != "" {
				continue
			}
			var resp JSONRPCResp
			if err := json.Unmarshal(raw, &resp); err != nil {
				continue
			}
			if resp.ID != id {
				continue
			}
			if resp.Error != nil {
				return nil, fmt.Errorf("RPC error: %v", resp.Error)
			}
			return resp.Result, nil
		}
	}

	callTool := func(name string, args any) (string, error) {
		result, err := call("tools/call", map[string]any{
			"name":      name,
			"arguments": args,
		})
		if err != nil {
			return "", err
		}
		var content struct{ Content []struct{ Text string } }
		json.Unmarshal(result, &content)
		if len(content.Content) > 0 {
			return content.Content[0].Text, nil
		}
		return "(no text content)", nil
	}

	pass := 0
	fail := 0
	check := func(name string, text string, err error) {
		if err != nil {
			fmt.Printf("❌ %s FAILED: %v\n", name, err)
			fail++
		} else {
			fmt.Printf("✅ %s OK\n%s\n", name, text)
			pass++
		}
	}

	// Initialize
	fmt.Println("==========================================")
	fmt.Println("=== Step 1: Initialize ===")
	fmt.Println("==========================================")
	result, err := call("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]string{"name": "mcp-test", "version": "1.0.0"},
	})
	if err != nil {
		log.Fatalf("Initialize failed: %v", err)
	}
	var initResult map[string]any
	json.Unmarshal(result, &initResult)
	fmt.Printf("✅ Initialize OK - Server: %v\n\n", initResult["serverInfo"])

	// Send initialized notification
	enc.Encode(JSONRPCReq{JSONRPC: "2.0", Method: "notifications/initialized"})
	time.Sleep(200 * time.Millisecond)

	// ====================================================
	// QUERY TOOLS
	// ====================================================
	fmt.Println("==========================================")
	fmt.Println("=== QUERY TOOLS ===")
	fmt.Println("==========================================")

	text, err := callTool("stats", map[string]any{})
	check("stats", text, err)

	text, err = callTool("list_topics", map[string]any{})
	check("list_topics", text, err)

	text, err = callTool("timeline", map[string]any{})
	check("timeline", text, err)

	text, err = callTool("search", map[string]any{"query": "数据库", "limit": 5})
	check("search", text, err)

	text, err = callTool("search_fulltext", map[string]any{"query": "数据库"})
	check("search_fulltext", text, err)

	text, err = callTool("recent_decisions", map[string]any{"hours": 72})
	check("recent_decisions", text, err)

	text, err = callTool("hot_decisions", map[string]any{})
	check("hot_decisions", text, err)

	text, err = callTool("forgotten_decisions", map[string]any{})
	check("forgotten_decisions", text, err)

	text, err = callTool("topic", map[string]any{"topic": "general"})
	check("topic", text, err)

	text, err = callTool("objection_list", map[string]any{})
	check("objection_list", text, err)

	text, err = callTool("conflict_list", map[string]any{})
	check("conflict_list（全部）", text, err)

	// ====================================================
	// CREATE TOOL
	// ====================================================
	fmt.Println("\n==========================================")
	fmt.Println("=== CREATE DECISION ===")
	fmt.Println("==========================================")

	text, err = callTool("create_decision", map[string]any{
		"title":    "MCP工具测试决策",
		"decision": "这是一个测试决策，用于验证MCP工具的创建功能是否正常。",
		"rationale": "需要验证MCP服务器的create_decision工具能正常工作。",
		"topic":    "general",
		"phase":    "testing",
	})
	check("create_decision", text, err)

	// Extract SDRID from result
	sdrid := ""
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "SDR ID") {
			parts := strings.Split(line, "**: ")
			if len(parts) > 1 {
				sdrid = strings.TrimSpace(parts[1])
			}
		}
	}

	if sdrid == "" {
		fmt.Println("⚠️  Could not extract SDRID from create result, skipping dependent tests")
	} else {
		fmt.Printf("\n📌 Created SDRID: %s\n", sdrid)

		// ====================================================
		// SINGLE DECISION QUERY TOOLS
		// ====================================================
		fmt.Println("\n==========================================")
		fmt.Println("=== SINGLE DECISION QUERY TOOLS ===")
		fmt.Println("==========================================")

		text, err = callTool("decision", map[string]any{"sdr_id": sdrid})
		check("decision", text, err)

		text, err = callTool("decision_card", map[string]any{"sdr_id": sdrid})
		check("decision_card", text, err)

		text, err = callTool("get_relations", map[string]any{"sdr_id": sdrid})
		check("get_relations", text, err)

		text, err = callTool("conflict_list", map[string]any{"sdr_id": sdrid})
		check("conflict_list（单个决策）", text, err)

		text, err = callTool("related_decisions", map[string]any{"sdr_id": sdrid})
		check("related_decisions", text, err)

		// ====================================================
		// UPDATE TOOL
		// ====================================================
		fmt.Println("\n==========================================")
		fmt.Println("=== UPDATE DECISION ===")
		fmt.Println("==========================================")

		text, err = callTool("update_decision", map[string]any{
			"sdr_id": sdrid,
			"title":  "MCP工具测试决策(已更新)",
			"status": "decided",
		})
		check("update_decision", text, err)

		// Verify update
		text, err = callTool("decision", map[string]any{"sdr_id": sdrid})
		check("decision（验证更新）", text, err)

		// ====================================================
		// DECISION HISTORY TOOLS
		// ====================================================
		fmt.Println("\n==========================================")
		fmt.Println("=== DECISION HISTORY TOOLS ===")
		fmt.Println("==========================================")

		text, err = callTool("decision_history", map[string]any{"sdr_id": sdrid})
		check("decision_history", text, err)

		text, err = callTool("git_blame", map[string]any{"sdr_id": sdrid})
		check("git_blame", text, err)
	}

	// ====================================================
	// GIT TOOLS
	// ====================================================
	fmt.Println("\n==========================================")
	fmt.Println("=== GIT TOOLS ===")
	fmt.Println("==========================================")

	text, err = callTool("git_history", map[string]any{"limit": 5})
	check("git_history", text, err)

	text, err = callTool("git_search", map[string]any{"query": "MCP", "project": "feishu-mem"})
	check("git_search", text, err)

	// ====================================================
	// SUMMARY
	// ====================================================
	fmt.Println("\n==========================================")
	fmt.Println("=== SUMMARY ===")
	fmt.Printf("✅ Passed: %d | ❌ Failed: %d\n", pass, fail)
	fmt.Println("==========================================")
	fmt.Println("工具分类:")
	fmt.Println("  查询类(8): stats, list_topics, timeline, search, search_fulltext, topic, conflict_list, objection_list")
	fmt.Println("  热点类(3): recent_decisions, hot_decisions, forgotten_decisions")
	fmt.Println("  单决策查询(5): decision, decision_card, get_relations, related_decisions, conflict_list")
	fmt.Println("  历史类(3): decision_history, git_blame, git_history, git_search")
	fmt.Println("  创建类(1): create_decision")
	fmt.Println("  更新类(1): update_decision")
}
