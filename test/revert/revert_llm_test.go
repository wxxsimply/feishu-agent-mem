package revert

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"feishu-mem/internal/decision"
	"feishu-mem/internal/llm"
	"feishu-mem/internal/llm/prompts"
)

// ========== 回溯检测类型定义 ==========

// RevertDecision LLM 回溯判断结果
type RevertDecision struct {
	ShouldRevert bool   `json:"should_revert"`
	TargetCommit string `json:"target_commit,omitempty"` // 回溯目标版本的标识
	Reason       string `json:"reason"`
	Confidence   float64 `json:"confidence"`
}

// RevertContext 回溯判断的上下文
type RevertContext struct {
	// 当前提取的新决策
	NewDecision *DecisionInfo

	// 已有的历史决策列表（按时间倒序）
	ExistingDecisions []*DecisionInfo

	// 文档变更内容（diff）
	DocDiff string

	// 文档标题
	DocTitle string
}

// DecisionInfo 决策信息
type DecisionInfo struct {
	ID          string
	Title       string
	Decision    string
	Rationale   string
	Status      decision.DecisionStatus
	CommitHash  string
	CreateTime  time.Time
	UpdateTime  time.Time
}

// ========== 回溯检测 Prompt ==========

var revertDetectionStaticPrompt = `# 系统提示词：决策回溯检测器

## 角色
你是一个项目决策版本管理专家。你需要分析文档变更，判断新提取的决策是否应该回溯到历史版本。

## 什么是决策回溯？
决策回溯是指：当前文档中的新决策实际上是对某个历史决策的恢复/撤销。

典型场景：
1. **撤销近期变更**：文档编辑者发现之前的修改有问题，恢复到更早的版本
   - 例：上次把"采用MySQL"改成了"采用TiDB"，现在又改回"采用MySQL"
   - 这种情况应该回溯到"采用MySQL"的版本

2. **纠正错误决策**：新决策与历史某个决策完全一致，说明是在纠正
   - 例：历史版本是"使用PostgreSQL"，后来改成了"使用MySQL"，现在文档又说"使用PostgreSQL"
   - 这种情况应该回溯到最初的"使用PostgreSQL"版本

3. **版本回退**：文档中出现了明显的版本回退信号
   - 例：使用了"恢复到"、"改回"、"回退到"、"撤销"等词语
   - 例：diff 中显示删除了新内容，恢复了旧内容

## 判断标准

### 必须回溯的情况（should_revert: true）
- 新决策与历史某个决策完全相同或高度相似（相似度 > 90%）
- 文档中出现明确的回退/恢复词语（"恢复"、"改回"、"回退"、"撤销"）
- diff 显示删除了近期新增内容，恢复了旧内容
- 新决策是历史决策的精确引用

### 不需要回溯的情况（should_revert: false）
- 新决策是全新决策，与历史无重复
- 新决策是对历史决策的补充或细化（不是完全相同）
- 新决策是历史决策的演进版本（有新增信息）

## 输入格式

你会收到：
1. **新提取的决策**：当前从文档中提取的决策信息
2. **历史决策列表**：该项目已有的决策，按时间倒序排列
3. **文档变更内容**：文档的 diff 或变更摘要

## 输出格式（严格遵守）

{
  "should_revert": true/false,
  "target_commit": "如果需要回溯，指定目标版本的 commit hash（从历史决策中选择）",
  "reason": "一句话说明判断理由",
  "confidence": 0.0-1.0
}

## 规则
- 只有明确是回退/恢复操作时才 should_revert: true
- 如果不确定，should_revert: false，宁可多创建一个决策也不要误回溯
- target_commit 必须是历史决策中存在的 commit hash
- confidence < 0.6 时不建议回溯
`

