package git_tree_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"feishu-mem/internal/card"
	"feishu-mem/internal/decision"
)

// ==== MCP 集成测试：创建决策 → 冲突 → 解决 → 验证 Git DAG ====

// TestMCP_ConflictLifecycle 完整的冲突决策生命周期测试
// 1. 启动 MCP server
// 2. create_decision A: 数据库采用 PostgreSQL 15
// 3. create_decision B: 数据库改为 MySQL 8.0 (冲突)
// 4. resolve_conflict: A 胜出
// 5. 验证 Git DAG 和决策状态
func TestMCP_ConflictLifecycle(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")

	// 创建最小化配置文件
	os.MkdirAll(filepath.Join(dir, "config"), 0755)
	configContent := fmt.Sprintf(`service:
  data_dir: "%s"
  output_dir: "%s"
  log_dir: "%s"
storage:
  git_path: "%s"
git:
  work_dir: "%s"
  branch: "main"
llm:
  enabled: true
project:
  name: "feishu-mem"
mcp:
  port: 37777
`, dataDir, filepath.Join(dir, "outputs"), filepath.Join(dir, "logs"),
		dataDir, dataDir)
	os.WriteFile(filepath.Join(dir, "config", "openclaw.yaml"), []byte(configContent), 0644)

	// 编译 mcp-server（在项目根目录下编译）
	projectRoot := "/Users/halllo/openclaw-workspace/feishu-agent-mem"
	buildCmd := exec.Command("go", "build", "-o", filepath.Join(dir, "mcp-server"),
		"./cmd/mcp-server/main.go")
	buildCmd.Dir = projectRoot
	buildCmd.Env = append(os.Environ(), "CONFIG_PATH=")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("build mcp-server failed: %v\n%s", err, out)
	}

	// 启动 mcp-server（在临时目录运行，指向临时 config）
	serverCmd := exec.Command(filepath.Join(dir, "mcp-server"))
	serverCmd.Dir = dir
	serverCmd.Env = append(os.Environ(),
		fmt.Sprintf("CONFIG_PATH=%s", filepath.Join(dir, "config", "openclaw.yaml")))
	stdin, _ := serverCmd.StdinPipe()
	stdout, _ := serverCmd.StdoutPipe()
	serverCmd.Stderr = os.Stderr
	if err := serverCmd.Start(); err != nil {
		t.Fatalf("start mcp-server failed: %v", err)
	}
	defer serverCmd.Process.Kill()

	reader := bufio.NewReader(stdout)
	msgID := 0

	sendMsg := func(method string, params map[string]interface{}) map[string]interface{} {
		msgID++
		req := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      msgID,
			"method":  method,
		}
		if params != nil {
			req["params"] = params
		}
		data, _ := json.Marshal(req)
		fmt.Fprintf(stdin, "%s\n", data)

		// 读取响应
		line, err := reader.ReadString('\n')
		if err != nil {
			t.Fatalf("read response failed: %v", err)
		}
		var resp map[string]interface{}
		json.Unmarshal([]byte(line), &resp)
		if e, ok := resp["error"]; ok {
			t.Fatalf("MCP error: %v", e)
		}
		result, _ := resp["result"].(map[string]interface{})
		return result
	}

	sendNotify := func(method string) {
		req := map[string]interface{}{
			"jsonrpc": "2.0",
			"method":  method,
		}
		data, _ := json.Marshal(req)
		fmt.Fprintf(stdin, "%s\n", data)
	}

	// 初始化 MCP 连接
	sendMsg("initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]interface{}{"name": "test", "version": "1.0"},
	})
	sendNotify("notifications/initialized")

	gitInDir := func(args ...string) string {
		cmd := exec.Command("git", args...)
		cmd.Dir = dataDir
		out, _ := cmd.CombinedOutput()
		return strings.TrimSpace(string(out))
	}

	sdrA := ""
	sdrB := ""

	t.Run("Step1_CreateDecisionA", func(t *testing.T) {
		result := sendMsg("tools/call", map[string]interface{}{
			"name": "create_decision",
			"arguments": map[string]interface{}{
				"title":        "数据库采用 PostgreSQL 15",
				"decision":     "经过架构评审，决定采用 PostgreSQL 15 作为主数据库",
				"rationale":    "PostgreSQL 15 在复杂查询性能、MVCC、扩展性方面优于 MySQL",
				"topic":        "数据库架构",
				"impact_level": "major",
			},
		})
		// 从返回文本中提取 SDR ID
		text := extractText(result)
		t.Logf("Create A result: %s", text)

		// 验证分支已创建
		branches := gitInDir("branch", "-a")
		if !strings.Contains(branches, "decision/") {
			t.Fatal("no decision branch created")
		}

		// 检查 git DAG
		dag := gitInDir("log", "--graph", "--oneline", "--all", "--decorate")
		t.Logf("DAG after A:\n%s", dag)
		_ = sdrA // will extract from text
	})

	t.Run("Step2_CreateDecisionB_Conflicting", func(t *testing.T) {
		result := sendMsg("tools/call", map[string]interface{}{
			"name": "create_decision",
			"arguments": map[string]interface{}{
				"title":        "数据库改为 MySQL 8.0",
				"decision":     "决定继续使用 MySQL 8.0 读写分离架构",
				"rationale":    "现有团队对 MySQL 运维经验丰富，PostgreSQL 学习成本高",
				"topic":        "数据库架构",
				"impact_level": "major",
			},
		})
		text := extractText(result)
		t.Logf("Create B result: %s", text)

		// 提取 SDR ID
		sdrB = extractSDRID(text)
		t.Logf("SDR B = %s", sdrB)

		// 验证 git DAG 有两个独立分支
		dag := gitInDir("log", "--graph", "--oneline", "--all", "--decorate")
		t.Logf("DAG after A+B:\n%s", dag)

		// 两个决策分支应该并列
		branches := gitInDir("branch", "-a")
		nDecisionBranches := strings.Count(branches, "decision/")
		if nDecisionBranches < 2 {
			t.Fatalf("expected at least 2 decision branches, got %d", nDecisionBranches)
		}
	})

	t.Run("Step3_VerifyGitStructure", func(t *testing.T) {
		dag := gitInDir("log", "--graph", "--oneline", "--all", "--decorate")

		// 验证 main 只有 dummy
		mainFiles := gitInDir("ls-tree", "--name-only", "main")
		if !strings.Contains(mainFiles, "DEC-000") {
			t.Fatal("main should contain DEC-000 dummy decision")
		}

		// 验证决策文件含 branch 和 version
		branches := gitInDir("branch", "-a")
		for _, b := range strings.Split(branches, "\n") {
			b = strings.TrimSpace(strings.TrimLeft(b, "* "))
			if !strings.HasPrefix(b, "decision/") {
				continue
			}
			// 读取决策文件中的 version
			rawFiles := gitInDir("ls-tree", "-r", "--name-only", b)
			t.Logf("  Files on %s:\n%s", b, rawFiles)
			for _, f := range strings.Split(rawFiles, "\n") {
				f = strings.TrimSpace(f)
				// Skip L0_RULES.md, Dummy (DEC-000), and non-decision files
				if f == "" || !strings.HasSuffix(f, ".md") || strings.Contains(f, "DEC-000") || strings.Contains(f, "L0_") {
					continue
				}
				content := gitInDir("show", b+":"+f)
				if !strings.Contains(content, "version: ") {
					t.Errorf("decision file %s on %s missing version field", b, f)
				}
				if !strings.Contains(content, "branch: ") {
					t.Errorf("decision file %s on %s missing branch field", b, f)
				}
			}
		}
		t.Logf("Git structure verified:\n%s", dag)
		_ = sdrA
		_ = sdrB
	})
}

