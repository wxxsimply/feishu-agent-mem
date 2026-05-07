package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"feishu-mem/internal/core"
	"feishu-mem/internal/decision"
)

// Config Git 存储配置
type Config struct {
	WorkDir  string
	Remote   string
	AutoPush bool
	Branch   string
}

// BlameEntry blame 条目
type BlameEntry struct {
	Commit  string
	LineNum int
	Content string
	Author  string
	Date    time.Time
}

// SearchHit 搜索命中条目
type SearchHit struct {
	File    string
	LineNum int
	Content string
}

// SyncDrift 同步漂移记录
type SyncDrift struct {
	SDRID       string
	BitableHash string
	GitHash     string
	DetectedAt  time.Time
	NeedsFix    bool
}

// GitStorage Git 存储
type GitStorage struct {
	workDir string
	remote  string
	cli     *GitCLI
	config  Config
}

// NewGitStorage 创建 Git 存储
func NewGitStorage(config Config) (*GitStorage, error) {
	if config.WorkDir == "" {
		config.WorkDir = "data"
	}
	if config.Branch == "" {
		config.Branch = "main"
	}

	gs := &GitStorage{
		workDir: config.WorkDir,
		remote:  config.Remote,
		config:  config,
		cli:     NewGitCLI(config.WorkDir),
	}

	// 初始化仓库
	if err := gs.initRepo(); err != nil {
		return nil, err
	}

	return gs, nil
}

func (gs *GitStorage) initRepo() error {
	// 确保工作目录存在
	if err := os.MkdirAll(gs.workDir, 0755); err != nil {
		return err
	}

	// 检查是否已经是 git 仓库
	gitDir := filepath.Join(gs.workDir, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return nil // 已经是 git 仓库
	}

	// 初始化 git 仓库
	if _, err := gs.cli.Run("init"); err != nil {
		return err
	}

	// 设置 git 配置
	if _, err := gs.cli.Run("config", "user.name", "feishu-agent-mem"); err != nil {
		return err
	}
	if _, err := gs.cli.Run("config", "user.email", "feishu-agent-mem@example.com"); err != nil {
		return err
	}

	// 创建初始 L0_RULES.md
	rulesPath := filepath.Join(gs.workDir, "L0_RULES.md")
	if _, err := os.Stat(rulesPath); os.IsNotExist(err) {
		_ = os.WriteFile(rulesPath, []byte("# L0 Rules\n\n"), 0644)
		_, _ = gs.cli.Run("add", "L0_RULES.md")
		_, _ = gs.cli.Run("commit", "-m", "Initial commit: L0 rules")
	}

	// 创建 Dummy 决策（仅首次）
	dummyPath := filepath.Join(gs.workDir, decision.DummySDRID+".md")
	if _, err := os.Stat(dummyPath); os.IsNotExist(err) {
		dummy := decision.NewDummyDecision()
		content := RenderDecisionFile(dummy)
		_ = os.WriteFile(dummyPath, []byte(content), 0644)
		_, _ = gs.cli.Run("add", decision.DummySDRID+".md")
		_, _ = gs.cli.Run("commit", "-m", "dummy: Root Dummy (v0)")
	}

	return nil
}

// WriteDecision 写入决策文件 + commit（在决策自身分支上操作）
func (gs *GitStorage) WriteDecision(node *decision.DecisionNode) (string, error) {
	// 确保决策分支存在并切换过去
	branch := node.GetDecisionBranch()
	if err := gs.EnsureDecisionBranch(branch); err != nil {
		return "", fmt.Errorf("ensure branch %s failed: %w", branch, err)
	}

	// 切换（确保在正确的分支上）
	currentBranch, _ := gs.GetCurrentBranch()
	if currentBranch != branch {
		if err := gs.SwitchBranch(branch); err != nil {
			return "", fmt.Errorf("switch to branch %s failed: %w", branch, err)
		}
	}

	// 构建文件路径
	dir := filepath.Join(gs.workDir, "decisions", node.Project, node.Topic)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	path := filepath.Join(dir, node.SDRID+".md")

	// 版本号 = 分支上实际 commit 数（从 git 推导，确保与 git 一致）
	countStr, err := gs.cli.Run("rev-list", "--count", branch, "^main")
	if err == nil {
		if count, e := strconv.Atoi(countStr); e == nil && count > 0 {
			node.Version = count
		}
	}

	// 渲染内容
	content := RenderDecisionFile(node)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}

	// git add + commit
	relPath, _ := filepath.Rel(gs.workDir, path)
	msg := gs.formatCommitMessage("decision", node)
	hash, err := gs.cli.Commit(relPath, msg)
	if err != nil {
		return "", err
	}

	node.GitCommitHash = hash

	// 留在决策分支上（后续 ReadDecision 操作将自动读取正确分支）

	// auto push
	if gs.config.AutoPush && gs.remote != "" {
		_ = gs.Push()
	}

	return hash, nil
}

