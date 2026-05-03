# 知识库内容比较功能

为 WikiExtractor 添加了类似 DocExtractor 的内容缓存和比较功能。

## 新增方法

### `FetchWikiNodeContent(nodeToken string)`
获取知识库节点的 Markdown 内容

### `CompareWikiNodeContent(nodeToken, old, new string)`
比较内容，返回可视化差异

### `GetWikiNodeContentDiff(nodeToken string)`
获取内容变化（自动缓存旧版本）

### `HasWikiNodeContentChanged(nodeToken, old, new string)`
检查内容是否有变化

### `GetWikiNodeContentChangeSummary(diffs)`
获取变化统计摘要

## 使用示例

```go
cfg := larkadapter.LoadConfig()
wikiExtractor := larkadapter.NewWikiExtractor(cfg)

nodeToken := "your-node-token"

// 第一次：建立基线
diff1, diffs1, err1 := wikiExtractor.GetWikiNodeContentDiff(nodeToken)

// 文档修改后...

// 第二次：获取变化
diff2, diffs2, err2 := wikiExtractor.GetWikiNodeContentDiff(nodeToken)
if diffs2 != nil {
    fmt.Println(diff2)
    summary := wikiExtractor.GetWikiNodeContentChangeSummary(diffs2)
    fmt.Printf("+%d, -%d\n", summary["added"], summary["deleted"])
}
```
