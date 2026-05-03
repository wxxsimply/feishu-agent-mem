# 文档内容比较功能

使用 `github.com/sergi/go-diff` 库实现的飞书文档内容变化检测和比较功能。

## 📦 依赖安装

已自动添加到 `go.mod`：

```go
require github.com/sergi/go-diff v1.3.1
```

运行 `go mod tidy` 安装依赖。

## 🎯 新增功能

### DocExtractor 新增方法

| 方法 | 说明 |
|------|------|
| `FetchDocumentContent(docToken)` | 获取文档 Markdown 内容 |
| `CompareDocumentContent(docToken, old, new)` | 比较文档内容，返回可视化差异 |
| `GetDocumentContentDiff(docToken)` | 获取内容变化（自动缓存旧版本） |
| `HasContentChanged(docToken, old, new)` | 检查内容是否有变化 |
| `GetContentChangeSummary(diffs)` | 获取变化统计摘要 |

## 💡 使用示例

### 1. 基本内容比较

```go
import larkadapter "feishu-mem/internal/lark-adapter"

cfg := larkadapter.LoadConfig()
docExtractor := larkadapter.NewDocExtractor(cfg)

oldContent := "项目计划\n- 阶段1\n- 阶段2"
newContent := "项目计划\n- 阶段1\n- 阶段2\n- 阶段3 (新增)"

// 比较内容
prettyDiff, diffs := docExtractor.CompareDocumentContent("test-doc", oldContent, newContent)
fmt.Println(prettyDiff)
```

### 2. 自动缓存比较

```go
docToken := "GLXCwtMeKiFzV7k1XyccgWgBnad"

// 第一次调用：缓存当前内容
diff, diffs, err := docExtractor.GetDocumentContentDiff(docToken)

// ... 文档内容发生变化 ...

// 第二次调用：自动比较新旧版本
diff, diffs, err = docExtractor.GetDocumentContentDiff(docToken)
```

### 3. 检查是否有变化

```go
if docExtractor.HasContentChanged(docToken, oldContent, newContent) {
    fmt.Println("文档内容有变化！")
}
```

### 4. 获取变化统计

```go
changeSummary := docExtractor.GetContentChangeSummary(diffs)
fmt.Printf("新增: %d 字符\n", changeSummary["added"])
fmt.Printf("删除: %d 字符\n", changeSummary["deleted"])
fmt.Printf("总变化: %d 字符\n", changeSummary["changed"])
```

## 📊 测试示例

运行测试：

```bash
go test -v ./test/detector -run TestDocContentDiff
```

预期输出：

```
=== RUN   TestDocContentDiff
    doc_content_diff_test.go:12: === 文档内容比较功能演示 ===
    doc_content_diff_test.go:31: 
        --- 原内容 ---
    doc_content_diff_test.go:32: Dwdadhkawhdgaku
        
        Dawhkudhad
        
    doc_content_diff_test.go:33: 
        --- 新内容 ---
    doc_content_diff_test.go:34: Dwdadhkawhdgaku
        
        Dawhkudhad
        
        Dkauwhdgka
        
    doc_content_diff_test.go:39: 
        --- 比较结果 ---
    doc_content_diff_test.go:40: Dwdadhkawhdgaku
        
        Dawhkudhad
        
        Dkauwhdgka
        
    doc_content_diff_test.go:44: 
        内容是否有变化: true
    doc_content_diff_test.go:48: 
        变化摘要:
    doc_content_diff_test.go:49:   - 新增字符数: 12
    doc_content_diff_test.go:50:   - 删除字符数: 0
    doc_content_diff_test.go:51:   - 总变化字符数: 0
--- PASS: TestDocContentDiff (0.00s)
```

## 🔍 差异输出格式

使用 go-diff 的 `DiffPrettyText()` 输出，包含：

- **无颜色**（默认）：普通文本（相同部分）
- **绿色**（`\033[32m`）：新增内容
- **红色**（`\033[31m`）：删除内容

## 🎓 完整工作流示例

```go
package main

import (
    "fmt"
    "time"

    larkadapter "feishu-mem/internal/lark-adapter"
)

func main() {
    cfg := larkadapter.LoadConfig()
    docExtractor := larkadapter.NewDocExtractor(cfg)

    docToken := "your-doc-token-here"

    // 1. 第一次检测：建立基线
    fmt.Println("=== 第一次检测 ===")
    diff1, diffs1, err1 := docExtractor.GetDocumentContentDiff(docToken)
    if err1 != nil {
        fmt.Printf("错误: %v\n", err1)
    }

    // 2. 等待用户修改文档
    fmt.Println("\n请修改文档内容，然后按回车继续...")
    var waitInput string
    fmt.Scanln(&waitInput)

    // 3. 第二次检测：查看变化
    fmt.Println("\n=== 第二次检测 ===")
    diff2, diffs2, err2 := docExtractor.GetDocumentContentDiff(docToken)
    if err2 != nil {
        fmt.Printf("错误: %v\n", err2)
        return
    }

    if diffs2 != nil {
        fmt.Println("\n=== 文档内容变化 ===")
        fmt.Println(diff2)

        summary := docExtractor.GetContentChangeSummary(diffs2)
        fmt.Printf("\n变化统计: +%d, -%d\n", summary["added"], summary["deleted"])
    }
}
```

## 📝 注意事项

1. **内容缓存**：`GetDocumentContentDiff` 会自动缓存每个文档的上次内容
2. **权限要求**：需要 `docs:read` 权限来获取文档内容
3. **性能考虑**：每次比较都会调用飞书 API 获取内容，不适合高频调用
4. **Markdown 格式**：目前只支持比较 Markdown 格式内容

## 🎉 功能特点

✅ 自动内容缓存，无需手动保存历史版本  
✅ 可视化差异输出，直观查看变化  
✅ 变化统计摘要，快速了解变动规模  
✅ 与现有检测器无缝集成  
✅ 完善的错误处理
