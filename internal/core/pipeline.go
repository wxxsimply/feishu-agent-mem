package core

import (
	"fmt"
	"log"

	"feishu-mem/internal/decision"
	"feishu-mem/internal/signal"
)

// PipelineEngine 流程引擎
type PipelineEngine struct {
	GitStorage   GitStorageInterface
	BitableStore BitableStoreInterface
	MemoryGraph  *MemoryGraph
}

// GitStorageInterface Git 存储接口
type GitStorageInterface interface {
	WriteDecision(node *decision.DecisionNode) (string, error)
	ReadDecision(project, topic, sdrID string) (*decision.DecisionNode, error)
	ListDecisions(project, topic string) ([]*decision.DecisionNode, error)
	WriteObjection(obj *decision.Objection) (string, error)
	ListObjections(project, topic string) ([]*decision.Objection, error)
	ReadDecisionAtCommit(project, topic, sdrID, commitHash string) (*decision.DecisionNode, error)
	GetDecisionHistory(project, topic, sdrID string) ([]CommitLogEntry, error)
}

// CommitLogEntry 提交日志条目
type CommitLogEntry struct {
	Hash    string
	Message string
}

// BitableStoreInterface Bitable 存储接口
type BitableStoreInterface interface {
	UpsertDecision(node *decision.DecisionNode) error
	UpsertDecisionWithConflict(node *decision.DecisionNode, conflictSDRID string) error
	UpdateConflictFields(sdrID, newConflictSDRID string) error
	ClearConflictFields(sdrID string) error
	QueryByTopic(topic, status string) ([]*decision.DecisionNode, error)
	QueryCrossTopic(topic string) ([]*decision.DecisionNode, error)
}

// NewPipelineEngine 创建流程引擎
func NewPipelineEngine(
	git GitStorageInterface,
	bitable BitableStoreInterface,
	memory *MemoryGraph,
) *PipelineEngine {
	return &PipelineEngine{
		GitStorage:   git,
		BitableStore: bitable,
		MemoryGraph:  memory,
	}
}

// ApplyMutation 执行决策变更
func (pe *PipelineEngine) ApplyMutation(mut *signal.DecisionMutation) error {
	switch mut.Type {
	case signal.MutationCreate:
		return pe.applyCreate(mut)
	case signal.MutationUpdate:
		return pe.applyUpdate(mut)
	case signal.MutationStatusChange:
		return pe.applyStatusChange(mut)
	case signal.MutationConflict:
		// 根据 ConflictAction 分发
		switch mut.ConflictAction {
		case "merge":
			return pe.applyConflictMerge(mut)
		default:
			return pe.applyConflictKeepBoth(mut)
		}
	case signal.MutationObjection:
		return pe.applyCreateObjection(mut)
	case signal.MutationDeprecate:
		return pe.applyDeprecate(mut)
	case signal.MutationRevert:
		return pe.applyRevert(mut)
	default:
		return fmt.Errorf("unknown mutation type: %s", mut.Type)
	}
}

func (pe *PipelineEngine) applyCreate(mut *signal.DecisionMutation) error {
	if mut.Node == nil {
		return fmt.Errorf("node is required for create mutation")
	}

	// 写入 Git
	hash, err := pe.GitStorage.WriteDecision(mut.Node)
	if err != nil {
		return fmt.Errorf("git write failed: %w", err)
	}
	mut.Node.GitCommitHash = hash

	// 同步 Bitable
	if pe.BitableStore != nil {
		if err := pe.BitableStore.UpsertDecision(mut.Node); err != nil {
			log.Printf("[Bitable] UpsertDecision failed: %v", err)
		} else {
			log.Printf("[Bitable] UpsertDecision OK: %s", mut.Node.SDRID)
		}
	} else {
		log.Printf("[Bitable] store is nil, skipping sync")
	}

	// 更新内存图
	pe.MemoryGraph.UpsertDecision(mut.Node, mut.Node.Project)

	return nil
}

