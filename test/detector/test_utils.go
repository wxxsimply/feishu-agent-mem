package detector

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// LarkTestUtils 飞书测试工具
type LarkTestUtils struct {
	cliPath string
}

// NewLarkTestUtils 创建测试工具
func NewLarkTestUtils() *LarkTestUtils {
	cliPath := "lark-cli"
	if path := os.Getenv("LARK_CLI_PATH"); path != "" {
		cliPath = path
	}
	return &LarkTestUtils{cliPath: cliPath}
}

// RunLarkCommand 运行 lark-cli 命令
func (u *LarkTestUtils) RunLarkCommand(args ...string) ([]byte, error) {
	var stdout, stderr bytes.Buffer

	cmd := exec.Command(u.cliPath, args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("命令失败: %v, stderr: %s", err, stderr.String())
	}

	return stdout.Bytes(), nil
}

// ==================== 测试数据创建工具 ====================

// CreateTestWikiPage 创建测试知识库页面
func (u *LarkTestUtils) CreateTestWikiPage(title string, content string) (map[string]any, error) {
	// 注意：这里使用 lark-cli 的 wiki 相关命令
	// 具体命令需要根据实际的 lark-cli 命令调整
	result := map[string]any{
		"title":      title,
		"created_at": time.Now(),
	}
	return result, nil
}

// CreateTestDoc 创建测试文档
func (u *LarkTestUtils) CreateTestDoc(title string, content string) (map[string]any, error) {
	// 使用 lark-cli 创建文档
	// 这里需要根据实际的 lark-cli 命令调整
	result := map[string]any{
		"title":      title,
		"content":    content,
		"created_at": time.Now(),
	}
	return result, nil
}

// CreateTestTask 创建测试任务
func (u *LarkTestUtils) CreateTestTask(title string, description string) (map[string]any, error) {
	// 这里需要根据实际的 lark-cli task 命令调整
	result := map[string]any{
		"title":       title,
		"description": description,
		"created_at":  time.Now(),
	}
	return result, nil
}

// CreateTestEvent 创建测试日程
func (u *LarkTestUtils) CreateTestEvent(title string, startTime time.Time, endTime time.Time) (map[string]any, error) {
	// 这里需要根据实际的 lark-cli calendar 命令调整
	result := map[string]any{
		"title":      title,
		"start_time": startTime,
		"end_time":   endTime,
		"created_at": time.Now(),
	}
	return result, nil
}

// SendTestMessage 发送测试消息
func (u *LarkTestUtils) SendTestMessage(chatID string, content string) (map[string]any, error) {
	// 这里需要根据实际的 lark-cli im 命令调整
	result := map[string]any{
		"chat_id":   chatID,
		"content":   content,
		"sent_at":   time.Now(),
	}
	return result, nil
}

// ==================== 测试辅助函数 ====================

// WaitForSeconds 等待指定秒数（给飞书API一些时间）
func (u *LarkTestUtils) WaitForSeconds(seconds int) {
	time.Sleep(time.Duration(seconds) * time.Second)
}

// ParseJSON 解析 JSON 输出
func (u *LarkTestUtils) ParseJSON(data []byte) (map[string]any, error) {
	var result map[string]any
	err := json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ParseJSONArray 解析 JSON 数组输出
func (u *LarkTestUtils) ParseJSONArray(data []byte) ([]map[string]any, error) {
	var result []map[string]any
	err := json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// ==================== 测试数据清理工具 ====================

// CleanupTestData 清理测试数据（标记测试数据以便后续清理）
func (u *LarkTestUtils) CleanupTestData() {
	// 这里可以实现清理测试数据的逻辑
	// 比如删除标记为测试的文档、任务等
}

// ==================== 决策关键词测试内容 ====================

// GetDecisionTestMessages 获取含决策关键词的测试消息
func GetDecisionTestMessages() []string {
	return []string{
		"我们决定采用方案B进行开发",
		"经过讨论，确认了第三季度的目标",
		"这个方案我批准了，就这么办",
		"LGTM，这个设计文档通过了",
		"结论是我们需要重构这部分代码",
	}
}

// GetDecisionTestTitles 获取含决策关键词的测试标题
func GetDecisionTestTitles() []string {
	return []string{
		"【决策记录】架构方案评审结果",
		"项目里程碑确认文档",
		"技术方案最终版",
		"团队会议决议",
	}
}
