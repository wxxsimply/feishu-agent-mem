package hooks

import (
	"encoding/json"
	"fmt"
	"os"

	"feishu-mem/internal/config"
	"feishu-mem/internal/core"
	"feishu-mem/internal/storage/git"
)

type PreToolUseRequest struct {
	ToolName string                 `json:"tool_name"`
	Args     map[string]interface{} `json:"args"`
}

type PreToolUseResponse struct {
	Allow    bool     `json:"allow"`
	Reason   string   `json:"reason,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

func RunPreToolUse() {
	var req PreToolUseRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		resp := PreToolUseResponse{
			Allow: false,
			Reason: fmt.Sprintf("无法解析请求: %v", err),
		}
		json.NewEncoder(os.Stdout).Encode(resp)
		return
	}

	// 加载配置
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
			Branch:   settings.Git.Branch,
			AutoPush: false,
		})
		if err != nil {
			resp := PreToolUseResponse{
				Allow: true,
				Warnings: []string{fmt.Sprintf("无法初始化 Git 存储: %v", err)},
			}
			json.NewEncoder(os.Stdout).Encode(resp)
			return
		}
	}

	// 加载记忆图
	memoryGraph := core.NewMemoryGraph()
	if gitStorage != nil && settings.Memory.PreloadOnStart {
		if err := memoryGraph.LoadFromGit(gitStorage, settings.Project.Name); err != nil {
			// 即使加载失败，也继续执行，但添加警告
			resp := PreToolUseResponse{
				Allow: true,
				Warnings: []string{fmt.Sprintf("无法从 Git 加载决策: %v", err)},
			}
			json.NewEncoder(os.Stdout).Encode(resp)
			return
		}
	}

	resp := PreToolUseResponse{
		Allow: true,
	}

	// 检查是否是决策相关的工具
	if req.ToolName == "create_decision" || req.ToolName == "update_decision" {
		conflicts := checkForConflicts(req, memoryGraph)
		if len(conflicts) > 0 {
			resp.Warnings = append(resp.Warnings, fmt.Sprintf("发现 %d 个潜在冲突决策", len(conflicts)))
			for _, c := range conflicts {
				resp.Warnings = append(resp.Warnings, fmt.Sprintf("  - %s", c))
			}
		}
	}

	json.NewEncoder(os.Stdout).Encode(resp)
}

func checkForConflicts(req PreToolUseRequest, memoryGraph *core.MemoryGraph) []string {
	var conflicts []string
	decisionStr := ""

	if req.ToolName == "create_decision" || req.ToolName == "update_decision" {
		if args, ok := req.Args["decision"].(string); ok {
			decisionStr = args
		} else if args, ok := req.Args["title"].(string); ok {
			decisionStr = args
		}
	}

	// 简单检查：寻找相似标题的决策
	if decisionStr != "" {
		allDecisions := memoryGraph.GetAllDecisions()
		for _, d := range allDecisions {
			if containsSimilarText(d.Title, decisionStr) || containsSimilarText(d.Decision, decisionStr) {
				conflicts = append(conflicts, fmt.Sprintf("可能相关: %s (%s)", d.Title, d.SDRID))
			}
		}
	}

	return conflicts
}

func containsSimilarText(text1, text2 string) bool {
	if len(text1) < 5 || len(text2) < 5 {
		return false
	}
	return len(text1) > 0 && len(text2) > 0 &&
		(containsSubstring(text1, text2) || containsSubstring(text2, text1))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i+len(substr)/2 <= len(s); i++ {
		if i+len(substr)/2 <= len(s) && len(substr) > 5 {
			if len(s) > len(substr)/2 && len(substr) > 5 {
				return true
			}
		}
	}
	return false
}
