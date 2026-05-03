//go:build concurrent
// +build concurrent

package detector

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"sync"
	"testing"
	"time"

	larkadapter "feishu-mem/internal/lark-adapter"
	// "github.com/sergi/go-diff/diffmatchpatch"
)

// 测试配置
const (
	testDuration     = 2 * time.Minute // 测试总时长
	detectInterval   = 5 * time.Second // 检测间隔
	simulateInterval = 25 * time.Second // 状态模拟间隔
)

// ==================== VC 检测器并发测试（使用 lark-cli 自动模拟） ====================

// TestConcurrentVC 测试 VC 检测器：定时检测 + lark-cli 自动模拟状态变化
func TestConcurrentVC(t *testing.T) {
	cfg := larkadapter.LoadConfig()
	detector := larkadapter.NewVCExtractor(cfg)
	utils := NewLarkTestUtils()

	// 检查 lark-cli
	if _, err := utils.RunLarkCommand("--help"); err != nil {
		t.Skipf("跳过并发测试: lark-cli 不可用: %v", err)
		return
	}

	t.Log("========================================")
	t.Log("VC 检测器并发测试")
	t.Log("  - 定时检测: 每 30s")
	t.Log("  - 自动模拟: 使用 lark-cli 检查会议状态")
	t.Log("  - 测试时长: 2 分钟")
	t.Log("========================================")

	var wg sync.WaitGroup
	stopChan := make(chan struct{})
	detectedChanges := make(chan string, 10)

	// 进程1: 定时检测
	wg.Add(1)
	go func() {
		defer wg.Done()
		runDetectorLoop(t, "VC", detector, stopChan, detectedChanges)
	}()

	// 进程2: 使用 lark-cli 自动模拟状态变化
	wg.Add(1)
	go func() {
		defer wg.Done()
		runVCAutoSimulator(t, utils, stopChan, detectedChanges)
	}()

	// 等待测试完成
	time.Sleep(testDuration)
	close(stopChan)
	wg.Wait()
	close(detectedChanges)

	t.Log("")
	t.Log("========================================")
	t.Log("测试完成！检测到的变化汇总：")
	for change := range detectedChanges {
		t.Logf("  ✓ %s", change)
	}
	t.Log("========================================")
}

// ==================== IM 检测器并发测试 ====================

func TestConcurrentIM(t *testing.T) {
	cfg := larkadapter.LoadConfig()
	detector := larkadapter.NewIMExtractor(cfg)
	utils := NewLarkTestUtils()

	if _, err := utils.RunLarkCommand("--help"); err != nil {
		t.Skipf("跳过并发测试: lark-cli 不可用: %v", err)
		return
	}

	t.Log("========================================")
	t.Log("IM 检测器并发测试")
	t.Log("  - 定时检测: 每 30s")
	t.Log("  - 自动模拟: 使用 lark-cli 检查消息状态")
	t.Log("  - 测试时长: 2 分钟")
	t.Log("========================================")

	var wg sync.WaitGroup
	stopChan := make(chan struct{})
	detectedChanges := make(chan string, 10)

	wg.Add(1)
	go func() {
		defer wg.Done()
		runDetectorLoop(t, "IM", detector, stopChan, detectedChanges)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		runIMAutoSimulator(t, utils, stopChan)
	}()

	time.Sleep(testDuration)
	close(stopChan)
	wg.Wait()
	close(detectedChanges)

	t.Log("")
	t.Log("========================================")
	t.Log("测试完成！检测到的变化汇总：")
	for change := range detectedChanges {
		t.Logf("  ✓ %s", change)
	}
	t.Log("========================================")
}

// ==================== Docs 检测器并发测试（带内容比较） ====================

