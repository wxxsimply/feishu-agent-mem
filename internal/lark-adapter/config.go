package larkadapter

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// Config 飞书适配器配置
type Config struct {
	AppID         string
	AppSecret     string
	ChatIDs       []string // 用于决策卡片推送
	DetectChatIDs []string // 用于IM消息检测
	UserID        string
}

// LoadEnv 从多个位置加载 .env 文件，提高兼容性
func LoadEnv() {
	// 按优先级尝试的路径
	candidates := []string{
		".env",
		"../.env",
		filepath.Join(os.Getenv("PROJECT_ROOT"), ".env"),
	}

	// 尝试通过可执行文件路径推断
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), ".env"))
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", ".env"))
	}

	loaded := false
	for _, path := range candidates {
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if _, err := os.Stat(absPath); err != nil {
			continue
		}
		if err := godotenv.Load(absPath); err == nil {
			log.Printf("[Config] Loaded .env from: %s", absPath)
			loaded = true
			break
		}
	}

	if !loaded {
		log.Println("[Config] No .env file found (using system environment variables)")
	}

	// 验证关键配置
	if os.Getenv("LARK_APP_ID") == "" {
		log.Println("[Config] WARNING: LARK_APP_ID is not set")
	}
	if os.Getenv("LARK_CHAT_IDS") != "" {
		log.Printf("[Config] LARK_CHAT_IDS=%s", os.Getenv("LARK_CHAT_IDS"))
	}
}

// LoadConfig 从环境变量加载配置（默认使用 LARK_*）
func LoadConfig() *Config {
	return LoadConfigWithPrefix("LARK_")
}

// LoadConfigWithPrefix 从环境变量加载配置（使用指定前缀）
func LoadConfigWithPrefix(prefix string) *Config {
	cfg := &Config{
		AppID:     os.Getenv(prefix + "APP_ID"),
		AppSecret: os.Getenv(prefix + "APP_SECRET"),
	}

	// 解析 CHAT_IDS（逗号分隔）- 用于推送
	if raw := os.Getenv(prefix + "CHAT_IDS"); raw != "" {
		for _, id := range strings.Split(raw, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				cfg.ChatIDs = append(cfg.ChatIDs, id)
			}
		}
	}

	// 解析 DETECT_CHAT_IDS（逗号分隔）- 用于检测消息
	// 如果没有设置，默认使用 CHAT_IDS
	if raw := os.Getenv(prefix + "DETECT_CHAT_IDS"); raw != "" {
		for _, id := range strings.Split(raw, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				cfg.DetectChatIDs = append(cfg.DetectChatIDs, id)
			}
		}
	} else {
		// 默认使用和推送相同的 chat ids
		cfg.DetectChatIDs = cfg.ChatIDs
	}

	cfg.UserID = os.Getenv(prefix + "USER_ID")

	return cfg
}

// FirstChatID 返回第一个 chat-id（常用场景），不存在则返回空串
func (c *Config) FirstChatID() string {
	if len(c.ChatIDs) > 0 {
		return c.ChatIDs[0]
	}
	return ""
}

// IsConfigured 检查配置是否完整（至少有 AppID 和 AppSecret）
func (c *Config) IsConfigured() bool {
	return c.AppID != "" && c.AppSecret != ""
}
