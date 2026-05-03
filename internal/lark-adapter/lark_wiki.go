package larkadapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// --- Snapshot 数据结构 ---

// WikiSnapshot 知识库状态的完整快照
type WikiSnapshot struct {
	Spaces []SpaceSnapshot `json:"spaces"`
}

// SpaceSnapshot 单个知识空间的快照
type SpaceSnapshot struct {
	SpaceID   string                  `json:"space_id"`
	SpaceName string                  `json:"space_name"`
	Nodes     map[string]NodeSnapshot `json:"nodes"` // keyed by node_token
}

// NodeSnapshot 单个知识库节点的快照
type NodeSnapshot struct {
	NodeToken       string `json:"node_token"`
	Title           string `json:"title"`
	ParentNodeToken string `json:"parent_node_token"`
	SpaceID         string `json:"space_id"`
	ObjEditTime     int64  `json:"obj_edit_time"`
	NodeCreateTime  int64  `json:"node_create_time"`
}

// WikiExtractor 知识库提取器
type WikiExtractor struct {
	config       *Config
	cli          *LarkCLI
	contentCache map[string]string
	cacheLock    sync.RWMutex
}

// NewWikiExtractor 创建知识库提取器
func NewWikiExtractor(cfg *Config) *WikiExtractor {
	return &WikiExtractor{
		config:       cfg,
		cli:          NewLarkCLI(),
		contentCache: make(map[string]string),
	}
}

// Name 实现 Extractor 接口
func (e *WikiExtractor) Name() string {
	return "lark_wiki"
}

// snapshotFilePath 返回快照文件路径
func (e *WikiExtractor) snapshotFilePath() string {
	return filepath.Join(StateDir(), "lark_wiki_snapshot.json")
}

// loadSnapshot 从磁盘加载上次快照
func (e *WikiExtractor) loadSnapshot() (*WikiSnapshot, error) {
	data, err := os.ReadFile(e.snapshotFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var snap WikiSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// saveSnapshot 保存当前快照到磁盘
func (e *WikiExtractor) saveSnapshot(snap *WikiSnapshot) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(e.snapshotFilePath(), data, 0644)
}

// buildCurrentState 获取当前所有知识空间及其节点的完整状态
func (e *WikiExtractor) buildCurrentState() (*WikiSnapshot, error) {
	spacesResult, err := e.getSpaces()
	if err != nil {
		return nil, err
	}

	spacesData := e.parseSpaceIDs(spacesResult)
	snap := &WikiSnapshot{}

	for _, space := range spacesData {
		spaceID := space["space_id"].(string)
		spaceName := space["space_name"].(string)

		ss := SpaceSnapshot{
			SpaceID:   spaceID,
			SpaceName: spaceName,
			Nodes:     make(map[string]NodeSnapshot),
		}

		nodesResult, err := e.getSpaceNodes(spaceID)
		if err != nil {
			// 空间不可达，仍然记录到快照中（空节点列表），避免误判为删除
			snap.Spaces = append(snap.Spaces, ss)
			continue
		}

		nodesMap, ok := nodesResult.(map[string]any)
		if !ok {
			snap.Spaces = append(snap.Spaces, ss)
			continue
		}
		data, ok := nodesMap["data"].(map[string]any)
		if !ok {
			snap.Spaces = append(snap.Spaces, ss)
			continue
		}
		items, ok := data["items"].([]any)
		if !ok {
			snap.Spaces = append(snap.Spaces, ss)
			continue
		}

		for _, item := range items {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}

			nodeToken, _ := itemMap["node_token"].(string)
			if nodeToken == "" {
				continue
			}

			title, _ := itemMap["title"].(string)
			parentToken, _ := itemMap["parent_node_token"].(string)
			createTimeStr, _ := itemMap["node_create_time"].(string)
			editTimeStr, _ := itemMap["obj_edit_time"].(string)

			ss.Nodes[nodeToken] = NodeSnapshot{
				NodeToken:       nodeToken,
				Title:           title,
				ParentNodeToken: parentToken,
				SpaceID:         spaceID,
				ObjEditTime:     parseNodeTime(editTimeStr),
				NodeCreateTime:  parseNodeTime(createTimeStr),
			}
		}

		snap.Spaces = append(snap.Spaces, ss)
	}

	return snap, nil
}