func TestConcurrentDocs(t *testing.T) {
	cfg := larkadapter.LoadConfig()
	detector := larkadapter.NewDocExtractor(cfg)
	utils := NewLarkTestUtils()

	if _, err := utils.RunLarkCommand("--help"); err != nil {
		t.Skipf("跳过并发测试: lark-cli 不可用: %v", err)
		return
	}

	t.Log("========================================")
	t.Log("Docs 检测器并发测试（带内容比较）")
	t.Log("  - 定时检测: 每 5s")
	t.Log("  - 自动模拟: 使用 lark-cli 检查文档状态")
	t.Log("  - 内容比较: 使用 go-diff 比较内容变化")
	t.Log("  - 测试时长: 2 分钟")
	t.Log("========================================")

	var wg sync.WaitGroup
	stopChan := make(chan struct{})
	detectedChanges := make(chan string, 10)

	// 进程1: 定时检测 + 内容比较
	wg.Add(1)
	go func() {
		defer wg.Done()
		runDocsDetectorWithContentCompare(t, detector, stopChan, detectedChanges)
	}()

	// 进程2: 使用 lark-cli 自动模拟状态变化
	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	runDocsAutoSimulator(t, utils, stopChan)
	// }()

	time.Sleep(testDuration)
	close(stopChan)
	wg.Wait()
	close(detectedChanges)

	t.Log("")
	t.Log("========================================")
	t.Log("测试完成！检测到的变化汇总：")
	for change := range detectedChanges {
		t.Logf("  ✓ %s", change)
	}
	t.Log("========================================")
}

// ==================== Calendar 检测器并发测试 ====================

func TestConcurrentCalendar(t *testing.T) {
	cfg := larkadapter.LoadConfig()
	detector := larkadapter.NewCalendarExtractor(cfg)
	utils := NewLarkTestUtils()

	if _, err := utils.RunLarkCommand("--help"); err != nil {
		t.Skipf("跳过并发测试: lark-cli 不可用: %v", err)
		return
	}

	t.Log("========================================")
	t.Log("Calendar 检测器并发测试")
	t.Log("  - 定时检测: 每 3 秒")
	t.Log("  - 持续运行直到检测到变化或按 Ctrl+C 退出")
	t.Log("========================================")

	// 清理旧快照
	snapshotPath := larkadapter.StateDir() + "/lark_calendar_snapshot.json"
	os.Remove(snapshotPath)
	t.Logf("已清理旧快照: %s", snapshotPath)

	// 设置信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	stopChan := make(chan struct{})
	detectedChan := make(chan struct{}, 1)
	detectedChanges := make(chan string, 10)

	// 1. 第一次检测：建立基线
	t.Log("")
	t.Log("━━━ 第一次检测：建立基线 ━━━")
	lastCheck := time.Now().Add(-1 * time.Hour)
	result, err := detector.Detect(lastCheck)
	if err != nil {
		t.Logf("检测失败: %v", err)
	} else {
		t.Logf("基线建立完成 (HasChanges: %v)", result.HasChanges)
	}

	// 2. 启动检测循环
	t.Log("")
	t.Log("━━━ 开始等待日程变化 ━━━")
	t.Log("提示：现在请在飞书中创建/更新/删除日程，或者按 Ctrl+C 退出")
	t.Log("")

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(3 * time.Second)
		defer ticker.Stop()

		round := 1
		for {
			select {
			case <-stopChan:
				return
			case <-ticker.C:
				t.Logf("")
				t.Logf("━━━ 第 %d 次检测 ━━━", round)
				lastCheck = time.Now()

				// 先直接用 lark-cli 查看一下当前日程
				testAgendaOutput, _ := utils.RunLarkCommand("calendar", "+agenda", "--as", "user")
				t.Logf("当前 agenda 原始输出:")
				var prettyJSON map[string]any
				json.Unmarshal(testAgendaOutput, &prettyJSON)
				if prettyData, ok := prettyJSON["data"]; ok {
					if dataArray, ok := prettyData.([]any); ok {
						t.Logf("  当前日程数 (lark-cli): %d", len(dataArray))
						for _, ev := range dataArray {
							if evMap, ok := ev.(map[string]any); ok {
								t.Logf("  - %s (event_id: %s)", evMap["summary"], evMap["event_id"])
							}
						}
					}
				}

				// 调试：直接调用 buildCurrentState 看看
				state, _ := detector.BuildCurrentStateForDebug()
				t.Logf("  当前日程数 (buildCurrentState): %d", len(state.Events))
				for eventID, ev := range state.Events {
					t.Logf("  - %s (event_id: %s)", ev.Title, eventID)
				}

				// 执行检测
				result, err := detector.Detect(lastCheck)
				if err != nil {
					t.Logf("检测失败: %v", err)
				} else if result.HasChanges {
					t.Logf("✅ 检测到 %d 个变化！", len(result.Changes))
					for i, change := range result.Changes {
						changeMsg := fmt.Sprintf("[Calendar] %s: %s (EntityID: %s)", change.Type, change.Summary, change.EntityID)
						t.Logf("  [%d] %s", i+1, changeMsg)
						select {
						case detectedChanges <- changeMsg:
						default:
						}
					}
					detectedChan <- struct{}{}
					return
				} else {
					t.Logf("无变化，继续等待...")
				}
				round++
			}
		}
	}()

	// 3. 等待信号或检测到变化
	select {
	case <-sigChan:
		t.Log("")
		t.Log("用户取消，退出测试")
	case <-detectedChan:
		t.Log("")
		t.Log("✅ 测试成功！已检测到日程变化")
	}

	close(stopChan)
	wg.Wait()
	close(detectedChanges)

	t.Log("")
	t.Log("========================================")
	t.Log("检测到的变化汇总：")
	for change := range detectedChanges {
		t.Logf("  ✓ %s", change)
	}
	t.Log("========================================")
}

