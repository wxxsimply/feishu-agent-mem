package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Settings 完整配置
type Settings struct {
	Project ProjectConfig    `yaml:"project"`
	LarkCLI LarkCLIConfig    `yaml:"lark_cli"`
	Git     GitStorageConfig `yaml:"git"`
	Bitable BitableConfig    `yaml:"bitable"`
	Events  EventsConfig     `yaml:"events"`
	Polling PollingConfig    `yaml:"polling"`
	MCP     MCPConfig        `yaml:"mcp"`
	Memory  MemoryConfig     `yaml:"memory"`
}

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
