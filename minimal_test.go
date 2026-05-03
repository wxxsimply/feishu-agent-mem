
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

func main() {
	log.Println("=== 最小化测试 ===")

	chatID := "oc_202372b4148e01ae309636d99764bc9a"

	// 1. 先手动测试 lark-cli
	log.Println("\n--- 步骤1：直接调用 lark-cli ---")
	lastCheck := time.Now().Add(-1 * time.Hour).Local()
	log.Printf("lastCheck=%v, RFC3339=%s", lastCheck, lastCheck.Format(time.RFC3339))

	args := []string{
		"im", "+chat-messages-list",
		"--chat-id", chatID,
		"--as", "user",
		"--start", lastCheck.Format(time.RFC3339),
		"--page-size", "10",
	}
	log.Printf("调用命令：lark-cli %v", strings.Join(args, " "))

	cmd := exec.Command("lark-cli", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("lark-cli 调用失败：%v\n输出：\n%s", err, string(output))
		return
	}
	log.Printf("lark-cli 调用成功！输出：\n%s", string(output))

	// 2. 解析输出
	log.Println("\n--- 步骤2：解析输出 ---")
	var result map[string]any
	if err := json.Unmarshal(output, &result); err != nil {
		log.Printf("JSON 解析失败：%v", err)
		return
	}
	log.Printf("解析后：ok=%v", result["ok"])

	data, ok := result["data"].(map[string]any)
	if !ok {
		log.Println("没有找到 data 字段")
		return
	}
	log.Printf("找到 data 字段，has_more=%v", data["has_more"])

	messages, ok := data["messages"].([]any)
	if !ok {
		log.Println("没有找到 messages 字段")
		return
	}
	log.Printf("找到 %d 条消息", len(messages))

	// 3. 解析每条消息
	log.Println("\n--- 步骤3：解析消息时间并过滤 ---")
	cutoff := lastCheck.Unix()
	log.Printf("cutoff=%v", cutoff)
	changes := 0

	for i, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		ctime, _ := msg["create_time"].(string)
		log.Printf("消息 %d：create_time=%s", i+1, ctime)
		ts := parseMessageTime(ctime)
		log.Printf("  解析为 unix=%v，time=%v", ts, time.Unix(ts, 0))
		if ts > cutoff {
			log.Println("  ✓ 在 cutoff 之后！")
			changes++
		} else {
			log.Printf("  ✗ 在 cutoff 之前或等于")
		}
	}

	log.Printf("\n--- 步骤4：结果 ---")
	log.Printf("有 %d 条新消息", changes)

}

func parseMessageTime(timeStr string) int64 {
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04",
		"2006-01-02T15:04:05-07:00",
	}
	localLoc, err := time.LoadLocation("Local")
	if err != nil {
		localLoc = time.UTC
	}
	for _, f := range formats {
		if t, err := time.ParseInLocation(f, timeStr, localLoc); err == nil {
			return t.Unix()
		}
	}
	return 0
}