// revertDetectionDynamicBuilder 动态构建回溯检测 prompt
func revertDetectionDynamicBuilder(ctx map[string]any) string {
	var sb strings.Builder

	// 新提取的决策
	if newDecision, ok := ctx["new_decision"].(*DecisionInfo); ok {
		sb.WriteString("\n## 新提取的决策\n")
		sb.WriteString(fmt.Sprintf("- 标题: %s\n", newDecision.Title))
		sb.WriteString(fmt.Sprintf("- 决策: %s\n", newDecision.Decision))
		sb.WriteString(fmt.Sprintf("- 依据: %s\n", newDecision.Rationale))
		sb.WriteString(fmt.Sprintf("- 时间: %s\n", newDecision.CreateTime.Format("2006-01-02 15:04:05")))
	}

	// 历史决策列表
	if existingDecisions, ok := ctx["existing_decisions"].([]*DecisionInfo); ok && len(existingDecisions) > 0 {
		sb.WriteString("\n## 历史决策列表（按时间倒序）\n")
		for i, d := range existingDecisions {
			sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, d.CommitHash[:min(7, len(d.CommitHash))], d.Title))
			sb.WriteString(fmt.Sprintf("   决策: %s\n", d.Decision))
			sb.WriteString(fmt.Sprintf("   状态: %s, 时间: %s\n", d.Status, d.CreateTime.Format("2006-01-02 15:04:05")))
		}
	}

	// 文档变更内容
	if docDiff, ok := ctx["doc_diff"].(string); ok && docDiff != "" {
		sb.WriteString(fmt.Sprintf("\n## 文档变更内容（diff）\n%s\n", docDiff))
	}

	// 文档标题
	if docTitle, ok := ctx["doc_title"].(string); ok && docTitle != "" {
		sb.WriteString(fmt.Sprintf("\n## 文档标题\n%s\n", docTitle))
	}

	return sb.String()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ========== 上下文拼接逻辑 ==========

// BuildRevertContext 构建回溯检测的上下文
// 与实际应用保持一致的上下文拼接方式
func BuildRevertContext(
	newDecision *DecisionInfo,
	existingDecisions []*DecisionInfo,
	docDiff string,
	docTitle string,
) *RevertContext {
	return &RevertContext{
		NewDecision:       newDecision,
		ExistingDecisions: existingDecisions,
		DocDiff:           docDiff,
		DocTitle:          docTitle,
	}
}

// FormatContextForLLM 将上下文格式化为 LLM 可用的格式
// 这个函数模拟实际应用中的上下文拼接
func FormatContextForLLM(ctx *RevertContext) map[string]any {
	return map[string]any{
		"new_decision":       ctx.NewDecision,
		"existing_decisions": ctx.ExistingDecisions,
		"doc_diff":           ctx.DocDiff,
		"doc_title":          ctx.DocTitle,
	}
}

// ========== LLM 调用逻辑 ==========

// DetectRevert 调用 LLM 检测是否需要回溯
func DetectRevert(revertCtx *RevertContext) (*RevertDecision, error) {
	// 创建 prompt manager
	pm := prompts.NewPromptManager()

	// 注册回溯检测模板
	pm.RegisterTemplate(&prompts.PromptTemplate{
		Name:        "revert_detection",
		Static:      revertDetectionStaticPrompt,
		Dynamic:     revertDetectionDynamicBuilder,
		MaxTokens:   2000,
		Temperature: 0.1, // 低温度确保判断一致
	})

	// 构建 prompt
	dynamicData := FormatContextForLLM(revertCtx)
	prompt, err := pm.BuildPrompt("revert_detection", dynamicData)
	if err != nil {
		return nil, fmt.Errorf("build prompt failed: %w", err)
	}

	// 直接调用 LLM 并解析原始响应
	ctx := context.Background()
	llmClient := llm.NewClient()

	// 获取模板配置
	template, ok := pm.GetTemplate("revert_detection")
	if !ok {
		return nil, fmt.Errorf("template not found")
	}

	// 调用 LLM
	response, err := llmClient.Call(ctx, prompt, "")
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// 直接解析 JSON 响应
	revertDecision := &RevertDecision{
		ShouldRevert: false,
		Confidence:   0,
	}

	if err := json.Unmarshal([]byte(response), revertDecision); err != nil {
		// 如果直接解析失败，尝试提取 JSON 部分
		start := strings.Index(response, "{")
		end := strings.LastIndex(response, "}")
		if start >= 0 && end > start {
			jsonStr := response[start : end+1]
			if err := json.Unmarshal([]byte(jsonStr), revertDecision); err != nil {
				return nil, fmt.Errorf("parse LLM response failed: %w", err)
			}
		} else {
			return nil, fmt.Errorf("no valid JSON found in response")
		}
	}

	// 使用模板配置的温度值（如果有的话）
	_ = template.Temperature // 确保模板被使用

	return revertDecision, nil
}

