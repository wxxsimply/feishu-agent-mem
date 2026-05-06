package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	AppID     string
	AppSecret string
	ChatIDs   []string
}

// LarkEvent 飞书事件结构
type LarkEvent struct {
	UUID      string         `json:"uuid"`
	Timestamp string         `json:"timestamp"`
	EventType string         `json:"event_type"`
	Event     map[string]any `json:"event"`
	Header    map[string]any `json:"header,omitempty"`
}

// IMMessageReceiveV1 接收消息事件
type IMMessageReceiveV1 struct {
	Header struct {
		EventID    string `json:"event_id"`
		EventType  string `json:"event_type"`
		CreateTime string `json:"create_time"`
		Token      string `json:"token"`
		AppID      string `json:"app_id"`
		TenantKey  string `json:"tenant_key"`
	} `json:"header"`
	Event struct {
		Sender struct {
			SenderID struct {
				OpenID  string `json:"open_id"`
				UserID  string `json:"user_id"`
				UnionID string `json:"union_id"`
			} `json:"sender_id"`
			SenderType string `json:"sender_type"`
			TenantKey  string `json:"tenant_key"`
		} `json:"sender"`
		Message struct {
			MessageID   string `json:"message_id"`
			RootID      string `json:"root_id"`
			ParentID    string `json:"parent_id"`
			CreateTime  string `json:"create_time"`
			ChatID      string `json:"chat_id"`
			ChatType    string `json:"chat_type"`
			MessageType string `json:"message_type"`
			Content     string `json:"content"`
			Mentions    []struct {
				Key       string `json:"key"`
				ID        struct {
					OpenID  string `json:"open_id"`
					UserID  string `json:"user_id"`
					UnionID string `json:"union_id"`
				} `json:"id"`
				Name string `json:"name"`
			} `json:"mentions"`
		} `json:"message"`
	} `json:"event"`
}

func loadConfig() (*Config, error) {
	// 先尝试加载当前目录的 .env
	_ = godotenv.Load()

	// 再尝试加载项目根目录的 .env
	// 获取当前可执行文件的目录，或者使用工作目录
	wd, err := os.Getwd()
	if err == nil {
		// 向上查找 .env 文件
		dir := wd
		for i := 0; i < 5; i++ { // 最多向上找5级
			envPath := filepath.Join(dir, ".env")
			if _, err := os.Stat(envPath); err == nil {
				log.Printf("加载配置文件: %s", envPath)
				_ = godotenv.Overload(envPath)
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir { // 到达根目录
				break
			}
			dir = parent
		}
	}

	appID := os.Getenv("LARK_APP_ID")
	appSecret := os.Getenv("LARK_APP_SECRET")
	chatIDsStr := os.Getenv("LARK_CHAT_IDS")

	if appID == "" || appSecret == "" {
		return nil, fmt.Errorf("LARK_APP_ID and LARK_APP_SECRET must be set")
	}

	chatIDs := []string{}
	if chatIDsStr != "" {
		for _, id := range strings.Split(chatIDsStr, ",") {
			id = strings.TrimSpace(id)
			if id != "" {
				chatIDs = append(chatIDs, id)
			}
		}
	}

	return &Config{
		AppID:     appID,
		AppSecret: appSecret,
		ChatIDs:   chatIDs,
	}, nil
}

func main() {
	fmt.Println("========================================")
	fmt.Println("  飞书长连接测试程序")
	fmt.Println("  使用 lark-cli event +subscribe")
	fmt.Println("========================================")
	fmt.Println()

	config, err := loadConfig()
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	fmt.Printf("配置:\n")
	fmt.Printf("  关注群聊: %v\n", config.ChatIDs)
	fmt.Println()

	// 构建 lark-cli 命令
	args := []string{"event", "+subscribe", "--as", "bot"}

	// 不过滤事件类型，接收所有事件，程序内部自己过滤

	fmt.Println("启动 lark-cli 事件订阅...")
	fmt.Printf("命令: lark-cli %s\n", strings.Join(args, " "))
	fmt.Println()

	cmd := exec.Command("lark-cli", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatalf("创建 stdout pipe 失败: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		log.Fatalf("创建 stderr pipe 失败: %v", err)
	}

	// 启动命令
	if err := cmd.Start(); err != nil {
		log.Fatalf("启动 lark-cli 失败: %v", err)
	}

	// 处理信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\n\n正在停止...")
		if cmd.Process != nil {
			cmd.Process.Signal(syscall.SIGTERM)
			cmd.Process.Wait()
		}
		os.Exit(0)
	}()

	// 读取 stderr 输出状态
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Fprintf(os.Stderr, "[lark-cli] %s\n", line)
		}
	}()

	// 读取 stdout 处理事件
	fmt.Println("等待接收事件...")
	fmt.Println()

	scanner := bufio.NewScanner(stdout)
	eventCount := 0

	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		eventCount++
		processEvent(line, config, eventCount)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("读取输出错误: %v", err)
	}

	cmd.Wait()
	fmt.Println("\n程序退出")
}