// Detect 检测知识库状态变化（新增/删除知识库、新增/删除/移动/更新文档）
func (e *WikiExtractor) Detect(lastCheck time.Time) (*DetectResult, error) {
	cutoff := lastCheck.Unix()

	// 1. 获取当前状态
	current, err := e.buildCurrentState()
	if err != nil {
		result := &DetectResult{
			Source:     e.Name(),
			HasChanges: false,
			DetectedAt: time.Now(),
			LastCheck:  lastCheck,
		}
		_ = SaveDetectResult(result)
		return result, nil
	}

	// 2. 加载上次快照
	previous, err := e.loadSnapshot()
	if err != nil || previous == nil {
		// 首次检测或快照损坏：保存基线快照，不报告变化
		_ = e.saveSnapshot(current)
		result := &DetectResult{
			Source:     e.Name(),
			HasChanges: false,
			DetectedAt: time.Now(),
			LastCheck:  lastCheck,
		}
		_ = SaveDetectResult(result)
		return result, nil
	}

	// 3. 构建快照 lookup map
	prevSpaceMap := make(map[string]*SpaceSnapshot, len(previous.Spaces))
	for i := range previous.Spaces {
		prevSpaceMap[previous.Spaces[i].SpaceID] = &previous.Spaces[i]
	}

	currSpaceMap := make(map[string]*SpaceSnapshot, len(current.Spaces))
	for i := range current.Spaces {
		currSpaceMap[current.Spaces[i].SpaceID] = &current.Spaces[i]
	}

	var changes []Change

	// === 3a. 知识库级别 diff ===

	// 新增知识库
	for _, cs := range current.Spaces {
		if _, exists := prevSpaceMap[cs.SpaceID]; !exists {
			changes = append(changes, Change{
				Type:       "new",
				EntityType: "wiki_space",
				EntityID:   cs.SpaceID,
				Summary:    fmt.Sprintf("新增知识库: %s", cs.SpaceName),
			})
		}
	}

	// 删除知识库
	for _, ps := range previous.Spaces {
		if _, exists := currSpaceMap[ps.SpaceID]; !exists {
			changes = append(changes, Change{
				Type:       "deleted",
				EntityType: "wiki_space",
				EntityID:   ps.SpaceID,
				Summary:    fmt.Sprintf("删除知识库: %s", ps.SpaceName),
			})
		}
	}

	// === 3b-3d. 节点级别 diff（仅对前后都存在的空间） ===

	for _, cs := range current.Spaces {
		ps, exists := prevSpaceMap[cs.SpaceID]
		if !exists {
			// 新增空间的节点由 space-level change 体现，避免重复
			continue
		}

		// 3b. 新增节点 + 3c 移动检测 + 3d 内容更新检测
		for nodeToken, cn := range cs.Nodes {
			pn, nodeExists := ps.Nodes[nodeToken]
			if !nodeExists {
				// 3b. 新增节点
				changes = append(changes, Change{
					Type:       "new",
					EntityType: "wiki_node",
					EntityID:   nodeToken,
					Summary:    fmt.Sprintf("知识库[%s]新增文档: %s", cs.SpaceName, cn.Title),
					Timestamp:  cn.NodeCreateTime,
				})
			} else {
				// 3c. 节点移动检测
				if cn.ParentNodeToken != pn.ParentNodeToken {
					changes = append(changes, Change{
						Type:       "moved",
						EntityType: "wiki_node",
						EntityID:   nodeToken,
						Summary:    fmt.Sprintf("知识库[%s]文档移动位置: %s", cs.SpaceName, cn.Title),
					})
				}

				// 3d. 节点内容更新检测
				if cn.ObjEditTime > cutoff && cn.ObjEditTime > pn.ObjEditTime {
					changes = append(changes, Change{
						Type:       "updated",
						EntityType: "wiki_node",
						EntityID:   nodeToken,
						Summary:    fmt.Sprintf("知识库[%s]文档内容更新: %s", cs.SpaceName, cn.Title),
						Timestamp:  cn.ObjEditTime,
					})
				}
			}
		}

		// 3b. 删除节点
		for nodeToken, pn := range ps.Nodes {
			if _, exists := cs.Nodes[nodeToken]; !exists {
				changes = append(changes, Change{
					Type:       "deleted",
					EntityType: "wiki_node",
					EntityID:   nodeToken,
					Summary:    fmt.Sprintf("知识库[%s]删除文档: %s", cs.SpaceName, pn.Title),
				})
			}
		}
	}

	// 4. 保存当前快照供下次对比
	_ = e.saveSnapshot(current)

	result := &DetectResult{
		Source:     e.Name(),
		HasChanges: len(changes) > 0,
		DetectedAt: time.Now(),
		LastCheck:  lastCheck,
		Changes:    changes,
	}

	_ = SaveDetectResult(result)
	return result, nil
}

