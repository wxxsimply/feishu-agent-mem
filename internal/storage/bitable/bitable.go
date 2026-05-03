package bitable

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"feishu-mem/internal/decision"
	larkadapter "feishu-mem/internal/lark-adapter"
)

// Config Bitable 配置
type Config struct {
	BaseToken string       `json:"base_token"`
	Tables    TablesConfig `json:"tables"`
}

// TablesConfig 表配置
type TablesConfig struct {
	Decision string `json:"decision"`
	Topic    string `json:"topic"`
	Phase    string `json:"phase"`
	Relation string `json:"relation"`
}

// BitableStore 飞书多维表格存储
type BitableStore struct {
	cli    *larkadapter.LarkCLI
	config Config
}

// NewBitableStore 创建 Bitable 存储
func NewBitableStore(config Config, cli *larkadapter.LarkCLI) *BitableStore {
	return &BitableStore{
		config: config,
		cli:    cli,
	}
}

// BitableResponse Bitable API 响应结构
type BitableResponse struct {
	Ok     bool       `json:"ok"`
	Data   BitableData `json:"data"`
}

// BitableData Bitable 数据
type BitableData struct {
	Data           [][]interface{} `json:"data"`
	Fields         []string        `json:"fields"`
	RecordIDList   []string        `json:"record_id_list"`
}

// RecordWithID 带 record_id 的记录
type RecordWithID struct {
	RecordID string
	SDRID    string
}

// UpsertDecision 插入或更新决策记录
func (bs *BitableStore) UpsertDecision(node *decision.DecisionNode) error {
	if bs.config.BaseToken == "" || bs.config.Tables.Decision == "" {
		return fmt.Errorf("bitable config not set: base_token or decision table missing")
	}

	log.Printf("[Bitable] 写入决策: %s", node.SDRID)

	// 先查找是否存在该 sdr_id 的记录
	existingRecordID, err := bs.findRecordIDBySDRID(node.SDRID)
	if err != nil {
		log.Printf("[Bitable] 查找记录失败: %v", err)
	}

	fields := map[string]interface{}{
		"sdr_id":          node.SDRID,
		"title":           node.Title,
		"topic":           node.Topic,
		"status":          string(node.Status),
		"impact_level":    string(node.ImpactLevel),
		"decision":        node.Decision,
		"proposer":        node.Proposer,
		"executor":        node.Executor,
		"git_commit_hash": node.GitCommitHash,
		"created_at":      node.CreatedAt.Format("2006-01-02 15:04:05"),
	}

	// AccessStats 字段
	fields["hot_score"] = node.AccessStats.HotScore
	fields["access_count"] = node.AccessStats.AccessCount
	fields["reference_count"] = node.AccessStats.ReferenceCount
	if node.AccessStats.LastAccessedAt != nil {
		fields["last_accessed_at"] = node.AccessStats.LastAccessedAt.Format("2006-01-02 15:04:05")
	}
	if node.AccessStats.LastCalculated != nil {
		fields["last_calculated"] = node.AccessStats.LastCalculated.Format("2006-01-02 15:04:05")
	}

	payload, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	log.Printf("[Bitable] Upsert payload: %s", string(payload))

	args := []string{
		"base", "+record-upsert",
		"--base-token", bs.config.BaseToken,
		"--table-id", bs.config.Tables.Decision,
		"--json", string(payload),
	}

	// 如果找到已有记录，用 record_id 来更新
	if existingRecordID != "" {
		log.Printf("[Bitable] 更新已有记录: %s", existingRecordID)
		args = append(args, "--record-id", existingRecordID)
	} else {
		log.Printf("[Bitable] 创建新记录")
	}

	_, err = bs.cli.RunCommand(args...)
	return err
}

// findRecordIDBySDRID 根据 sdr_id 查找 record_id
func (bs *BitableStore) findRecordIDBySDRID(sdrID string) (string, error) {
	log.Printf("[Bitable] 查找 sdr_id: %s", sdrID)

	output, err := bs.cli.RunCommand(
		"base", "+record-list",
		"--base-token", bs.config.BaseToken,
		"--table-id", bs.config.Tables.Decision,
	)
	if err != nil {
		return "", err
	}

	var resp BitableResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}

	fieldIndex := make(map[string]int)
	for i, field := range resp.Data.Fields {
		fieldIndex[field] = i
	}

	sdrIDIndex, hasSDRID := fieldIndex["sdr_id"]
	if !hasSDRID {
		return "", fmt.Errorf("找不到 sdr_id 字段")
	}

	for i, row := range resp.Data.Data {
		if sdrIDIndex < len(row) {
			if s, ok := row[sdrIDIndex].(string); ok && s == sdrID {
				if i < len(resp.Data.RecordIDList) {
					log.Printf("[Bitable] 找到记录: sdr_id=%s, record_id=%s", sdrID, resp.Data.RecordIDList[i])
					return resp.Data.RecordIDList[i], nil
				}
			}
		}
	}

	log.Printf("[Bitable] 未找到记录: sdr_id=%s", sdrID)
	return "", nil
}

// ListAllDecisions 列出所有决策
func (bs *BitableStore) ListAllDecisions() ([]*decision.DecisionNode, error) {
	if bs.config.BaseToken == "" || bs.config.Tables.Decision == "" {
		return []*decision.DecisionNode{}, nil
	}

	log.Printf("[Bitable] 开始获取所有决策...")

	output, err := bs.cli.RunCommand(
		"base", "+record-list",
		"--base-token", bs.config.BaseToken,
		"--table-id", bs.config.Tables.Decision,
	)
	if err != nil {
		return []*decision.DecisionNode{}, err
	}

	var resp BitableResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return []*decision.DecisionNode{}, fmt.Errorf("解析响应失败: %w", err)
	}

	log.Printf("[Bitable] 字段列表: %v", resp.Data.Fields)
	log.Printf("[Bitable] 记录数: %d", len(resp.Data.Data))

	return bs.parseDecisionsFromResponse(&resp.Data), nil
}

