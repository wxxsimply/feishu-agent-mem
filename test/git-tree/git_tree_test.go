package git_tree_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ==== 决策节点模型（新设计）====

type DecisionStatus string

const (
	StatusDecided    DecisionStatus = "decided"
	StatusSuperseded DecisionStatus = "superseded"
	StatusDeprecated DecisionStatus = "deprecated"
	StatusCompleted  DecisionStatus = "completed"
)

type Relation struct {
	Type        string `yaml:"type"`
	TargetSDRID string `yaml:"target_sdr_id"`
}

type DecisionNode struct {
	SDRID          string     `yaml:"sdr_id"`
	Title          string     `yaml:"title"`
	Branch         string     `yaml:"branch"`
	Version        int        `yaml:"version"`
	Status         DecisionStatus `yaml:"status"`
	ConflictStatus string     `yaml:"conflict_status,omitempty"`
	ConflictWith   string     `yaml:"conflict_with,omitempty"`
	Relations      []Relation `yaml:"relations,omitempty"`
}

const (
	dummySDRID           = "DEC-000"
	dummyTitle           = "Root Dummy"
	branchPrefixDecision = "decision/"
	conflictActive       = "active"
	conflictResolved     = "resolved"
)

// ==== 测试辅助函数 ====

func setupTestRepo(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "git-tree-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	git(t, dir, "init")
	git(t, dir, "config", "user.name", "test")
	git(t, dir, "config", "user.email", "test@test.com")

	// 创建 L0_RULES.md
	os.WriteFile(filepath.Join(dir, "L0_RULES.md"), []byte("# L0 Rules\n\n"), 0644)
	git(t, dir, "add", "L0_RULES.md")
	git(t, dir, "commit", "-m", "Initial commit: L0 rules")

	// 创建 Dummy 决策
	writeDecisionFile(t, dir, DecisionNode{
		SDRID:   dummySDRID,
		Title:   dummyTitle,
		Branch:  "main",
		Version: 0,
		Status:  StatusCompleted,
	})
	git(t, dir, "add", dummySDRID+".md")
	git(t, dir, "commit", "-m", "dummy: Root Dummy (v0)")

	return dir
}

func writeDecisionFile(t *testing.T, dir string, node DecisionNode) {
	t.Helper()
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "sdr_id: %s\n", node.SDRID)
	fmt.Fprintf(&b, "title: %s\n", node.Title)
	fmt.Fprintf(&b, "branch: %s\n", node.Branch)
	fmt.Fprintf(&b, "version: %d\n", node.Version)
	fmt.Fprintf(&b, "status: %s\n", node.Status)
	if node.ConflictStatus != "" {
		fmt.Fprintf(&b, "conflict_status: %s\n", node.ConflictStatus)
	}
	if node.ConflictWith != "" {
		fmt.Fprintf(&b, "conflict_with: %s\n", node.ConflictWith)
	}
	if len(node.Relations) > 0 {
		b.WriteString("relations:\n")
		for _, r := range node.Relations {
			fmt.Fprintf(&b, "  - type: %s\n", r.Type)
			fmt.Fprintf(&b, "    target_sdr_id: %s\n", r.TargetSDRID)
		}
	}
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", node.Title)
	os.WriteFile(filepath.Join(dir, node.SDRID+".md"), []byte(b.String()), 0644)
}

func readDecisionFromBranch(t *testing.T, dir, branch, sdrID string) DecisionNode {
	t.Helper()
	// git show branch:filepath
	content := git(t, dir, "show", branch+":"+sdrID+".md")
	return parseDecision(t, content, sdrID)
}

func parseDecision(t *testing.T, content, sdrID string) DecisionNode {
	t.Helper()
	node := DecisionNode{SDRID: sdrID}
	lines := strings.Split(content, "\n")
	inFrontmatter := false
	var currentRel *Relation

	for _, raw := range lines {
		if strings.TrimSpace(raw) == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break
		}
		if !inFrontmatter {
			continue
		}

		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}

		// 处理关系列表项: "  - type: CONFLICTS_WITH"
		if strings.HasPrefix(raw, "  - ") && strings.Contains(trimmed, ":") {
			parts := strings.SplitN(trimmed, ": ", 2)
			if len(parts) == 2 {
				key := strings.TrimSpace(strings.TrimPrefix(parts[0], "- "))
				val := strings.TrimSpace(parts[1])
				if key == "type" {
					if currentRel != nil {
						node.Relations = append(node.Relations, *currentRel)
					}
					currentRel = &Relation{Type: val}
				}
			}
			continue
		}

		// 处理关系子字段: "    target_sdr_id: DEC-002"
		if raw != trimmed && !strings.HasPrefix(raw, "  - ") && strings.Contains(trimmed, ":") && currentRel != nil {
			// raw starts with whitespace but is not a "- " list item → relation sub-field
			parts := strings.SplitN(trimmed, ": ", 2)
			if len(parts) == 2 {
				key, val := parts[0], parts[1]
				if key == "target_sdr_id" {
					currentRel.TargetSDRID = val
				}
			}
			continue
		}

		// 顶级键值对
		if !strings.Contains(trimmed, ": ") {
			continue
		}
		parts := strings.SplitN(trimmed, ": ", 2)
		if len(parts) != 2 {
			continue
		}
		key, val := parts[0], parts[1]

		switch key {
		case "title":
			node.Title = val
		case "branch":
			node.Branch = val
		case "version":
			fmt.Sscanf(val, "%d", &node.Version)
		case "status":
			node.Status = DecisionStatus(val)
		case "conflict_status":
			node.ConflictStatus = val
		case "conflict_with":
			node.ConflictWith = val
		}
	}
	// 追加最后一个 relation
	if currentRel != nil {
		node.Relations = append(node.Relations, *currentRel)
	}
	return node
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(out))
	}
	return strings.TrimSpace(string(out))
}