func processEvent(line string, config *Config, count int) {
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")
	fmt.Printf("事件 #%d | %s\n", count, time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\n")

	// 尝试解析为通用事件
	var event LarkEvent
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		fmt.Printf("解析失败: %v\n", err)
		fmt.Printf("原始内容: %s\n", line)
		fmt.Println()
		return
	}

	fmt.Printf("事件类型: %s\n", event.EventType)
	fmt.Printf("事件UUID: %s\n", event.UUID)
	fmt.Printf("时间戳: %s\n", event.Timestamp)
	fmt.Println()

	// 根据事件类型处理
	switch event.EventType {
	case "im.message.receive_v1", "im.message.message_receive_v1":
		processIMMessage(line, config)
	default:
		fmt.Printf("收到其他类型事件: %s\n", event.EventType)
		fmt.Printf("原始内容: %s\n", line)
	}

	fmt.Println()
}

func processIMMessage(line string, config *Config) {
	var msg IMMessageReceiveV1
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		fmt.Printf("解析消息事件失败: %v\n", err)
		return
	}

	chatID := msg.Event.Message.ChatID
	fmt.Printf("聊天ID: %s\n", chatID)
	fmt.Printf("聊天类型: %s\n", msg.Event.Message.ChatType)

	// 检查是否是关注的群聊
	isWatched := false
	if len(config.ChatIDs) > 0 {
		isWatched = slices.Contains(config.ChatIDs, chatID)
	} else {
		isWatched = true // 没有配置群聊时，显示所有消息
	}

	if isWatched {
		fmt.Printf("✅ 这是关注的群聊\n")
	} else {
		fmt.Printf("⚠️  非关注群聊\n")
	}
	fmt.Println()

	// 显示发送者
	fmt.Printf("发送者:\n")
	fmt.Printf("  OpenID: %s\n", msg.Event.Sender.SenderID.OpenID)
	fmt.Printf("  类型: %s\n", msg.Event.Sender.SenderType)
	fmt.Println()

	// 显示消息内容
	fmt.Printf("消息:\n")
	fmt.Printf("  消息ID: %s\n", msg.Event.Message.MessageID)
	fmt.Printf("  类型: %s\n", msg.Event.Message.MessageType)
	fmt.Printf("  创建时间: %s\n", msg.Event.Message.CreateTime)

	if msg.Event.Message.Content != "" {
		fmt.Printf("  内容: %s\n", msg.Event.Message.Content)

		// 尝试解析内容 JSON
		var content map[string]any
		if err := json.Unmarshal([]byte(msg.Event.Message.Content), &content); err == nil {
			if text, ok := content["text"].(string); ok {
				fmt.Printf("  文本: %s\n", text)
			}
		}
	}

	// 显示提及
	if len(msg.Event.Message.Mentions) > 0 {
		fmt.Printf("\n提及:\n")
		for _, m := range msg.Event.Message.Mentions {
			fmt.Printf("  - %s (OpenID: %s)\n", m.Name, m.ID.OpenID)
		}
	}
}