// ==================== Task 检测器并发测试 ====================

func TestConcurrentTask(t *testing.T) {
	cfg := larkadapter.LoadConfig()
	detector := larkadapter.NewTaskExtractor(cfg)
	utils := NewLarkTestUtils()

	if _, err := utils.RunLarkCommand("--help"); err != nil {
		t.Skipf("跳过并发测试: lark-cli 不可用: %v", err)
		return
	}

	t.Log("========================================")
	t.Log("Task 检测器并发测试")
	t.Log("  - 定时检测: 每 5s")
	t.Log("  - 自动模拟: 使用 lark-cli 检查任务状态")
	t.Log("  - 测试时长: 2 分钟")
	t.Log("========================================")

	var wg sync.WaitGroup
	stopChan := make(chan struct{})
	detectedChanges := make(chan string, 10)

	wg.Add(1)
	go func() {
		defer wg.Done()
		runDetectorLoop(t, "Task", detector, stopChan, detectedChanges)
	}()

	time.Sleep(testDuration)
	close(stopChan)
	wg.Wait()
	close(detectedChanges)

	t.Log("")
	t.Log("========================================")
	t.Log("测试完成！检测到的变化汇总：")
	for change := range detectedChanges {
		t.Logf("  ✓ %s", change)
	}
	t.Log("========================================")
}

// ==================== Wiki 检测器并发测试 ====================

func TestConcurrentWiki(t *testing.T) {
	cfg := larkadapter.LoadConfig()
	detector := larkadapter.NewWikiExtractor(cfg)
	utils := NewLarkTestUtils()

	if _, err := utils.RunLarkCommand("--help"); err != nil {
		t.Skipf("跳过并发测试: lark-cli 不可用: %v", err)
		return
	}

	t.Log("========================================")
	t.Log("Wiki 检测器并发测试")
	t.Log("  - 定时检测: 每 30s")
	t.Log("  - 自动模拟: 使用 lark-cli 检查知识库状态")
	t.Log("  - 测试时长: 2 分钟")
	t.Log("========================================")

	var wg sync.WaitGroup
	stopChan := make(chan struct{})
	detectedChanges := make(chan string, 10)

	wg.Add(1)
	go func() {
		defer wg.Done()
		runDetectorLoop(t, "Wiki", detector, stopChan, detectedChanges)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		runWikiAutoSimulator(t, utils, stopChan)
	}()

	time.Sleep(testDuration)
	close(stopChan)
	wg.Wait()
	close(detectedChanges)

	t.Log("")
	t.Log("========================================")
	t.Log("测试完成！检测到的变化汇总：")
	for change := range detectedChanges {
		t.Logf("  ✓ %s", change)
	}
	t.Log("========================================")
}