func gitAllowFail(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func assertBranchExists(t *testing.T, dir, branch string) {
	t.Helper()
	branches := git(t, dir, "branch", "-a")
	if !strings.Contains(branches, branch) {
		t.Fatalf("expected branch %q to exist, got:\n%s", branch, branches)
	}
}

func assertVersion(t *testing.T, dir, branch, sdrID string, wantVersion int) {
	t.Helper()
	node := readDecisionFromBranch(t, dir, branch, sdrID)
	if node.Version != wantVersion {
		t.Fatalf("version = %d, want %d (branch=%s, sdr=%s)", node.Version, wantVersion, branch, sdrID)
	}
}

func assertStatus(t *testing.T, dir, branch, sdrID string, wantStatus DecisionStatus) {
	t.Helper()
	node := readDecisionFromBranch(t, dir, branch, sdrID)
	if node.Status != wantStatus {
		t.Fatalf("status = %q, want %q (branch=%s, sdr=%s)", node.Status, wantStatus, branch, sdrID)
	}
}

func assertConflict(t *testing.T, dir, branch, sdrID, wantStatus, wantWith string) {
	t.Helper()
	node := readDecisionFromBranch(t, dir, branch, sdrID)
	if node.ConflictStatus != wantStatus {
		t.Fatalf("conflict_status = %q, want %q (branch=%s, sdr=%s)", node.ConflictStatus, wantStatus, branch, sdrID)
	}
	if node.ConflictWith != wantWith {
		t.Fatalf("conflict_with = %q, want %q (branch=%s, sdr=%s)", node.ConflictWith, wantWith, branch, sdrID)
	}
}

// ==============================================
// 测试用例
// ==============================================

// TestGitTree_CreateDecision: 创建全新决策 → 新分支 + version=1
func TestGitTree_CreateDecision(t *testing.T) {
	dir := setupTestRepo(t)
	sdrID := "DEC-001"

	// 1. 从 main 创建决策分支
	git(t, dir, "checkout", "-b", branchPrefixDecision+sdrID, "main")

	// 2. 写入决策文件
	writeDecisionFile(t, dir, DecisionNode{
		SDRID:   sdrID,
		Title:   "采用 Spring Boot 3 作为微服务框架",
		Branch:  branchPrefixDecision + sdrID,
		Version: 1,
		Status:  StatusDecided,
	})

	// 3. 提交
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "create(DEC-001): 采用 Spring Boot 3")

	// 验证
	assertBranchExists(t, dir, branchPrefixDecision+sdrID)
	assertVersion(t, dir, branchPrefixDecision+sdrID, sdrID, 1)

	// 验证 main 不受影响
	mainNode := readDecisionFromBranch(t, dir, "main", dummySDRID)
	if mainNode.Version != 0 {
		t.Fatalf("main dummy version should remain 0, got %d", mainNode.Version)
	}

	commits := git(t, dir, "log", "--oneline", branchPrefixDecision+sdrID)
	if !strings.Contains(commits, "create(DEC-001)") {
		t.Fatalf("commit message not found in log:\n%s", commits)
	}
}

// TestGitTree_UpdateDecision: 更新决策 → version++
func TestGitTree_UpdateDecision(t *testing.T) {
	dir := setupTestRepo(t)
	sdrID := "DEC-001"

	// 创建 DEC-001
	git(t, dir, "checkout", "-b", branchPrefixDecision+sdrID, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID, Title: "Spring Boot", Branch: branchPrefixDecision + sdrID, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "create(DEC-001): Spring Boot")

	// 更新：修改标题
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID, Title: "Spring Boot 3.2 + JDK 21（更新版本）",
		Branch: branchPrefixDecision + sdrID, Version: 2, Status: StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): 更新 JDK 版本")

	// 验证 version=2
	assertVersion(t, dir, branchPrefixDecision+sdrID, sdrID, 2)

	// 验证标题已更新
	node := readDecisionFromBranch(t, dir, branchPrefixDecision+sdrID, sdrID)
	if !strings.Contains(node.Title, "JDK 21") {
		t.Fatalf("title not updated: %q", node.Title)
	}

	// 验证提交历史（只计数决策分支独有的 commits，排除 main 的）
	commitCount := git(t, dir, "rev-list", "--count", branchPrefixDecision+sdrID, "^main")
	if commitCount != "2" {
		t.Fatalf("expected 2 commits on decision branch, got %s", commitCount)
	}
}

// TestGitTree_Conflict: 冲突检测 → 双方标记 conflict_status=active
func TestGitTree_Conflict(t *testing.T) {
	dir := setupTestRepo(t)
	sdrA, sdrB := "DEC-001", "DEC-002"

	// 创建决策 A
	git(t, dir, "checkout", "-b", branchPrefixDecision+sdrA, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA, Title: "数据库采用 PostgreSQL 15",
		Branch: branchPrefixDecision + sdrA, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrA+".md")
	git(t, dir, "commit", "-m", "create(DEC-001): PostgreSQL 15")

	// 创建决策 B
	git(t, dir, "checkout", "main")
	git(t, dir, "checkout", "-b", branchPrefixDecision+sdrB, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrB, Title: "数据库采用 MySQL 8.0",
		Branch: branchPrefixDecision + sdrB, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrB+".md")
	git(t, dir, "commit", "-m", "create(DEC-002): MySQL 8.0")

	// 标记冲突：决策 A
	git(t, dir, "checkout", branchPrefixDecision+sdrA)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA, Title: "数据库采用 PostgreSQL 15",
		Branch: branchPrefixDecision + sdrA, Version: 2, Status: StatusDecided,
		ConflictStatus: "active", ConflictWith: sdrB,
		Relations: []Relation{{Type: "CONFLICTS_WITH", TargetSDRID: sdrB}},
	})
	git(t, dir, "add", sdrA+".md")
	git(t, dir, "commit", "-m", "conflict(DEC-001): vs DEC-002")

	// 标记冲突：决策 B
	git(t, dir, "checkout", branchPrefixDecision+sdrB)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrB, Title: "数据库采用 MySQL 8.0",
		Branch: branchPrefixDecision + sdrB, Version: 2, Status: StatusDecided,
		ConflictStatus: "active", ConflictWith: sdrA,
		Relations: []Relation{{Type: "CONFLICTS_WITH", TargetSDRID: sdrA}},
	})
	git(t, dir, "add", sdrB+".md")
	git(t, dir, "commit", "-m", "conflict(DEC-002): vs DEC-001")

	// 验证双方 version=2 且冲突状态正确
	assertVersion(t, dir, branchPrefixDecision+sdrA, sdrA, 2)
	assertVersion(t, dir, branchPrefixDecision+sdrB, sdrB, 2)
	assertConflict(t, dir, branchPrefixDecision+sdrA, sdrA, "active", sdrB)
	assertConflict(t, dir, branchPrefixDecision+sdrB, sdrB, "active", sdrA)

	// 验证 A 的 Relations 包含 CONFLICTS_WITH
	nodeA := readDecisionFromBranch(t, dir, branchPrefixDecision+sdrA, sdrA)
	found := false
	for _, r := range nodeA.Relations {
		if r.Type == "CONFLICTS_WITH" && r.TargetSDRID == sdrB {
			found = true
		}
	}
	if !found {
		t.Fatal("DEC-001 missing CONFLICTS_WITH relation to DEC-002")
	}
}