// ReadDecision 读取决策文件
func (gs *GitStorage) ReadDecision(project, topic, sdrID string) (*decision.DecisionNode, error) {
	path := filepath.Join(gs.workDir, "decisions", project, topic, sdrID+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return ParseDecisionFile(data)
}

// ListDecisions 列出议题下所有决策
func (gs *GitStorage) ListDecisions(project, topic string) ([]*decision.DecisionNode, error) {
	dir := filepath.Join(gs.workDir, "decisions", project, topic)
	files, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*decision.DecisionNode{}, nil
		}
		return nil, err
	}

	var decisions []*decision.DecisionNode
	for _, f := range files {
		name := f.Name()
		if strings.HasSuffix(name, ".md") {
			sdrID := strings.TrimSuffix(name, ".md")
			if d, err := gs.ReadDecision(project, topic, sdrID); err == nil {
				decisions = append(decisions, d)
			}
		}
	}

	return decisions, nil
}

// WriteObjection 写入反对意见文件 + commit
func (gs *GitStorage) WriteObjection(obj *decision.Objection) (string, error) {
	dir := filepath.Join(gs.workDir, "objections", obj.Project, obj.Topic)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	path := filepath.Join(dir, obj.OID+".md")

	content := RenderObjectionFile(obj)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}

	relPath, _ := filepath.Rel(gs.workDir, path)
	msg := fmt.Sprintf("objection(%s): %s - %s", obj.Topic, obj.OID, obj.ObjectionContent)
	hash, err := gs.cli.Commit(relPath, msg)
	if err != nil {
		return "", err
	}

	return hash, nil
}

// ListObjections 列出反对意见列表
func (gs *GitStorage) ListObjections(project, topic string) ([]*decision.Objection, error) {
	if topic == "" {
		topic = "general"
	}
	dir := filepath.Join(gs.workDir, "objections", project, topic)
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return []*decision.Objection{}, nil
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var objections []*decision.Objection
	for _, f := range files {
		name := f.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		obj, err := ParseObjectionFile(data)
		if err != nil {
			continue
		}
		objections = append(objections, obj)
	}
	return objections, nil
}

// ListTopics 列出项目下所有议题
func (gs *GitStorage) ListTopics(project string) ([]string, error) {
	dir := filepath.Join(gs.workDir, "decisions", project)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var topics []string
	for _, e := range entries {
		if e.IsDir() {
			topics = append(topics, e.Name())
		}
	}

	return topics, nil
}

// GetHeadHash 获取当前 HEAD 的 hash
func (gs *GitStorage) GetHeadHash() (string, error) {
	return gs.cli.GetHeadHash()
}

// GetFileHash 获取指定文件的当前 hash
func (gs *GitStorage) GetFileHash(project, topic, sdrID string) (string, error) {
	path := filepath.Join("decisions", project, topic, sdrID+".md")
	return gs.cli.Run("rev-parse", "HEAD:"+path)
}

// Push 推送到远程
func (gs *GitStorage) Push() error {
	if gs.remote == "" {
		return nil
	}
	_, err := gs.cli.Run("push", "-u", "origin", gs.config.Branch)
	return err
}

// Pull 拉取远程最新
func (gs *GitStorage) Pull() error {
	_, err := gs.cli.Run("pull", "origin", gs.config.Branch)
	return err
}

// CreateBranch 创建分支
func (gs *GitStorage) CreateBranch(branchName string) error {
	return gs.cli.CreateBranch(branchName)
}

// SwitchBranch 切换分支
func (gs *GitStorage) SwitchBranch(branchName string) error {
	return gs.cli.SwitchBranch(branchName)
}

// MergeBranch 合并分支
func (gs *GitStorage) MergeBranch(branchName string) error {
	return gs.cli.MergeBranch(branchName)
}

// GetCurrentBranch 获取当前分支
func (gs *GitStorage) GetCurrentBranch() (string, error) {
	return gs.cli.GetCurrentBranch()
}

// ListBranches 列出所有分支
func (gs *GitStorage) ListBranches() ([]string, error) {
	return gs.cli.ListBranches()
}

// EnsureDecisionBranch 确保决策分支存在（从 main 创建）
func (gs *GitStorage) EnsureDecisionBranch(branchName string) error {
	// 检查分支是否已存在
	branches, err := gs.ListBranches()
	if err != nil {
		return err
	}
	for _, b := range branches {
		if b == branchName {
			return nil // 已存在
		}
	}
	// 从 main 创建新分支
	return gs.cli.CreateBranchFrom(branchName, "main")
}

// ListDecisionBranches 列出所有 decision/ 前缀的分支
func (gs *GitStorage) ListDecisionBranches() ([]string, error) {
	branches, err := gs.ListBranches()
	if err != nil {
		return nil, err
	}
	var decisionBranches []string
	for _, b := range branches {
		if strings.HasPrefix(b, decision.BranchPrefixDecision) {
			decisionBranches = append(decisionBranches, b)
		}
	}
	return decisionBranches, nil
}

