package main

import (
	"fmt"

	larkadapter "feishu-mem/internal/lark-adapter"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("  IM 消息聚合测试")
	fmt.Println("========================================")

	// 模拟飞书群聊消息
	messages := []larkadapter.MessageRecord{
		{MessageID: "m1", ChatID: "chat1", SenderName: "张三", Content: "数据库选型讨论，PostgreSQL 还是 MySQL？", CreateTime: 1000, MsgType: "text"},
		{MessageID: "m2", ChatID: "chat1", SenderName: "李四", Content: "👍", CreateTime: 1001, MsgType: "text"},
		{MessageID: "m3", ChatID: "chat1", SenderName: "王五", Content: "今天天气不错", CreateTime: 1002, MsgType: "text"},
		{MessageID: "m4", ChatID: "chat1", SenderName: "赵六", Content: "PG 的 JSONB 更适合我们的半结构化数据场景", CreateTime: 1003, MsgType: "text"},
		{MessageID: "m5", ChatID: "chat1", SenderName: "张三", Content: "那就 PostgreSQL 吧，李四负责迁移", CreateTime: 1005, MsgType: "text"},
		{MessageID: "m6", ChatID: "chat1", SenderName: "李四", Content: "收到", CreateTime: 1006, MsgType: "text"},
		// 第二个讨论（时间间隔较大）
		{MessageID: "m7", ChatID: "chat1", SenderName: "王五", Content: "缓存方案用 Redis 还是 Memcached？", CreateTime: 2000, MsgType: "text"},
		{MessageID: "m8", ChatID: "chat1", SenderName: "赵六", Content: "Redis 支持更多数据类型", CreateTime: 2003, MsgType: "text"},
		{MessageID: "m9", ChatID: "chat1", SenderName: "张三", Content: "确认用 Redis，王五负责接入", CreateTime: 2005, MsgType: "text"},
	}

	// 测试噪声过滤
	fmt.Println("\n--- 噪声过滤测试 ---")
	filter := larkadapter.NewNoiseFilter()
	for _, msg := range messages {
		isNoise := filter.IsNoise(msg)
		status := "保留"
		if isNoise {
			status = "噪声"
		}
		fmt.Printf("  [%s] %s: %s\n", status, msg.SenderName, truncate(msg.Content, 40))
	}

	// 测试消息聚合
	fmt.Println("\n--- 消息聚合测试 ---")
	batcher := larkadapter.NewMessageBatcher()
	groups := batcher.GroupMessages(messages)

	fmt.Printf("共 %d 条消息 → %d 个讨论组\n", len(messages), len(groups))
	for i, group := range groups {
		fmt.Printf("\n讨论组 [%d] (%d 条消息):\n", i+1, len(group))
		for _, msg := range group {
			fmt.Printf("  %s: %s\n", msg.SenderName, truncate(msg.Content, 50))
		}
	}

	// 测试 BuildGroupContext
	fmt.Println("\n--- 构建上下文测试 ---")
	for i, group := range groups {
		ctx := batcher.BuildGroupContext(group)
		if ctx != nil {
			fmt.Printf("\n讨论组 [%d] 的 LLM 输入:\n", i+1)
			fmt.Println(ctx.Change.ContextText)
		}
	}

	fmt.Println("========================================")
	fmt.Println("  测试完成!")
	fmt.Println("========================================")
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
