package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"

	openai "github.com/sashabaranov/go-openai"

	"github.com/invopop/jsonschema"
	"github.com/joho/godotenv"

	"feishu-mem/internal/llm/tools"
)


// LLM 调用计数
var (
	llmCallCount atomic.Int64
)

func GetLLMCallCount() int64 {
	return llmCallCount.Load()
}

// GenerateSchema 泛型生成 JSON Schema（供结构化输出使用）
func GenerateSchema[T any]() *jsonschema.Schema {
	reflector := jsonschema.Reflector{
		AllowAdditionalProperties: false,
		DoNotReference:            true,
	}
	return reflector.Reflect(new(T))
}

// Config LLM 配置
type Config struct {
	APIKey  string
	BaseURL string
	Model   string
}

// Client LLM 客户端
type Client struct {
	config *Config
	client *openai.Client
}

// NewClient 创建 LLM 客户端
func NewClient() *Client {
	cfg := LoadConfig()
	c := &Client{
		config: cfg,
	}

	if cfg.APIKey != "" {
		clientConfig := openai.DefaultConfig(cfg.APIKey)
		if cfg.BaseURL != "" {
			clientConfig.BaseURL = cfg.BaseURL
		}
		c.client = openai.NewClientWithConfig(clientConfig)
	}

	return c
}

// LoadConfig 从环境变量加载配置
func LoadConfig() *Config {
	paths := []string{
		".env",
		"../.env",
		"../../.env",
	}

	for _, path := range paths {
		godotenv.Load(path)
	}

	log.Printf("[LLM] LoadConfig: DEEPSEEK_MODEL=%s", os.Getenv("DEEPSEEK_MODEL"))
	return &Config{
		APIKey:  os.Getenv("DEEPSEEK_API_KEY"),
		BaseURL: os.Getenv("DEEPSEEK_BASE_URL"),
		Model:   os.Getenv("DEEPSEEK_MODEL"),
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
		log.Println("[LLM] ERROR: DEEPSEEK_API_KEY is not set")
		return "", fmt.Errorf("DEEPSEEK_API_KEY is not set")
	}

	llmCallCount.Add(1)
	startTime := time.Now()

	modelName := c.config.Model
	if modelName == "" {
		modelName = "deepseek-chat"
	}

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: modelName,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: userPrompt,
			},
		},
	})
	if err != nil {
		log.Printf("[LLM] ERROR: LLM call failed: %v", err)
		log.Println("========== LLM CALL FAILED ==========")
		return "", fmt.Errorf("llm call failed: %w", err)
	}

	elapsed := time.Since(startTime)
	log.Printf("[LLM] ✅ LLM call succeeded in %v (total calls: %d)", elapsed, llmCallCount.Load())

	if len(resp.Choices) == 0 {
		log.Println("[LLM] ERROR: No response from LLM")
		log.Println("========== LLM CALL FAILED ==========")
		return "", fmt.Errorf("no response from llm")
	}

	result := resp.Choices[0].Message.Content

	log.Printf("[LLM] LLM call succeeded in %v", elapsed)
	log.Printf("[LLM] Response length: %d chars", len(result))
	log.Printf("[LLM] Response preview: %s", truncateForLog(result, 300))
	log.Println("========== LLM CALL END ==========")

	return result, nil
}

// CallWithJSONSchema 调用 LLM 并强制结构化输出（JSON Schema）
func (c *Client) CallWithJSONSchema(ctx context.Context, systemPrompt, userPrompt string, schema *jsonschema.Schema, schemaName, schemaDesc string) (string, error) {
	log.Println("========== LLM JSON SCHEMA CALL START ==========")
	log.Printf("[LLM] Schema: %s", schemaName)
	log.Printf("[LLM] System prompt length: %d chars", len(systemPrompt))

	if c.config.APIKey == "" {
		return "", fmt.Errorf("DEEPSEEK_API_KEY is not set")
	}

	modelName := c.config.Model
	if modelName == "" {
		modelName = "deepseek-chat"
	}

	// 使用 json_object response format + 在 prompt 中描述 schema
	schemaJSON, _ := json.Marshal(schema)
	enhancedUserPrompt := fmt.Sprintf("%s\n\n请严格按照以下 JSON Schema 返回结果（直接返回 JSON，不要包含其他文本）：\n%s", userPrompt, string(schemaJSON))

	resp, err := c.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: modelName,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: enhancedUserPrompt,
			},
		},
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
	if err != nil {
		log.Printf("[LLM] JSON Schema call failed: %v", err)
		return "", fmt.Errorf("llm json schema call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no response from llm")
	}

	result := resp.Choices[0].Message.Content
	log.Printf("[LLM] JSON Schema response: %s", truncateForLog(result, 300))
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

// ParseExtractionResult 解析决策提取结果（含容错处理）
func ParseExtractionResult(content string) (*ExtractionResult, error) {
	log.Printf("[LLM] ParseExtractionResult called")
	rawJSON := ExtractJSON(content)
	log.Printf("[LLM] Extracted JSON: %s", truncateForLog(rawJSON, 200))

	// 使用 ParseTool 进行容错解析
	pt := &tools.ParseTool{}
	parsed, parseErr := pt.ParseJSON(rawJSON)
	if parseErr != nil {
		log.Printf("[LLM] ERROR: JSON parse failed: %v", parseErr)
		return nil, fmt.Errorf("json parse failed after retries: %w", parseErr)
	}

	// 修复常见字段类型错误
	parsed = pt.FixExtractionResult(parsed)

	// 转回 JSON 并反序列化为结构体
	fixedJSON, err := json.Marshal(parsed)
	if err != nil {
		return nil, fmt.Errorf("re-marshal failed: %w", err)
	}

	var result ExtractionResult
	if err := json.Unmarshal(fixedJSON, &result); err != nil {
		log.Printf("[LLM] ERROR: Re-parse failed: %v", err)
		return nil, fmt.Errorf("re-parse failed: %w", err)
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

// ParseDedupResult 解析去重+冲突联合判断结果
func ParseDedupResult(content string) (*DedupResult, error) {
	log.Printf("[LLM] ParseDedupResult called")
	jsonStr := ExtractJSON(content)

	var result DedupResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log.Printf("[LLM] ERROR: Dedup JSON parse failed: %v, raw=%s", err, truncateForLog(content, 200))
		return nil, fmt.Errorf("dedup json parse failed: %w", err)
	}

	// 验证 action 值
	switch result.Action {
	case "skip", "update", "conflict":
		// valid
	default:
		log.Printf("[LLM] WARNING: unknown dedup action '%s', defaulting to conflict", result.Action)
		result.Action = "conflict"
	}

	log.Printf("[LLM] Dedup parse result: action=%s, reason=%s", result.Action, result.Reason)
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
