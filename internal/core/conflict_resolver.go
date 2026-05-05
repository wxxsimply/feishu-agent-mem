package core

import (
	"fmt"
	"time"

	"feishu-mem/internal/decision"
)

// ConflictResolver 冲突仲裁器
type ConflictResolver struct {
	LLMClient LLMClientInterface
}

// LLMClientInterface LLM 客户端接口
type LLMClientInterface interface {
	EvaluateContradiction(a, b *decision.DecisionNode) (float64, error)
	ClassifyTopic(content string, topics []string) (string, float64, error)
	JudgeCrossTopic(node *decision.DecisionNode, candidateTopics []string) ([]string, error)
}

// NewConflictResolver 创建冲突仲裁器
func NewConflictResolver(llm LLMClientInterface) *ConflictResolver {
	return &ConflictResolver{
		LLMClient: llm,
	}
}

// ResolveResult 冲突解决结果
type ResolveResult struct {
	Action   string
	OldSDRID string
	NewSDRID string
	Reason   string
	NeedsUser bool
}

// Resolve 解决冲突
func (cr *ConflictResolver) Resolve(
	newDecision *decision.DecisionNode,
	existingDecision *decision.DecisionNode,
) (*ResolveResult, error) {

	// 步骤 1: 权重比较
	newWeight := impactLevelWeight(newDecision.ImpactLevel)
	oldWeight := impactLevelWeight(existingDecision.ImpactLevel)

	// 步骤 2: 检查冲突
	var contradictionScore float64 = 0.0
	var err error
	if cr.LLMClient != nil {
		contradictionScore, err = cr.LLMClient.EvaluateContradiction(newDecision, existingDecision)
		if err != nil {
			contradictionScore = 0.0 // LLM 失败时降级为无冲突
		}
	}

	// 步骤 3: 根据分数决定动作
	switch {
	case contradictionScore < 0.3:
		return &ResolveResult{
			Action: "no_conflict",
			Reason: "No contradiction detected",
		}, nil
	case contradictionScore < 0.6:
		return &ResolveResult{
			Action: "relates",
			Reason: "Related but not contradictory",
		}, nil
	case newWeight > oldWeight:
		return &ResolveResult{
			Action: "supersedes",
			OldSDRID: existingDecision.SDRID,
			NewSDRID: newDecision.SDRID,
			Reason: fmt.Sprintf("New decision (impact: %s) supersedes old (impact: %s)",
				newDecision.ImpactLevel, existingDecision.ImpactLevel),
		}, nil
	case newWeight == oldWeight:
		return &ResolveResult{
			Action: "conflict",
			OldSDRID: existingDecision.SDRID,
			NewSDRID: newDecision.SDRID,
			NeedsUser: true,
			Reason: "Same impact level, needs user decision",
		}, nil
	default:
		return &ResolveResult{
			Action: "blocked",
			Reason: "Existing decision has higher priority",
		}, nil
	}
}

// DetectConflicts 批量检测冲突
func (cr *ConflictResolver) DetectConflicts(
	newNode *decision.DecisionNode,
	existingNodes []*decision.DecisionNode,
) ([]Conflict, error) {
	var conflicts []Conflict
	for _, existing := range existingNodes {
		if existing.SDRID == newNode.SDRID {
			continue // 跳过自己
		}

		score := 0.0
		if cr.LLMClient != nil {
			s, err := cr.LLMClient.EvaluateContradiction(newNode, existing)
			if err == nil {
				score = s
			}
		}

		if score > 0.3 {
			conflicts = append(conflicts, Conflict{
				ConflictID:       fmt.Sprintf("conflict_%s_%s_%d", newNode.SDRID, existing.SDRID, time.Now().Unix()),
				DecisionA:       newNode.SDRID,
				DecisionB:       existing.SDRID,
				ContradictionScore: score,
				Description:       "Potential conflict detected",
			})
		}
	}
	return conflicts, nil
}

func impactLevelWeight(level decision.ImpactLevel) int {
	switch level {
	case decision.ImpactCritical:
		return 4
	case decision.ImpactMajor:
		return 3
	case decision.ImpactMinor:
		return 2
	case decision.ImpactAdvisory:
		return 1
	default:
		return 0
	}
}

// AddRelationToDecision 添加关系到决策
func AddRelationToDecision(
	node *decision.DecisionNode,
	relType decision.RelationType,
	targetSDRID string,
	desc string,
) {
	node.Relations = append(node.Relations, decision.Relation{
		Type:        relType,
		TargetSDRID: targetSDRID,
		Description: desc,
	})
}

// NotifyConflictViaMCP 记录冲突通知（被 pipeline 调用）
// mcp 包中也定义了同名函数供跨包使用
func NotifyConflictViaMCP(sdrID1, sdrID2, reason string) {
	fmt.Printf("[Conflict] ⚠️ Unresolved: %s vs %s - %s\n", sdrID1, sdrID2, reason)
}
