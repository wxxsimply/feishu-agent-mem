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
		log.Printf("[Bitable] 配置不完整，跳过写入")
		return nil
	}

	log.Printf("[Bitable] 写入决策: %s", node.SDRID)

	existingRecordID, err := bs.findRecordIDBySDRID(node.SDRID)
	if err != nil {
		log.Printf("[Bitable] 查找记录失败: %v", err)
	}

	fields := buildDecisionFields(node)
	fields["conflict_status"] = "none"
	fields["conflict_sdr_ids"] = ""

	return bs.upsertRecord(fields, existingRecordID)
}

// findRecordIDBySDRID 根据 sdr_id 查找 record_id
func (bs *BitableStore) findRecordIDBySDRID(sdrID string) (string, error) {
	if bs.config.BaseToken == "" || bs.config.Tables.Decision == "" {
		return "", nil
	}

	log.Printf("[Bitable] 查找 sdr_id: %s", sdrID)

	output, err := bs.cli.RunCommand(
		"base", "+record-list",
		"--base-token", bs.config.BaseToken,
		"--table-id", bs.config.Tables.Decision,
	)
	if err != nil {
		log.Printf("[Bitable] 调用 record-list 失败: %v", err)
		return "", err
	}

	if len(output) > 0 && output[0] == '`' {
		log.Printf("[Bitable] 收到 Markdown 格式响应，跳过查找")
		return "", nil
	}

	var resp BitableResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		log.Printf("[Bitable] JSON 解析失败: %v (前200字符: %s)", err, string(output[:min(200, len(output))]))
		return "", nil
	}

	fieldIndex := make(map[string]int)
	for i, field := range resp.Data.Fields {
		fieldIndex[field] = i
	}

	sdrIDIndex, hasSDRID := fieldIndex["sdr_id"]
	if !hasSDRID {
		log.Printf("[Bitable] 找不到 sdr_id 字段")
		return "", nil
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
		log.Printf("[Bitable] 解析响应失败: %v", err)
		return []*decision.DecisionNode{}, nil
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
				} else if t, err := time.Parse("2006/01/02", s); err == nil {
					d.CreatedAt = t
				}
			}
		}

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

		if d.SDRID != "" {
			log.Printf("[Bitable] 解析到决策: %s", d.SDRID)
			decisions = append(decisions, d)
		}
	}

	return decisions
}

// UpsertDecisionWithConflict 插入决策并标记冲突状态
func (bs *BitableStore) UpsertDecisionWithConflict(node *decision.DecisionNode, conflictSDRID string) error {
	if bs.config.BaseToken == "" || bs.config.Tables.Decision == "" {
		log.Printf("[Bitable] 配置不完整，跳过写入")
		return nil
	}

	log.Printf("[Bitable] 写入冲突决策: %s (冲突: %s)", node.SDRID, conflictSDRID)

	existingRecordID, err := bs.findRecordIDBySDRID(node.SDRID)
	if err != nil {
		log.Printf("[Bitable] 查找记录失败: %v", err)
	}

	fields := buildDecisionFields(node)
	fields["conflict_status"] = "active_conflict"
	fields["conflict_sdr_ids"] = conflictSDRID

	return bs.upsertRecord(fields, existingRecordID)
}

// UpdateConflictFields 更新已有决策的冲突字段
func (bs *BitableStore) UpdateConflictFields(sdrID, newConflictSDRID string) error {
	if bs.config.BaseToken == "" || bs.config.Tables.Decision == "" {
		return fmt.Errorf("bitable config not set")
	}

	log.Printf("[Bitable] 更新冲突字段: %s <- %s", sdrID, newConflictSDRID)

	existingRecordID, err := bs.findRecordIDBySDRID(sdrID)
	if err != nil || existingRecordID == "" {
		log.Printf("[Bitable] 找不到记录: %s", sdrID)
		return nil
	}

	existingSDRIDs := bs.getConflictSDRIDs(sdrID)
	found := false
	for _, id := range existingSDRIDs {
		if id == newConflictSDRID {
			found = true
			break
		}
	}
	if !found {
		existingSDRIDs = append(existingSDRIDs, newConflictSDRID)
	}

	fields := map[string]interface{}{
		"conflict_status":  "active_conflict",
		"conflict_sdr_ids": strings.Join(existingSDRIDs, ","),
	}

	return bs.upsertRecord(fields, existingRecordID)
}