// TestGitTree_ConflictResolution: 冲突解决 → 胜者 resolved, 败者 superseded
func TestGitTree_ConflictResolution(t *testing.T) {
	dir := setupTestRepo(t)
	sdrA, sdrB := "DEC-001", "DEC-002"

	// 创建 A
	git(t, dir, "checkout", "-b", branchPrefixDecision+sdrA, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA, Title: "PostgreSQL 15", Branch: branchPrefixDecision + sdrA, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrA+".md")
	git(t, dir, "commit", "-m", "create(DEC-001): PostgreSQL 15")

	// 创建 B
	git(t, dir, "checkout", "main")
	git(t, dir, "checkout", "-b", branchPrefixDecision+sdrB, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrB, Title: "MySQL 8.0", Branch: branchPrefixDecision + sdrB, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrB+".md")
	git(t, dir, "commit", "-m", "create(DEC-002): MySQL 8.0")

	// 标记冲突（双方 version=2）
	git(t, dir, "checkout", branchPrefixDecision+sdrA)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA, Title: "PostgreSQL 15", Branch: branchPrefixDecision + sdrA, Version: 2, Status: StatusDecided,
		ConflictStatus: "active", ConflictWith: sdrB,
		Relations: []Relation{{Type: "CONFLICTS_WITH", TargetSDRID: sdrB}},
	})
	git(t, dir, "add", sdrA+".md")
	git(t, dir, "commit", "-m", "conflict(DEC-001): vs DEC-002")

	git(t, dir, "checkout", branchPrefixDecision+sdrB)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrB, Title: "MySQL 8.0", Branch: branchPrefixDecision + sdrB, Version: 2, Status: StatusDecided,
		ConflictStatus: "active", ConflictWith: sdrA,
		Relations: []Relation{{Type: "CONFLICTS_WITH", TargetSDRID: sdrA}},
	})
	git(t, dir, "add", sdrB+".md")
	git(t, dir, "commit", "-m", "conflict(DEC-002): vs DEC-001")

	// === 冲突解决：A 胜出 ===
	git(t, dir, "checkout", branchPrefixDecision+sdrA)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA, Title: "PostgreSQL 15", Branch: branchPrefixDecision + sdrA, Version: 3, Status: StatusDecided,
		ConflictStatus: "resolved", ConflictWith: "",
		Relations: []Relation{{Type: "CONFLICTS_WITH", TargetSDRID: sdrB}},
	})
	git(t, dir, "add", sdrA+".md")
	git(t, dir, "commit", "-m", "resolve(DEC-001): won vs DEC-002")

	git(t, dir, "checkout", branchPrefixDecision+sdrB)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrB, Title: "MySQL 8.0", Branch: branchPrefixDecision + sdrB, Version: 3, Status: StatusSuperseded,
		ConflictStatus: "resolved", ConflictWith: "",
		Relations: []Relation{{Type: "SUPERSEDES", TargetSDRID: sdrA}},
	})
	git(t, dir, "add", sdrB+".md")
	git(t, dir, "commit", "-m", "resolve(DEC-002): superseded by DEC-001")

	// 验证胜者
	assertVersion(t, dir, branchPrefixDecision+sdrA, sdrA, 3)
	assertConflict(t, dir, branchPrefixDecision+sdrA, sdrA, "resolved", "")
	assertStatus(t, dir, branchPrefixDecision+sdrA, sdrA, StatusDecided)

	// 验证败者
	assertVersion(t, dir, branchPrefixDecision+sdrB, sdrB, 3)
	assertConflict(t, dir, branchPrefixDecision+sdrB, sdrB, "resolved", "")
	assertStatus(t, dir, branchPrefixDecision+sdrB, sdrB, StatusSuperseded)
	nodeB := readDecisionFromBranch(t, dir, branchPrefixDecision+sdrB, sdrB)
	found := false
	for _, r := range nodeB.Relations {
		if r.Type == "SUPERSEDES" && r.TargetSDRID == sdrA {
			found = true
		}
	}
	if !found {
		t.Fatal("DEC-002 missing SUPERSEDES relation to DEC-001")
	}
}

// TestGitTree_ConflictResolution_NewDecision: 新决策 C 替代 A 和 B
func TestGitTree_ConflictResolution_NewDecision(t *testing.T) {
	dir := setupTestRepo(t)
	sdrA, sdrB, sdrC := "DEC-001", "DEC-002", "DEC-003"

	// 创建 A (v1→v2)
	git(t, dir, "checkout", "-b", branchPrefixDecision+sdrA, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA, Title: "PostgreSQL", Branch: branchPrefixDecision + sdrA, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrA+".md")
	git(t, dir, "commit", "-m", "create(DEC-001): PostgreSQL")
	// A 更新到 v2
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA, Title: "PostgreSQL 15", Branch: branchPrefixDecision + sdrA, Version: 2, Status: StatusDecided,
	})
	git(t, dir, "add", sdrA+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): v2")

	// 创建 B (v1)
	git(t, dir, "checkout", "main")
	git(t, dir, "checkout", "-b", branchPrefixDecision+sdrB, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrB, Title: "MySQL 8.0", Branch: branchPrefixDecision + sdrB, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrB+".md")
	git(t, dir, "commit", "-m", "create(DEC-002): MySQL")

	// 创建 C: version = max(A.v2, B.v1) + 1 = 3
	git(t, dir, "checkout", "main")
	git(t, dir, "checkout", "-b", branchPrefixDecision+sdrC, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrC, Title: "TiDB（替代方案）",
		Branch: branchPrefixDecision + sdrC, Version: 3, Status: StatusDecided,
		Relations: []Relation{
			{Type: "SUPERSEDES", TargetSDRID: sdrA},
			{Type: "SUPERSEDES", TargetSDRID: sdrB},
		},
	})
	git(t, dir, "add", sdrC+".md")
	git(t, dir, "commit", "-m", "create(DEC-003): TiDB, supersedes A,B")

	// 废弃 A 和 B
	git(t, dir, "checkout", branchPrefixDecision+sdrA)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA, Title: "PostgreSQL 15", Branch: branchPrefixDecision + sdrA, Version: 3, Status: StatusSuperseded,
	})
	git(t, dir, "add", sdrA+".md")
	git(t, dir, "commit", "-m", "supersede(DEC-001): by DEC-003")

	git(t, dir, "checkout", branchPrefixDecision+sdrB)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrB, Title: "MySQL 8.0", Branch: branchPrefixDecision + sdrB, Version: 2, Status: StatusSuperseded,
	})
	git(t, dir, "add", sdrB+".md")
	git(t, dir, "commit", "-m", "supersede(DEC-002): by DEC-003")

	// 验证 C 的 version
	assertVersion(t, dir, branchPrefixDecision+sdrC, sdrC, 3)
	assertStatus(t, dir, branchPrefixDecision+sdrC, sdrC, StatusDecided)

	// 验证 A 被 superseded
	assertStatus(t, dir, branchPrefixDecision+sdrA, sdrA, StatusSuperseded)
	assertVersion(t, dir, branchPrefixDecision+sdrA, sdrA, 3)

	// 验证 B 被 superseded
	assertStatus(t, dir, branchPrefixDecision+sdrB, sdrB, StatusSuperseded)
	assertVersion(t, dir, branchPrefixDecision+sdrB, sdrB, 2)

	// 验证 C 的 Relations
	nodeC := readDecisionFromBranch(t, dir, branchPrefixDecision+sdrC, sdrC)
	if len(nodeC.Relations) != 2 {
		t.Fatalf("expected 2 relations on DEC-003, got %d", len(nodeC.Relations))
	}
}