func (pe *PipelineEngine) applyUpdate(mut *signal.DecisionMutation) error {
	project := mut.Node.Project
	topic := mut.Node.Topic
	if project == "" {
		project = "feishu-mem"
	}
	if topic == "" {
		topic = "general"
	}
	existing, err := pe.GitStorage.ReadDecision(project, topic, mut.SDRID)
	if err != nil {
		return fmt.Errorf("read existing decision failed: %w", err)
	}

	for k, v := range mut.FieldChanges {
		switch k {
		case "title":
			existing.Title = v.(string)
		case "decision":
			existing.Decision = v.(string)
		case "rationale":
			existing.Rationale = v.(string)
		case "impact_level":
			existing.ImpactLevel = decision.ImpactLevel(v.(string))
		case "executor":
			existing.Executor = v.(string)
		case "proposer":
			existing.Proposer = v.(string)
		case "status":
			existing.Status = decision.DecisionStatus(v.(string))
		}
	}

	// 合并新节点的 FeishuLinks（保留已有 token 并追加新的）
	if mut.Node != nil {
		for _, token := range mut.Node.FeishuLinks.RelatedDocTokens {
			existing.FeishuLinks.RelatedDocTokens = appendUnique(
				existing.FeishuLinks.RelatedDocTokens, token)
		}
	}

	hash, err := pe.GitStorage.WriteDecision(existing)
	if err != nil {
		return fmt.Errorf("git write update failed: %w", err)
	}
	existing.GitCommitHash = hash

	if pe.BitableStore != nil {
		if err := pe.BitableStore.UpsertDecision(existing); err != nil {
			log.Printf("[Bitable] UpsertDecision update failed: %v", err)
		} else {
			log.Printf("[Bitable] UpsertDecision update OK: %s", mut.SDRID)
		}
	}

	pe.MemoryGraph.UpsertDecision(existing, existing.Project)
	log.Printf("[Pipeline] Updated decision %s", mut.SDRID)
	return nil
}

func (pe *PipelineEngine) applyStatusChange(mut *signal.DecisionMutation) error {
	// 从内存图获取 project/topic
	project := "feishu-mem"
	topic := "general"
	if existingNode, ok := pe.MemoryGraph.GetDecision(mut.SDRID); ok {
		project = existingNode.Project
		topic = existingNode.Topic
	}
	existing, err := pe.GitStorage.ReadDecision(project, topic, mut.SDRID)
	if err != nil {
		return err
	}

	existing.Status = mut.NewStatus
	hash, err := pe.GitStorage.WriteDecision(existing)
	if err != nil {
		return err
	}
	existing.GitCommitHash = hash

	pe.MemoryGraph.UpsertDecision(existing, existing.Project)

	return nil
}

// applyConflictMerge — LLM 自动合并冲突
// Git: 覆盖更新已有决策文件
// Bitable: 更新已有记录 + 清除 conflict_status
func (pe *PipelineEngine) applyConflictMerge(mut *signal.DecisionMutation) error {
	if mut.Node == nil {
		return fmt.Errorf("node is required for conflict merge mutation")
	}

	// 读取已有决策
	project := mut.Node.Project
	topic := mut.Node.Topic
	if project == "" {
		project = "feishu-mem"
	}
	if topic == "" {
		topic = "general"
	}
	existing, err := pe.GitStorage.ReadDecision(project, topic, mut.SDRID)
	if err != nil {
		return fmt.Errorf("read existing decision for merge failed: %w", err)
	}

	// 用新决策内容覆盖
	existing.Title = mut.Node.Title
	existing.Decision = mut.Node.Decision
	existing.Rationale = mut.Node.Rationale
	existing.ImpactLevel = mut.Node.ImpactLevel
	existing.Executor = mut.Node.Executor

	hash, err := pe.GitStorage.WriteDecision(existing)
	if err != nil {
		return fmt.Errorf("git write merge failed: %w", err)
	}
	existing.GitCommitHash = hash

	log.Printf("[Pipeline] ✅ Conflict auto-merged into %s: %s", mut.SDRID, mut.ConflictReason)

	if pe.BitableStore != nil {
		// 更新 Bitable（清除冲突状态）
		if err := pe.BitableStore.UpsertDecision(existing); err != nil {
			log.Printf("[Bitable] UpsertDecision merge failed: %v", err)
		} else {
			log.Printf("[Bitable] Merged decision OK: %s", mut.SDRID)
		}
		// 清除冲突标记
		if err := pe.BitableStore.ClearConflictFields(mut.SDRID); err != nil {
			log.Printf("[Bitable] ClearConflictFields failed: %v", err)
		}
	}

	pe.MemoryGraph.UpsertDecision(existing, existing.Project)
	return nil
}

// applyConflictKeepBoth — LLM 无法解决冲突，保留双方
func (pe *PipelineEngine) applyConflictKeepBoth(mut *signal.DecisionMutation) error {
	if mut.Node == nil {
		return fmt.Errorf("node is required for keep_both mutation")
	}

	hash, err := pe.GitStorage.WriteDecision(mut.Node)
	if err != nil {
		return fmt.Errorf("git write conflict failed: %w", err)
	}
	mut.Node.GitCommitHash = hash

	log.Printf("[Pipeline] ⚠️ CONFLICT %s vs %s: %s", mut.SDRID, mut.ConflictSDRID, mut.ConflictReason)

	if pe.BitableStore != nil {
		if err := pe.BitableStore.UpsertDecisionWithConflict(mut.Node, mut.ConflictSDRID); err != nil {
			log.Printf("[Bitable] UpsertDecision conflict failed: %v", err)
		}
		if mut.ConflictSDRID != "" {
			if err := pe.BitableStore.UpdateConflictFields(mut.ConflictSDRID, mut.SDRID); err != nil {
				log.Printf("[Bitable] UpdateConflictFields failed: %v", err)
			}
		}
	}

	pe.MemoryGraph.UpsertDecision(mut.Node, mut.Node.Project)

	// MCP 通知占位
	NotifyConflictViaMCP(mut.SDRID, mut.ConflictSDRID, mut.ConflictReason)
	return nil
}

