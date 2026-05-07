package recall

import (
	"strings"
	"sync"
	"time"

	"feishu-mem/internal/core"
	"feishu-mem/internal/decision"
)

// RecallType 回忆类型
type RecallType string

const (
	RecallExact    RecallType = "exact"     // 精确匹配
	RecallFuzzy    RecallType = "fuzzy"     // 模糊匹配
	RecallForgotten RecallType = "forgotten" // 遗忘决策
	RecallRelated  RecallType = "related"   // 关联推荐
)

// DecisionCard 决策卡片
type DecisionCard struct {
	Decision    *decision.DecisionNode
	HotScore    float64
	RecallType  RecallType
	Rendered    string
}

// RecallEngine 回忆引擎
type RecallEngine struct {
	memory     *core.MemoryGraph
	calculator *HotScoreCalculator
	mu         sync.RWMutex
	hotScores  map[string]float64 // 缓存热点值
}

// NewRecallEngine 创建回忆引擎
func NewRecallEngine(memory *core.MemoryGraph) *RecallEngine {
	return &RecallEngine{
		memory:     memory,
		calculator: NewHotScoreCalculator(),
		hotScores:  make(map[string]float64),
	}
}

// CalculateHotScore 计算决策的热点值
func (e *RecallEngine) CalculateHotScore(node *decision.DecisionNode) float64 {
	return e.calculator.Calculate(node)
}

// SearchRecall 执行回忆检索，返回最多 limit 个结果
func (e *RecallEngine) SearchRecall(query string, limit int) []*DecisionCard {
	var results []*DecisionCard

	// 1. 精确匹配：SDRID
	if strings.HasPrefix(query, "SDR-") {
		if node, ok := e.memory.GetDecision(query); ok {
			results = append(results, e.wrapCard(node, RecallExact))
		}
	}

	// 2. MemoryGraph 关键词搜索
	keywordResults := e.searchByKeyword(query, limit-len(results))
	results = append(results, keywordResults...)

	// 如果已有足够结果，返回
	if len(results) >= limit {
		return results[:limit]
	}

	// 3. 按热点值补充结果
	if len(results) < limit {
		hotResults := e.getHotDecisions(limit-len(results))
		results = append(results, hotResults...)
	}

	// 去重（按 SDRID）
	seen := make(map[string]bool)
	var unique []*DecisionCard
	for _, card := range results {
		if !seen[card.Decision.SDRID] {
			seen[card.Decision.SDRID] = true
			unique = append(unique, card)
		}
	}

	return unique
}

// searchByKeyword 在 MemoryGraph 中执行关键词搜索
func (e *RecallEngine) searchByKeyword(query string, limit int) []*DecisionCard {
	var results []*DecisionCard

	for _, node := range e.memory.GetAllDecisions() {
		if strings.Contains(strings.ToLower(node.Title), strings.ToLower(query)) ||
			strings.Contains(strings.ToLower(node.Decision), strings.ToLower(query)) ||
			strings.Contains(strings.ToLower(node.Rationale), strings.ToLower(query)) {
			results = append(results, e.wrapCard(node, RecallFuzzy))
		}

		if len(results) >= limit {
			break
		}
	}

	// 按热点值排序
	e.sortByHotScore(results)
	return results
}

// GetForgottenDecisions 获取遗忘决策
func (e *RecallEngine) GetForgottenDecisions(threshold float64) []*DecisionCard {
	if threshold == 0 {
		threshold = 20 // 默认阈值
	}

	var results []*DecisionCard
	for _, node := range e.memory.GetAllDecisions() {
		if !node.IsActive() {
			continue
		}
		hotScore := e.calculator.Calculate(node)
		if hotScore < threshold {
			results = append(results, e.wrapCardWithScore(node, RecallForgotten, hotScore))
		}
	}

	e.sortByHotScore(results)
	return results
}

// GetRelatedDecisions 获取关联决策
func (e *RecallEngine) GetRelatedDecisions(sdrID string) []*DecisionCard {
	var results []*DecisionCard

	// 获取主决策
	node, ok := e.memory.GetDecision(sdrID)
	if !ok {
		return nil
	}

	// 从关系中查找关联决策
	for _, rel := range node.Relations {
		if relatedNode, ok := e.memory.GetDecision(rel.TargetSDRID); ok {
			results = append(results, e.wrapCard(relatedNode, RecallRelated))
		}
	}

	return results
}

// GetRecentDecisions 获取最近的决策
func (e *RecallEngine) GetRecentDecisions(since time.Time) []*DecisionCard {
	var results []*DecisionCard
	for _, node := range e.memory.GetAllDecisions() {
		if node.CreatedAt.After(since) {
			results = append(results, e.wrapCard(node, RecallExact))
		}
	}

	// 按创建时间倒序排序
	for i := range results {
		for j := i + 1; j < len(results); j++ {
			if results[i].Decision.CreatedAt.Before(results[j].Decision.CreatedAt) {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	return results
}

// GetHotDecisions 获取高热点值决策
func (e *RecallEngine) GetHotDecisions(minScore float64, limit int) []*DecisionCard {
	if minScore == 0 {
		minScore = 50
	}

	var results []*DecisionCard
	for _, node := range e.memory.GetAllDecisions() {
		if !node.IsActive() {
			continue
		}
		hotScore := e.calculator.Calculate(node)
		if hotScore >= minScore {
			results = append(results, e.wrapCardWithScore(node, RecallExact, hotScore))
		}
	}

	e.sortByHotScore(results)
	if len(results) > limit {
		return results[:limit]
	}
	return results
}

// 内部辅助方法

func (e *RecallEngine) wrapCard(node *decision.DecisionNode, recallType RecallType) *DecisionCard {
	hotScore := e.calculator.Calculate(node)
	return &DecisionCard{
		Decision:   node,
		HotScore:   hotScore,
		RecallType: recallType,
	}
}

func (e *RecallEngine) wrapCardWithScore(node *decision.DecisionNode, recallType RecallType, hotScore float64) *DecisionCard {
	return &DecisionCard{
		Decision:   node,
		HotScore:   hotScore,
		RecallType: recallType,
	}
}

func (e *RecallEngine) sortByHotScore(cards []*DecisionCard) {
	for i := range cards {
		for j := i + 1; j < len(cards); j++ {
			if cards[i].HotScore < cards[j].HotScore {
				cards[i], cards[j] = cards[j], cards[i]
			}
		}
	}
}

func (e *RecallEngine) getHotDecisions(limit int) []*DecisionCard {
	return e.GetHotDecisions(0, limit)
}
