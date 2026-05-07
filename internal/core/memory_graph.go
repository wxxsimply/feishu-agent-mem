package core

import (
	"fmt"
	"math"
	"sync"
	"time"

	"feishu-mem/internal/decision"
)

// GitReader Git 读取接口
type GitReader interface {
	ListDecisions(project, topic string) ([]*decision.DecisionNode, error)
	ListTopics(project string) ([]string, error)
	ListDecisionBranches() ([]string, error)
	ReadDecision(project, topic, sdrID string) (*decision.DecisionNode, error)
	ReadDecisionFromBranch(branch, sdrID string) (*decision.DecisionNode, error)
}

// MemoryGraph 内存决策图 — 运行时加速
type MemoryGraph struct {
	mu sync.RWMutex

	// 决策索引: sdr_id -> *DecisionNode
	decisions map[string]*decision.DecisionNode

	// 议题索引: topic_name -> []sdr_id
	topics map[string][]string

	// 关系索引: sdr_id -> []Relation
	relations map[string][]decision.Relation

	// 跨议题引用索引: topic -> []related_topic
	crossTopicRefs map[string][]string

	// 项目索引
	projects map[string][]string // project -> []topic_name

	// 脏标记
	dirtyDecisions map[string]struct{}
}

// NewMemoryGraph 创建空图
func NewMemoryGraph() *MemoryGraph {
	return &MemoryGraph{
		decisions:      make(map[string]*decision.DecisionNode),
		topics:         make(map[string][]string),
		relations:      make(map[string][]decision.Relation),
		crossTopicRefs: make(map[string][]string),
		projects:       make(map[string][]string),
		dirtyDecisions: make(map[string]struct{}),
	}
}

// LoadFromGit 启动时从 Git 全量加载决策
// 遍历所有 decision/ 分支，读取每个分支的最新 HEAD
func (mg *MemoryGraph) LoadFromGit(reader GitReader, project string) error {
	mg.mu.Lock()
	defer mg.mu.Unlock()

	// 列出所有决策分支
	branches, err := reader.ListDecisionBranches()
	if err != nil {
		return fmt.Errorf("list decision branches failed: %w", err)
	}

	for _, branch := range branches {
		// 从分支名提取 sdrID: "decision/DEC-001" -> "DEC-001"
		sdrID := stringsTrimPrefix(branch, decision.BranchPrefixDecision)
		if sdrID == "" {
			continue
		}
		node, err := reader.ReadDecisionFromBranch(branch, sdrID)
		if err != nil {
			continue // 跳过读取失败的分支
		}
		mg.addDecisionInternal(node, project)
	}

	// 也加载 Dummy 决策（从 main 分支）
	if dummy, err := reader.ReadDecision(project, "general", decision.DummySDRID); err == nil && dummy != nil {
		mg.addDecisionInternal(dummy, project)
	}

	return nil
}

// stringsTrimPrefix 是 strings.TrimPrefix 的内联版本（避免 import strings）
func stringsTrimPrefix(s, prefix string) string {
	if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):]
	}
	return s
}

func (mg *MemoryGraph) addDecisionInternal(node *decision.DecisionNode, project string) {
	// 添加决策
	mg.decisions[node.SDRID] = node

	// 添加议题索引
	key := project + "/" + node.Topic
	mg.topics[key] = append(mg.topics[key], node.SDRID)

	// 添加项目索引
	mg.projects[project] = appendUnique(mg.projects[project], node.Topic)

	// 添加关系
	if len(node.Relations) > 0 {
		mg.relations[node.SDRID] = append(mg.relations[node.SDRID], node.Relations...)
	}

	// 添加跨议题引用
	for _, refTopic := range node.CrossTopicRefs {
		refKey := project + "/" + refTopic
		mg.crossTopicRefs[key] = appendUnique(mg.crossTopicRefs[key], refKey)
	}
}

// QueryByTopic 按议题检索 active 决策
func (mg *MemoryGraph) QueryByTopic(project, topic string) []*decision.DecisionNode {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	key := project + "/" + topic
	sdrIDs := mg.topics[key]

	var result []*decision.DecisionNode
	for _, sdrID := range sdrIDs {
		if d, ok := mg.decisions[sdrID]; ok {
			if d.IsActive() {
				result = append(result, d)
			}
		}
	}

	return result
}

