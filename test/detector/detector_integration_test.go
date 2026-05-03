package detector

import (
	"testing"
	"time"

	larkadapter "feishu-mem/internal/lark-adapter"
	"feishu-mem/internal/signal"
)

// TestAllDetectorsTogether 测试所有检测器一起工作
func TestAllDetectorsTogether(t *testing.T) {
	cfg := larkadapter.LoadConfig()

	// 创建所有检测器
	detectors := map[signal.AdapterType]larkadapter.Detector{
		signal.AdapterIM:       larkadapter.NewIMExtractor(cfg),
		signal.AdapterVC:       larkadapter.NewVCExtractor(cfg),
		signal.AdapterDocs:     larkadapter.NewDocExtractor(cfg),
		signal.AdapterCalendar: larkadapter.NewCalendarExtractor(cfg),
		signal.AdapterTask:     larkadapter.NewTaskExtractor(cfg),
		signal.AdapterWiki:     larkadapter.NewWikiExtractor(cfg),
	}

	t.Log("开始运行所有检测器集成测试...")
	t.Logf("共 %d 个检测器", len(detectors))

	// 逐个测试每个检测器
	for adapterType, detector := range detectors {
		t.Run(string(adapterType), func(t *testing.T) {
			t.Logf("测试 %s 检测器...", adapterType)

			// 第一次检测：建立基线（应该不会有太多变化）
			result, err := detector.Detect(time.Now().Add(-24 * time.Hour))
			if err != nil {
				t.Errorf("%s 检测器失败: %v", adapterType, err)
				return
			}

			t.Logf("%s 检测结果: HasChanges=%v, Changes=%d",
				adapterType, result.HasChanges, len(result.Changes))

			for i, c := range result.Changes {
				t.Logf("  [%d] [%s] %s: %s", i+1, c.Type, c.EntityType, c.Summary)
			}
		})
	}

	t.Log("所有检测器单独测试完成！")
}

// TestDetectorsParallel 测试检测器并行运行
func TestDetectorsParallel(t *testing.T) {
	cfg := larkadapter.LoadConfig()

	// 创建所有检测器
	detectors := map[signal.AdapterType]larkadapter.Detector{
		signal.AdapterIM:       larkadapter.NewIMExtractor(cfg),
		signal.AdapterVC:       larkadapter.NewVCExtractor(cfg),
		signal.AdapterDocs:     larkadapter.NewDocExtractor(cfg),
		signal.AdapterCalendar: larkadapter.NewCalendarExtractor(cfg),
		signal.AdapterTask:     larkadapter.NewTaskExtractor(cfg),
		signal.AdapterWiki:     larkadapter.NewWikiExtractor(cfg),
	}

	t.Log("开始并行运行检测器测试...")

	// 并行测试
	t.Run("parallel detection", func(t *testing.T) {
		for adapterType, detector := range detectors {
			adapterType := adapterType // capture for closure
			detector := detector

			t.Run(string(adapterType), func(t *testing.T) {
				t.Parallel() // 并行运行

				t.Logf("并行测试 %s 检测器...", adapterType)

				result, err := detector.Detect(time.Now().Add(-1 * time.Hour))
				if err != nil {
					t.Errorf("%s 检测器失败: %v", adapterType, err)
					return
				}

				t.Logf("%s 检测完成: %d 个变化", adapterType, len(result.Changes))
			})
		}
	})
}

// TestDetectorEmitterChain 测试检测器 -> Emitter 链路
func TestDetectorEmitterChain(t *testing.T) {
	cfg := larkadapter.LoadConfig()
	emitters := signal.NewEmitters()

	testCases := []struct {
		name        string
		adapterType signal.AdapterType
		detector    larkadapter.Detector
	}{
		{
			name:        "IM",
			adapterType: signal.AdapterIM,
			detector:    larkadapter.NewIMExtractor(cfg),
		},
		{
			name:        "VC",
			adapterType: signal.AdapterVC,
			detector:    larkadapter.NewVCExtractor(cfg),
		},
		{
			name:        "Docs",
			adapterType: signal.AdapterDocs,
			detector:    larkadapter.NewDocExtractor(cfg),
		},
		{
			name:        "Calendar",
			adapterType: signal.AdapterCalendar,
			detector:    larkadapter.NewCalendarExtractor(cfg),
		},
		{
			name:        "Task",
			adapterType: signal.AdapterTask,
			detector:    larkadapter.NewTaskExtractor(cfg),
		},
		{
			name:        "Wiki",
			adapterType: signal.AdapterWiki,
			detector:    larkadapter.NewWikiExtractor(cfg),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name+"_emitter_chain", func(t *testing.T) {
			// 1. 检测器运行
			result, err := tc.detector.Detect(time.Now().Add(-24 * time.Hour))
			if err != nil {
				t.Errorf("%s 检测器失败: %v", tc.name, err)
				return
			}

			t.Logf("%s 检测器: %d 个变化", tc.name, len(result.Changes))

			// 2. Emitter 处理
			emitter, ok := emitters[tc.adapterType]
			if !ok {
				t.Errorf("未找到 %s 的 Emitter", tc.name)
				return
			}

			sig, err := emitter.EmitSignal(result)
			if err != nil {
				t.Errorf("%s Emitter 失败: %v", tc.name, err)
				return
			}

			if sig != nil {
				t.Logf("%s Emitter 生成信号: Strength=%s, DecisionSignals=%v",
					tc.name, sig.Strength, sig.Context.DecisionSignals)
			} else {
				t.Logf("%s Emitter 没有生成信号（正常，如果没有相关变化）", tc.name)
			}
		})
	}
}
