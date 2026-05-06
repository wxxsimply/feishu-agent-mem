package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CommitLogEntry 提交日志条目
type CommitLogEntry struct {
	Hash    string
	Message string
}

// GitCLI Git 命令执行封装
type GitCLI struct {
	workDir string
}

// NewGitCLI 创建 Git CLI
func NewGitCLI(workDir string) *GitCLI {
	return &GitCLI{workDir: workDir}
}

// Run 执行 git 命令
func (g *GitCLI) Run(args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = g.workDir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("git %v: %w: %s", args, err, stderr.String())
	}

	return strings.TrimSpace(stdout.String()), nil
}

// Commit 执行 git add + commit
func (g *GitCLI) Commit(path, message string) (string, error) {
	if _, err := g.Run("add", path); err != nil {
		return "", err
	}

	if _, err := g.Run("commit", "-m", message); err != nil {
		// 可能没有变化
		if strings.Contains(err.Error(), "nothing to commit") {
			// 获取最新 commit hash
			return g.GetHeadHash()
		}
		return "", err
	}

	return g.GetHeadHash()
}

// GetHeadHash 获取当前 HEAD 的 hash
func (g *GitCLI) GetHeadHash() (string, error) {
	return g.Run("rev-parse", "HEAD")
}

// CreateBranch 创建分支
func (g *GitCLI) CreateBranch(branchName string) error {
	_, err := g.Run("checkout", "-b", branchName)
	return err
}

// SwitchBranch 切换分支
func (g *GitCLI) SwitchBranch(branchName string) error {
	_, err := g.Run("checkout", branchName)
	return err
}

// MergeBranch 合并分支
func (g *GitCLI) MergeBranch(branchName string) error {
	_, err := g.Run("merge", "--no-ff", branchName)
	return err
}

// GetCurrentBranch 获取当前分支
func (g *GitCLI) GetCurrentBranch() (string, error) {
	return g.Run("rev-parse", "--abbrev-ref", "HEAD")
}

// ListBranches 列出所有分支
func (g *GitCLI) ListBranches() ([]string, error) {
	output, err := g.Run("branch", "-a")
	if err != nil {
		return nil, err
	}
	lines := strings.Split(output, "\n")
	var branches []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "remotes/") {
			branches = append(branches, strings.TrimPrefix(line, "* "))
		}
	}
	return branches, nil
}

// GetCommitLog 获取提交历史
func (g *GitCLI) GetCommitLog(path string, limit int) ([]CommitLogEntry, error) {
	args := []string{"log", "--oneline"}
	if limit > 0 {
		args = append(args, fmt.Sprintf("-%d", limit))
	}
	if path != "" {
		args = append(args, "--", path)
	}
	output, err := g.Run(args...)
	if err != nil {
		return nil, err
	}

	var logs []CommitLogEntry
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) >= 1 {
			entry := CommitLogEntry{Hash: parts[0]}
			if len(parts) >= 2 {
				entry.Message = parts[1]
			}
			logs = append(logs, entry)
		}
	}
	return logs, nil
}

// MoveWithGit 使用 git mv 移动
func (g *GitCLI) MoveWithGit(src, dst string) error {
	_, err := g.Run("mv", src, dst)
	return err
}

// GitGrep Git Grep 全文搜索
func (g *GitCLI) GitGrep(pattern, path string) ([]SearchHit, error) {
	args := []string{"grep", "-i", "-n", pattern}
	if path != "" {
		args = append(args, "--", path)
	}
	output, err := g.Run(args...)
	if err != nil {
		if strings.Contains(err.Error(), "exit status 1") {
			return []SearchHit{}, nil
		}
		return nil, err
	}

	return parseGrepOutput(output), nil
}

// GitBlame Git Blame 逐行追溯
func (g *GitCLI) GitBlame(path string) ([]BlameEntry, error) {
	output, err := g.Run("blame", "-p", path)
	if err != nil {
		return nil, err
	}

	return parseBlameOutput(output), nil
}

func parseGrepOutput(output string) []SearchHit {
	var hits []SearchHit
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, ":", 3)
		if len(parts) >= 3 {
			hits = append(hits, SearchHit{
				File:    parts[0],
				LineNum: 0,
				Content: parts[2],
			})
		}
	}
	return hits
}

func parseBlameOutput(_ string) []BlameEntry {
	var entries []BlameEntry
	return entries
}

// ReadFileAtCommit 读取指定提交时的文件内容
func (g *GitCLI) ReadFileAtCommit(filePath, commitHash string) (string, error) {
	if commitHash == "" || commitHash == "HEAD" {
		return g.ReadFile(filePath)
	}
	return g.Run("show", commitHash+":"+filePath)
}

// ReadFile 读取当前工作区的文件内容
func (g *GitCLI) ReadFile(filePath string) (string, error) {
	fullPath := filepath.Join(g.workDir, filePath)
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
