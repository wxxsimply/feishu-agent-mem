package detector

import (
	"fmt"
	"testing"

	larkadapter "feishu-mem/internal/lark-adapter"
)

// TestDocContentDiff 演示文档内容比较功能
func TestDocContentDiff(t *testing.T) {
	t.Log("=== 文档内容比较功能演示 ===")

	cfg := larkadapter.LoadConfig()
	docExtractor := larkadapter.NewDocExtractor(cfg)

	// 示例旧内容
	oldContent := `Dwdadhkawhdgaku

Dawhkudhad
`

	// 示例新内容
	newContent := `Dwdadhkawhdgaku

Dawhkudhad

Dkauwhdgka
`

	t.Log("\n--- 原内容 ---")
	t.Log(oldContent)
	t.Log("\n--- 新内容 ---")
	t.Log(newContent)

	// 比较内容
	prettyDiff, diffs := docExtractor.CompareDocumentContent("test-doc", oldContent, newContent)

	t.Log("\n--- 比较结果 ---")
	t.Log(prettyDiff)

	// 检查是否有变化
	hasChanged := docExtractor.HasContentChanged("test-doc", oldContent, newContent)
	t.Logf("\n内容是否有变化: %v", hasChanged)

	// 获取变化摘要
	changeSummary := docExtractor.GetContentChangeSummary(diffs)
	t.Logf("\n变化摘要:")
	t.Logf("  - 新增字符数: %d", changeSummary["added"])
	t.Logf("  - 删除字符数: %d", changeSummary["deleted"])
	t.Logf("  - 总变化字符数: %d", changeSummary["changed"])
}

// TestRealDocDiff 演示获取真实文档内容并比较（如果有缓存）
func TestRealDocDiff(t *testing.T) {
	t.Log("=== 真实文档内容比较演示 ===")

	cfg := larkadapter.LoadConfig()
	docExtractor := larkadapter.NewDocExtractor(cfg)

	// 这个演示假设我们已经有一个文档的 token
	// 实际使用时替换为真实的 doc token
	testDocToken := "GLXCwtMeKiFzV7k1XyccgWgBnad"

	t.Logf("文档 Token: %s", testDocToken)

	// 第一次获取（缓存当前内容）
	t.Log("\n第一次获取文档内容...")
	diff, diffs, err := docExtractor.GetDocumentContentDiff(testDocToken)

	if err != nil {
		t.Logf("获取文档失败: %v", err)
		t.Log("(这是预期的，如果文档不存在或没有权限)")
	} else if diffs == nil {
		t.Log("第一次获取，已缓存当前内容，没有旧内容比较")
	} else {
		t.Logf("\n内容比较结果:")
		t.Log(diff)
	}
}

// ExampleDocContentDiff 文档内容比较使用示例
func ExampleDocExtractor_CompareDocumentContent() {
	cfg := larkadapter.LoadConfig()
	docExtractor := larkadapter.NewDocExtractor(cfg)

	// 旧版本内容
	oldContent := `项目计划
- 阶段1
- 阶段2
`

	// 新版本内容
	newContent := `项目计划
- 阶段1
- 阶段2
- 阶段3 (新增)
`

	// 比较内容
	prettyDiff, diffs := docExtractor.CompareDocumentContent("project-plan", oldContent, newContent)

	fmt.Println("=== 文档内容比较 ===")
	fmt.Println(prettyDiff)

	// 检查是否有变化
	if docExtractor.HasContentChanged("project-plan", oldContent, newContent) {
		fmt.Println("\n文档内容有变化！")

		// 获取变化统计
		summary := docExtractor.GetContentChangeSummary(diffs)
		fmt.Printf("新增: %d 字符\n", summary["added"])
		fmt.Printf("删除: %d 字符\n", summary["deleted"])
		fmt.Printf("总变化: %d 字符\n", summary["changed"])
	}
}

// ExampleDocContentCache 文档内容缓存使用示例
func ExampleDocExtractor_GetDocumentContentDiff() {
	cfg := larkadapter.LoadConfig()
	docExtractor := larkadapter.NewDocExtractor(cfg)

	docToken := "your-doc-token-here"

	// 第一次调用：缓存当前内容
	fmt.Println("第一次获取文档...")
	_, diffs1, err1 := docExtractor.GetDocumentContentDiff(docToken)
	if err1 != nil {
		fmt.Printf("错误: %v\n", err1)
		return
	}
	if diffs1 == nil {
		fmt.Println("第一次获取，已缓存内容")
	}

	// 假设文档内容发生了变化...

	// 第二次调用：比较新旧内容
	fmt.Println("\n第二次获取文档（假设内容已变化）...")
	diff2, diffs2, err2 := docExtractor.GetDocumentContentDiff(docToken)
	if err2 != nil {
		fmt.Printf("错误: %v\n", err2)
		return
	}
	if diffs2 != nil {
		fmt.Println("\n内容变化比较:")
		fmt.Println(diff2)

		summary := docExtractor.GetContentChangeSummary(diffs2)
		fmt.Printf("\n变化统计: +%d, -%d\n", summary["added"], summary["deleted"])
	}
}
