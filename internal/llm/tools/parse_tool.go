package tools

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// ParseTool JSON 解析工具（容错处理）
type ParseTool struct{}

func NewParseTool() *ParseTool {
	return &ParseTool{}
}

// ParseJSON 解析 JSON（容错）
func (t *ParseTool) ParseJSON(input string) (map[string]any, error) {
	// 1. 尝试直接解析
	var result map[string]any
	if err := json.Unmarshal([]byte(input), &result); err == nil {
		return result, nil
	}

	// 2. 提取 JSON 块
	jsonBlock := t.extractJSONBlock(input)
	if jsonBlock != "" {
		if err := json.Unmarshal([]byte(jsonBlock), &result); err == nil {
			return result, nil
		}
		// 对提取的块也做修复
		fixed := t.fixCommonIssues(jsonBlock)
		if err := json.Unmarshal([]byte(fixed), &result); err == nil {
			return result, nil
		}
	}

	// 3. 修复原始输入
	fixed := t.fixCommonIssues(input)
	if err := json.Unmarshal([]byte(fixed), &result); err == nil {
		return result, nil
	}

	return nil, fmt.Errorf("JSON parse failed after all attempts")
}

// FixExtractionResult 修复 ExtractionResult 中的常见字段类型错误
func (t *ParseTool) FixExtractionResult(raw map[string]any) map[string]any {
	if dec, ok := raw["decision"].(map[string]any); ok {
		// 修复 rationale 数组 → 字符串
		if rationale, ok := dec["rationale"].([]any); ok {
			var parts []string
			for _, r := range rationale {
				if s, ok := r.(string); ok {
					parts = append(parts, s)
				}
			}
			dec["rationale"] = strings.Join(parts, "; ")
		}
		// 修复 rationale 数字 → 字符串
		if rationale, ok := dec["rationale"].(float64); ok {
			dec["rationale"] = fmt.Sprintf("%v", rationale)
		}
		raw["decision"] = dec
	}
	return raw
}

// extractJSONBlock 提取 JSON 块
func (t *ParseTool) extractJSONBlock(input string) string {
	if idx := strings.Index(input, "```json"); idx != -1 {
		start := idx + 7
		if end := strings.Index(input[start:], "```"); end != -1 {
			return strings.TrimSpace(input[start : start+end])
		}
	}
	if idx := strings.Index(input, "```"); idx != -1 {
		start := idx + 3
		if end := strings.Index(input[start:], "```"); end != -1 {
			return strings.TrimSpace(input[start : start+end])
		}
	}
	if start := strings.Index(input, "{"); start != -1 {
		if end := strings.LastIndex(input, "}"); end != -1 && end > start {
			return input[start : end+1]
		}
	}
	return ""
}

// fixCommonIssues 修复常见格式问题
func (t *ParseTool) fixCommonIssues(input string) string {
	// 1. 去除控制字符（除换行和制表符外）
	ctrlRe := regexp.MustCompile(`[\x00-\x08\x0B\x0C\x0E-\x1F]`)
	input = ctrlRe.ReplaceAllString(input, "")

	// 2. 修复 trailing 逗号: {"a":1,} → {"a":1}
	trailingCommaRe := regexp.MustCompile(`,\s*([}\]])`)
	input = trailingCommaRe.ReplaceAllString(input, "$1")

	// 3. 修复注释: // ... 或 /* ... */
	singleLineComment := regexp.MustCompile(`//[^\n]*`)
	input = singleLineComment.ReplaceAllString(input, "")
	multiLineComment := regexp.MustCompile(`/\*[\s\S]*?\*/`)
	input = multiLineComment.ReplaceAllString(input, "")

	// 4. 修复单引号代替双引号的 key: {'key' → {"key"
	singleQuoteKey := regexp.MustCompile(`'([^']+)'\s*:`)
	input = singleQuoteKey.ReplaceAllString(input, `"$1":`)

	// 5. 修复末尾多余逗号在最后一个字段后
	input = strings.TrimSpace(input)
	if strings.HasSuffix(input, ",") {
		input = input[:len(input)-1]
	}

	return input
}
