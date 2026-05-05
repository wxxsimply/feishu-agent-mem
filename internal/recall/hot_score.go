package recall

import (
	"math"
	"time"

	"feishu-mem/internal/decision"
)

const (
	// 权重配置
	weightReference = 0.4
	weightTimeDecay = 0.25
	weightAccess    = 0.2
	weightRelations = 0.15

	// 衰减率（每天衰减的分数）
	decayRatePerDay = 1.5
)

// HotScoreCalculator 热点值计算器
type HotScoreCalculator struct {
}

// NewHotScoreCalculator 创建热点值计算器
func NewHotScoreCalculator() *HotScoreCalculator {
	return &HotScoreCalculator{}
}

// Calculate 计算决策的热点值
func (c *HotScoreCalculator) Calculate(node *decision.DecisionNode) float64 {
	if node == nil {
		return 0
	}

	referenceScore := c.calculateReferenceScore(node.AccessStats.ReferenceCount)
	timeDecayScore := c.calculateTimeDecayScore(node.CreatedAt)
	accessScore := c.calculateAccessScore(node.AccessStats.AccessCount)
	relationScore := c.calculateRelationScore(len(node.Relations))

	hotScore := referenceScore*weightReference +
		timeDecayScore*weightTimeDecay +
		accessScore*weightAccess +
		relationScore*weightRelations

	// 确保热点值在 0-100 范围内
	hotScore = math.Max(0, math.Min(100, hotScore))

	// 更新节点的热点值和计算时间
	now := time.Now()
	node.AccessStats.HotScore = hotScore
	node.AccessStats.LastCalculated = &now

	return hotScore
}

// calculateReferenceScore 计算引用频次得分（0-100）
func (c *HotScoreCalculator) calculateReferenceScore(count int) float64 {
	return math.Min(100, float64(count)*20)
}

// calculateTimeDecayScore 计算时间衰减得分（0-100）
func (c *HotScoreCalculator) calculateTimeDecayScore(createdAt time.Time) float64 {
	daysSince := time.Since(createdAt).Hours() / 24
	score := 100 - daysSince*decayRatePerDay
	return math.Max(0, score)
}

// calculateAccessScore 计算访问频次得分（0-100）
func (c *HotScoreCalculator) calculateAccessScore(count int) float64 {
	return math.Min(100, float64(count)*15)
}

// calculateRelationScore 计算关联密度得分（0-100）
func (c *HotScoreCalculator) calculateRelationScore(count int) float64 {
	return math.Min(100, float64(count)*25)
}

// GetHotCategory 获取热点值分类
func (c *HotScoreCalculator) GetHotCategory(hotScore float64) HotCategory {
	switch {
	case hotScore >= 80:
		return HotCategoryActive
	case hotScore >= 50:
		return HotCategoryNormal
	case hotScore >= 20:
		return HotCategoryFuzzy
	default:
		return HotCategoryForgotten
	}
}

// HotCategory 热点值分类
type HotCategory string

const (
	HotCategoryActive    HotCategory = "active"    // 活跃决策
	HotCategoryNormal   HotCategory = "normal"   // 常规决策
	HotCategoryFuzzy    HotCategory = "fuzzy"    // 模糊决策
	HotCategoryForgotten HotCategory = "forgotten" // 遗忘决策
)
