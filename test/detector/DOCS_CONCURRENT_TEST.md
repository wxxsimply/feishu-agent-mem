# Docs 检测器并发测试（带内容比较）

## 概述

`TestConcurrentDocs` 是一个增强的并发测试，使用新增的 DocExtractor 内容比较功能：

1. **进程1** - DocExtractor 检测器（每 30 秒）
   - 普通检测：发现文档变化
   - 内容比较：使用 go-diff 比较文档内容变化
   - 自动缓存：第一次获取内容后缓存，后续比较变化

2. **进程2** - lark-cli 模拟器
   - 自动检查文档状态
   - 使用 `docs +search` 命令

## 新增功能

### runDocsDetectorWithContentCompare

专门的 Docs 检测器循环，集成内容比较功能：

- 检测到新文档时自动获取内容
- 使用缓存的旧版本比较变化
- 使用 `github.com/sergi/go-diff` 可视化差异
- 显示变化统计（新增/删除字符数）

### checkAndCompareDocContent

文档内容检查和比较函数：

- 调用 `GetDocumentContentDiff` 获取差异
- 显示 `DiffPrettyText` 格式化结果
- 使用 `GetContentChangeSummary` 显示统计

## 使用方法

### 方式1：使用脚本

```bash
# 只测试 Docs 检测器（带内容比较）
./test/detector/run_concurrent_test.sh docs

# 测试所有检测器（Docs 会使用内容比较）
./test/detector/run_concurrent_test.sh all
```

### 方式2：直接运行 go test

```bash
# 只测试 Docs 检测器
go test -v -tags=concurrent ./test/detector -run TestConcurrentDocs

# 测试所有检测器
go test -v -tags=concurrent ./test/detector -run TestConcurrentAll
```

## 示例输出

```
[Docs] 🔍 Docs 检测器进程已启动（带内容比较）

[Docs] ━━━━ 第 1 轮检测 ━━━━
[Docs] 检测结果: HasChanges=true, Changes=1
[Docs]   ✓ [1] doc_created - 新建新文档: Xinjianwendang 1
[Docs]   📝 发现新文档，尝试比较内容变化...
[Docs]   📝 第一次获取文档，已缓存当前内容

[Docs] ━━━━ 第 2 轮检测 ━━━━
[Docs] 检测结果: HasChanges=true, Changes=1
[Docs]   ✓ [1] doc_content_updated - 新文档内容更新: Xinjianwendang 1
[Docs]   📊 文档内容有变化！
[Docs]   变化内容:
Dwdadhkawhdgaku

Dawhkudhad
Dkauwhdgka
[Docs]   变化统计: +12, -0
```

## 与其他模块集成

在 `TestConcurrentAll` 中，Docs 检测器会自动使用内容比较功能，而其他模块使用普通检测。

## 主要改进

1. ✅ 新增 go-diff 依赖
2. ✅ DocExtractor 新增内容比较方法
3. ✅ 自动缓存文档内容历史版本
4. ✅ 并发测试中集成内容比较功能
5. ✅ 可视化差异显示
6. ✅ 变化统计报告
