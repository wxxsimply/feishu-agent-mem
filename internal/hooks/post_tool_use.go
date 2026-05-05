package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"feishu-mem/internal/config"
	"feishu-mem/internal/core"
	"feishu-mem/internal/decision"
	"feishu-mem/internal/storage/git"
)

type PostToolUseRequest struct {
	ToolName string                 `json:"tool_name"`
	Args     map[string]interface{} `json:"args"`
	Result   interface{}            `json:"result"`
	Context  map[string]interface{} `json:"context"`
}

type PostToolUseResponse struct {
	Recorded bool   `json:"recorded"`
	SdrID    string `json:"sdr_id,omitempty"`
	Error    string `json:"error,omitempty"`
}

func RunPostToolUse() {
	var req PostToolUseRequest
	if err := json.NewDecoder(os.Stdin).Decode(&req); err != nil {
		resp := PostToolUseResponse{
			Error: fmt.Sprintf("无法解析请求: %v", err),
		}
		json.NewEncoder(os.Stdout).Encode(resp)
		return
	}

	settings := config.DefaultSettings()
	if cfgPath := os.Getenv("CONFIG_PATH"); cfgPath != "" {
		if s, err := config.LoadSettings(cfgPath); err == nil {
			settings = s
		}
	} else if s, err := config.LoadSettings("config/openclaw.yaml"); err == nil {
		settings = s
	}

	var gitStorage *git.GitStorage
	if settings.Git.WorkDir != "" {
		var err error
		gitStorage, err = git.NewGitStorage(git.Config{
			WorkDir: settings.Git.WorkDir,
			Branch:  settings.Git.Branch,
		})
		if err != nil {
			resp := PostToolUseResponse{
				Error: fmt.Sprintf("无法初始化 Git 存储: %v", err),
			}
			json.NewEncoder(os.Stdout).Encode(resp)
			return
		}
	}

	memoryGraph := core.NewMemoryGraph()
	if gitStorage != nil && settings.Memory.PreloadOnStart {
		memoryGraph.LoadFromGit(gitStorage, settings.Project.Name)
	}

	resp := PostToolUseResponse{}

	if shouldRecordDecision(req) {
		d := extractDecisionFromToolUse(req)
		if d != nil {
			memoryGraph.UpsertDecision(d, settings.Project.Name)
			if gitStorage != nil {
				if _, err := gitStorage.WriteDecision(d); err != nil {
					resp.Error = fmt.Sprintf("无法保存到 Git: %v", err)
					json.NewEncoder(os.Stdout).Encode(resp)
					return
				}
			}
			resp.Recorded = true
			resp.SdrID = d.SDRID
		}
	}

	json.NewEncoder(os.Stdout).Encode(resp)
}

func shouldRecordDecision(req PostToolUseRequest) bool {
	recordableTools := map[string]bool{
		"create_decision": true,
		"update_decision": true,
		"search":          true,
		"topic":           true,
		"decision":        true,
	}
	if recordableTools[req.ToolName] {
		return true
	}

	resultStr := fmt.Sprintf("%v", req.Result)
	if len(resultStr) > 50 {
		return true
	}

	return false
}

func extractDecisionFromToolUse(req PostToolUseRequest) *decision.DecisionNode {
	var title, dec, rationale, topic string

	if argsTitle, ok := req.Args["title"].(string); ok {
		title = argsTitle
	}
	if argsDec, ok := req.Args["decision"].(string); ok {
		dec = argsDec
	}
	if argsRationale, ok := req.Args["rationale"].(string); ok {
		rationale = argsRationale
	}
	if argsTopic, ok := req.Args["topic"].(string); ok {
		topic = argsTopic
	}

	if title == "" {
		switch req.ToolName {
		case "search":
			title = fmt.Sprintf("搜索: %v", req.Args["query"])
		case "topic":
			title = fmt.Sprintf("议题: %v", req.Args["topic"])
		case "decision":
			title = fmt.Sprintf("查看决策: %v", req.Args["sdr_id"])
		default:
			title = fmt.Sprintf("工具调用: %s", req.ToolName)
		}
	}

	if dec == "" {
		dec = fmt.Sprintf("工具 %s 执行结果: %v", req.ToolName, req.Result)
	}

	if topic == "" {
		topic = "general"
	}

	d := decision.NewDecisionNode("", title, "", topic)
	d.Decision = dec
	d.Rationale = rationale
	d.Status = decision.StatusDecided
	d.ImpactLevel = decision.ImpactMinor
	d.CreatedAt = time.Now()
	d.DecidedAt = &d.CreatedAt

	return d
}