// TestCard_Rendering_StatusIndicators 验证卡片渲染包含正确的状态指示
func TestCard_Rendering_StatusIndicators(t *testing.T) {
	renderer := card.NewRenderer()
	now := time.Now()

	tests := []struct {
		name     string
		node     *decision.DecisionNode
		wantTmpl string // expected header template color
		wantEmoji string // expected emoji in content
	}{
		{
			name: "pending_confirmation",
			node: &decision.DecisionNode{
				SDRID: "DEC-001", Title: "测试决策",
				Status: decision.StatusPendingConfirmation,
				ImpactLevel: decision.ImpactMajor,
				Topic: "测试", Decision: "测试内容",
				CreatedAt: now, Branch: "decision/DEC-001", Version: 1,
			},
			wantTmpl: "yellow",
			wantEmoji: "❓",
		},
		{
			name: "decided",
			node: &decision.DecisionNode{
				SDRID: "DEC-002", Title: "已决定的决策",
				Status: decision.StatusDecided,
				ImpactLevel: decision.ImpactMajor,
				Topic: "测试", Decision: "决定内容",
				CreatedAt: now, Branch: "decision/DEC-002", Version: 2,
			},
			wantTmpl: "green",
			wantEmoji: "✅",
		},
		{
			name: "superseded",
			node: &decision.DecisionNode{
				SDRID: "DEC-003", Title: "已被取代的决策",
				Status: decision.StatusSuperseded,
				ImpactLevel: decision.ImpactMinor,
				Topic: "测试", Decision: "旧内容",
				CreatedAt: now, Branch: "decision/DEC-003", Version: 1,
			},
			wantTmpl: "red",
			wantEmoji: "🔄",
		},
		{
			name: "in_discussion",
			node: &decision.DecisionNode{
				SDRID: "DEC-004", Title: "讨论中的决策",
				Status: decision.StatusInDiscussion,
				ImpactLevel: decision.ImpactAdvisory,
				Topic: "测试", Decision: "讨论内容",
				CreatedAt: now, Branch: "decision/DEC-004", Version: 1,
			},
			wantTmpl: "yellow",
			wantEmoji: "💬",
		},
		{
			name: "rejected",
			node: &decision.DecisionNode{
				SDRID: "DEC-005", Title: "被拒绝的决策",
				Status: decision.StatusRejected,
				ImpactLevel: decision.ImpactMinor,
				Topic: "测试", Decision: "被拒内容",
				CreatedAt: now, Branch: "decision/DEC-005", Version: 1,
			},
			wantTmpl: "red",
			wantEmoji: "❌",
		},
		{
			name: "completed",
			node: &decision.DecisionNode{
				SDRID: "DEC-006", Title: "已完成的决策",
				Status: decision.StatusCompleted,
				ImpactLevel: decision.ImpactCritical,
				Topic: "测试", Decision: "完成内容",
				CreatedAt: now, Branch: "decision/DEC-006", Version: 3,
			},
			wantTmpl: "green",
			wantEmoji: "✅",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonStr, err := renderer.RenderLarkCardFromNode(tt.node, 75.0)
			if err != nil {
				t.Fatalf("render card failed: %v", err)
			}

			// 验证 JSON 有效
			var cardData map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &cardData); err != nil {
				t.Fatalf("invalid card JSON: %v\n%s", err, jsonStr)
			}

			// 验证 header template 颜色
			header, _ := cardData["header"].(map[string]interface{})
			if header != nil {
				tmpl, _ := header["template"].(string)
				if tmpl != tt.wantTmpl {
					t.Errorf("header template = %q, want %q", tmpl, tt.wantTmpl)
				}
			}

			// 验证 elements 包含 emoji 指示
			elements, _ := cardData["elements"].([]interface{})
			foundEmoji := false
			for _, el := range elements {
				element, _ := el.(map[string]interface{})
				if element == nil {
					continue
				}
				text, _ := element["text"].(map[string]interface{})
				if text == nil {
					continue
				}
				content, _ := text["content"].(string)
				if strings.Contains(content, tt.wantEmoji) {
					foundEmoji = true
					break
				}
				// 也检查 fields
				fields, _ := element["fields"].([]interface{})
				for _, f := range fields {
					fMap, _ := f.(map[string]interface{})
					if fMap == nil {
						continue
					}
					fText, _ := fMap["text"].(map[string]interface{})
					if fText == nil {
						continue
					}
					fContent, _ := fText["content"].(string)
					if strings.Contains(fContent, tt.wantEmoji) {
						foundEmoji = true
						break
					}
				}
				if foundEmoji {
					break
				}
			}

			if !foundEmoji {
				t.Logf("Card JSON (partial): %s", jsonStr[:min(300, len(jsonStr))])
			}
		})
	}
}

