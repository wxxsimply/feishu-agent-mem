package larkadapter

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DocEditState 单个文档的编辑状态
type DocEditState struct {
	DocToken      string    `json:"doc_token"`
	LastChange    time.Time `json:"last_change"`    // 最后检测到变更的时间
	FirstDetected time.Time `json:"first_detected"` // 首次检测到变更的时间
	IsStaged      bool      `json:"is_staged"`      // 是否已处理
	ProcessedAt   time.Time `json:"processed_at"`   // 最后处理时间
	ContentHash   string    `json:"content_hash"`   // 最后处理的内容哈希
}

// DocDebounceTracker 文档防抖追踪器
type DocDebounceTracker struct {
	mu             sync.Mutex
	filePath       string
	states         map[string]*DocEditState
	debounceWindow time.Duration
}

// NewDocDebounceTracker 创建文档防抖追踪器
func NewDocDebounceTracker(stateDir string, debounceWindow time.Duration) *DocDebounceTracker {
	tracker := &DocDebounceTracker{
		filePath:       filepath.Join(stateDir, "debounce_state.json"),
		states:         make(map[string]*DocEditState),
		debounceWindow: debounceWindow,
	}
	_ = tracker.load()
	return tracker
}

// OnDocumentChanged 当检测到文档变更时调用
func (t *DocDebounceTracker) OnDocumentChanged(docToken string, contentHash string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	state, exists := t.states[docToken]
	if !exists {
		state = &DocEditState{
			DocToken:      docToken,
			FirstDetected: now,
			IsStaged:      false,
		}
		t.states[docToken] = state
	}

	state.LastChange = now

	// 如果内容哈希变化了，重置 staged 状态
	if contentHash != "" && state.ContentHash != contentHash {
		state.IsStaged = false
		state.ContentHash = contentHash
	}

	_ = t.save()
}

// CanProcessNow 检查文档现在是否可以处理（已过静默期）
func (t *DocDebounceTracker) CanProcessNow(docToken string) (bool, string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	state, exists := t.states[docToken]
	if !exists {
		return true, "no prior state"
	}

	// 检查静默期
	timeSinceChange := time.Since(state.LastChange)
	if timeSinceChange < t.debounceWindow {
		remaining := t.debounceWindow - timeSinceChange
		return false, fmt.Sprintf("waiting for debounce: %v remaining", remaining.Round(time.Second))
	}

	// 检查是否已经处理过相同内容
	if state.IsStaged && state.ContentHash != "" {
		// 内容相同，跳过
		return false, fmt.Sprintf("content already processed at %v", state.ProcessedAt.Format(time.RFC3339))
	}

	return true, "ready"
}

// MarkProcessed 标记文档已处理
func (t *DocDebounceTracker) MarkProcessed(docToken string, contentHash string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.states[docToken]; exists {
		state.IsStaged = true
		state.ProcessedAt = time.Now()
		if contentHash != "" {
			state.ContentHash = contentHash
		}
		_ = t.save()
	}
}

// GetState 获取文档状态（用于调试）
func (t *DocDebounceTracker) GetState(docToken string) *DocEditState {
	t.mu.Lock()
	defer t.mu.Unlock()

	if state, exists := t.states[docToken]; exists {
		// 返回副本
		copy := *state
		return &copy
	}
	return nil
}

// GetAllStates 获取所有文档状态（用于调试）
func (t *DocDebounceTracker) GetAllStates() map[string]*DocEditState {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 返回副本
	result := make(map[string]*DocEditState)
	for k, v := range t.states {
		copy := *v
		result[k] = &copy
	}
	return result
}

func (t *DocDebounceTracker) load() error {
	data, err := os.ReadFile(t.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(data, &t.states)
}

func (t *DocDebounceTracker) save() error {
	data, err := json.MarshalIndent(t.states, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(t.filePath, data, 0644)
}

// ComputeContentHash 计算内容哈希
func ComputeContentHash(content string) string {
	hash := sha256.Sum256([]byte(content))
	return hex.EncodeToString(hash[:])
}