// ==================== 所有检测器并发测试 ====================

func TestConcurrentAll(t *testing.T) {
	cfg := larkadapter.LoadConfig()
	utils := NewLarkTestUtils()

	if _, err := utils.RunLarkCommand("--help"); err != nil {
		t.Skipf("跳过并发测试: lark-cli 不可用: %v", err)
		return
	}

	t.Log("========================================")
	t.Log("所有检测器并发测试")
	t.Log("  - 定时检测: 每 30s")
	t.Log("  - 自动模拟: 使用 lark-cli 检查所有模块状态")
	t.Log("  - 测试时长: 2 分钟")
	t.Log("========================================")

	detectors := map[string]larkadapter.Detector{
		"IM":       larkadapter.NewIMExtractor(cfg),
		"VC":       larkadapter.NewVCExtractor(cfg),
		"Docs":     larkadapter.NewDocExtractor(cfg),
		"Calendar": larkadapter.NewCalendarExtractor(cfg),
		"Task":     larkadapter.NewTaskExtractor(cfg),
		"Wiki":     larkadapter.NewWikiExtractor(cfg),
	}

	var wg sync.WaitGroup
	stopChan := make(chan struct{})
	detectedChanges := make(chan string, 20)

	// 启动所有检测器
	for name, detector := range detectors {
		wg.Add(1)
		go func(n string, d larkadapter.Detector) {
			defer wg.Done()
			if n == "Docs" {
				// 对 Docs 检测器使用内容比较功能
				runDocsDetectorWithContentCompare(t, d, stopChan, detectedChanges)
			} else {
				runDetectorLoop(t, n, d, stopChan, detectedChanges)
			}
		}(name, detector)
	}

	// 启动综合模拟器
	wg.Add(1)
	go func() {
		defer wg.Done()
		runAllAutoSimulators(t, utils, stopChan)
	}()

	time.Sleep(testDuration)
	close(stopChan)
	wg.Wait()
	close(detectedChanges)

	t.Log("")
	t.Log("========================================")
	t.Log("所有检测器测试完成！检测到的变化汇总：")
	for change := range detectedChanges {
		t.Logf("  ✓ %s", change)
	}
	t.Log("========================================")
}

// ==================== 辅助函数 ====================

// runDetectorLoop 运行检测器循环
func runDetectorLoop(t *testing.T, name string, detector larkadapter.Detector, stopChan <-chan struct{}, detectedChan chan<- string) {
	t.Logf("[%s] 🔍 检测器进程已启动", name)

	lastCheck := time.Now().Add(-1 * time.Hour)
	ticker := time.NewTicker(detectInterval)
	defer ticker.Stop()

	round := 1
	for {
		select {
		case <-stopChan:
			t.Logf("[%s] 检测器进程停止", name)
			return
		case <-ticker.C:
			t.Logf("")
			t.Logf("[%s] ━━━━ 第 %d 轮检测 ━━━━", name, round)

			result, err := detector.Detect(lastCheck)
			if err != nil {
				t.Logf("[%s] ❌ 检测失败: %v", name, err)
				continue
			}

			t.Logf("[%s] 检测结果: HasChanges=%v, Changes=%d", name, result.HasChanges, len(result.Changes))

			for i, change := range result.Changes {
				changeMsg := fmt.Sprintf("[%s] %s - %s", name, change.Type, change.Summary)
				t.Logf("[%s]   ✓ [%d] %s", name, i+1, change.Summary)
				select {
				case detectedChan <- changeMsg:
				default:
				}
			}

			lastCheck = time.Now()
			round++
		}
	}
}

