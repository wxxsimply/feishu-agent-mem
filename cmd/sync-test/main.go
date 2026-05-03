package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"feishu-mem/internal/decision"
	"feishu-mem/internal/sync"
	"feishu-mem/internal/storage/bitable"
	gitstorage "feishu-mem/internal/storage/git"
	larkadapter "feishu-mem/internal/lark-adapter"
)

func main() {
	logFile, err := os.OpenFile("sync-test.log", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("无法创建日志文件: %v", err)
	}
	defer logFile.Close()
	log.SetOutput(io.MultiWriter(os.Stdout, logFile))

	log.Println("=== Git ↔ Bitable 双向同步测试 ===")

	dataDir := "./data"
	baseToken := "NnnMb5mWJaBJkXsHf69cIpfMn8b"
	tableID := "tblEBXkSxaqxnY6l"

	gitCLI := gitstorage.NewGitCLI(dataDir)
	gitConfig := gitstorage.Config{WorkDir: dataDir, Branch: "main"}
	gitStore, err := gitstorage.NewGitStorage(gitConfig)
	if err != nil {
		log.Printf("初始化 Git 存储失败: %v", err)
	}
	larkCLI := larkadapter.NewLarkCLI()
	bitableConfig := bitable.Config{BaseToken: baseToken, Tables: bitable.TablesConfig{Decision: tableID}}
	bitableStore := bitable.NewBitableStore(bitableConfig, larkCLI)
	syncManager := sync.NewSyncManager(gitStore, bitableStore, gitCLI, larkCLI, baseToken, tableID, dataDir)

	fmt.Println("\n--- 步骤 1: 清空 Git 和 Bitable ---")
	runClearScripts()
	gitStore, _ = gitstorage.NewGitStorage(gitConfig)
	time.Sleep(1 * time.Second)

	fmt.Println("\n--- 步骤 2: Git 端操作（5次）---")
	performGitOperations(syncManager)

	fmt.Println("\n--- 步骤 3: 正向同步 Git → Bitable ---")
	syncManager.SyncGitToBitable()

	fmt.Println("\n--- 调试: 检查 Bitable 原始数据 ---")
	dumpBitableRawData(larkCLI, baseToken, tableID)

	fmt.Println("\n--- 步骤 4: Bitable 端操作（5次）---")
	performBitableOperations(syncManager)

	fmt.Println("\n--- 步骤 5: 反向同步 Bitable → Git ---")
	syncManager.SyncBitableToGit()

	fmt.Println("\n--- 步骤 6: 最终双向同步 ---")
	syncManager.SyncGitToBitable()
	syncManager.SyncBitableToGit()

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("=== 测试完成 - 变更总结 ===")
	fmt.Println(strings.Repeat("=", 80))
	printChangeLog(syncManager.GetChangeLog())

	fmt.Println("\n" + strings.Repeat("=", 80))
	fmt.Println("=== 当前状态 ===")
	fmt.Println(strings.Repeat("=", 80))
	printCurrentState(syncManager)
}

func dumpBitableRawData(larkCLI *larkadapter.LarkCLI, baseToken, tableID string) {
	output, err := larkCLI.RunCommand("base", "+record-list", "--base-token", baseToken, "--table-id", tableID)
	if err != nil {
		log.Printf("获取 Bitable 原始数据失败: %v", err)
		return
	}
	log.Printf("Bitable 原始输出:\n%s", string(output))
}

func runClearScripts() {
	log.Println("运行清空脚本...")
	if err := os.Chdir("/Users/halllo/openclaw-workspace/feishu-agent-mem"); err == nil {
		cmd := []string{"bash", "-c", "./scripts/clear-git.sh"}
		output, _ := runCommand(cmd[0], cmd[1:]...)
		log.Println(string(output))
	}
	cmd := []string{"bash", "-c", "./scripts/clear-bitable.sh"}
	output, _ := runCommand(cmd[0], cmd[1:]...)
	log.Println(string(output))
}

func performGitOperations(sm *sync.SyncManager) {
	d1 := createTestDecision("dec_20260502_001", "用户认证模块设计", "采用 OAuth 2.0")
	hash1, _ := sm.CreateInGit(d1)
	log.Printf("1. 创建决策: dec_20260502_001, hash=%s", hash1)
	time.Sleep(500 * time.Millisecond)

	d2 := createTestDecision("dec_20260502_002", "数据库连接池配置", "最大连接数 100")
	hash2, _ := sm.CreateInGit(d2)
	log.Printf("2. 创建决策: dec_20260502_002, hash=%s", hash2)
	time.Sleep(500 * time.Millisecond)

	d1Update := *d1
	d1Update.Title = "用户认证模块设计（修订版）"
	d1Update.Decision = "采用 OAuth 2.0 + JWT 双机制"
	d1Update.Rationale = "需要支持移动端和 Web 端两种场景"
	d1Update.Status = decision.StatusDecided
	hash3, _ := sm.UpdateInGit(&d1Update)
	log.Printf("3. 更新决策: dec_20260502_001, hash=%s", hash3)
	time.Sleep(500 * time.Millisecond)

	d3 := createTestDecision("dec_20260502_003", "API 响应格式规范", "统一使用 JSON 格式")
	d3.ImpactLevel = decision.ImpactMajor
	hash4, _ := sm.CreateInGit(d3)
	log.Printf("4. 创建决策: dec_20260502_003, hash=%s", hash4)
	time.Sleep(500 * time.Millisecond)

	d2Update := *d2
	d2Update.Title = "数据库连接池配置（优化）"
	d2Update.Executor = "李四"
	d2Update.Status = decision.StatusExecuting
	hash5, _ := sm.UpdateInGit(&d2Update)
	log.Printf("5. 更新决策: dec_20260502_002, hash=%s", hash5)
	time.Sleep(500 * time.Millisecond)
}

