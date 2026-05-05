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
}

// BitableStoreInterface Bitable 存储接口
type BitableStoreInterface interface {
	UpsertDecision(node *decision.DecisionNode) error
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
		return pe.applyConflict(mut)
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
	// 读取现有决策
	existing, err := pe.GitStorage.ReadDecision("", "", mut.SDRID)
	if err != nil {
		return err
	}

	// 应用字段变更
	for k, v := range mut.FieldChanges {
		// 简化的字段更新
		switch k {
		case "title":
			existing.Title = v.(string)
		case "decision":
			existing.Decision = v.(string)
		case "rationale":
			existing.Rationale = v.(string)
		}
	}

	// 写回
	hash, err := pe.GitStorage.WriteDecision(existing)
	if err != nil {
		return err
	}
	existing.GitCommitHash = hash

	// 更新内存图
	pe.MemoryGraph.UpsertDecision(existing, existing.Project)

	return nil
}

func (pe *PipelineEngine) applyStatusChange(mut *signal.DecisionMutation) error {
	existing, err := pe.GitStorage.ReadDecision("", "", mut.SDRID)
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

func (pe *PipelineEngine) applyConflict(mut *signal.DecisionMutation) error {
	// 冲突处理，记录冲突关系
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