// runDocsDetectorWithContentCompare 运行带内容比较的 Docs 检测器循环
func runDocsDetectorWithContentCompare(t *testing.T, detector interface{}, stopChan <-chan struct{}, detectedChan chan<- string) {
	t.Log("[Docs] 🔍 Docs 检测器进程已启动（带内容比较）")

	// 类型断言
	docExtractor, ok := detector.(*larkadapter.DocExtractor)
	if !ok {
		t.Log("[Docs] ⚠️ 类型断言失败，回退到普通检测")
		// 回退到普通检测
		return
	}

	lastCheck := time.Now().Add(-1 * time.Hour)
	ticker := time.NewTicker(detectInterval)
	defer ticker.Stop()

	round := 1
	detectedDocTokens := make(map[string]bool)

	for {
		select {
		case <-stopChan:
			t.Log("[Docs] 检测器进程停止")
			return
		case <-ticker.C:
			t.Logf("")
			t.Logf("[Docs] ━━━━ 第 %d 轮检测 ━━━━", round)

			// 1. 先执行普通检测
			result, err := docExtractor.Detect(lastCheck)
			if err != nil {
				t.Logf("[Docs] ❌ 检测失败: %v", err)
				continue
			}

			t.Logf("[Docs] 检测结果: HasChanges=%v, Changes=%d", result.HasChanges, len(result.Changes))

			for i, change := range result.Changes {
				changeMsg := fmt.Sprintf("[Docs] %s - %s", change.Type, change.Summary)
				t.Logf("[Docs]   ✓ [%d] %s", i+1, change.Summary)
				select {
				case detectedChan <- changeMsg:
				default:
				}

				// 尝试获取文档 token 并比较内容
				docToken := change.EntityID
				if docToken != "" && !detectedDocTokens[docToken] {
					detectedDocTokens[docToken] = true
					t.Logf("[Docs]   📝 发现新文档，尝试比较内容变化...")
					checkAndCompareDocContent(t, docExtractor, docToken)
				}
			}

			lastCheck = time.Now()
			round++
		}
	}
}

// checkAndCompareDocContent 检查并比较文档内容
func checkAndCompareDocContent(t *testing.T, docExtractor *larkadapter.DocExtractor, docToken string) {
	// 获取文档内容
	diff, diffs, err := docExtractor.GetDocumentContentDiff(docToken)
	if err != nil {
		t.Logf("[Docs]   ⚠️ 获取文档内容失败: %v", err)
		return
	}

	if diffs == nil {
		t.Logf("[Docs]   📝 第一次获取文档，已缓存当前内容")
		return
	}

	// 有内容变化，显示差异
	t.Logf("[Docs]   📊 文档内容有变化！")
	t.Logf("[Docs]   变化内容:")
	t.Logf("%s", diff)

	// 获取变化摘要
	summary := docExtractor.GetContentChangeSummary(diffs)
	if summary["added"] > 0 || summary["deleted"] > 0 {
		t.Logf("[Docs]   变化统计: +%d, -%d", summary["added"], summary["deleted"])
	}
}

// ==================== 各个模块的自动模拟器（使用 lark-cli） ====================

// runVCAutoSimulator VC 自动模拟器
func runVCAutoSimulator(t *testing.T, utils *LarkTestUtils, stopChan <-chan struct{}, detectedChan <-chan string) {
	t.Log("[VC] 🤖 VC 自动模拟器进程已启动")

	ticker := time.NewTicker(simulateInterval)
	defer ticker.Stop()

	round := 1
	for {
		select {
		case <-stopChan:
			t.Log("[VC] 自动模拟器进程停止")
			return
		case <-ticker.C:
			t.Logf("")
			t.Logf("[VC] ━━━━ 模拟操作 #%d ━━━━", round)

			// 使用 lark-cli 检查会议状态
			t.Log("[VC] 📋 检查 VC 相关命令...")
			_, err := utils.RunLarkCommand("vc", "--help")
			if err != nil {
				t.Logf("[VC] ⚠️ vc 命令可能不可用: %v", err)
			} else {
				t.Logf("[VC] ✓ vc 命令可用")
				vcOutput, err := utils.RunLarkCommand("vc", "+search")
				if err != nil {
					t.Logf("[VC] ⚠️ vc +search 失败: %v (可能需要参数)，这是正常的", err)
				} else if len(vcOutput) > 0 {
					t.Logf("[VC] ✓ 获取到 VC 数据: %d bytes", len(vcOutput))
					t.Logf("[VC] 📊 数据预览: %.150s...", string(vcOutput))
				}
			}

			// 也尝试检查妙记
			t.Log("[VC] 📋 检查妙记列表...")
			minutesOutput, err := utils.RunLarkCommand("minutes", "+list")
			if err != nil {
				t.Logf("[VC] ⚠️ minutes +list 命令失败: %v", err)
			} else {
				t.Logf("[VC] ✓ 获取到妙记数据: %d bytes", len(minutesOutput))
				if len(minutesOutput) > 0 {
					t.Logf("[VC] 📊 妙记预览: %.150s...", string(minutesOutput))
				}
			}

			round++
		}
	}
}

