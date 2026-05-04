package ws

import (
	"encoding/json"
	"time"
)

// 消息类型常量
const (
	MsgTypeHeartbeat      = "heartbeat"
	MsgTypeHeartbeatAck   = "heartbeat_ack"
	MsgTypeDetectResult   = "detect_result"
	MsgTypeRegister       = "register"
	MsgTypeControl        = "control"
	MsgTypeStateSync      = "state_sync"
)

// 控制命令常量
const (
	CmdPause    = "pause"
	CmdResume   = "resume"
	CmdStop     = "stop"
	CmdConfigUpdate = "config_update"
)

// 通用消息结构
type Message struct {
	Type      string          `json:"type"`
	Timestamp int64          `json:"timestamp"`
	Data      json.RawMessage `json:"data,omitempty"`
}

// 检测器注册消息
type RegisterMessage struct {
	DetectorName string         `json:"detector"`
	Version     string         `json:"version"`
	Config      DetectorConfig `json:"config"`
}

// 检测器配置（注册时发送）
type DetectorConfig struct {
	Enabled           bool          `json:"enabled"`
	Interval          string        `json:"interval"`
	BurstInterval     string        `json:"burst_interval"`
	BurstTimeout      string        `json:"burst_timeout"`
	HeartbeatInterval string        `json:"heartbeat_interval"`
}

// 心跳消息
type HeartbeatMessage struct {
	DetectorName string          `json:"detector"`
	Version      string          `json:"version"`
	Status       DetectorStatus  `json:"status"`
}

// 检测器状态
type DetectorStatus struct {
	State         string    `json:"state"` // running, paused, error, disconnected
	LastCheck     string    `json:"last_check"`
	LastChange    string    `json:"last_change"`
	InBurstMode   bool      `json:"in_burst_mode"`
}

// 心跳响应消息
type HeartbeatAckMessage struct {
	ServerTime int64  `json:"server_time"`
	Status     string `json:"status"`
}

// 检测结果消息
type DetectResultMessage struct {
	DetectorName string                 `json:"detector"`
	Result       DetectResult           `json:"result"`
}

// 检测结果数据结构
type DetectResult struct {
	Source      string        `json:"source"`
	HasChanges  bool          `json:"has_changes"`
	DetectedAt string        `json:"detected_at"`
	LastCheck   string        `json:"last_check"`
	Changes     []ChangeItem  `json:"changes"`
}

// 变化项
type ChangeItem struct {
	Type       string `json:"type"`
	EntityType string `json:"entity_type"`
	EntityID   string `json:"entity_id"`
	Summary    string `json:"summary"`
	Timestamp  int64  `json:"timestamp,omitempty"`
	Content    string `json:"content,omitempty"` // 可选，完整内容
}

// 控制命令消息
type ControlMessage struct {
	Command string                 `json:"command"`
	Payload map[string]interface{} `json:"payload,omitempty"`
}

// 全局状态同步消息
type StateSyncMessage struct {
	Timestamp   int64       `json:"timestamp"`
	GlobalState GlobalState `json:"global_state"`
}

// 全局状态
type GlobalState struct {
	Detectors map[string]DetectorGlobalState `json:"detectors"`
}

// 单个检测器的全局状态
type DetectorGlobalState struct {
	State         string       `json:"state"`
	LastHeartbeat string       `json:"last_heartbeat"`
	LastChange    string       `json:"last_change"`
	TotalChanges  int64        `json:"total_changes"`
	TotalErrors   int64        `json:"total_errors"`
	Config        DetectorConfig `json:"config"`
}

// NewMessage 创建一个新消息
func NewMessage(msgType string, data interface{}) (*Message, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	return &Message{
		Type:      msgType,
		Timestamp: time.Now().Unix(),
		Data:      jsonData,
	}, nil
}

// Encode 编码消息为JSON
func (m *Message) Encode() ([]byte, error) {
	return json.Marshal(m)
}

// DecodeMessage 解码JSON为Message
func DecodeMessage(data []byte) (*Message, error) {
	var msg Message
	err := json.Unmarshal(data, &msg)
	return &msg, err
}

// DecodeData 解码消息Data部分到指定结构体
func DecodeData(raw json.RawMessage, v interface{}) error {
	return json.Unmarshal(raw, v)
}