// getConflictSDRIDs 获取指定决策当前的冲突 SDRID 列表
func (bs *BitableStore) getConflictSDRIDs(sdrID string) []string {
	output, err := bs.cli.RunCommand(
		"base", "+record-list",
		"--base-token", bs.config.BaseToken,
		"--table-id", bs.config.Tables.Decision,
	)
	if err != nil {
		return nil
	}

	var resp BitableResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil
	}

	fieldIndex := make(map[string]int)
	for i, field := range resp.Data.Fields {
		fieldIndex[field] = i
	}

	sdrIDIndex, hasSDRID := fieldIndex["sdr_id"]
	conflictIndex, hasConflict := fieldIndex["conflict_sdr_ids"]
	if !hasSDRID || !hasConflict {
		return nil
	}

	for _, row := range resp.Data.Data {
		if sdrIDIndex < len(row) {
			if s, ok := row[sdrIDIndex].(string); ok && s == sdrID {
				if conflictIndex < len(row) {
					if s, ok := row[conflictIndex].(string); ok && s != "" {
						return strings.Split(s, ",")
					}
				}
				return nil
			}
		}
	}
	return nil
}

// buildDecisionFields 构建决策字段 map
func buildDecisionFields(node *decision.DecisionNode) map[string]interface{} {
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
		"hot_score":       node.AccessStats.HotScore,
		"access_count":    node.AccessStats.AccessCount,
		"reference_count": node.AccessStats.ReferenceCount,
	}

	// 新增字段：决策依据、时间关联、项目阶段
	if node.Rationale != "" {
		fields["rationale"] = node.Rationale
	}
	if node.ProjectPhase != "" {
		fields["project_phase"] = node.ProjectPhase
	}
	if node.DecisionTime != "" {
		fields["decision_time"] = node.DecisionTime
	}
	if node.EffectiveTime != "" {
		fields["effective_time"] = node.EffectiveTime
	}
	if node.Deadline != "" {
		fields["deadline"] = node.Deadline
	}
	if node.DecisionType != "" {
		fields["decision_type"] = node.DecisionType
	}
	if node.Phase != "" {
		fields["phase"] = node.Phase
	}

	// 飞书关联
	if len(node.FeishuLinks.RelatedChatIDs) > 0 {
		fields["related_chat_ids"] = strings.Join(node.FeishuLinks.RelatedChatIDs, ",")
	}
	if len(node.FeishuLinks.RelatedDocTokens) > 0 {
		fields["related_doc_tokens"] = strings.Join(node.FeishuLinks.RelatedDocTokens, ",")
	}

	if node.AccessStats.LastAccessedAt != nil {
		fields["last_accessed_at"] = node.AccessStats.LastAccessedAt.Format("2006-01-02 15:04:05")
	}
	if node.AccessStats.LastCalculated != nil {
		fields["last_calculated"] = node.AccessStats.LastCalculated.Format("2006-01-02 15:04:05")
	}
	return fields
}

// upsertRecord 通用记录 upsert
func (bs *BitableStore) upsertRecord(fields map[string]interface{}, recordID string) error {
	if bs.config.BaseToken == "" || bs.config.Tables.Decision == "" {
		return nil
	}

	payload, err := json.Marshal(fields)
	if err != nil {
		return err
	}
	log.Printf("[Bitable] 写入 payload: %s", string(payload))

	args := []string{
		"base", "+record-upsert",
		"--base-token", bs.config.BaseToken,
		"--table-id", bs.config.Tables.Decision,
		"--json", string(payload),
	}
	if recordID != "" {
		args = append(args, "--record-id", recordID)
	}

	output, err := bs.cli.RunCommand(args...)
	if err != nil {
		log.Printf("[Bitable] Upsert 失败: %v", err)
		log.Printf("[Bitable] 输出: %s", string(output))
	}
	return err
}

// ClearConflictFields 清除决策的冲突标记
func (bs *BitableStore) ClearConflictFields(sdrID string) error {
	if bs.config.BaseToken == "" || bs.config.Tables.Decision == "" {
		return fmt.Errorf("bitable config not set")
	}

	log.Printf("[Bitable] 清除冲突标记: %s", sdrID)

	existingRecordID, err := bs.findRecordIDBySDRID(sdrID)
	if err != nil || existingRecordID == "" {
		log.Printf("[Bitable] 找不到记录: %s", sdrID)
		return nil
	}

	fields := map[string]interface{}{
		"conflict_status":  "none",
		"conflict_sdr_ids": "",
	}
	return bs.upsertRecord(fields, existingRecordID)
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