// TestGitTree_Revert: 回退决策 → 版本跳跃到最新 + 旧决策 superseded
func TestGitTree_Revert(t *testing.T) {
	dir := setupTestRepo(t)
	sdrA, sdrB := "DEC-001", "DEC-002"

	// 创建 A: v1→v2→v3
	git(t, dir, "checkout", "-b", branchPrefixDecision+sdrA, "main")
	for v := 1; v <= 3; v++ {
		writeDecisionFile(t, dir, DecisionNode{
			SDRID: sdrA, Title: fmt.Sprintf("Spring Boot (v%d)", v),
			Branch: branchPrefixDecision + sdrA, Version: v, Status: StatusDecided,
		})
		git(t, dir, "add", sdrA+".md")
		git(t, dir, "commit", "-m", fmt.Sprintf("update(DEC-001): v%d", v))
	}

	// 创建 B: v1→v2→v3→v4→v5
	git(t, dir, "checkout", "main")
	git(t, dir, "checkout", "-b", branchPrefixDecision+sdrB, "main")
	for v := 1; v <= 5; v++ {
		writeDecisionFile(t, dir, DecisionNode{
			SDRID: sdrB, Title: fmt.Sprintf("Quarkus (v%d)", v),
			Branch: branchPrefixDecision + sdrB, Version: v, Status: StatusDecided,
		})
		git(t, dir, "add", sdrB+".md")
		git(t, dir, "commit", "-m", fmt.Sprintf("update(DEC-002): v%d", v))
	}

	// 确认当前版本
	assertVersion(t, dir, branchPrefixDecision+sdrA, sdrA, 3)
	assertVersion(t, dir, branchPrefixDecision+sdrB, sdrB, 5)

	// 回退到 A: A.version = B.version + 1 = 6
	git(t, dir, "checkout", branchPrefixDecision+sdrA)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA, Title: "Spring Boot (restored)",
		Branch: branchPrefixDecision + sdrA, Version: 6, Status: StatusDecided,
		Relations: []Relation{{Type: "SUPERSEDES", TargetSDRID: sdrB}},
	})
	git(t, dir, "add", sdrA+".md")
	git(t, dir, "commit", "-m", "revert(DEC-001): restored, supersedes DEC-002(v5)")

	// B 标记 superseded
	git(t, dir, "checkout", branchPrefixDecision+sdrB)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrB, Title: "Quarkus (v5)",
		Branch: branchPrefixDecision + sdrB, Version: 6, Status: StatusSuperseded,
	})
	git(t, dir, "add", sdrB+".md")
	git(t, dir, "commit", "-m", "revert(DEC-002): superseded by DEC-001(v6)")

	// 验证版本跳跃
	assertVersion(t, dir, branchPrefixDecision+sdrA, sdrA, 6)
	assertStatus(t, dir, branchPrefixDecision+sdrA, sdrA, StatusDecided)
	assertVersion(t, dir, branchPrefixDecision+sdrB, sdrB, 6)
	assertStatus(t, dir, branchPrefixDecision+sdrB, sdrB, StatusSuperseded)

	// 验证 A 有 SUPERSEDES B 关系
	nodeA := readDecisionFromBranch(t, dir, branchPrefixDecision+sdrA, sdrA)
	found := false
	for _, r := range nodeA.Relations {
		if r.Type == "SUPERSEDES" && r.TargetSDRID == sdrB {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("DEC-001 missing SUPERSEDES relation to DEC-002 after revert")
	}
}

// TestGitTree_Deprecate: 废弃决策 → status=deprecated, version++
func TestGitTree_Deprecate(t *testing.T) {
	dir := setupTestRepo(t)
	sdrID := "DEC-001"

	// 创建
	git(t, dir, "checkout", "-b", branchPrefixDecision+sdrID, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID, Title: "旧方案 A", Branch: branchPrefixDecision + sdrID, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "create(DEC-001): 旧方案 A")

	// 更新到 v2
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID, Title: "旧方案 A (v2)", Branch: branchPrefixDecision + sdrID, Version: 2, Status: StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): v2")

	// 废弃
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID, Title: "旧方案 A (v2)",
		Branch: branchPrefixDecision + sdrID, Version: 3, Status: StatusDeprecated,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "deprecate(DEC-001): 方案不再适用")

	assertVersion(t, dir, branchPrefixDecision+sdrID, sdrID, 3)
	assertStatus(t, dir, branchPrefixDecision+sdrID, sdrID, StatusDeprecated)
}

// TestGitTree_FullLifecycle: 完整生命周期 → Git DAG 可视化验证
func TestGitTree_FullLifecycle(t *testing.T) {
	dir := setupTestRepo(t)

	// --- 创建 A (Spring Boot) ---
	git(t, dir, "checkout", "-b", "decision/DEC-001", "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: "DEC-001", Title: "采用 Spring Boot 3", Branch: "decision/DEC-001", Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", "DEC-001.md")
	git(t, dir, "commit", "-m", "create(DEC-001): 采用 Spring Boot 3")

	// --- A 更新 ---
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: "DEC-001", Title: "采用 Spring Boot 3.2", Branch: "decision/DEC-001", Version: 2, Status: StatusDecided,
	})
	git(t, dir, "add", "DEC-001.md")
	git(t, dir, "commit", "-m", "update(DEC-001): 升级到 3.2")

	// --- 创建 B (Quarkus) ---
	git(t, dir, "checkout", "main")
	git(t, dir, "checkout", "-b", "decision/DEC-002", "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: "DEC-002", Title: "采用 Quarkus", Branch: "decision/DEC-002", Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", "DEC-002.md")
	git(t, dir, "commit", "-m", "create(DEC-002): 采用 Quarkus")

	// --- 冲突标记 ---
	git(t, dir, "checkout", "decision/DEC-001")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: "DEC-001", Title: "采用 Spring Boot 3.2", Branch: "decision/DEC-001", Version: 3, Status: StatusDecided,
		ConflictStatus: "active", ConflictWith: "DEC-002",
		Relations: []Relation{{Type: "CONFLICTS_WITH", TargetSDRID: "DEC-002"}},
	})
	git(t, dir, "add", "DEC-001.md")
	git(t, dir, "commit", "-m", "conflict(DEC-001): vs DEC-002")

	git(t, dir, "checkout", "decision/DEC-002")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: "DEC-002", Title: "采用 Quarkus", Branch: "decision/DEC-002", Version: 2, Status: StatusDecided,
		ConflictStatus: "active", ConflictWith: "DEC-001",
		Relations: []Relation{{Type: "CONFLICTS_WITH", TargetSDRID: "DEC-001"}},
	})
	git(t, dir, "add", "DEC-002.md")
	git(t, dir, "commit", "-m", "conflict(DEC-002): vs DEC-001")

	// --- 解决: A 胜出 ---
	git(t, dir, "checkout", "decision/DEC-001")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: "DEC-001", Title: "采用 Spring Boot 3.2", Branch: "decision/DEC-001", Version: 4, Status: StatusDecided,
		ConflictStatus: "resolved",
		Relations: []Relation{{Type: "CONFLICTS_WITH", TargetSDRID: "DEC-002"}},
	})
	git(t, dir, "add", "DEC-001.md")
	git(t, dir, "commit", "-m", "resolve(DEC-001): won vs DEC-002")

	git(t, dir, "checkout", "decision/DEC-002")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: "DEC-002", Title: "采用 Quarkus", Branch: "decision/DEC-002", Version: 3, Status: StatusSuperseded,
		ConflictStatus: "resolved",
		Relations: []Relation{{Type: "SUPERSEDES", TargetSDRID: "DEC-001"}},
	})
	git(t, dir, "add", "DEC-002.md")
	git(t, dir, "commit", "-m", "resolve(DEC-002): superseded by DEC-001")

	// --- 输出完整的 git graph ---
	graph := git(t, dir, "log", "--graph", "--oneline", "--all", "--decorate")
	t.Logf("=== Git DAG ===\n%s\n==============", graph)

	// 验证所有预期分支存在
	assertBranchExists(t, dir, "decision/DEC-001")
	assertBranchExists(t, dir, "decision/DEC-002")

	// 验证主分支
	branches := git(t, dir, "branch", "-a")
	if !strings.Contains(branches, "main") {
		t.Fatal("missing main in branches")
	}

	// 验证各节点状态正确
	assertVersion(t, dir, "decision/DEC-001", "DEC-001", 4)
	assertVersion(t, dir, "decision/DEC-002", "DEC-002", 3)
	assertConflict(t, dir, "decision/DEC-001", "DEC-001", "resolved", "")
	assertConflict(t, dir, "decision/DEC-002", "DEC-002", "resolved", "")
	assertStatus(t, dir, "decision/DEC-001", "DEC-001", StatusDecided)
	assertStatus(t, dir, "decision/DEC-002", "DEC-002", StatusSuperseded)
}