// applyCreateObjection 创建反对意见
func (pe *PipelineEngine) applyCreateObjection(mut *signal.DecisionMutation) error {
	if mut.Objection == nil {
		return fmt.Errorf("objection is required for objection mutation")
	}
	hash, err := pe.GitStorage.WriteObjection(mut.Objection)
	if err != nil {
		return fmt.Errorf("git write objection failed: %w", err)
	}
	log.Printf("[Pipeline] Created objection: %s (git: %s)", mut.Objection.OID, hash)
	return nil
}

// applyDeprecate 废弃/取代决策
func (pe *PipelineEngine) applyDeprecate(mut *signal.DecisionMutation) error {
	project := "feishu-mem"
	topic := "general"
	if existingNode, ok := pe.MemoryGraph.GetDecision(mut.SDRID); ok {
		project = existingNode.Project
		topic = existingNode.Topic
	}
	existing, err := pe.GitStorage.ReadDecision(project, topic, mut.SDRID)
	if err != nil {
		return fmt.Errorf("read decision for deprecation failed: %w", err)
	}
	existing.Status = mut.NewStatus
	hash, err := pe.GitStorage.WriteDecision(existing)
	if err != nil {
		return fmt.Errorf("git write deprecation failed: %w", err)
	}
	existing.GitCommitHash = hash
	if pe.BitableStore != nil {
		if err := pe.BitableStore.UpsertDecision(existing); err != nil {
			log.Printf("[Bitable] UpsertDecision deprecation failed: %v", err)
		}
	}
	pe.MemoryGraph.UpsertDecision(existing, existing.Project)
	log.Printf("[Pipeline] Deprecated decision %s -> %s", mut.SDRID, mut.NewStatus)
	return nil
}

// BatchApply 批量应用变更
func (pe *PipelineEngine) BatchApply(mutations []*signal.DecisionMutation) error {
	for _, mut := range mutations {
		if err := pe.ApplyMutation(mut); err != nil {
			return err
		}
	}
	return nil
}

// ValidateDecision 验证决策
func (pe *PipelineEngine) ValidateDecision(node *decision.DecisionNode) []string {
	var issues []string

	if node.SDRID == "" {
		issues = append(issues, "SDRID is required")
	}
	if node.Title == "" {
		issues = append(issues, "Title is required")
	}
	if node.Topic == "" {
		issues = append(issues, "Topic is required")
	}
	if !node.Status.IsValid() {
		issues = append(issues, "Invalid status")
	}
	if !node.ImpactLevel.IsValid() {
		issues = append(issues, "Invalid impact level")
	}

	return issues
}

// applyRevert 执行回溯到指定版本
func (pe *PipelineEngine) applyRevert(mut *signal.DecisionMutation) error {
	// 从内存图获取当前决策的 project 和 topic
	project := "feishu-mem"
	topic := "general"
	if existingNode, ok := pe.MemoryGraph.GetDecision(mut.SDRID); ok {
		project = existingNode.Project
		topic = existingNode.Topic
	}

	// 读取目标提交时的决策
	targetNode, err := pe.GitStorage.ReadDecisionAtCommit(project, topic, mut.SDRID, mut.TargetCommitHash)
	if err != nil {
		return fmt.Errorf("read target commit failed: %w", err)
	}

	// 读取当前版本（用于保存 previous commit hash）
	currentNode, err := pe.GitStorage.ReadDecision(project, topic, mut.SDRID)
	if err == nil {
		// 在目标节点保存上一版本的 commit hash
		targetNode.PreviousCommitHash = currentNode.GitCommitHash
	}

	// 用目标提交的内容写入新版本（不覆盖历史，新增 commit）
	hash, err := pe.GitStorage.WriteDecision(targetNode)
	if err != nil {
		return fmt.Errorf("git write failed: %w", err)
	}
	targetNode.GitCommitHash = hash

	// 同步 Bitable
	if pe.BitableStore != nil {
		if err := pe.BitableStore.UpsertDecision(targetNode); err != nil {
			log.Printf("[Bitable] UpsertDecision (revert) failed: %v", err)
		} else {
			log.Printf("[Bitable] UpsertDecision (revert) OK: %s", mut.SDRID)
		}
	} else {
		log.Printf("[Bitable] store is nil, skipping sync")
	}

	// 更新内存图
	pe.MemoryGraph.UpsertDecision(targetNode, targetNode.Project)

	log.Printf("[Pipeline] Reverted decision %s to commit %s", mut.SDRID, mut.TargetCommitHash)
	return nil
}
