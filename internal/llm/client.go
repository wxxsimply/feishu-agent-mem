package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

// Config LLM 配置
type Config struct {
	APIKey     string
	BaseURL    string
	Model      string
}

// Client LLM 客户端
type Client struct {
	config *Config
}

// NewClient 创建 LLM 客户端
func NewClient() *Client {
	return &Client{
		config: LoadConfig(),
	}
}

// LoadConfig 从环境变量加载配置
func LoadConfig() *Config {
	// 尝试多个路径加载 .env
	paths := []string{
		".env",
		"../.env",
		"../../.env",
	}

	for _, path := range paths {
		godotenv.Load(path)
	}

	return &Config{
		APIKey:  os.Getenv("ARK_API_KEY"),
		BaseURL: os.Getenv("ARK_BASE_URL"),
		Model:   os.Getenv("ARK_MODEL"),
	}
}

// IsAvailable 检查 LLM 是否可用
func (c *Client) IsAvailable() bool {
	return c.config.APIKey != ""
}

// Call 调用 LLM
func (c *Client) Call(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	log.Println("========== LLM CALL START ==========")
	log.Printf("[LLM] IsAvailable: %v", c.IsAvailable())
	log.Printf("[LLM] Model: %s", c.config.Model)
	log.Printf("[LLM] BaseURL: %s", c.config.BaseURL)
	log.Printf("[LLM] System prompt length: %d chars", len(systemPrompt))
	log.Printf("[LLM] User prompt length: %d chars", len(userPrompt))
	log.Printf("[LLM] User prompt preview: %s", truncateForLog(userPrompt, 200))

	if c.config.APIKey == "" {
		log.Println("[LLM] ERROR: ARK_API_KEY is not set")
		return "", fmt.Errorf("ARK_API_KEY is not set")
	}

	startTime := time.Now()

	baseURL := c.config.BaseURL
	if baseURL == "" {
		baseURL = "https://ark.cn-beijing.volces.com/api/v3"
	}

	modelName := c.config.Model
	if modelName == "" {
		modelName = "doubao-1-5-pro-32k-250115"
	}

	client := arkruntime.NewClientWithApiKey(c.config.APIKey, arkruntime.WithBaseUrl(baseURL))

	req := model.CreateChatCompletionRequest{
		Model: modelName,
		Messages: []*model.ChatCompletionMessage{
			{
				Role: model.ChatMessageRoleSystem,
				Content: &model.ChatCompletionMessageContent{
					StringValue: volcengine.String(systemPrompt),
				},
			},
			{
				Role: model.ChatMessageRoleUser,
				Content: &model.ChatCompletionMessageContent{
					StringValue: volcengine.String(userPrompt),
				},
			},
		},
	}

	log.Printf("[LLM] Sending request to LLM API...")

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		log.Printf("[LLM] ERROR: LLM call failed: %v", err)
		log.Println("========== LLM CALL FAILED ==========")
		return "", fmt.Errorf("llm call failed: %w", err)
	}

	elapsed := time.Since(startTime)

	if len(resp.Choices) == 0 || resp.Choices[0].Message.Content == nil {
		log.Println("[LLM] ERROR: No response from LLM")
		log.Println("========== LLM CALL FAILED ==========")
		return "", fmt.Errorf("no response from llm")
	}

	result := *resp.Choices[0].Message.Content.StringValue

	log.Printf("[LLM] LLM call succeeded in %v", elapsed)
	log.Printf("[LLM] Response length: %d chars", len(result))
	log.Printf("[LLM] Response preview: %s", truncateForLog(result, 300))
	log.Println("========== LLM CALL END ==========")

	return result, nil
}

// ExtractJSON 从 LLM 响应中提取 JSON
func ExtractJSON(content string) string {
	cleaned := content

	if strings.Contains(content, "```json") {
		start := strings.Index(content, "```json") + 7
		if end := strings.Index(content[start:], "```"); end != -1 {
			cleaned = strings.TrimSpace(content[start : start+end])
		}
	} else if strings.Contains(content, "```") {
		start := strings.Index(content, "```") + 3
		if end := strings.Index(content[start:], "```"); end != -1 {
			cleaned = strings.TrimSpace(content[start : start+end])
		}
	} else {
		start := strings.Index(content, "{")
		end := strings.LastIndex(content, "}")
		if start != -1 && end != -1 && end > start {
			cleaned = content[start : end+1]
		}
	}

	return cleaned
}

// ParseExtractionResult 解析决策提取结果
func ParseExtractionResult(content string) (*ExtractionResult, error) {
	log.Printf("[LLM] ParseExtractionResult called")
	jsonStr := ExtractJSON(content)
	log.Printf("[LLM] Extracted JSON: %s", truncateForLog(jsonStr, 200))

	var result ExtractionResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log.Printf("[LLM] ERROR: JSON parse failed: %v", err)
		return nil, fmt.Errorf("json parse failed: %w", err)
	}

	log.Printf("[LLM] Parse result: HasDecision=%v, Confidence=%.2f", result.HasDecision, result.Confidence)
	return &result, nil
}

// ParseClassificationResult 解析议题分类结果
func ParseClassificationResult(content string) (*ClassificationResult, error) {
	log.Printf("[LLM] ParseClassificationResult called")
	jsonStr := ExtractJSON(content)

	var result ClassificationResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log.Printf("[LLM] ERROR: JSON parse failed: %v", err)
		return nil, fmt.Errorf("json parse failed: %w", err)
	}

	return &result, nil
}

// ParseCrossTopicResult 解析跨议题检测结果
func ParseCrossTopicResult(content string) (*CrossTopicResult, error) {
	log.Printf("[LLM] ParseCrossTopicResult called")
	jsonStr := ExtractJSON(content)

	var result CrossTopicResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log.Printf("[LLM] ERROR: JSON parse failed: %v", err)
		return nil, fmt.Errorf("json parse failed: %w", err)
	}

	return &result, nil
}

// ParseConflictResult 解析冲突评估结果
func ParseConflictResult(content string) (*ConflictResult, error) {
	log.Printf("[LLM] ParseConflictResult called")
	jsonStr := ExtractJSON(content)

	var result ConflictResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log.Printf("[LLM] ERROR: JSON parse failed: %v", err)
		return nil, fmt.Errorf("json parse failed: %w", err)
	}

	return &result, nil
}