// QueryCrossTopic 跨议题检索（含 cross_topic_refs）
func (mg *MemoryGraph) QueryCrossTopic(project, topic string) []*decision.DecisionNode {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	key := project + "/" + topic
	var result []*decision.DecisionNode

	// 当前议题
	sdrIDs := mg.topics[key]
	for _, sdrID := range sdrIDs {
		if d, ok := mg.decisions[sdrID]; ok {
			result = append(result, d)
		}
	}

	// 相关议题
	for _, refKey := range mg.crossTopicRefs[key] {
		for _, sdrID := range mg.topics[refKey] {
			if d, ok := mg.decisions[sdrID]; ok {
				result = append(result, d)
			}
		}
	}

	return result
}

// GetDecision 获取单个决策
func (mg *MemoryGraph) GetDecision(sdrID string) (*decision.DecisionNode, bool) {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	d, ok := mg.decisions[sdrID]
	return d, ok
}

// UpsertDecision 插入或更新决策
func (mg *MemoryGraph) UpsertDecision(node *decision.DecisionNode, project string) {
	mg.mu.Lock()
	defer mg.mu.Unlock()

	mg.addDecisionInternal(node, project)
	mg.dirtyDecisions[node.SDRID] = struct{}{}
}

// DeleteDecision 删除决策
func (mg *MemoryGraph) DeleteDecision(sdrID string) {
	mg.mu.Lock()
	defer mg.mu.Unlock()

	delete(mg.decisions, sdrID)
	delete(mg.dirtyDecisions, sdrID)
	// 注意：从索引中完全移除需要更多工作
}

// Count 返回决策数量
func (mg *MemoryGraph) Count() int {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	return len(mg.decisions)
}

// TopicCount 返回议题数量
func (mg *MemoryGraph) TopicCount(project string) int {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	return len(mg.projects[project])
}

// ListAllTopics 列出所有议题
func (mg *MemoryGraph) ListAllTopics(project string) []string {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	return append([]string{}, mg.projects[project]...)
}

// DetectConflicts 检测新决策与现有决策的冲突
func (mg *MemoryGraph) DetectConflicts(newNode *decision.DecisionNode) []Conflict {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	var conflicts []Conflict

	// 检查 CONFLICTS_WITH 关系
	for sdrID, relations := range mg.relations {
		for _, rel := range relations {
			if rel.Type == decision.RelationConflictsWith {
				if rel.TargetSDRID == newNode.SDRID {
					if other, ok := mg.decisions[sdrID]; ok {
						conflicts = append(conflicts, Conflict{
							DecisionA: sdrID,
							DecisionB: newNode.SDRID,
							Description: fmt.Sprintf("Conflict detected between %s and %s",
								other.Title, newNode.Title),
						})
					}
				}
			}
		}
	}

	return conflicts
}

// Conflict 冲突
type Conflict struct {
	ConflictID         string
	DecisionA          string
	DecisionB          string
	Description        string
	ContradictionScore float64
}

// SearchByKeywords 按关键词搜索活跃决策
func (mg *MemoryGraph) SearchByKeywords(query, topic string) []*decision.DecisionNode {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	var result []*decision.DecisionNode
	for _, d := range mg.decisions {
		if !d.IsActive() {
			continue
		}
		if topic != "" && d.Topic != topic {
			continue
		}
		// 简单关键词匹配
		if containsIgnoreCase(d.Title, query) ||
			containsIgnoreCase(d.Decision, query) ||
			containsIgnoreCase(d.Rationale, query) {
			result = append(result, d)
		}
	}

	return result
}

// GetAllDecisions 获取所有决策
func (mg *MemoryGraph) GetAllDecisions() []*decision.DecisionNode {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	result := make([]*decision.DecisionNode, 0, len(mg.decisions))
	for _, d := range mg.decisions {
		result = append(result, d)
	}
	return result
}

// GetRelations 获取决策的所有关系
func (mg *MemoryGraph) GetRelations(sdrID string) []decision.Relation {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	return append([]decision.Relation{}, mg.relations[sdrID]...)
}

// GetRelatedDecisions 获取与给定决策相关的决策
func (mg *MemoryGraph) GetRelatedDecisions(sdrID string) []*decision.DecisionNode {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	relations := mg.relations[sdrID]
	var result []*decision.DecisionNode
	for _, rel := range relations {
		if d, ok := mg.decisions[rel.TargetSDRID]; ok {
			result = append(result, d)
		}
	}
	return result
}

// UpdateAccessStats 更新访问统计（标记为脏，等待持久化）
func (mg *MemoryGraph) UpdateAccessStats(sdrID string) error {
	mg.mu.Lock()
	defer mg.mu.Unlock()

	if d, ok := mg.decisions[sdrID]; ok {
		d.AccessStats.RecordAccess()
		mg.dirtyDecisions[sdrID] = struct{}{}
		return nil
	}
	return fmt.Errorf("decision not found: %s", sdrID)
}