// ========== 测试用例 ==========

func TestRevertDetection(t *testing.T) {
	// 创建 5 个测试决策场景
	testCases := []struct {
		name           string
		newDecision    *DecisionInfo
		existingDecisions []*DecisionInfo
		docDiff        string
		docTitle       string
		expectRevert   bool
		description    string
	}{
		{
			name: "场景1: 撤销近期变更 - 应回溯",
			newDecision: &DecisionInfo{
				ID:         "DEC-001-new",
				Title:      "数据库选型决策",
				Decision:   "采用MySQL作为主数据库",
				Rationale:  "性能测试显示MySQL更适合我们的场景",
				Status:     decision.StatusPending,
				CommitHash: "abc1234",
				CreateTime: time.Now(),
			},
			existingDecisions: []*DecisionInfo{
				{
					ID:         "DEC-001-v3",
					Title:      "数据库选型决策",
					Decision:   "采用TiDB作为主数据库替代MySQL",
					Rationale:  "分布式需求增加",
					Status:     decision.StatusDecided,
					CommitHash: "def5678",
					CreateTime: time.Now().Add(-24 * time.Hour),
				},
				{
					ID:         "DEC-001-v2",
					Title:      "数据库选型决策",
					Decision:   "采用MySQL作为主数据库替代PostgreSQL",
					Rationale:  "MySQL生态更成熟",
					Status:     decision.StatusDeprecated,
					CommitHash: "ghi9012",
					CreateTime: time.Now().Add(-48 * time.Hour),
				},
				{
					ID:         "DEC-001-v1",
					Title:      "数据库选型决策",
					Decision:   "采用PostgreSQL作为主数据库",
					Rationale:  "JSONB支持更好",
					Status:     decision.StatusDeprecated,
					CommitHash: "jkl3456",
					CreateTime: time.Now().Add(-72 * time.Hour),
				},
			},
			docDiff:      "删除: 采用TiDB作为主数据库替代MySQL\n添加: 采用MySQL作为主数据库",
			docTitle:     "技术选型文档",
			expectRevert: true,
			description:  "新决策与历史v2版本相同，应该回溯到v2版本",
		},
		{
			name: "场景2: 全新决策 - 不应回溯",
			newDecision: &DecisionInfo{
				ID:         "DEC-002",
				Title:      "消息队列选型",
				Decision:   "采用Kafka作为消息队列",
				Rationale:  "高吞吐量需求",
				Status:     decision.StatusPending,
				CommitHash: "new1234",
				CreateTime: time.Now(),
			},
			existingDecisions: []*DecisionInfo{
				{
					ID:         "DEC-001",
					Title:      "数据库选型决策",
					Decision:   "采用MySQL作为主数据库",
					Rationale:  "性能测试",
					Status:     decision.StatusDecided,
					CommitHash: "old1234",
					CreateTime: time.Now().Add(-24 * time.Hour),
				},
			},
			docDiff:      "添加: 消息队列选型部分\n采用Kafka作为消息队列",
			docTitle:     "技术选型文档",
			expectRevert: false,
			description:  "全新决策，与历史无重复，不应回溯",
		},
		{
			name: "场景3: 明确回退词语 - 应回溯",
			newDecision: &DecisionInfo{
				ID:         "DEC-003",
				Title:      "缓存方案决策",
				Decision:   "恢复到使用Redis作为缓存方案",
				Rationale:  "Memcached不支持复杂数据结构",
				Status:     decision.StatusPending,
				CommitHash: "restore1",
				CreateTime: time.Now(),
			},
			existingDecisions: []*DecisionInfo{
				{
					ID:         "DEC-003-v2",
					Title:      "缓存方案决策",
					Decision:   "采用Memcached作为缓存方案",
					Rationale:  "简单高效",
					Status:     decision.StatusDecided,
					CommitHash: "memc1234",
					CreateTime: time.Now().Add(-24 * time.Hour),
				},
				{
					ID:         "DEC-003-v1",
					Title:      "缓存方案决策",
					Decision:   "使用Redis作为缓存方案",
					Rationale:  "支持丰富数据结构",
					Status:     decision.StatusDeprecated,
					CommitHash: "redis123",
					CreateTime: time.Now().Add(-48 * time.Hour),
				},
			},
			docDiff:      "删除: 采用Memcached作为缓存方案\n添加: 恢复到使用Redis作为缓存方案",
			docTitle:     "架构设计文档",
			expectRevert: true,
			description:  "包含明确的回退词语'恢复到'，应该回溯到v1版本",
		},
		{
			name: "场景4: 决策补充细化 - 不应回溯",
			newDecision: &DecisionInfo{
				ID:         "DEC-004",
				Title:      "API版本管理",
				Decision:   "采用URL版本控制方案（v1/v2），支持向后兼容",
				Rationale:  "简化客户端升级路径",
				Status:     decision.StatusPending,
				CommitHash: "api1234",
				CreateTime: time.Now(),
			},
			existingDecisions: []*DecisionInfo{
				{
					ID:         "DEC-004-v1",
					Title:      "API版本管理",
					Decision:   "采用URL版本控制方案",
					Rationale:  "简单直接",
					Status:     decision.StatusDecided,
					CommitHash: "oldapi12",
					CreateTime: time.Now().Add(-24 * time.Hour),
				},
			},
			docDiff:      "修改: API版本控制方案\n旧: 采用URL版本控制方案\n新: 采用URL版本控制方案（v1/v2），支持向后兼容",
			docTitle:     "API设计规范",
			expectRevert: false,
			description:  "对历史决策的补充细化，不是回退，不应回溯",
		},
		{
			name: "场景5: 相似但不同的决策 - 不应回溯",
			newDecision: &DecisionInfo{
				ID:         "DEC-005",
				Title:      "日志框架选型",
				Decision:   "采用Zap作为日志框架",
				Rationale:  "性能优于Logrus",
				Status:     decision.StatusPending,
				CommitHash: "zap1234",
				CreateTime: time.Now(),
			},
			existingDecisions: []*DecisionInfo{
				{
					ID:         "DEC-005-v1",
					Title:      "日志框架选型",
					Decision:   "采用Logrus作为日志框架",
					Rationale:  "社区活跃",
					Status:     decision.StatusDecided,
					CommitHash: "logrus1",
					CreateTime: time.Now().Add(-24 * time.Hour),
				},
			},
			docDiff:      "修改: 日志框架选型\n旧: 采用Logrus作为日志框架\n新: 采用Zap作为日志框架",
			docTitle:     "基础设施文档",
			expectRevert: false,
			description:  "相似但不同的决策（Logrus vs Zap），是演进不是回退，不应回溯",
		},
	}

	// 运行测试
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("测试: %s", tc.description)

			// 构建上下文
			revertCtx := BuildRevertContext(
				tc.newDecision,
				tc.existingDecisions,
				tc.docDiff,
				tc.docTitle,
			)

			// 格式化为 LLM 输入
			llmInput := FormatContextForLLM(revertCtx)

			// 构建完整 prompt 进行测试
			pm := prompts.NewPromptManager()
			pm.RegisterTemplate(&prompts.PromptTemplate{
				Name:        "revert_detection",
				Static:      revertDetectionStaticPrompt,
				Dynamic:     revertDetectionDynamicBuilder,
				MaxTokens:   2000,
				Temperature: 0.1,
			})

			prompt, err := pm.BuildPrompt("revert_detection", llmInput)
			if err != nil {
				t.Fatalf("Build prompt failed: %v", err)
			}

			// 输出构建的 prompt（调试用）
			t.Logf("\n=== 构建的 Prompt ===\n%s\n", prompt)

			// 调用 LLM 检测
			revertDecision, err := DetectRevert(revertCtx)
			if err != nil {
				t.Logf("LLM 调用失败（可能未配置 API）: %v", err)
				// 如果 LLM 不可用，跳过断言
				return
			}

			t.Logf("LLM 判断结果: ShouldRevert=%v, Confidence=%.2f, Reason=%s",
				revertDecision.ShouldRevert, revertDecision.Confidence, revertDecision.Reason)

			// 验证结果
			if revertDecision.ShouldRevert != tc.expectRevert {
				t.Errorf("期望 ShouldRevert=%v, 实际=%v", tc.expectRevert, revertDecision.ShouldRevert)
			}
		})
	}
}

