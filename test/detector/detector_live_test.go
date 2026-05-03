//go:build live
// +build live

package detector

import (
	"testing"
	"time"

	larkadapter "feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/signal"
)

// TestLiveLarkCLI 测试 lark-cli 本身是否工作正常
func TestLiveLarkCLI(t *testing.T) {
	utils := NewLarkTestUtils()

	t.Log("测试 lark-cli 基本功能...")

	// 测试1: 检查 lark-cli 版本或帮助
	t.Run("lark-cli basic", func(t *testing.T) {
		_, err := utils.RunLarkCommand("--help")
		if err != nil {
			t.Skipf("lark-cli 不可用: %v (跳过实时测试)", err)
			return
		}
		t.Log("✓ lark-cli 可用")
	})

	// 测试2: 尝试一些只读命令来验证连接
	t.Run("test read-only commands", func(t *testing.T) {
		// 这些是安全的只读测试
		testCases := []struct {
			name string
			args []string
		}{
			{"im help", []string{"im", "--help"}},
			{"vc help", []string{"vc", "--help"}},
			{"docs help", []string{"docs", "--help"}},
			{"calendar help", []string{"calendar", "--help"}},
			{"task help", []string{"task", "--help"}},
			{"wiki help", []string{"wiki", "--help"}},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := utils.RunLarkCommand(tc.args...)
				if err != nil {
					t.Logf("⚠ 命令 %v 可能不可用: %v", tc.args, err)
				} else {
					t.Logf("✓ 命令 %v 可用", tc.args)
				}
			})
		}
	})
}

// TestLiveDetectorWithRealData 使用真实数据测试检测器
func TestLiveDetectorWithRealData(t *testing.T) {
	cfg := larkadapter.LoadConfig()
	utils := NewLarkTestUtils()

	t.Log("开始使用真实数据测试检测器...")

	// 先检查 lark-cli 是否可用
	_, err := utils.RunLarkCommand("--help")
	if err != nil {
		t.Skipf("跳过实时测试: lark-cli 不可用: %v", err)
		return
	}

	// 定义时间窗口：检测过去24小时的变化
	since := time.Now().Add(-24 * time.Hour)

	// 逐个测试每个检测器的真实数据检测
	testCases := []struct {
		name        string
		adapterType signal.AdapterType
		detector    larkadapter.Detector
	}{
		{
			name:        "IM Detector",
			adapterType: signal.AdapterIM,
			detector:    larkadapter.NewIMExtractor(cfg),
		},
		{
			name:        "VC Detector",
			adapterType: signal.AdapterVC,
			detector:    larkadapter.NewVCExtractor(cfg),
		},
		{
			name:        "Docs Detector",
			adapterType: signal.AdapterDocs,
			detector:    larkadapter.NewDocExtractor(cfg),
		},
		{
			name:        "Calendar Detector",
			adapterType: signal.AdapterCalendar,
			detector:    larkadapter.NewCalendarExtractor(cfg),
		},
		{
			name:        "Task Detector",
			adapterType: signal.AdapterTask,
			detector:    larkadapter.NewTaskExtractor(cfg),
		},
		{
			name:        "Wiki Detector",
			adapterType: signal.AdapterWiki,
			detector:    larkadapter.NewWikiExtractor(cfg),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("========== 测试 %s ==========", tc.name)

			// 运行检测
			result, err := tc.detector.Detect(since)
			if err != nil {
				t.Errorf("检测失败: %v", err)
				return
			}

			// 输出结果
			t.Logf("检测结果: HasChanges=%v, Changes=%d",
				result.HasChanges, len(result.Changes))

			// 如果有变化，详细列出
			if len(result.Changes) > 0 {
				t.Logf("检测到的变化:")
				for i, change := range result.Changes {
					t.Logf("  [%d] Type=%s, EntityType=%s, Summary=%s",
						i+1, change.Type, change.EntityType, change.Summary)
				}
			} else {
				t.Logf("在指定时间范围内没有检测到变化（这是正常的）")
			}

			// 测试 Emitter 链路
			emitters := signal.NewEmitters()
			if emitter, ok := emitters[tc.adapterType]; ok {
				sig, err := emitter.EmitSignal(result)
				if err != nil {
					t.Logf("Emitter 错误: %v", err)
				} else if sig != nil {
					t.Logf("Emitter 结果: Strength=%s, DecisionSignals=%v",
						sig.Strength, sig.Context.DecisionSignals)
				} else {
					t.Logf("Emitter 没有产生信号（正常，如果没有相关决策内容）")
				}
			}
		})
	}
}

// TestLiveDetectorChain 测试完整的检测链
func TestLiveDetectorChain(t *testing.T) {
	cfg := larkadapter.LoadConfig()
	utils := NewLarkTestUtils()

	_, err := utils.RunLarkCommand("--help")
	if err != nil {
		t.Skipf("跳过实时测试: lark-cli 不可用: %v", err)
		return
	}

	t.Log("========== 测试完整检测链 ==========")

	// 1. 创建所有检测器
	detectors := map[signal.AdapterType]larkadapter.Detector{
		signal.AdapterIM:       larkadapter.NewIMExtractor(cfg),
		signal.AdapterVC:       larkadapter.NewVCExtractor(cfg),
		signal.AdapterDocs:     larkadapter.NewDocExtractor(cfg),
		signal.AdapterCalendar: larkadapter.NewCalendarExtractor(cfg),
		signal.AdapterTask:     larkadapter.NewTaskExtractor(cfg),
		signal.AdapterWiki:     larkadapter.NewWikiExtractor(cfg),
	}

	// 2. 创建所有 Emitter
	emitters := signal.NewEmitters()

	// 3. 时间窗口
	since := time.Now().Add(-6 * time.Hour)

	// 4. 运行完整链路
	results := make(map[signal.AdapterType]*larkadapter.DetectResult)
	signals := make(map[signal.AdapterType]*signal.StateChangeSignal)

	t.Logf("检测时间范围: 过去6小时")

	for adapterType, detector := range detectors {
		t.Logf("--- 处理 %s ---", adapterType)

		// 检测
		result, err := detector.Detect(since)
		if err != nil {
			t.Logf("  检测失败: %v", err)
			continue
		}
		results[adapterType] = result

		t.Logf("  检测: %d 个变化", len(result.Changes))

		// Emitter
		if emitter, ok := emitters[adapterType]; ok {
			sig, err := emitter.EmitSignal(result)
			if err != nil {
				t.Logf("  Emitter 失败: %v", err)
			} else if sig != nil {
				signals[adapterType] = sig
				t.Logf("  Emitter: 信号强度=%s, 决策信号=%v",
					sig.Strength, sig.Context.DecisionSignals)
			} else {
				t.Logf("  Emitter: 无信号")
			}
		}
	}

	// 5. 总结
	t.Log("")
	t.Log("========== 检测链总结 ==========")
	t.Logf("检测器运行数量: %d", len(detectors))
	t.Logf("产生变化的检测器: %d", len(results))
	t.Logf("产生信号的检测器: %d", len(signals))

	for adapterType := range signals {
		t.Logf("  ✓ %s 产生了决策信号", adapterType)
	}
}