// RecordReference 记录决策被引用（标记为脏，等待持久化）
func (mg *MemoryGraph) RecordReference(sdrID string) error {
	mg.mu.Lock()
	defer mg.mu.Unlock()

	if d, ok := mg.decisions[sdrID]; ok {
		d.AccessStats.RecordReference()
		mg.dirtyDecisions[sdrID] = struct{}{}
		return nil
	}
	return fmt.Errorf("decision not found: %s", sdrID)
}

// RecalculateHotScore 重新计算决策热点值并及时生效
// 每次决策被引用或访问时调用，确保热点值实时反映讨论活跃度
func (mg *MemoryGraph) RecalculateHotScore(sdrID string) error {
	mg.mu.Lock()
	defer mg.mu.Unlock()

	if d, ok := mg.decisions[sdrID]; ok {
		refScore := math.Min(100, float64(d.AccessStats.ReferenceCount)*20)
		accessScore := math.Min(100, float64(d.AccessStats.AccessCount)*15)
		relationScore := math.Min(100, float64(len(d.Relations))*25)
		// 加权计算：reference 占 40%，access 占 20%，relation 占 15%
		// 剩余 25% 为基础值（新决策基础值 = 25，老决策会随时间衰减）
		baseScore := math.Min(25, 25-math.Max(0, float64(time.Since(d.CreatedAt).Hours()/24)*1.5))
		hotScore := refScore*0.40 + accessScore*0.20 + relationScore*0.15 + baseScore
		hotScore = math.Max(0, math.Min(100, hotScore))

		d.AccessStats.HotScore = hotScore
		now := time.Now()
		d.AccessStats.LastCalculated = &now
		mg.dirtyDecisions[sdrID] = struct{}{}
		return nil
	}
	return fmt.Errorf("decision not found: %s", sdrID)
}

// GetDirtyAndClean 获取脏决策列表并清除脏标记
func (mg *MemoryGraph) GetDirtyAndClean() []*decision.DecisionNode {
	mg.mu.Lock()
	defer mg.mu.Unlock()

	var result []*decision.DecisionNode
	for sdrID := range mg.dirtyDecisions {
		if d, ok := mg.decisions[sdrID]; ok {
			result = append(result, d)
		}
	}
	mg.dirtyDecisions = make(map[string]struct{})
	return result
}

// GetDecisionsByHotScore 按热点值获取活跃决策（从高到低）
// 已弃用/已拒绝/已搁置等非活跃决策不参与排序
func (mg *MemoryGraph) GetDecisionsByHotScore(minScore float64) []*decision.DecisionNode {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	var result []*decision.DecisionNode
	for _, d := range mg.decisions {
		if d.IsActive() && d.AccessStats.HotScore >= minScore {
			result = append(result, d)
		}
	}

	// 按热点值排序
	for i := range result {
		for j := i + 1; j < len(result); j++ {
			if result[i].AccessStats.HotScore < result[j].AccessStats.HotScore {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// GetRecentDecisions 获取最近的活跃决策
func (mg *MemoryGraph) GetRecentDecisions(since time.Time) []*decision.DecisionNode {
	mg.mu.RLock()
	defer mg.mu.RUnlock()

	var result []*decision.DecisionNode
	for _, d := range mg.decisions {
		if d.IsActive() && d.CreatedAt.After(since) {
			result = append(result, d)
		}
	}

	// 按创建时间倒序排序
	for i := range result {
		for j := i + 1; j < len(result); j++ {
			if result[i].CreatedAt.Before(result[j].CreatedAt) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// 辅助函数
func appendUnique(slice []string, item string) []string {
	for _, s := range slice {
		if s == item {
			return slice
		}
	}
	return append(slice, item)
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(len(substr) == 0 ||
			(len(s) > 0 &&
				(len(s) >= len(substr) &&
					stringsContainsIgnoreCase(s, substr))))
}

func stringsContainsIgnoreCase(s, substr string) bool {
	// 简单实现
	ls := stringsToLower(s)
	lsub := stringsToLower(substr)
	return len(ls) >= len(lsub) && ls != "" && lsub != "" &&
		indexOf(ls, lsub) >= 0
}

func stringsToLower(s string) string {
	// 简化版本
	var res []byte
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		res = append(res, c)
	}
	return string(res)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
