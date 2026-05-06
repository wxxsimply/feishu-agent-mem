package larkadapter

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Extractor 数据提取器接口
type Extractor interface {
	// Extract 提取数据
	Extract() error
	// Name 提取器名称
	Name() string
}

// Detector 状态变化检测接口
type Detector interface {
	// Detect 检测自 lastCheck 以来的状态变化，只返回增量部分
	Detect(lastCheck time.Time) (*DetectResult, error)
	// Name 检测器名称
	Name() string
}

// DetectResult 检测结果
type DetectResult struct {
	HasChanges bool      `json:"has_changes"`
	Source     string    `json:"source"`
	DetectedAt time.Time `json:"detected_at"`
	LastCheck  time.Time `json:"last_check"`
	Changes    []Change  `json:"changes"`
}

// Change 变化条目
type Change struct {
	Type       string `json:"type"`        // "new" | "updated" | "deleted"
	EntityType string `json:"entity_type"` // "message", "pin", "event", "doc", "task", etc.
	EntityID   string `json:"entity_id"`
	Summary    string `json:"summary"`
	Timestamp  int64  `json:"timestamp,omitempty"`

	// IM 消息上下文
	ChatID      string   `json:"chat_id,omitempty"`      // 群聊 ID
	ThreadID    string   `json:"thread_id,omitempty"`    // 线程 ID
	SenderID    string   `json:"sender_id,omitempty"`    // 发送者 open_id
	SenderName  string   `json:"sender_name,omitempty"`  // 发送者姓名
	MentionIDs  []string `json:"mention_ids,omitempty"`  // @提及的用户 ID
	RawContent  string   `json:"raw_content,omitempty"`  // 原始消息全文
	ContextText string            `json:"context_text,omitempty"`
	CommentID  string             `json:"comment_id,omitempty"` // LLM 拼接上下文文本
	Meta        map[string]string `json:"meta,omitempty"`         // 附加元数据（如实际 doc token）
}

// ExtractionResult 提取结果
type ExtractionResult struct {
	Source      string    `json:"source"`
	ExtractedAt time.Time `json:"extracted_at"`
	RawData     any       `json:"raw_data"`
	Formatted   any       `json:"formatted"`
}

// SaveToJSON 保存提取结果到 JSON 文件
func SaveToJSON(sourceName string, result *ExtractionResult) error {
	outputDir := "outputs"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	filename := filepath.Join(outputDir, sourceName+".json")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

// StateManager 状态追踪管理器
type StateManager struct {
	mu       sync.Mutex
	filePath string
	state    map[string]SourceState
}

// SourceState 单个数据源的检测状态
type SourceState struct {
	LastCheck    time.Time `json:"last_check"`
	LastDetected time.Time `json:"last_detected"`
	Version      int       `json:"version"`
}

// GetLastCheck 获取指定源的最后检测时间
func (sm *StateManager) GetLastCheck(source string) time.Time {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s, ok := sm.state[source]
	if !ok {
		return time.Time{} // zero time indicates never checked
	}
	return s.LastCheck
}

// GetLastDetected 获取指定源最后检测到变化的时间
func (sm *StateManager) GetLastDetected(source string) time.Time {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s, ok := sm.state[source]
	if !ok {
		return time.Time{} // zero time indicates never detected changes
	}
	return s.LastDetected
}

// UpdateLastCheck 更新指定源的检测时间（每次检测都调用）
func (sm *StateManager) UpdateLastCheck(source string, t time.Time) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s, ok := sm.state[source]
	if !ok {
		s = SourceState{Version: 1}
	}
	s.LastCheck = t
	s.Version++
	sm.state[source] = s

	return sm.save()
}

// UpdateLastDetected 更新指定源最后检测到变化的时间（仅当检测到变化时调用）
func (sm *StateManager) UpdateLastDetected(source string, t time.Time) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	s, ok := sm.state[source]
	if !ok {
		s = SourceState{Version: 1}
	}
	s.LastDetected = t
	s.Version++
	sm.state[source] = s

	return sm.save()
}

func (sm *StateManager) load() error {
	data, err := os.ReadFile(sm.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			sm.state = make(map[string]SourceState)
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &sm.state)
}

func (sm *StateManager) save() error {
	data, err := json.MarshalIndent(sm.state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sm.filePath, data, 0644)
}

// NewStateManager 创建状态管理器
func NewStateManager(filePath string) *StateManager {
	sm := &StateManager{
		filePath: filePath,
		state:    make(map[string]SourceState),
	}
	_ = sm.load()
	return sm
}

// StateDir 返回用于存储状态文件的目录，并确保目录存在
func StateDir() string {
	dir := "outputs"
	_ = os.MkdirAll(dir, 0755)
	return dir
}

// ExtractDetect 封装：先 Detect 检测变化，如有变化再 Extract 提取详情
func ExtractDetect(d Detector) (*DetectResult, error) {
	sm := NewStateManager(filepath.Join(StateDir(), "detect_state.json"))
	lastCheck := sm.GetLastCheck(d.Name())

	result, err := d.Detect(lastCheck)
	if err != nil {
		return nil, err
	}

	// 更新检测时间
	_ = sm.UpdateLastCheck(d.Name(), time.Now())

	return result, nil
}

// SaveDetectResult 保存检测结果到文件
func SaveDetectResult(result *DetectResult) error {
	filename := filepath.Join(StateDir(), result.Source+"_detect.json")
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