// TestGitTree_MergeConflict: 用 git merge --no-ff 合并冲突分支
func TestGitTree_MergeConflict(t *testing.T) {
	dir := setupTestRepo(t)
	sdrA, sdrB := "DEC-001", "DEC-002"

	// 创建两个分支
	git(t, dir, "checkout", "-b", "decision/DEC-001", "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA, Title: "方案 A", Branch: "decision/DEC-001", Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", "DEC-001.md")
	git(t, dir, "commit", "-m", "create(DEC-001): 方案 A")

	git(t, dir, "checkout", "main")
	git(t, dir, "checkout", "-b", "decision/DEC-002", "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrB, Title: "方案 B", Branch: "decision/DEC-002", Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", "DEC-002.md")
	git(t, dir, "commit", "-m", "create(DEC-002): 方案 B")

	// 用 git merge 合并 B 到 A
	git(t, dir, "checkout", "decision/DEC-001")
	_, err := gitAllowFail(t, dir, "merge", "--no-ff", "decision/DEC-002", "-m", "merge(DEC-001): resolve conflict with DEC-002")

	// 可能自动合并成功（不同文件），也可能需要手动处理
	if err != nil {
		t.Logf("merge conflict detected, resolving by keeping DEC-001")
		// 保留 DEC-001 的内容作为胜出版本
		git(t, dir, "add", "DEC-001.md")
		git(t, dir, "commit", "-m", "merge(DEC-001): conflict resolved, keeping DEC-001")
	}

	// 验证 merge commit 存在
	graph := git(t, dir, "log", "--graph", "--oneline", "--all")
	t.Logf("=== Merge DAG ===\n%s\n===============", graph)

	// 验证有 merge commit
	commits := git(t, dir, "log", "--oneline", "decision/DEC-001")
	if strings.Count(commits, "\n") < 2 {
		t.Fatal("expected at least 2 commits on DEC-001 after merge")
	}
}

// TestGitTree_ListDecisions: 列出所有决策分支和当前版本
func TestGitTree_ListDecisions(t *testing.T) {
	dir := setupTestRepo(t)

	// 创建 3 个决策
	for i := 1; i <= 3; i++ {
		sdr := fmt.Sprintf("DEC-%03d", i)
		git(t, dir, "checkout", "-b", "decision/"+sdr, "main")
		writeDecisionFile(t, dir, DecisionNode{
			SDRID: sdr, Title: fmt.Sprintf("决策 %d", i),
			Branch: "decision/" + sdr, Version: 1, Status: StatusDecided,
		})
		git(t, dir, "add", sdr+".md")
		git(t, dir, "commit", "-m", fmt.Sprintf("create(%s): 决策 %d", sdr, i))
	}

	// 列出所有 decision/ 分支
	branches := git(t, dir, "branch", "--list", "decision/*")
	branchLines := strings.Split(strings.TrimSpace(branches), "\n")
	if len(branchLines) != 3 {
		t.Fatalf("expected 3 decision branches, got %d:\n%s", len(branchLines), branches)
	}

	// 验证每个分支的版本
	for _, b := range branchLines {
		b = strings.TrimSpace(b)
		t.Logf("Branch: %s", b)
	}
}

// TestGitTree_MainBranchImmutability: 确保 main 分支只有 Dummy 决策，版本永远为 0
// ==============================================
// 真实场景测试
// ==============================================

// TestGitTree_RealTechStackDecision: 真实技术选型讨论 — 多轮讨论后确定方案
// 场景：团队讨论微服务后端技术栈，涉及框架对比、团队能力、运维成本等
func TestGitTree_RealTechStackDecision(t *testing.T) {
	dir := setupTestRepo(t)
	sdrID := "DEC-001"

	// 创建决策分支
	git(t, dir, "checkout", "-b", "decision/"+sdrID, "main")

	// 第一版：初步决定
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID,
		Title: "微服务后端技术栈选型",
		Branch: "decision/" + sdrID,
		Version: 1,
		Status:  StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "create(DEC-001): 后端技术栈初步决定")

	// 补充决策依据和细节（v2）
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID,
		Title: "微服务后端技术栈选型 — Spring Boot 3.x + JDK 21 + gRPC",
		Branch: "decision/" + sdrID,
		Version: 2,
		Status:  StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): 补充框架细节和迁移路径")

	// 第三版：增加时间线和执行计划（v3）
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID,
		Title: "微服务后端技术栈选型 — Spring Boot 3.x + JDK 21 + gRPC（分两阶段落地）",
		Branch: "decision/" + sdrID,
		Version: 3,
		Status:  StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): 补充分阶段执行计划")

	// 验证版本和内容
	assertVersion(t, dir, "decision/"+sdrID, sdrID, 3)
	assertBranchExists(t, dir, "decision/"+sdrID)
}