func (bs *BitableStore) parseDecisionsFromResponse(data *BitableData) []*decision.DecisionNode {
	var decisions []*decision.DecisionNode
	fieldIndex := make(map[string]int)
	for i, field := range data.Fields {
		fieldIndex[field] = i
	}

	for _, row := range data.Data {
		d := &decision.DecisionNode{}

		if idx, ok := fieldIndex["sdr_id"]; ok && idx < len(row) {
			if s, ok := row[idx].(string); ok {
				d.SDRID = s
			}
		}
		if idx, ok := fieldIndex["git_commit_hash"]; ok && idx < len(row) {
			if s, ok := row[idx].(string); ok {
				d.GitCommitHash = s
			}
		}
		if idx, ok := fieldIndex["title"]; ok && idx < len(row) {
			if s, ok := row[idx].(string); ok {
				d.Title = s
			}
		}
		if idx, ok := fieldIndex["decision"]; ok && idx < len(row) {
			if s, ok := row[idx].(string); ok {
				d.Decision = s
			}
		}
		if idx, ok := fieldIndex["topic"]; ok && idx < len(row) {
			if s, ok := row[idx].(string); ok {
				d.Topic = s
			}
		}
		if idx, ok := fieldIndex["impact_level"]; ok && idx < len(row) {
			if s, ok := row[idx].(string); ok {
				d.ImpactLevel = decision.ImpactLevel(s)
			}
		}
		if idx, ok := fieldIndex["status"]; ok && idx < len(row) {
			if s, ok := row[idx].(string); ok {
				d.Status = decision.DecisionStatus(s)
			}
		}
		if idx, ok := fieldIndex["proposer"]; ok && idx < len(row) {
			if s, ok := row[idx].(string); ok {
				d.Proposer = s
			}
		}
		if idx, ok := fieldIndex["executor"]; ok && idx < len(row) {
			if s, ok := row[idx].(string); ok {
				d.Executor = s
			}
		}
		if idx, ok := fieldIndex["created_at"]; ok && idx < len(row) {
			if s, ok := row[idx].(string); ok {
				if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
					d.CreatedAt = t
				}
			}
		}

		// AccessStats 字段解析
		if idx, ok := fieldIndex["hot_score"]; ok && idx < len(row) {
			if val, ok := row[idx].(float64); ok {
				d.AccessStats.HotScore = val
			}
		}
		if idx, ok := fieldIndex["access_count"]; ok && idx < len(row) {
			if val, ok := row[idx].(float64); ok {
				d.AccessStats.AccessCount = int(val)
			}
		}
		if idx, ok := fieldIndex["reference_count"]; ok && idx < len(row) {
			if val, ok := row[idx].(float64); ok {
				d.AccessStats.ReferenceCount = int(val)
			}
		}
		if idx, ok := fieldIndex["last_accessed_at"]; ok && idx < len(row) {
			if s, ok := row[idx].(string); ok {
				if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
					d.AccessStats.LastAccessedAt = &t
				}
			}
		}
		if idx, ok := fieldIndex["last_calculated"]; ok && idx < len(row) {
			if s, ok := row[idx].(string); ok {
				if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
					d.AccessStats.LastCalculated = &t
				}
			}
		}

		if d.SDRID != "" {
			log.Printf("[Bitable] 解析到决策: %s (hash: %q)", d.SDRID, d.GitCommitHash)
			decisions = append(decisions, d)
		}
	}

	return decisions
}

// QueryByTopic 按主题查询
func (bs *BitableStore) QueryByTopic(topic, status string) ([]*decision.DecisionNode, error) {
	allDecisions, err := bs.ListAllDecisions()
	if err != nil {
		return nil, err
	}

	var filtered []*decision.DecisionNode
	for _, d := range allDecisions {
		if topic != "" && d.Topic != topic {
			continue
		}
		if status != "" && string(d.Status) != status {
			continue
		}
		filtered = append(filtered, d)
	}
	return filtered, nil
}

// QueryCrossTopic 跨主题查询
func (bs *BitableStore) QueryCrossTopic(topic string) ([]*decision.DecisionNode, error) {
	return []*decision.DecisionNode{}, nil
}

// QueryByPhase 按阶段查询
func (bs *BitableStore) QueryByPhase(phase string) ([]*decision.DecisionNode, error) {
	return []*decision.DecisionNode{}, nil
}

// ListTopics 列出所有主题
func (bs *BitableStore) ListTopics() ([]TopicDef, error) {
	return []TopicDef{}, nil
}

// SearchContent 全文搜索 Bitable 记录
func (bs *BitableStore) SearchContent(query string, topic string) ([]*decision.DecisionNode, error) {
	allDecisions, err := bs.ListAllDecisions()
	if err != nil {
		return nil, err
	}

	var filtered []*decision.DecisionNode
	for _, d := range allDecisions {
		if topic != "" && d.Topic != topic {
			continue
		}
		if query != "" &&
			!strings.Contains(strings.ToLower(d.Title), strings.ToLower(query)) &&
			!strings.Contains(strings.ToLower(d.Decision), strings.ToLower(query)) {
			continue
		}
		filtered = append(filtered, d)
	}
	return filtered, nil
}

// TopicDef 主题定义
type TopicDef struct {
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}