// ========== 辅助函数 ==========

// CreateTestDecision 创建测试用决策节点
func CreateTestDecision(id, title, decisionContent, rationale string, status decision.DecisionStatus, createTime time.Time) *DecisionInfo {
	return &DecisionInfo{
		ID:         id,
		Title:      title,
		Decision:   decisionContent,
		Rationale:  rationale,
		Status:     status,
		CommitHash: fmt.Sprintf("commit-%s", id),
		CreateTime: createTime,
	}
}

// SaveTestDecisions 保存测试决策到临时文件（模拟 Git 历史）
func SaveTestDecisions(decisions []*DecisionInfo, dir string) error {
	for _, d := range decisions {
		node := &decision.DecisionNode{
			SDRID:    d.ID,
			Title:    d.Title,
			Decision: d.Decision,
			Rationale: d.Rationale,
			Status:   d.Status,
			CreatedAt: d.CreateTime,
		}

		data, err := json.MarshalIndent(node, "", "  ")
		if err != nil {
			return err
		}

		filename := fmt.Sprintf("%s/%s.json", dir, d.ID)
		if err := os.WriteFile(filename, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

// PrintRevertResult 打印回溯检测结果
func PrintRevertResult(name string, result *RevertDecision) {
	fmt.Printf("\n=== %s ===\n", name)
	fmt.Printf("Should Revert: %v\n", result.ShouldRevert)
	fmt.Printf("Confidence: %.2f\n", result.Confidence)
	fmt.Printf("Reason: %s\n", result.Reason)
	if result.ShouldRevert && result.TargetCommit != "" {
		fmt.Printf("Target Commit: %s\n", result.TargetCommit)
	}
}

// ========== 独立运行的测试 ==========

func TestRevertDetectionStandalone(t *testing.T) {
	// 这个测试可以独立运行，不需要 LLM API
	// 主要验证上下文拼接逻辑

	// 创建测试决策
	newDecision := &DecisionInfo{
		ID:         "DEC-TEST",
		Title:      "测试决策",
		Decision:   "采用MySQL作为数据库",
		Rationale:  "性能测试结果",
		Status:     decision.StatusPending,
		CommitHash: "test1234",
		CreateTime: time.Now(),
	}

	existingDecisions := []*DecisionInfo{
		{
			ID:         "DEC-TEST-v2",
			Title:      "测试决策",
			Decision:   "采用TiDB作为数据库",
			Rationale:  "分布式需求",
			Status:     decision.StatusDecided,
			CommitHash: "old12345",
			CreateTime: time.Now().Add(-24 * time.Hour),
		},
		{
			ID:         "DEC-TEST-v1",
			Title:      "测试决策",
			Decision:   "采用MySQL作为数据库",
			Rationale:  "性能测试",
			Status:     decision.StatusDeprecated,
			CommitHash: "v1xxxxx",
			CreateTime: time.Now().Add(-48 * time.Hour),
		},
	}

	// 构建上下文
	ctx := BuildRevertContext(newDecision, existingDecisions, "变更内容", "测试文档")

	// 格式化为 LLM 输入
	llmInput := FormatContextForLLM(ctx)

	// 验证上下文格式
	if _, ok := llmInput["new_decision"]; !ok {
		t.Error("缺少 new_decision")
	}
	if _, ok := llmInput["existing_decisions"]; !ok {
		t.Error("缺少 existing_decisions")
	}
	if _, ok := llmInput["doc_diff"]; !ok {
		t.Error("缺少 doc_diff")
	}
	if _, ok := llmInput["doc_title"]; !ok {
		t.Error("缺少 doc_title")
	}

	t.Log("上下文拼接测试通过")
}

// ========== 主函数 ==========

func TestMain(m *testing.M) {
	// 设置日志
	log.SetFlags(log.Ltime | log.Lshortfile)

	// 运行测试
	os.Exit(m.Run())
}
