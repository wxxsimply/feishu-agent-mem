package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// ProjectConfig 项目配置
type ProjectConfig struct {
	Name   string   `yaml:"name"`
	Topics []string `yaml:"topics"`
	Phases []string `yaml:"phases"`
}

// LarkCLIConfig Lark CLI 配置
type LarkCLIConfig struct {
	Bin             string `yaml:"bin"`
	DefaultIdentity string `yaml:"default_identity"`
}

// GitStorageConfig Git 存储配置
type GitStorageConfig struct {
	WorkDir          string                 `yaml:"work_dir"`
	Remote           string                 `yaml:"remote"`
	AutoPush         bool                   `yaml:"auto_push"`
	Branch           string                 `yaml:"branch"`
	ConsistencyCheck ConsistencyCheckConfig `yaml:"consistency_check"`
	Archive          ArchiveConfig          `yaml:"archive"`
	Maintenance      MaintenanceConfig      `yaml:"maintenance"`
}

// ConsistencyCheckConfig 一致性检查配置
type ConsistencyCheckConfig struct {
	Enabled  bool          `yaml:"enabled"`
	Interval time.Duration `yaml:"interval"`
}

// ArchiveConfig 归档配置
type ArchiveConfig struct {
	Enabled bool          `yaml:"enabled"`
	Age     time.Duration `yaml:"age"`
}

// MaintenanceConfig 维护配置
type MaintenanceConfig struct {
	GCGCInterval time.Duration `yaml:"gc_interval"`
}

// BitableConfig Bitable 配置
type BitableConfig struct {
	BaseToken string       `yaml:"base_token"`
	Tables    TablesConfig `yaml:"tables"`
}

// TablesConfig 表配置
type TablesConfig struct {
	Decision string `yaml:"decision"`
	Topic    string `yaml:"topic"`
	Phase    string `yaml:"phase"`
	Relation string `yaml:"relation"`
}

// EventsConfig 事件配置
type EventsConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Subscribe []string `yaml:"subscribe"`
}

// PollingConfig 轮询配置
type PollingConfig struct {
	Interval time.Duration `yaml:"interval"`
}

// MCPConfig MCP 配置
type MCPConfig struct {
	Port         int    `yaml:"port"`
	RegisterPath string `yaml:"register_path"`
}

// MemoryConfig 内存配置
type MemoryConfig struct {
	PreloadOnStart     bool `yaml:"preload_on_start"`
	MaxCacheSize       int  `yaml:"max_cache_size"`
	DirtyFlushInterval int  `yaml:"dirty_flush_interval_seconds"`
}

// ServiceConfig mem-service配置 (v2)
type ServiceConfig struct {
	Name              string        `yaml:"name"`
	Version           string        `yaml:"version"`
	WSPort           int           `yaml:"ws_port"`
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	DataDir          string        `yaml:"data_dir"`
	OutputDir        string        `yaml:"output_dir"`
	LogDir          string        `yaml:"log_dir"`
}

// DetectorsConfig 检测器配置 (v2)
type DetectorsConfig struct {
	LarkIM       DetectorConfig `yaml:"lark_im"`
	LarkDoc      DetectorConfig `yaml:"lark_doc"`
	LarkWiki     DetectorConfig `yaml:"lark_wiki"`
	LarkCalendar DetectorConfig `yaml:"lark_calendar"`
	LarkTask     DetectorConfig `yaml:"lark_task"`
	LarkVC       DetectorConfig `yaml:"lark_vc"`
	LarkContact  DetectorConfig `yaml:"lark_contact"`
}

// DetectorConfig 单个检测器配置
type DetectorConfig struct {
	Enabled              bool          `yaml:"enabled"`
	Interval             time.Duration `yaml:"interval"`
	BurstInterval        time.Duration `yaml:"burst_interval"`
	BurstTimeout         time.Duration `yaml:"burst_timeout"`
	HeartbeatInterval    time.Duration `yaml:"heartbeat_interval"`
	CommentCheckInterval int           `yaml:"comment_check_interval_seconds"` // 评论检测周期（秒），0=不检测
}

// StorageConfigV2 存储配置 (v2)
type StorageConfigV2 struct {
	GitPath        string `yaml:"git_path"`
	BitableEnabled bool  `yaml:"bitable_enabled"`
}

// LLMConfig LLM配置 (v2)
type LLMConfig struct {
	Enabled         bool   `yaml:"enabled"`
	ExtractPrompt   string `yaml:"extract_prompt"`
	DocExtractPrompt string `yaml:"doc_extract_prompt"`
}

// LogConfig 日志配置 (v2)
type LogConfig struct {
	Level  string `yaml:"level"`
	Format string `yaml:"format"`
}

