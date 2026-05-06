# lark-doc 白名单 + 防抖 + LLM 决策提取 测试报告

**测试日期**: 2026-05-07
**测试人员**: 莫文豪
**测试目标**: 验证白名单文档检测、防抖机制、LLM 决策提取的完整流程

---

## 1. 测试环境

| 配置项 | 值 |
|--------|-----|
| 服务模式 | mem-service (v2) |
| LLM 模型 | deepseek-chat (Deepseek API) |
| LLM SDK | OpenAI SDK (兼容) |
| 白名单文档 | `CKCtduQ03o7mSTx7KdJcGZJqnrb` (Q2后端架构升级方案) |
| 防抖窗口 | 120s |
| 检测间隔 | 10s (normal) / 5s (burst) |
| 已有决策数 | 29 |
| 检测器 | lark_doc only (IM disabled) |

---

## 2. 修改清单

### 2.1 已完成的代码修改

| # | 文件 | 修改内容 | 状态 |
|---|------|----------|------|
| 1 | `internal/lark-adapter/lark_doc.go` | 移除 `containsDecisionKeyword` 和 `decisionKeywords` | ✅ |
| 2 | `internal/lark-adapter/lark_doc.go` | `searchDocsByTime` 添加白名单过滤 | ✅ |
| 3 | `internal/lark-adapter/lark_doc.go` | `checkWhitelistedDocs` 修复首次缓存逻辑（先缓存再判断变更） | ✅ |
| 4 | `internal/lark-adapter/lark_doc.go` | `checkWhitelistedDocs` 支持防抖检查周期 | ✅ |
| 5 | `internal/lark-adapter/lark_doc.go` | 所有 doc change 统一为 `doc_updated` 类型 | ✅ |
| 6 | `internal/lark-adapter/debounce_tracker.go` | 修复 contentHash 不变时不更新 LastChange | ✅ |
| 7 | `internal/lark-adapter/debounce_tracker.go` | 添加 `GetReadyDocs()` 方法 | ✅ |
| 8 | `internal/signal/emitter.go` | 移除 DocsEmitter 中的关键词匹配（IMEmitter 保留） | ✅ |
| 9 | `internal/signal/worker.go` | 移除 `processDocFallback` 和 `processWikiFallback` | ✅ |
| 10 | `internal/signal/worker.go` | 移除 `containsDecisionKeyword` 判断 | ✅ |
| 11 | `internal/llm/agent.go` | dedup fallback 改为返回 error | ✅ |
| 12 | `internal/signal/engine.go` | 删除废弃的 `matchDecisionKeywords` | ✅ |
| 13 | `config/openclaw.yaml` | IM 禁用，lark_doc 启用 | ✅ |

### 2.2 未修改（确认可保留）

| 文件 | 内容 | 原因 |
|------|------|------|
| `internal/signal/emitter.go` | IMEmitter 的 `MatchKeywords` | 仅用于 IM 消息快速分类，非文档决策 |

---

## 3. 测试结果

### 3.1 白名单过滤

| 测试项 | 结果 | 日志证据 |
|--------|------|----------|
| 首次检测缓存 Hash | ✅ | `Whitelist first seen: CKCtduQ03o7mSTx7KdJcGZJqnrb (hash=4b0a3e59)` |
| 非白名单文档过滤 | ✅ | `filtered out 15 old results, 15 dedup skipped`（search 结果全部过滤） |
| 内容变更检测 | ✅ | `Whitelist content changed: CKCtduQ03o7mSTx7KdJcGZJqnrb (old=4b0a3e59 new=6ca3dc9b)` |

### 3.2 防抖机制

| 测试项 | 结果 | 日志证据 |
|--------|------|----------|
| 防抖启动 | ✅ | `Debounce] Skipping change 1: waiting for debounce: 2m0s remaining` |
| 防抖计时器递减 | ✅ | `1m48s → 1m25s → 1m14s → 51s → 39s → 28s → 16s → 5s` |
| 防抖到期触发 | ✅ | 2分钟后自动提交 Worker 处理 |
| 防抖期间不产生重复 Change | ✅ | 每次检测只产生 1 个 debounce check change |
| 处理完成标记 | ✅ | `Marked doc CKCtduQ03o7mSTx7KdJcGZJqnrb as processed` |

### 3.3 LLM 决策提取

| 测试项 | 结果 | 说明 |
|--------|------|------|
| 决策提取数 | ✅ 12 条 | 从文档中提取了所有决策（含已存在的和新增的） |
| 非决策过滤 | ✅ | 无垃圾决策（无 "auto-extracted decision"） |
| 去重检查 | ✅ | 29 条已有决策，全部正确比对去重 |

提取的决策标题（全部为合法技术决策）：
- 采用MySQL 8.0作为主数据库
- 增加MySQL读写分离架构
- 采用Redis 7.x集群缓存
- 采用Prometheus+Grafana作为监控方案
- 选型GitHub Actions作为CI/CD工具
- 制定数据库备份策略
- 等共12条

### 3.4 推送通知

| 测试项 | 结果 |
|--------|------|
| 决策卡片推送 | ✅ 12 条决策卡片全部成功推送到飞书群 `oc_096c0cd1dfe93cb2f1264e59490946d2` |

### 3.5 已知问题

| 问题 | 严重度 | 说明 |
|------|--------|------|
| Bitable `decision_type` 字段不存在 | 低 | 飞书多维表缺少 `decision_type` 字段，但不影响 Git 存储和推送 |

---

## 4. 验证清单

```
✅ 白名单文档变更检测
✅ 非白名单文档被过滤
✅ 防抖 120s 静默期
✅ 防抖计时器持续递减
✅ 静默期到期后触发提取
✅ LLM 正确提取决策（无关键词匹配）
✅ 去重检查
✅ 决策推送通知
✅ 所有 change type 为 doc_updated（非 doc_decision）
✅ 无 containsDecisionKeyword 调用
✅ 无 processDocFallback 调用
✅ 无 processWikiFallback 调用
✅ 无 "auto-extracted decision" 日志
```

---

## 5. 结论

**测试通过。** lark-doc 的白名单过滤、防抖机制、LLM 决策提取完整链路运行正常。所有修改均已验证有效：

1. 白名单文档变更通过 content hash 比对精确检测
2. 非白名单文档在检测早期阶段即被过滤
3. 防抖 120s 窗口正常工作，计时器持续递减，到期后自动触发处理
4. LLM 成功提取文档中的技术决策，无垃圾决策产生
5. 决策卡片通过飞书群推送通知

剩余的小问题（Bitable 字段映射）为预先存在的配置问题，不影响核心功能。