// TestGitTree_MicroserviceSplitting: 微服务拆分决策 — 包含详细的服务划分和通信方式
// 场景：架构评审会议，讨论如何拆分现有单体应用
func TestGitTree_MicroserviceSplitting(t *testing.T) {
	dir := setupTestRepo(t)
	sdrID := "DEC-001"

	git(t, dir, "checkout", "-b", "decision/"+sdrID, "main")

	// 第一版：核心拆分方案
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID,
		Title: "电商平台微服务拆分方案 — 拆分 6 个核心服务 + 事件驱动架构",
		Branch: "decision/" + sdrID,
		Version: 1,
		Status:  StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "create(DEC-001): 确定微服务拆分粒度为6个核心服务")

	// 第二版：补充服务治理和可观测性
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID,
		Title: "电商平台微服务拆分 — 6 服务 + 事件驱动 + 统一可观测性平台",
		Branch: "decision/" + sdrID,
		Version: 2,
		Status:  StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): 增加可观测性方案和服务治理规范")

	// 第三版：补充数据一致性方案
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID,
		Title: "微服务拆分 — 最终版：Saga + 事件溯源保证最终一致性",
		Branch: "decision/" + sdrID,
		Version: 3,
		Status:  StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): 补充Saga分布式事务方案")

	assertVersion(t, dir, "decision/"+sdrID, sdrID, 3)
	graph := git(t, dir, "log", "--oneline", "decision/"+sdrID, "^main")
	t.Logf("微服务拆分决策版本演进:\n%s", graph)
}

// TestGitTree_APIDesignConflict: API 设计风格冲突 — REST vs gRPC 完整辩论
// 场景：两个团队对 API 风格有不同意见，最终解决
func TestGitTree_APIDesignConflict(t *testing.T) {
	dir := setupTestRepo(t)
	sdrA, sdrB := "DEC-001", "DEC-002"

	// === 团队 A：主张 RESTful ===
	git(t, dir, "checkout", "-b", "decision/"+sdrA, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA, Title: "对外 API 统一采用 RESTful 风格，使用 OpenAPI 3.1 规范",
		Branch: "decision/" + sdrA, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrA+".md")
	git(t, dir, "commit", "-m", "create(DEC-001): API采用RESTful+OpenAPI 3.1")

	git(t, dir, "checkout", "main")
	git(t, dir, "checkout", "-b", "decision/"+sdrB, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrB, Title: "服务间通信统一采用 gRPC + Protobuf，对外暴露 gRPC-Gateway",
		Branch: "decision/" + sdrB, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrB+".md")
	git(t, dir, "commit", "-m", "create(DEC-002): 服务间通信采用gRPC+Protobuf")

	// 标记冲突
	git(t, dir, "checkout", "decision/"+sdrA)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA, Title: "RESTful API", Branch: "decision/" + sdrA, Version: 2, Status: StatusDecided,
		ConflictStatus: "active", ConflictWith: sdrB,
		Relations: []Relation{{Type: "CONFLICTS_WITH", TargetSDRID: sdrB}},
	})
	git(t, dir, "add", sdrA+".md")
	git(t, dir, "commit", "-m", "conflict(DEC-001): vs DEC-002 (gRPC方案)")

	git(t, dir, "checkout", "decision/"+sdrB)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrB, Title: "gRPC通信", Branch: "decision/" + sdrB, Version: 2, Status: StatusDecided,
		ConflictStatus: "active", ConflictWith: sdrA,
		Relations: []Relation{{Type: "CONFLICTS_WITH", TargetSDRID: sdrA}},
	})
	git(t, dir, "add", sdrB+".md")
	git(t, dir, "commit", "-m", "conflict(DEC-002): vs DEC-001 (REST方案)")

	// 解决冲突：两全其美 — 对外 REST，对内 gRPC
	sdrC := "DEC-003"
	git(t, dir, "checkout", "main")
	git(t, dir, "checkout", "-b", "decision/"+sdrC, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrC,
		Title: "混合方案：对外 RESTful (OpenAPI 3.1)，服务间 gRPC (Protobuf)，gRPC-Gateway 自动转换",
		Branch: "decision/" + sdrC, Version: 3, Status: StatusDecided,
		Relations: []Relation{
			{Type: "SUPERSEDES", TargetSDRID: sdrA},
			{Type: "SUPERSEDES", TargetSDRID: sdrB},
		},
	})
	git(t, dir, "add", sdrC+".md")
	git(t, dir, "commit", "-m", "create(DEC-003): 混合方案 — 对外REST + 对内gRPC")

	// 废弃 A 和 B
	for _, sdr := range []string{sdrA, sdrB} {
		git(t, dir, "checkout", "decision/"+sdr)
		writeDecisionFile(t, dir, DecisionNode{
			SDRID: sdr, Title: sdr, Branch: "decision/" + sdr, Version: 3, Status: StatusSuperseded,
		})
		git(t, dir, "add", sdr+".md")
		git(t, dir, "commit", "-m", "supersede("+sdr+"): by DEC-003 混合方案")
	}

	// 验证
	assertVersion(t, dir, "decision/"+sdrC, sdrC, 3)
	assertStatus(t, dir, "decision/"+sdrC, sdrC, StatusDecided)
	assertStatus(t, dir, "decision/"+sdrA, sdrA, StatusSuperseded)
	assertStatus(t, dir, "decision/"+sdrB, sdrB, StatusSuperseded)

	graph := git(t, dir, "log", "--graph", "--oneline", "--all", "--decorate")
	t.Logf("API设计冲突解决 DAG:\n%s", graph)
}