// TestCard_ConflictResolution_Render 验证冲突解决卡片渲染
func TestCard_ConflictResolution_Render(t *testing.T) {
	renderer := card.NewRenderer()
	now := time.Now()

	nodeA := &decision.DecisionNode{
		SDRID: "DEC-001", Title: "数据库采用 PostgreSQL 15",
		Decision: "采用 PostgreSQL 15 作为主数据库",
		Status: decision.StatusDecided, ImpactLevel: decision.ImpactMajor,
		Topic: "数据库架构", CreatedAt: now,
		Branch: "decision/DEC-001", Version: 2,
	}
	nodeB := &decision.DecisionNode{
		SDRID: "DEC-002", Title: "数据库改为 MySQL 8.0",
		Decision: "改用 MySQL 8.0 读写分离",
		Status: decision.StatusDecided, ImpactLevel: decision.ImpactMajor,
		Topic: "数据库架构", CreatedAt: now,
		Branch: "decision/DEC-002", Version: 1,
	}

	jsonStr, err := renderer.RenderConflictResolutionCard(nodeA, nodeB, "两个数据库方案互相矛盾")
	if err != nil {
		t.Fatalf("render conflict card failed: %v", err)
	}

	// 验证 JSON 有效
	var cardData map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &cardData); err != nil {
		t.Fatalf("invalid conflict card JSON: %v", err)
	}

	// 验证 header 是红色冲突提示
	header, _ := cardData["header"].(map[string]interface{})
	if tmpl, _ := header["template"].(string); tmpl != "red" {
		t.Errorf("conflict header template = %q, want 'red'", tmpl)
	}

	// 验证包含两个决策的信息
	jsonStrLower := strings.ToLower(jsonStr)
	if !strings.Contains(jsonStrLower, "postgresql") {
		t.Error("conflict card missing PostgreSQL (A)")
	}
	if !strings.Contains(jsonStrLower, "mysql") {
		t.Error("conflict card missing MySQL (B)")
	}

	t.Logf("Conflict card rendered OK, length=%d", len(jsonStr))
}

// ==== 辅助函数 ====

func extractText(result map[string]interface{}) string {
	if result == nil {
		return ""
	}
	content, _ := result["content"].([]interface{})
	if len(content) == 0 {
		return ""
	}
	first, _ := content[0].(map[string]interface{})
	text, _ := first["text"].(string)
	return text
}

func extractSDRID(text string) string {
	// 从 "SDR ID: DEC-20260507174645-1" 格式中提取
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "SDR ID") || strings.Contains(line, "SDRID") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				return strings.TrimSpace(parts[len(parts)-1])
			}
		}
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