func performBitableOperations(sm *sync.SyncManager) {
	d4 := createTestDecision("dec_20260502_004", "日志系统架构", "使用 ELK Stack")
	d4.Status = decision.StatusInDiscussion
	d4.ImpactLevel = decision.ImpactMinor
	sm.CreateInBitable(d4)
	log.Println("6. 在 Bitable 创建决策: dec_20260502_004")
	time.Sleep(500 * time.Millisecond)

	d3Update := createTestDecision("dec_20260502_003", "API 响应格式规范（v2）", "统一使用 JSON 格式，支持分页")
	d3Update.Status = decision.StatusDecided
	d3Update.Executor = "王五"
	sm.UpdateInBitable(d3Update)
	log.Println("7. 在 Bitable 更新决策: dec_20260502_003")
	time.Sleep(500 * time.Millisecond)

	d5 := createTestDecision("dec_20260502_005", "缓存策略设计", "Redis + 本地缓存两级")
	d5.ImpactLevel = decision.ImpactCritical
	sm.CreateInBitable(d5)
	log.Println("8. 在 Bitable 创建决策: dec_20260502_005")
	time.Sleep(500 * time.Millisecond)

	d4Update := *d4
	d4Update.Title = "日志系统架构（完善）"
	d4Update.Rationale = "需要支持日志分级和查询性能"
	d4Update.Status = decision.StatusDecided
	sm.UpdateInBitable(&d4Update)
	log.Println("9. 在 Bitable 更新决策: dec_20260502_004")
	time.Sleep(500 * time.Millisecond)

	d6 := createTestDecision("dec_20260502_006", "监控告警方案", "Prometheus + Grafana")
	d6.ImpactLevel = decision.ImpactMinor
	sm.CreateInBitable(d6)
	log.Println("10. 在 Bitable 创建决策: dec_20260502_006")
	time.Sleep(500 * time.Millisecond)
}

func createTestDecision(sdrID, title, decisionText string) *decision.DecisionNode {
	now := time.Now()
	return &decision.DecisionNode{
		SDRID:         sdrID,
		Title:         title,
		Decision:      decisionText,
		Rationale:     "根据项目需求讨论决定",
		Project:       "feishu-mem",
		Topic:         "general",
		Phase:         "dev",
		PhaseScope:    decision.PhaseScopeSpan,
		ImpactLevel:   decision.ImpactMajor,
		Status:        decision.StatusPending,
		Proposer:      "张三",
		Executor:      "",
		Stakeholders:  []string{"张三", "李四"},
		CrossTopicRefs: []string{},
		CreatedAt:     now,
		DecidedAt:     nil,
	}
}

func printChangeLog(changelog []sync.ChangeLogEntry) {
	fmt.Printf("\n共 %d 次变更:\n\n", len(changelog))
	for i, entry := range changelog {
		fmt.Printf("%d. [%s] %s - %s\n", i+1, entry.Source, strings.ToUpper(entry.Op), entry.SDRID)
		if entry.Title != "" {
			fmt.Printf("   标题: %s\n", entry.Title)
		}
		fmt.Printf("   详情: %s\n", entry.Details)
		fmt.Printf("   时间: %s\n", entry.Time.Format("2006-01-02 15:04:05"))
		fmt.Println()
	}

	gitCount := 0
	bitableCount := 0
	createCount := 0
	updateCount := 0
	for _, e := range changelog {
		if e.Source == "git" { gitCount++ } else { bitableCount++ }
		if e.Op == "create" { createCount++ } else if e.Op == "update" { updateCount++ }
	}
	fmt.Printf("统计: Git端:%d次, Bitable端:%d次, 创建:%d次, 更新:%d次\n", gitCount, bitableCount, createCount, updateCount)
}

func printCurrentState(sm *sync.SyncManager) {
	fmt.Println("\n--- Git 中的决策 ---")
	gitDecisions, _ := sm.ListFromGit()
	if len(gitDecisions) == 0 {
		fmt.Println("  (空)")
	} else {
		for _, d := range gitDecisions {
			fmt.Printf("  - %s: %s [%s] (hash: %q)\n", d.SDRID, d.Title, d.Status, d.GitCommitHash)
		}
	}

	fmt.Println("\n--- Bitable 中的决策 ---")
	bitableDecisions, _ := sm.ListFromBitable()
	if len(bitableDecisions) == 0 {
		fmt.Println("  (空)")
	} else {
		for _, d := range bitableDecisions {
			fmt.Printf("  - %s: %s [%s] (hash: %q)\n", d.SDRID, d.Title, d.Status, d.GitCommitHash)
		}
	}
}

func runCommand(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = "/Users/halllo/openclaw-workspace/feishu-agent-mem"
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		return stdout.Bytes(), fmt.Errorf("%s %v: %w: %s", name, args, err, stderr.String())
	}
	return stdout.Bytes(), nil
}