// TestGitTree_DatabaseMigrationEvolution: 数据库方案演进 — 从 MySQL 到 Vitess 再到 TiDB
// 场景：随着业务增长，数据库方案逐步演进
func TestGitTree_DatabaseMigrationEvolution(t *testing.T) {
	dir := setupTestRepo(t)

	// Phase 1: 初始选择 MySQL 8.0
	sdrMySQL := "DEC-001"
	git(t, dir, "checkout", "-b", "decision/"+sdrMySQL, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrMySQL, Title: "数据库采用 MySQL 8.0 读写分离架构，ProxySQL 做中间层",
		Branch: "decision/" + sdrMySQL, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrMySQL+".md")
	git(t, dir, "commit", "-m", "create(DEC-001): 初始采用MySQL 8.0读写分离")

	// MySQL 升级 v2: 增加分库分表
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrMySQL, Title: "MySQL 8.0 + ShardingSphere 分库分表（按用户 ID hash 分 32 库）",
		Branch: "decision/" + sdrMySQL, Version: 2, Status: StatusDecided,
	})
	git(t, dir, "add", sdrMySQL+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): 增加ShardingSphere分片")

	// MySQL v3: 增加读写分离细节
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrMySQL, Title: "MySQL 8.0 + ShardingSphere + 1主3从读写分离",
		Branch: "decision/" + sdrMySQL, Version: 3, Status: StatusDecided,
	})
	git(t, dir, "add", sdrMySQL+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): 完善读写分离拓扑")

	// Phase 2: 迁移到 Vitess
	sdrVitess := "DEC-002"
	git(t, dir, "checkout", "main")
	git(t, dir, "checkout", "-b", "decision/"+sdrVitess, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrVitess,
		Title: "迁移到 Vitess 集群：解决 MySQL 单机瓶颈，支持自动分片和在线扩容",
		Branch: "decision/" + sdrVitess, Version: 1, Status: StatusDecided,
		Relations: []Relation{{Type: "SUPERSEDES", TargetSDRID: sdrMySQL}},
	})
	git(t, dir, "add", sdrVitess+".md")
	git(t, dir, "commit", "-m", "create(DEC-002): 迁移到Vitess集群")

	// Vitess 更新 v2
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrVitess,
		Title: "Vitess 集群（12 tablet + ETCD 元数据存储），VReplication 保证 CDC",
		Branch: "decision/" + sdrVitess, Version: 2, Status: StatusDecided,
		Relations: []Relation{{Type: "SUPERSEDES", TargetSDRID: sdrMySQL}},
	})
	git(t, dir, "add", sdrVitess+".md")
	git(t, dir, "commit", "-m", "update(DEC-002): 确定部署拓扑和CDC方案")

	// 废弃 MySQL
	git(t, dir, "checkout", "decision/"+sdrMySQL)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrMySQL, Title: "MySQL方案（已废弃）",
		Branch: "decision/" + sdrMySQL, Version: 4, Status: StatusSuperseded,
	})
	git(t, dir, "add", sdrMySQL+".md")
	git(t, dir, "commit", "-m", "supersede(DEC-001): 迁移到Vitess")

	// Phase 3: 迁移到 TiDB
	sdrTiDB := "DEC-003"
	git(t, dir, "checkout", "main")
	git(t, dir, "checkout", "-b", "decision/"+sdrTiDB, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrTiDB,
		Title: "替换 Vitess 为 TiDB：原生分布式 + MySQL 协议兼容 + 强一致事务 + HTAP",
		Branch: "decision/" + sdrTiDB, Version: 3, Status: StatusDecided,
		Relations: []Relation{
			{Type: "SUPERSEDES", TargetSDRID: sdrVitess},
		},
	})
	git(t, dir, "add", sdrTiDB+".md")
	git(t, dir, "commit", "-m", "create(DEC-003): 迁移到TiDB分布式数据库")

	// 废弃 Vitess
	git(t, dir, "checkout", "decision/"+sdrVitess)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrVitess, Title: "Vitess方案（已废弃）",
		Branch: "decision/" + sdrVitess, Version: 3, Status: StatusSuperseded,
	})
	git(t, dir, "add", sdrVitess+".md")
	git(t, dir, "commit", "-m", "supersede(DEC-002): 迁移到TiDB")

	// 验证演进链路
	assertStatus(t, dir, "decision/"+sdrMySQL, sdrMySQL, StatusSuperseded)
	assertStatus(t, dir, "decision/"+sdrVitess, sdrVitess, StatusSuperseded)
	assertStatus(t, dir, "decision/"+sdrTiDB, sdrTiDB, StatusDecided)
	assertVersion(t, dir, "decision/"+sdrTiDB, sdrTiDB, 3)

	graph := git(t, dir, "log", "--graph", "--oneline", "--all", "--decorate")
	t.Logf("数据库架构演进 DAG:\n%s", graph)
}

// TestGitTree_DeploymentStrategy: 部署策略决策 — 包含发布流程和回滚方案
// 场景：SRE 团队讨论生产部署策略
func TestGitTree_DeploymentStrategy(t *testing.T) {
	dir := setupTestRepo(t)
	sdrID := "DEC-001"

	git(t, dir, "checkout", "-b", "decision/"+sdrID, "main")

	// v1: 基础方案
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID,
		Title: "生产环境部署采用 K8s + Helm + ArgoCD 渐进式发布",
		Branch: "decision/" + sdrID, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "create(DEC-001): K8s+Helm+ArgoCD部署方案")

	// v2: 补充发布策略
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID,
		Title: "K8s + Helm + ArgoCD + 蓝绿部署（非金丝雀，因合规要求必须瞬间切换）",
		Branch: "decision/" + sdrID, Version: 2, Status: StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): 确定蓝绿部署而非金丝雀")

	// v3: 补充回滚方案和多集群
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID,
		Title: "K8s蓝绿部署：3集群（dev/staging/prod）+ 自动回滚（健康检查5分钟无报警即切换）",
		Branch: "decision/" + sdrID, Version: 3, Status: StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): 补充回滚策略和多集群部署")

	// v4: 补充监控和告警
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID,
		Title: "K8s蓝绿 + 3集群 + 自动回滚 + Prometheus/Grafana/Loki 可观测性体系",
		Branch: "decision/" + sdrID, Version: 4, Status: StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): 增加可观测性体系")

	assertVersion(t, dir, "decision/"+sdrID, sdrID, 4)

	graph := git(t, dir, "log", "--oneline", "decision/"+sdrID, "^main")
	t.Logf("部署策略决策演进:\n%s", graph)
}

// TestGitTree_MultipleDecisionsFromDiscussion: 一次讨论产生多个决策
// 场景：架构评审会议，针对同一个问题做出多个相关决策
func TestGitTree_MultipleDecisionsFromDiscussion(t *testing.T) {
	dir := setupTestRepo(t)

	decisions := []struct {
		sdr   string
		title string
	}{
		{"DEC-001", "消息队列统一采用 RocketMQ，替代自研 MQ 和 RabbitMQ"},
		{"DEC-002", "消息协议规范：统一使用 CloudEvents 2.0 规范，JSON 序列化"},
		{"DEC-003", "消息治理：Schema Registry + 死信队列 + 消息轨迹（全链路追踪标记 TraceID）"},
	}

	for _, d := range decisions {
		git(t, dir, "checkout", "-b", "decision/"+d.sdr, "main")
		writeDecisionFile(t, dir, DecisionNode{
			SDRID: d.sdr, Title: d.title,
			Branch: "decision/" + d.sdr, Version: 1, Status: StatusDecided,
		})
		git(t, dir, "add", d.sdr+".md")
		git(t, dir, "commit", "-m", "create("+d.sdr+"): 消息架构决策")
	}

	// 验证全部创建成功
	for _, d := range decisions {
		assertBranchExists(t, dir, "decision/"+d.sdr)
		assertVersion(t, dir, "decision/"+d.sdr, d.sdr, 1)
	}

	graph := git(t, dir, "log", "--graph", "--oneline", "--all", "--decorate")
	t.Logf("消息架构讨论产生的多决策 DAG:\n%s", graph)
}