// Extract 提取知识库文档信息（全量提取，保持不变）
func (e *WikiExtractor) Extract() error {
	rawData := make(map[string]any)
	errors := make(map[string]string)

	spacesResult, err := e.getSpaces()
	if err == nil {
		rawData["spaces"] = spacesResult

		spacesData := []map[string]any{}
		spacesMap, ok := spacesResult.(map[string]any)
		if ok {
			if data, ok := spacesMap["data"].(map[string]any); ok {
				if items, ok := data["items"].([]any); ok {
					for _, item := range items {
						if itemMap, ok := item.(map[string]any); ok {
							spaceID, _ := itemMap["space_id"].(string)
							spaceName, _ := itemMap["name"].(string)

							if spaceID == "" {
								continue
							}

							nodes, err := e.getSpaceNodes(spaceID)
							spaceInfo := map[string]any{
								"space_id":   spaceID,
								"space_name": spaceName,
							}
							if err == nil {
								spaceInfo["documents"] = nodes
							} else {
								spaceInfo["error"] = err.Error()
							}
							spacesData = append(spacesData, spaceInfo)
						}
					}
				}
			}
		}
		rawData["spaces_documents"] = spacesData
	} else {
		errors["spaces"] = err.Error()
	}

	if len(errors) > 0 {
		rawData["_errors"] = errors
	}

	formatted := map[string]any{
		"extracted": true,
	}

	result := &ExtractionResult{
		Source:      e.Name(),
		ExtractedAt: time.Now(),
		RawData:     rawData,
		Formatted:   formatted,
	}

	if err := SaveToJSON(e.Name(), result); err != nil {
		return fmt.Errorf("save result failed: %w", err)
	}

	return nil
}