// Settings 完整配置
type Settings struct {
	Project   ProjectConfig     `yaml:"project"`
	LarkCLI   LarkCLIConfig     `yaml:"lark_cli"`
	Git       GitStorageConfig `yaml:"git"`
	Bitable   BitableConfig     `yaml:"bitable"`
	Events    EventsConfig      `yaml:"events"`
	Polling   PollingConfig     `yaml:"polling"`
	MCP       MCPConfig        `yaml:"mcp"`
	Memory    MemoryConfig     `yaml:"memory"`
	// v2.0 新增配置
	Service ServiceConfig      `yaml:"service"`
	Detectors DetectorsConfig  `yaml:"detectors"`
	StorageV2 StorageConfigV2  `yaml:"storage"`
	LLM      LLMConfig         `yaml:"llm"`
	Log      LogConfig         `yaml:"log"`
}

// DefaultSettings 默认配置
func DefaultSettings() *Settings {
	return &Settings{
		Project: ProjectConfig{
			Name:   "feishu-mem",
			Topics: []string{"general"},
			Phases: []string{"initial"},
		},
		LarkCLI: LarkCLIConfig{
			Bin: "lark-cli",
		},
		Git: GitStorageConfig{
			WorkDir: "./data",
			Branch:  "main",
		},
		Polling: PollingConfig{
			Interval: 5 * time.Second,
		},
		MCP: MCPConfig{
			Port: 37777,
		},
		Memory: MemoryConfig{
			PreloadOnStart: true,
		},
		// v2.0 默认配置
		Service: ServiceConfig{
			Name: "feishu-memory-service",
			Version: "2.0.0",
			WSPort: 8765,
			HeartbeatInterval: 30 * time.Second,
			DataDir: "./data",
			OutputDir: "./outputs",
			LogDir: "./logs",
		},
		Detectors: DetectorsConfig{
			LarkIM: DetectorConfig{
				Enabled: true,
				Interval: 5 * time.Second,
				BurstInterval: 5 * time.Second,
				BurstTimeout: 1 * time.Minute,
				HeartbeatInterval: 15 * time.Second,
			},
			LarkDoc: DetectorConfig{
				Enabled: true,
				Interval: 30 * time.Second,
				BurstInterval: 5 * time.Second,
				BurstTimeout: 1 * time.Minute,
				HeartbeatInterval: 15 * time.Second,
				CommentCheckInterval: 150,
			},
			LarkWiki: DetectorConfig{
				Enabled: true,
				Interval: 30 * time.Second,
				BurstInterval: 5 * time.Second,
				BurstTimeout: 1 * time.Minute,
				HeartbeatInterval: 15 * time.Second,
			},
			LarkCalendar: DetectorConfig{
				Enabled: true,
				Interval: 1 * time.Minute,
				BurstInterval: 10 * time.Second,
				BurstTimeout: 1 * time.Minute,
				HeartbeatInterval: 15 * time.Second,
			},
			LarkTask: DetectorConfig{
				Enabled: true,
				Interval: 10 * time.Second,
				BurstInterval: 5 * time.Second,
				BurstTimeout: 1 * time.Minute,
				HeartbeatInterval: 15 * time.Second,
			},
			LarkVC: DetectorConfig{
				Enabled: true,
				Interval: 30 * time.Second,
				BurstInterval: 5 * time.Second,
				BurstTimeout: 1 * time.Minute,
				HeartbeatInterval: 15 * time.Second,
			},
			LarkContact: DetectorConfig{
				Enabled: false,
				Interval: 1 * time.Minute,
				BurstInterval: 30 * time.Second,
				BurstTimeout: 1 * time.Minute,
				HeartbeatInterval: 30 * time.Second,
			},
		},
		StorageV2: StorageConfigV2{
			GitPath: "./data/git",
			BitableEnabled: true,
		},
		LLM: LLMConfig{
			Enabled: true,
			ExtractPrompt: "extraction",
			DocExtractPrompt: "extraction_doc",
		},
		Log: LogConfig{
			Level: "info",
			Format: "text",
		},
	}
}

// LoadSettings 从文件加载配置
func LoadSettings(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	settings := DefaultSettings()
	if err := yaml.Unmarshal(data, settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// SaveSettings 保存配置到文件
func (s *Settings) SaveSettings(path string) error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadFromEnv 从环境变量加载
func (s *Settings) LoadFromEnv() {
	if v := os.Getenv("FEISHU_BASE_TOKEN"); v != "" {
		s.Bitable.BaseToken = v
	}
	if v := os.Getenv("MCP_PORT"); v != "" {
		// 解析...
	}
}
