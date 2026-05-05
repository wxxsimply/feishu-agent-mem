package decision

// RelationType 关系类型
type RelationType string

const (
	RelationDependsOn     RelationType = "DEPENDS_ON"
	RelationSupersedes    RelationType = "SUPERSEDES"
	RelationRefines       RelationType = "REFINES"
	RelationConflictsWith RelationType = "CONFLICTS_WITH"
	RelationRelatesTo     RelationType = "RELATES_TO"
	RelationObjection     RelationType = "OBJECTION"
)

// Relation 关系边
type Relation struct {
	Type        RelationType `json:"type" yaml:"type"`
	TargetSDRID string       `json:"target_sdr_id" yaml:"target_sdr_id"`
	Description string       `json:"description,omitempty" yaml:"description,omitempty"`
}

// NewRelation 创建新的关系
func NewRelation(relType RelationType, targetSDRID, description string) Relation {
	return Relation{
		Type:        relType,
		TargetSDRID: targetSDRID,
		Description: description,
	}
}

// IsValid 验证关系类型是否有效
func (r RelationType) IsValid() bool {
	switch r {
	case RelationDependsOn, RelationSupersedes, RelationRefines, RelationConflictsWith, RelationRelatesTo, RelationObjection:
		return true
	default:
		return false
	}
}