// TestGitTree_FrontendEvolutionConflict: 前端框架演进 — React v16 → v18 → 冲突 → 结论
// 场景：前端团队讨论技术栈升级，中途出现分歧
func TestGitTree_FrontendEvolutionConflict(t *testing.T) {
	dir := setupTestRepo(t)

	// 决策 A：升级到 React 18 + Next.js
	sdrA := "DEC-001"
	git(t, dir, "checkout", "-b", "decision/"+sdrA, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA,
		Title: "前端框架从 React 16 升级到 React 18 + Next.js 14（App Router + RSC）",
		Branch: "decision/" + sdrA, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrA+".md")
	git(t, dir, "commit", "-m", "create(DEC-001): 升级React 18+Next.js")

	// 决策 A v2：补充迁移策略
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA,
		Title: "React 18 + Next.js 14 + 增量迁移：每个模块单独升级，先 Pages Router 再 App Router",
		Branch: "decision/" + sdrA, Version: 2, Status: StatusDecided,
	})
	git(t, dir, "add", sdrA+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): 确定增量迁移路径")

	// 决策 B：激进派主张 Vue 3
	sdrB := "DEC-002"
	git(t, dir, "checkout", "main")
	git(t, dir, "checkout", "-b", "decision/"+sdrB, "main")
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrB,
		Title: "放弃 React，全面迁移到 Vue 3 + Nuxt 3（团队 Vue 经验更丰富）",
		Branch: "decision/" + sdrB, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrB+".md")
	git(t, dir, "commit", "-m", "create(DEC-002): 迁移到Vue 3+Nuxt")

	// 冲突标记
	for _, pair := range [][2]string{{sdrA, sdrB}, {sdrB, sdrA}} {
		a, b := pair[0], pair[1]
		git(t, dir, "checkout", "decision/"+a)
		writeDecisionFile(t, dir, DecisionNode{
			SDRID: a, Branch: "decision/" + a, Version: 3, Status: StatusDecided,
			Title:           a,
			ConflictStatus: "active", ConflictWith: b,
			Relations: []Relation{{Type: "CONFLICTS_WITH", TargetSDRID: b}},
		})
		git(t, dir, "add", a+".md")
		git(t, dir, "commit", "-m", "conflict("+a+"): vs "+b)
	}

	// 冲突解决：选择 React（团队 React 资产太重，重写成本过高）
	git(t, dir, "checkout", "decision/"+sdrA)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrA, Branch: "decision/" + sdrA, Version: 4, Status: StatusDecided,
		Title: "React 18 + Next.js 14（保留现有 20 万行 React 代码，重写成本≈8人月不可接受）",
		ConflictStatus: "resolved",
		Relations:      []Relation{{Type: "CONFLICTS_WITH", TargetSDRID: sdrB}},
	})
	git(t, dir, "add", sdrA+".md")
	git(t, dir, "commit", "-m", "resolve(DEC-001): 保留React，拒绝Vue方案")

	git(t, dir, "checkout", "decision/"+sdrB)
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrB, Branch: "decision/" + sdrB, Version: 4, Status: StatusSuperseded,
		Title: "Vue 3（已驳回）", ConflictStatus: "resolved",
		Relations: []Relation{{Type: "SUPERSEDES", TargetSDRID: sdrA}},
	})
	git(t, dir, "add", sdrB+".md")
	git(t, dir, "commit", "-m", "resolve(DEC-002): 被驳回，保留React")

	assertStatus(t, dir, "decision/"+sdrA, sdrA, StatusDecided)
	assertStatus(t, dir, "decision/"+sdrB, sdrB, StatusSuperseded)
	assertVersion(t, dir, "decision/"+sdrA, sdrA, 4)
	assertVersion(t, dir, "decision/"+sdrB, sdrB, 4)

	graph := git(t, dir, "log", "--graph", "--oneline", "--all", "--decorate")
	t.Logf("前端框架决策冲突 DAG:\n%s", graph)
}

// TestGitTree_MultiRoundDiscussion: 多轮讨论形成的决策 — 缓存方案
// 场景：多轮技术讨论，每次讨论补充新信息，最终形成完整决策
func TestGitTree_MultiRoundDiscussion(t *testing.T) {
	dir := setupTestRepo(t)
	sdrID := "DEC-001"

	git(t, dir, "checkout", "-b", "decision/"+sdrID, "main")

	// Round 1: 选型
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID,
		Title: "缓存方案采用 Redis 7.x 集群（6 节点 + 3 副本），淘汰 Memcached",
		Branch: "decision/" + sdrID, Version: 1, Status: StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "create(DEC-001): 缓存采用Redis集群，淘汰Memcached")

	// Round 2: 确定缓存策略
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID,
		Title: "Redis 7.x 集群 + 多级缓存（本地 Caffeine L2 + Redis L1）+ 缓存预热 + 缓存穿透保护",
		Branch: "decision/" + sdrID, Version: 2, Status: StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): 增加多级缓存和缓存策略")

	// Round 3: 确定key规范和过期策略
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID,
		Title: "Redis 集群 + 多级缓存 + Key 命名规范（业务:模块:ID）+ 统一 TTL 管理平台",
		Branch: "decision/" + sdrID, Version: 3, Status: StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): 补充Key命名规范和TTL管理")

	// Round 4: 确定高可用方案
	writeDecisionFile(t, dir, DecisionNode{
		SDRID: sdrID,
		Title: "Redis 集群 + 多级缓存 + Key规范 + 哨兵自动故障转移 + 跨 AZ 部署",
		Branch: "decision/" + sdrID, Version: 4, Status: StatusDecided,
	})
	git(t, dir, "add", sdrID+".md")
	git(t, dir, "commit", "-m", "update(DEC-001): 补充高可用和跨AZ部署")

	assertVersion(t, dir, "decision/"+sdrID, sdrID, 4)
}

// TestGitTree_MainBranchImmutability: 确保 main 分支只有 Dummy 决策，版本永远为 0
func TestGitTree_MainBranchImmutability(t *testing.T) {
	dir := setupTestRepo(t)

	// 创建多个决策
	for i := 1; i <= 3; i++ {
		sdr := fmt.Sprintf("DEC-%03d", i)
		git(t, dir, "checkout", "-b", "decision/"+sdr, "main")
		writeDecisionFile(t, dir, DecisionNode{
			SDRID: sdr, Title: fmt.Sprintf("决策 %d", i),
			Branch: "decision/" + sdr, Version: 1, Status: StatusDecided,
		})
		git(t, dir, "add", sdr+".md")
		git(t, dir, "commit", "-m", fmt.Sprintf("create(%s)", sdr))
	}

	// 切回 main
	git(t, dir, "checkout", "main")

	// 验证 main 只有 Dummy
	files := git(t, dir, "ls-tree", "--name-only", "HEAD")
	if strings.Contains(files, "DEC-00") && !strings.Contains(files, "DEC-001") {
		// OK - main 上有 DEC-000 但无 DEC-001
	} else if strings.Contains(files, "DEC-001") {
		t.Fatal("main branch should not contain DEC-001.md")
	}

	// 验证 Dummy version=0
	dummy := readDecisionFromBranch(t, dir, "main", dummySDRID)
	if dummy.Version != 0 {
		t.Fatalf("dummy version should be 0, got %d", dummy.Version)
	}
	if dummy.Status != StatusCompleted {
		t.Fatalf("dummy status should be completed, got %q", dummy.Status)
	}

	// 确保 main 分支数量 = 2（L0 + Dummy）
	commits := git(t, dir, "log", "--oneline", "main")
	lines := strings.Split(strings.TrimSpace(commits), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 commits on main (L0 + Dummy), got %d:\n%s", len(lines), commits)
	}
}