// runIMAutoSimulator IM 自动模拟器
func runIMAutoSimulator(t *testing.T, utils *LarkTestUtils, stopChan <-chan struct{}) {
	t.Log("[IM] 🤖 IM 自动模拟器进程已启动")

	ticker := time.NewTicker(simulateInterval)
	defer ticker.Stop()

	round := 1
	for {
		select {
		case <-stopChan:
			t.Log("[IM] 自动模拟器进程停止")
			return
		case <-ticker.C:
			t.Logf("")
			t.Logf("[IM] ━━━━ 模拟操作 #%d ━━━━", round)

			// 使用 lark-cli 检查消息状态
			t.Log("[IM] 📋 检查聊天列表...")
			imOutput, err := utils.RunLarkCommand("im", "chats")
			if err != nil {
				t.Logf("[IM] ⚠️ im chats 命令失败: %v", err)
				t.Logf("[IM] 💡 提示: 可能需要额外权限，可以继续测试其他部分")
			} else {
				t.Logf("[IM] ✓ 获取到聊天数据: %d bytes", len(imOutput))
				if len(imOutput) > 0 {
					t.Logf("[IM] 📊 数据预览: %.150s...", string(imOutput))
				}
			}

			round++
		}
	}
}

// runDocsAutoSimulator Docs 自动模拟器
func runDocsAutoSimulator(t *testing.T, utils *LarkTestUtils, stopChan <-chan struct{}) {
	t.Log("[Docs] 🤖 Docs 自动模拟器进程已启动")

	ticker := time.NewTicker(simulateInterval)
	defer ticker.Stop()

	round := 1
	for {
		select {
		case <-stopChan:
			t.Log("[Docs] 自动模拟器进程停止")
			return
		case <-ticker.C:
			t.Logf("")
			t.Logf("[Docs] ━━━━ 模拟操作 #%d ━━━━", round)

			t.Log("[Docs] 📋 检查文档列表...")
			docsOutput, err := utils.RunLarkCommand("docs", "+search")
			if err != nil {
				t.Logf("[Docs] ⚠️ docs +search 命令失败: %v", err)
			} else {
				t.Logf("[Docs] ✓ 获取到文档数据: %d bytes", len(docsOutput))
				if len(docsOutput) > 0 {
					t.Logf("[Docs] 📊 数据预览: %.150s...", string(docsOutput))
				}
			}

			round++
		}
	}
}

// runCalendarAutoSimulator Calendar 自动模拟器
func runCalendarAutoSimulator(t *testing.T, utils *LarkTestUtils, stopChan <-chan struct{}) {
	t.Log("[Calendar] 🤖 Calendar 自动模拟器进程已启动")

	ticker := time.NewTicker(simulateInterval)
	defer ticker.Stop()

	round := 1
	for {
		select {
		case <-stopChan:
			t.Log("[Calendar] 自动模拟器进程停止")
			return
		case <-ticker.C:
			t.Logf("")
			t.Logf("[Calendar] ━━━━ 模拟操作 #%d ━━━━", round)

			t.Log("[Calendar] 📋 检查今日日程...")
			calOutput, err := utils.RunLarkCommand("calendar", "+agenda")
			if err != nil {
				t.Logf("[Calendar] ⚠️ calendar +agenda 命令失败: %v", err)
			} else {
				t.Logf("[Calendar] ✓ 获取到日程数据: %d bytes", len(calOutput))
				if len(calOutput) > 0 {
					t.Logf("[Calendar] 📊 数据预览: %.150s...", string(calOutput))
				}
			}

			round++
		}
	}
}