// ReadDecisionFromBranch 从指定分支读取决策文件
func (gs *GitStorage) ReadDecisionFromBranch(branch, sdrID string) (*decision.DecisionNode, error) {
	// 用 git ls-tree 找到分支上的决策文件路径
	output, err := gs.cli.Run("ls-tree", "-r", "--name-only", branch)
	if err != nil {
		return nil, fmt.Errorf("ls-tree on %s failed: %w", branch, err)
	}
	for _, line := range strings.Split(output, "\n") {
		line = strings.Trim(line, "\" \t\r\n")
		if strings.HasSuffix(line, "/"+sdrID+".md") {
			content, err := gs.cli.ReadFileAtCommit(line, branch)
			if err != nil {
				return nil, fmt.Errorf("read %s at %s failed: %w", line, branch, err)
			}
			return ParseDecisionFile([]byte(content))
		}
	}
	return nil, fmt.Errorf("decision %s not found on branch %s", sdrID, branch)
}

// GetCommitLog 获取提交历史
func (gs *GitStorage) GetCommitLog(path string, limit int) ([]CommitLogEntry, error) {
	return gs.cli.GetCommitLog(path, limit)
}

// BlameDecision 逐行追溯
func (gs *GitStorage) BlameDecision(project, topic, sdrID string) ([]BlameEntry, error) {
	path := filepath.Join("decisions", project, topic, sdrID+".md")
	return gs.cli.GitBlame(path)
}

// SearchContent Git Grep 全文搜索
func (gs *GitStorage) SearchContent(project, query string) ([]SearchHit, error) {
	var path string
	if project != "" {
		path = filepath.Join("decisions", project)
	} else {
		path = "decisions"
	}
	return gs.cli.GitGrep(query, path)
}

// ReadDecisionAtCommit 读取指定提交时的决策
func (gs *GitStorage) ReadDecisionAtCommit(project, topic, sdrID, commitHash string) (*decision.DecisionNode, error) {
	path := filepath.Join("decisions", project, topic, sdrID+".md")
	content, err := gs.cli.ReadFileAtCommit(path, commitHash)
	if err != nil {
		return nil, fmt.Errorf("read at commit %s failed: %w", commitHash, err)
	}
	return ParseDecisionFile([]byte(content))
}

// GetDecisionHistory 获取决策的提交历史
func (gs *GitStorage) GetDecisionHistory(project, topic, sdrID string) ([]core.CommitLogEntry, error) {
	path := filepath.Join("decisions", project, topic, sdrID+".md")
	logs, err := gs.cli.GetCommitLog(path, 0)
	if err != nil {
		return nil, err
	}
	var result []core.CommitLogEntry
	for _, l := range logs {
		result = append(result, core.CommitLogEntry{
			Hash:    l.Hash,
			Message: l.Message,
		})
	}
	return result, nil
}

// ArchiveProject 归档项目
func (gs *GitStorage) ArchiveProject(project string) error {
	src := filepath.Join("decisions", project)
	dst := filepath.Join("archive", project)

	// 确保 archive 目录存在
	if err := os.MkdirAll(filepath.Join(gs.workDir, "archive"), 0755); err != nil {
		return err
	}

	// 使用 git mv 移动
	srcPath := filepath.Join(gs.workDir, src)
	if _, err := os.Stat(srcPath); err == nil {
		if err := gs.cli.MoveWithGit(src, dst); err != nil {
			return err
		}
		_, err := gs.cli.Run("commit", "-m", fmt.Sprintf("archive(%s): 项目 %s 已归档", project, project))
		return err
	}
	return nil
}

// CheckConsistency 检查 Git 与 Bitable 的一致性
func (gs *GitStorage) CheckConsistency(bitableRecords map[string]string) ([]SyncDrift, error) {
	var drifts []SyncDrift

	// 遍历所有 Bitable 记录
	for sdrID, bitableHash := range bitableRecords {
		// 假设我们能从 SDRID 反推出 project 和 topic（这里简化处理）
		gitHash, _ := gs.GetFileHash("test", "sync", sdrID)

		if gitHash != bitableHash {
			drifts = append(drifts, SyncDrift{
				SDRID:       sdrID,
				BitableHash: bitableHash,
				GitHash:     gitHash,
				DetectedAt:  time.Now(),
				NeedsFix:    true,
			})
		}
	}

	return drifts, nil
}

func (gs *GitStorage) formatCommitMessage(typ string, node *decision.DecisionNode) string {
	return fmt.Sprintf("%s(%s): %s - %s\n\nDecision: %s\nRationale: %s\nImpact: %s\nProposer: %s",
		typ,
		node.Topic,
		node.SDRID,
		node.Title,
		truncate(node.Decision, 100),
		truncate(node.Rationale, 100),
		strings.Join(node.CrossTopicRefs, ", "),
		node.Proposer,
	)
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
