package larkadapter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// --- Snapshot 数据结构 ---

// CalendarSnapshot 日程状态快照
type CalendarSnapshot struct {
	Events map[string]EventSnapshot `json:"events"` // keyed by event_id
}

// EventSnapshot 单个日程的快照
type EventSnapshot struct {
	EventID   string `json:"event_id"`
	Title     string `json:"title"`
	StartTime int64  `json:"start_time"`
	UpdatedAt int64  `json:"updated_at"`
}

// CalendarExtractor 日程提取器
type CalendarExtractor struct {
	config *Config
	cli    *LarkCLI
}

// NewCalendarExtractor 创建日程提取器
func NewCalendarExtractor(cfg *Config) *CalendarExtractor {
	return &CalendarExtractor{
		config: cfg,
		cli:    NewLarkCLI(),
	}
}

// Name 实现 Extractor 接口
func (e *CalendarExtractor) Name() string {
	return "lark_calendar"
}

// snapshotFilePath 返回快照文件路径
func (e *CalendarExtractor) snapshotFilePath() string {
	return filepath.Join(StateDir(), "lark_calendar_snapshot.json")
}

// loadSnapshot 从磁盘加载上次快照
func (e *CalendarExtractor) loadSnapshot() (*CalendarSnapshot, error) {
	data, err := os.ReadFile(e.snapshotFilePath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var snap CalendarSnapshot
	if err := json.Unmarshal(data, &snap); err != nil {
		return nil, err
	}
	return &snap, nil
}

// saveSnapshot 保存当前快照到磁盘
func (e *CalendarExtractor) saveSnapshot(snap *CalendarSnapshot) error {
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(e.snapshotFilePath(), data, 0644)
}

// buildCurrentState 获取当前日程状态
func (e *CalendarExtractor) buildCurrentState() (*CalendarSnapshot, error) {
	snap := &CalendarSnapshot{
		Events: make(map[string]EventSnapshot),
	}

	// 先尝试直接调用 +agenda（不带日期参数），获取默认日程
	agenda, err := e.getTodayAgenda()
	if err != nil {
		return snap, nil
	}

	events := e.parseAgendaEvents(agenda)
	for _, ev := range events {
		eventID, _ := ev["event_id"].(string)
		if eventID == "" {
			continue
		}

		title, _ := ev["title"].(string)
		startTime, _ := ev["start_time"].(int64)

		snap.Events[eventID] = EventSnapshot{
			EventID:   eventID,
			Title:     title,
			StartTime: startTime,
			UpdatedAt: time.Now().Unix(),
		}
	}

	return snap, nil
}

// BuildCurrentStateForDebug 调试用：获取当前状态
func (e *CalendarExtractor) BuildCurrentStateForDebug() (*CalendarSnapshot, error) {
	return e.buildCurrentState()
}

// Detect 检测日程变化（新增/更新/删除日程、参会人变化、RSVP 变化等）
func (e *CalendarExtractor) Detect(lastCheck time.Time) (*DetectResult, error) {
	changes := []Change{}

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
		// 首次检测：保存基线快照，不报告变化
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

	// 3. 对比找出变化

	// 新增日程
	for eventID, ev := range current.Events {
		if _, exists := previous.Events[eventID]; !exists {
			changes = append(changes, Change{
				Type:       "new_event",
				EntityType: "event",
				EntityID:   eventID,
				Summary:    fmt.Sprintf("新日程: %s", ev.Title),
				Timestamp:  ev.StartTime,
			})
		}
	}

	// 更新日程
	for eventID, currEv := range current.Events {
		prevEv, exists := previous.Events[eventID]
		if exists {
			if currEv.Title != prevEv.Title || currEv.StartTime != prevEv.StartTime {
				changes = append(changes, Change{
					Type:       "updated_event",
					EntityType: "event",
					EntityID:   eventID,
					Summary:    fmt.Sprintf("日程更新: %s", currEv.Title),
					Timestamp:  time.Now().Unix(),
				})
			}
		}
	}

	// 已删除的日程（可选，暂不报告）

	// 4. 保存当前快照
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

// analyzeEventChanges 分析日程的详细变化类型
func (e *CalendarExtractor) analyzeEventChanges(event map[string]any) []string {
	var changes []string

	// 这里我们模拟从事件数据中分析变化类型
	// 实际应用中需要对比历史快照

	// 检查是否有参会人相关字段
	if attendees, ok := event["attendees"]; ok {
		if attList, ok := attendees.([]any); ok && len(attList) > 0 {
			// 如果有参会人数据，可以进一步分析
			changes = append(changes, "attendee_added")
		}
	}

	// 检查是否有会议室
	if rooms, ok := event["rooms"]; ok {
		if roomList, ok := rooms.([]any); ok && len(roomList) > 0 {
			changes = append(changes, "room_added")
		}
	}

	// 检查是否有 RSVP 信息
	if rsvp, ok := event["rsvp_status"]; ok && rsvp != nil {
		changes = append(changes, "rsvp_changed")
	}

	// 如果没有具体变化，默认返回 new
	if len(changes) == 0 {
		changes = append(changes, "new")
	}

	return changes
}

// Extract 提取日程决策信息
func (e *CalendarExtractor) Extract() error {
	rawData := make(map[string]any)
	errors := make(map[string]string)

	if agenda, err := e.getTodayAgenda(); err == nil {
		rawData["today_agenda"] = agenda
	} else {
		errors["today_agenda"] = err.Error()
	}

	if events, err := e.searchEvents(); err == nil {
		rawData["events"] = events
	} else {
		errors["events"] = err.Error()
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

func (e *CalendarExtractor) getTodayAgenda() ([]any, error) {
	today := time.Now().Format("2006-01-02")
	output, err := e.cli.RunCommand("calendar", "+agenda", "--date", today, "--as", "user")
	if err != nil {
		output, err = e.cli.RunCommand("calendar", "+agenda", "--as", "user")
		if err != nil {
			return nil, err
		}
	}

	// 解析 { ok: true, data: [...] } 结构
	var response map[string]any
	if err := json.Unmarshal(output, &response); err != nil {
		return nil, err
	}

	// 提取 data 字段
	if data, ok := response["data"]; ok {
		if dataArray, ok := data.([]any); ok {
			return dataArray, nil
		}
	}
	return []any{}, nil
}

func (e *CalendarExtractor) getAgendaRange(start, end time.Time) ([]any, error) {
	// 逐日检测，使用 +agenda 命令
	var allResults []any
	for d := start; d.Before(end) || d.Equal(end); d = d.AddDate(0, 0, 1) {
		date := d.Format("2006-01-02")
		output, err := e.cli.RunCommand("calendar", "+agenda", "--date", date, "--as", "user")
		if err != nil {
			continue
		}

		// 解析 { ok: true, data: [...] } 结构
		var response map[string]any
		if err := json.Unmarshal(output, &response); err != nil {
			continue
		}

		// 提取 data 字段
		if data, ok := response["data"]; ok {
			if dataArray, ok := data.([]any); ok {
				allResults = append(allResults, dataArray...)
			}
		}
	}
	return allResults, nil
}

func (e *CalendarExtractor) searchEvents() ([]any, error) {
	output, err := e.cli.RunCommand(
		"calendar", "events", "search",
		"--params", `{"query": "会议"}`,
	)
	if err != nil {
		return nil, err
	}

	var result []any
	if err := json.Unmarshal(output, &result); err != nil {
		var single any
		if err := json.Unmarshal(output, &single); err != nil {
			return nil, err
		}
		result = []any{single}
	}
	return result, nil
}

func (e *CalendarExtractor) parseAgendaEvents(agenda []any) []map[string]any {
	var events []map[string]any

	for _, item := range agenda {
		itemMap, ok := item.(map[string]any)
		if !ok {
			continue
		}

		// 尝试获取真实的 event_id
		eventID, _ := itemMap["event_id"].(string)
		if eventID == "" {
			eventID, _ = itemMap["id"].(string)
		}

		// 从 +agenda 输出中提取日程信息
		title, _ := itemMap["summary"].(string)
		if title == "" {
			title, _ = itemMap["title"].(string)
		}
		if title == "" {
			continue
		}

		startTime := parseStartTimeFromAgenda(itemMap)
		if startTime == 0 {
			startTime = time.Now().Unix()
		}

		// 如果没有真实 event_id，用 title+startTime 生成一个
		if eventID == "" {
			eventID = fmt.Sprintf("cal_%s_%d", title, startTime)
		}

		events = append(events, map[string]any{
			"event_id":   eventID,
			"title":      title,
			"start_time": startTime,
		})
	}

	return events
}

func parseStartTimeFromAgenda(m map[string]any) int64 {
	// 尝试直接的方式：start_time.datetime (来自 +agenda 格式
	if startObj, ok := m["start_time"]; ok {
		if startMap, ok := startObj.(map[string]any); ok {
			if datetime, ok := startMap["datetime"].(string); ok {
				t, err := time.Parse(time.RFC3339, datetime)
				if err == nil {
					return t.Unix()
				}
			}
		}
	}
	// 备用方式：start (来自 +create 返回格式
	if startStr, ok := m["start"].(string); ok {
		t, err := time.Parse(time.RFC3339, startStr)
		if err == nil {
			return t.Unix()
		}
	}
	return 0
}