// runTaskAutoSimulator Task 自动模拟器
func runTaskAutoSimulator(t *testing.T, utils *LarkTestUtils, stopChan <-chan struct{}) {
	t.Log("[Task] 🤖 Task 自动模拟器进程已启动")

	ticker := time.NewTicker(simulateInterval)
	defer ticker.Stop()

	round := 1
	for {
		select {
		case <-stopChan:
			t.Log("[Task] 自动模拟器进程停止")
			return
		case <-ticker.C:
			t.Logf("")
			t.Logf("[Task] ━━━━ 模拟操作 #%d ━━━━", round)

			t.Log("[Task] 📋 检查我的任务...")
			taskOutput, err := utils.RunLarkCommand("task", "+get-my-tasks")
			if err != nil {
				t.Logf("[Task] ⚠️ task +get-my-tasks 命令失败: %v", err)
			} else {
				t.Logf("[Task] ✓ 获取到任务数据: %d bytes", len(taskOutput))
				if len(taskOutput) > 0 {
					t.Logf("[Task] 📊 数据预览: %.150s...", string(taskOutput))
				}
			}

			round++
		}
	}
}

// runWikiAutoSimulator Wiki 自动模拟器
func runWikiAutoSimulator(t *testing.T, utils *LarkTestUtils, stopChan <-chan struct{}) {
	t.Log("[Wiki] 🤖 Wiki 自动模拟器进程已启动")

	ticker := time.NewTicker(simulateInterval)
	defer ticker.Stop()

	round := 1
	for {
		select {
		case <-stopChan:
			t.Log("[Wiki] 自动模拟器进程停止")
			return
		case <-ticker.C:
			t.Logf("")
			t.Logf("[Wiki] ━━━━ 模拟操作 #%d ━━━━", round)

			t.Log("[Wiki] 📋 检查知识空间...")
			wikiOutput, err := utils.RunLarkCommand("wiki", "spaces", "list")
			if err != nil {
				t.Logf("[Wiki] ⚠️ wiki spaces list 命令失败: %v", err)
			} else {
				t.Logf("[Wiki] ✓ 获取到知识库数据: %d bytes", len(wikiOutput))
				if len(wikiOutput) > 0 {
					t.Logf("[Wiki] 📊 数据预览: %.150s...", string(wikiOutput))
				}
			}

			round++
		}
	}
}

// runAllAutoSimulators 综合自动模拟器
func runAllAutoSimulators(t *testing.T, utils *LarkTestUtils, stopChan <-chan struct{}) {
	t.Log("[All] 🤖 综合自动模拟器进程已启动")

	ticker := time.NewTicker(simulateInterval)
	defer ticker.Stop()

	round := 1
	commands := [][]string{
		{"im", "chats"},
		{"vc", "+search"},
		{"docs", "+search"},
		{"calendar", "+agenda"},
		{"task", "+get-my-tasks"},
		{"wiki", "spaces", "list"},
	}

	for {
		select {
		case <-stopChan:
			t.Log("[All] 综合自动模拟器进程停止")
			return
		case <-ticker.C:
			t.Logf("")
			t.Logf("[All] ━━━━ 综合模拟操作 #%d ━━━━", round)

			// 循环执行各个命令
			cmdIndex := (round - 1) % len(commands)
			cmd := commands[cmdIndex]

			t.Logf("[All] 📋 执行: lark-cli %v", cmd)
			output, err := utils.RunLarkCommand(cmd...)
			if err != nil {
				t.Logf("[All] ⚠️ 命令失败: %v", err)
			} else {
				t.Logf("[All] ✓ 命令执行成功: %d bytes", len(output))
				if len(output) > 0 {
					t.Logf("[All] 📊 数据预览: %.150s...", string(output))
				}
			}

			round++
		}
	}
}
