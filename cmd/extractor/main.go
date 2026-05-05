package main

import (
	"fmt"
	"log"
	"time"

	larkadapter "feishu-mem/internal/lark-adapter"
)

func main() {
	// 加载 .env 文件
	larkadapter.LoadEnv()

	// 检查飞书配置
	missing := larkadapter.CheckLarkConfig()
	if len(missing) > 0 {
		fmt.Printf("⚠️  缺少飞书配置变量: %v\n", missing)
		fmt.Println("请在 .env 文件中配置以下变量:")
		fmt.Println("  - LARK_APP_ID: 飞书应用 ID")
		fmt.Println("  - LARK_APP_SECRET: 飞书应用密钥")
		fmt.Println("  - LARK_CHAT_IDS: 监控的群聊 ID（可选，逗号分隔）")
		fmt.Println()
		fmt.Println("当前使用模拟数据运行...")
		fmt.Println()
	}

	// 加载配置
	cfg := larkadapter.LoadConfig()

	// 创建所有提取器
	extractors := []larkadapter.Extractor{
		larkadapter.NewIMExtractor(cfg),
		larkadapter.NewCalendarExtractor(cfg),
		larkadapter.NewDocExtractor(cfg),
		larkadapter.NewWikiExtractor(cfg),
		larkadapter.NewVCExtractor(cfg),
		larkadapter.NewMinutesExtractor(cfg),
		larkadapter.NewTaskExtractor(cfg),
		larkadapter.NewOKRExtractor(cfg),
		larkadapter.NewContactExtractor(cfg),
	}

	// 执行提取
	fmt.Println("========== 完整提取 ==========")
	fmt.Println("开始提取决策信息...")
	fmt.Println("================================")

	for _, e := range extractors {
		fmt.Printf("正在提取: %s... ", e.Name())
		if err := e.Extract(); err != nil {
			log.Printf("提取 %s 失败: %v", e.Name(), err)
		} else {
			fmt.Println("✓ 完成")
		}
	}

	fmt.Println()
	fmt.Println("========== 增量检测 ==========")
	fmt.Println("开始检测状态变化...")
	fmt.Println("================================")

	// 创建检测器列表（Extractor 也实现了 Detector 时才跑）
	detectors := []larkadapter.Detector{
		larkadapter.NewIMExtractor(cfg),
		larkadapter.NewCalendarExtractor(cfg),
		larkadapter.NewDocExtractor(cfg),
		larkadapter.NewWikiExtractor(cfg),
		larkadapter.NewVCExtractor(cfg),
		larkadapter.NewMinutesExtractor(cfg),
		larkadapter.NewTaskExtractor(cfg),
		larkadapter.NewOKRExtractor(cfg),
		larkadapter.NewContactExtractor(cfg),
	}

	// 第一次检测：从头开始（基线），应返回无变化
	fmt.Println("--- 首次检测（基线） ---")
	detectAll(detectors, false)

	fmt.Println()
	fmt.Println("--- 二次检测（增量，模拟 1 小时后）---")
	// 模拟一小时后再次检测
	fakeLastCheck := time.Now().Add(-1 * time.Hour)
	detectAllWithLastCheck(detectors, fakeLastCheck)

	fmt.Println()
	fmt.Println("================================")
	fmt.Println("提取完成！结果已保存到 outputs/ 目录")
}

func detectAll(detectors []larkadapter.Detector, verbose bool) {
	for _, d := range detectors {
		// 使用 ExtractDetect 封装（会自动管理状态）
		result, err := larkadapter.ExtractDetect(d)
		if err != nil {
			log.Printf("检测 %s 失败: %v", d.Name(), err)
			continue
		}

		if result.HasChanges {
			fmt.Printf("  %s: 🔄 %d 个变化\n", d.Name(), len(result.Changes))
			if verbose {
				for _, c := range result.Changes {
					fmt.Printf("    - [%s] %s: %s\n", c.Type, c.EntityType, c.Summary)
				}
			}
		} else {
			fmt.Printf("  %s: ✓ 无变化\n", d.Name())
		}
	}
}

func detectAllWithLastCheck(detectors []larkadapter.Detector, lastCheck time.Time) {
	for _, d := range detectors {
		result, err := d.Detect(lastCheck)
		if err != nil {
			log.Printf("检测 %s 失败: %v", d.Name(), err)
			continue
		}

		if result.HasChanges {
			fmt.Printf("  %s: 🔄 %d 个变化\n", d.Name(), len(result.Changes))
			for _, c := range result.Changes {
				fmt.Printf("    - [%s] %s: %s\n", c.Type, c.EntityType, c.Summary)
			}
		} else {
			fmt.Printf("  %s: ✓ 无变化\n", d.Name())
		}
	}
}
