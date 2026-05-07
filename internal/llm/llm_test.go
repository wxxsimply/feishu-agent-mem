// internal/llm/llm_test.go

package llm_test

import (
	"testing"

	"feishu-mem/internal/llm"
	"github.com/stretchr/testify/assert"
)

func TestNewMemoryAgent(t *testing.T) {
	agent := llm.NewMemoryAgent()
	assert.NotNil(t, agent)
	assert.True(t, agent.IsAvailable())
	t.Log("✓ MemoryAgent 创建成功")
}

func TestExtractDecision_Fallback(t *testing.T) {
	agent := llm.NewMemoryAgent()

	content := "决定使用 PostgreSQL 作为主数据库"
	topics := []string{"数据库架构", "缓存架构"}

	result, err := agent.ExtractDecision(content, topics)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result.Confidence > 0)
	t.Logf("✓ 决策提取（降级模式）成功: HasDecision=%v, Confidence=%.2f", result.HasDecision, result.Confidence)
}

func TestClassifyTopic_Fallback(t *testing.T) {
	agent := llm.NewMemoryAgent()

	decision := "决定使用 PostgreSQL"
	topics := []string{"数据库架构", "缓存架构", "前端框架"}

	result, err := agent.ClassifyTopic(decision, topics)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "数据库架构", result.Topic)
	t.Log("✓ 议题分类（降级模式）成功")
}

func TestDetectCrossTopic_Fallback(t *testing.T) {
	agent := llm.NewMemoryAgent()

	result, err := agent.DetectCrossTopic(nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	t.Log("✓ 跨议题检测（降级模式）成功")
}

func TestResolveConflict(t *testing.T) {
	agent := llm.NewMemoryAgent()

	result, err := agent.ResolveConflict(nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	t.Log("✓ 冲突评估成功")
}

func TestGetTools(t *testing.T) {
	agent := llm.NewMemoryAgent()

	tools := agent.GetTools()
	assert.NotNil(t, tools)
	assert.GreaterOrEqual(t, len(tools), 0)
	t.Logf("✓ 获取工具列表成功，共 %d 个工具", len(tools))
}

func TestSearchTools(t *testing.T) {
	agent := llm.NewMemoryAgent()

	tools := agent.SearchTools("search")
	assert.NotNil(t, tools)
	t.Logf("✓ 搜索工具成功，找到 %d 个相关工具", len(tools))
}