func (e *WikiExtractor) getSpaces() (any, error) {
	output, err := e.cli.RunCommand("wiki", "spaces", "list")
	if err != nil {
		return nil, err
	}

	var result any
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (e *WikiExtractor) getSpaceNodes(spaceID string) (any, error) {
	output, err := e.cli.RunCommand(
		"wiki", "nodes", "list",
		"--params", fmt.Sprintf(`{"space_id": "%s"}`, spaceID),
		"--page-all",
	)
	if err != nil {
		output, err = e.cli.RunCommand(
			"wiki", "nodes", "list",
			"--params", fmt.Sprintf(`{"space_id": "%s"}`, spaceID),
		)
		if err != nil {
			return nil, err
		}
	}

	var result any
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func (e *WikiExtractor) parseSpaceIDs(spacesResult any) []map[string]any {
	var spaces []map[string]any
	spacesMap, ok := spacesResult.(map[string]any)
	if !ok {
		return spaces
	}
	data, ok := spacesMap["data"].(map[string]any)
	if !ok {
		return spaces
	}
	items, ok := data["items"].([]any)
	if !ok {
		return spaces
	}
	for _, item := range items {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}
		spaceID, _ := itemMap["space_id"].(string)
		spaceName, _ := itemMap["name"].(string)
		if spaceID == "" {
			continue
		}
		spaces = append(spaces, map[string]any{
			"space_id":   spaceID,
			"space_name": spaceName,
		})
	}
	return spaces
}

func parseNodeTime(timeStr string) int64 {
	if ts, err := strconv.ParseInt(timeStr, 10, 64); err == nil {
		return ts
	}
	t, err := time.Parse(time.RFC3339, timeStr)
	if err == nil {
		return t.Unix()
	}
	return 0
}

// FetchWikiNodeContent 获取知识库节点内容
func (e *WikiExtractor) FetchWikiNodeContent(nodeToken string) (string, error) {
	output, err := e.cli.RunCommand("wiki", "nodes", "get", "--node-token", nodeToken)
	if err != nil {
		return "", err
	}

	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		return "", err
	}

	if data, ok := result["data"].(map[string]any); ok {
		if markdown, ok := data["markdown"].(string); ok {
			return markdown, nil
		}
	}

	return "", fmt.Errorf("could not extract markdown content")
}

// CompareWikiNodeContent 比较知识库节点内容
func (e *WikiExtractor) CompareWikiNodeContent(nodeToken, oldContent, newContent string) (string, []diffmatchpatch.Diff) {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldContent, newContent, false)
	prettyDiff := dmp.DiffPrettyText(diffs)
	return prettyDiff, diffs
}

// GetWikiNodeContentDiff 获取知识库节点内容变化（自动缓存）
func (e *WikiExtractor) GetWikiNodeContentDiff(nodeToken string) (string, []diffmatchpatch.Diff, error) {
	newContent, err := e.FetchWikiNodeContent(nodeToken)
	if err != nil {
		return "", nil, err
	}

	e.cacheLock.RLock()
	oldContent, hasOld := e.contentCache[nodeToken]
	e.cacheLock.RUnlock()

	if !hasOld {
		e.cacheLock.Lock()
		e.contentCache[nodeToken] = newContent
		e.cacheLock.Unlock()
		return "", nil, nil
	}

	e.cacheLock.Lock()
	e.contentCache[nodeToken] = newContent
	e.cacheLock.Unlock()

	prettyDiff, diffs := e.CompareWikiNodeContent(nodeToken, oldContent, newContent)
	return prettyDiff, diffs, nil
}

// HasWikiNodeContentChanged 检查知识库节点内容是否有变化
func (e *WikiExtractor) HasWikiNodeContentChanged(nodeToken, oldContent, newContent string) bool {
	dmp := diffmatchpatch.New()
	diffs := dmp.DiffMain(oldContent, newContent, false)

	for _, diff := range diffs {
		if diff.Type != diffmatchpatch.DiffEqual {
			return true
		}
	}
	return false
}

// GetWikiNodeContentChangeSummary 获取内容变化摘要
func (e *WikiExtractor) GetWikiNodeContentChangeSummary(diffs []diffmatchpatch.Diff) map[string]int {
	changes := map[string]int{
		"added":   0,
		"deleted": 0,
		"changed": 0,
	}

	for _, diff := range diffs {
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			changes["added"] += len(diff.Text)
		case diffmatchpatch.DiffDelete:
			changes["deleted"] += len(diff.Text)
		}
	}

	if changes["added"] > 0 && changes["deleted"] > 0 {
		changes["changed"] = changes["added"] + changes["deleted"]
	}

	return changes
}
