# feishu-agent-mem 整体评测方案

## 背景

`feishu-agent-mem` 是一个飞书决策记忆系统，通过轮询飞书 API（IM、文档、会议、日历、任务、Wiki、OKR、联系人、妙记）检测决策信号，经多因子评分 + LLM 提取后，双向持久化到 Git 仓库和飞书多维表格，并通过 MCP 协议对外提供查询接口。

当前测试现状：

- 约 90-100 个测试函数，但分布严重偏向集成测试（依赖真实 API 凭据）
- 纯单元测试仅约 12 个，`internal/` 下大部分核心包无白盒测试
- 无 Mock 层，所有测试都依赖真实飞书/LLM API
- 无 CI/CD 流水线，Makefile test 目标已损坏
- 无性能基准测试、无模糊测试、无安全测试、无覆盖率门禁

以下方案覆盖 8 个层次、7 种测试方法，可逐步落地。

---

## 一、各层级评测指标

### 1. Lark Adapters（9 个探测器）

| 指标 | 定义 | 测量方式 | 目标 |
|------|------|----------|------|
| 检测延迟 P50/P95/P99 | `Detect()` 调用耗时 | `time.Now()` 埋点 | P50 < 3s, P95 < 10s |
| 变更召回率 | 真实变更中被检测到的比例 | 人工标注时间窗口对比 | > 90% |
| 变更精确率 | 检测结果中真实变更的比例 | 审计 `DetectResult.Changes` | > 95% |
| API 错误率 | 飞书 API 调用失败比例 | 计数 `RunCommand()` 错误 | < 2% |
| 重复率 | 连续轮询中重复发出的变更 | 比较 `EntityID` 去重 | 0% |
| 上下文覆盖率 | 携带 `ContextText` 的变更占比 | 检查 `Change.ContextText` | > 80% |

### 2. Signal Engine（多因子评分）

| 指标 | 定义 | 目标 |
|------|------|------|
| 检测精确率 | 标记为 IsDecision 的信号中真实决策比例 | > 85% |
| 检测召回率 | 真实决策中被检测到的比例 | > 80% |
| 评分校准 | 0.65 阈值是否真正区分决策/非决策 | AUC > 0.90 |
| 反信号准确率 | 反信号正确阻止噪声的比例 | > 90% |
| 工作池吞吐量 | 每秒处理的作业数 | > 50 jobs/sec |
| 状态机正确性 | 合法状态转换的比例 | 100% |
| SDR ID 唯一性 | 重复 ID 比例 | 0% |

### 3. LLM Agent

| 指标 | 定义 | 目标 |
|------|------|------|
| 提取精确率 | 提取结果中包含真实决策的比例 | > 85% |
| 提取召回率 | 标注决策中被提取的比例 | > 80% |
| 分类准确率 | 主题分类准确率 | > 90% |
| 跨主题检测准确率 | 跨主题标记准确率 | 精确率 > 80%, 召回率 > 70% |
| 冲突检测一致性 | 与人工评分的 Cohen's Kappa | > 0.7 |
| 降级覆盖率 | LLM 失败时降级产生可用结果的比例 | > 70% |
| JSON 解析成功率 | LLM 响应成功解析的比例 | > 95% |
| 幻觉率 | Agent 添加源文本没有的信息 | < 5% |
| 熔断器有效性 | 正确阈值跳闸 | 达到 N 次失败后跳闸 |

### 4. Pipeline Engine & Core

| 指标 | 目标 |
|------|------|
| 写入延迟 | < 5s |
| 批量吞吐量 | > 20 mutations/s |
| 校验拒绝率 | 100% 非法 mutation 被拒绝 |
| 内存图一致性 | 0 个孤儿节点 |

### 5. 存储层

| 指标 | 目标 |
|------|------|
| Git 写入成功率 | > 99% |
| 飞书多维表格写入成功率 | > 98% |
| 读写一致性 | 100% |
| 存储格式正确性 | 100% 匹配 schema |

### 6. 双向同步

| 指标 | 目标 |
|------|------|
| 双向一致性 | 100%（Git→Bitable→Git 决策不变） |
| 同步延迟 | < 60s |
| 变更日志完整性 | 100% |
| 增量同步效率 | O(n) 非 O(n²) |

### 7. MCP Server

| 指标 | 目标 |
|------|------|
| 协议合规性 | 100% |
| 工具响应延迟 | < 2s（非 LLM），< 20s（LLM 工具） |
| 并发处理 | 50 并发无死锁/无数据损坏 |
| 错误码正确性 | 100% 匹配 schema |

### 8. 端到端系统

| 指标 | 目标 |
|------|------|
| 决策端到端延迟 | < 2 min |
| 流水线完整性 | 高置信信号→存储 > 95% |
| 数据持久性 | 重启后 100% 可加载 |
| 优雅降级 | 单适配器故障不影响其他适配器 |

---

## 二、七种评测方法

### 2.1 单元测试（无外部依赖）

| 包 | 测试内容 | 用例数 | Tag |
|----|----------|--------|-----|
| `internal/signal/detector.go` | 四个分析器独立测、反信号分类、分数计算、阈值分类、关键词穷举 | 30+ | 无 |
| `internal/signal/state_machine.go` | 所有合法状态转换、Mutation 创建、SDR ID 唯一性 | 15 | 无 |
| `internal/llm/fallback.go` | 所有关键词路径、主题分类、边界情况（空/特长/混合语言） | 12 | 无 |
| `internal/llm/client.go` | ExtractJSON 含 fence/无 fence/畸形 JSON，ParseExtractionResult 错误路径 | 20 | 无 |
| `internal/core/memory_graph.go` | Upsert/get/query/delete/search 并发安全、relation 跟踪、冲突检测 | 25 | race |
| `internal/core/pipeline.go` | 每种 mutation 类型、校验拒绝、畸形 mutation | 15 | 无 |
| `internal/decision/node.go` | Status/ImpactLevel 有效性、Phase 枚举 | 8 | 无 |
| `internal/storage/git/format.go` | Node→Markdown→Node 往返、空字段/Unicode/长文本边界 | 10 | 无 |
| `internal/sync/sync.go` | hasChanges 字段级比对、mergeDecisions 冲突策略、ChangeLog 线程安全 | 12 | 无 |

### 2.2 集成测试（真实飞书 API + 真实 LLM API）

**Tag: `integration`**

- **9 个探测器**：每探测器 4 个用例（单次轮询、多次轮询去重、空轮询基线、上下文覆盖率）
- **LLM Agent**：真实内容提取、分类、跨主题、冲突 + 空输入/超时/熔断器
- **MCP Server**：8 个工具的真实调用 + 并发测试 + 协议错误场景
- **双向同步**：Git→多维表格、多维表格→Git、完整往返、冲突合并

### 2.3 基准测试

**Tag: `bench`**

```
BenchmarkEnhancedDetector_Analyze          -- 10000 次调用，ns/op, allocs/op
BenchmarkWorkerPool_JobProcessing          -- 1000 个作业，吞吐量
BenchmarkMemoryGraph_UpsertAndQuery        -- 10000 节点插入 + 查询
BenchmarkLLMClient_JSONExtraction          -- 1000 响应字符串，解析性能
BenchmarkPipelineEngine_BatchApply         -- 批大小 1/10/50/100
BenchmarkSyncManager_hasChanges            -- 10000 对比较
BenchmarkMCP_RequestParsing                -- 10000 JSON-RPC 反序列化
```

### 2.4 场景测试（业务连贯性）

**Tag: `scenario`**

| # | 场景 | 涉及的探测器 | 预期结果 |
|---|------|-------------|----------|
| S1 | IM 发送"决定使用PostgreSQL" | IM | 提取到决策，主题"数据库架构" |
| S2 | 创建含"确认使用Redis"的会议纪要 | VC | 从 AI 摘要中提取决策 |
| S3 | 文档评论 "/approve" | Docs | 提取到批准信号 |
| S4 | 日历事件 "[决策评审] 架构方案" | Calendar+Docs | 检测到事件并记录 |
| S5 | 任务完成标题含"方案确认" | Task | 若含决策关键词则提取 |
| S6 | 同一决策出现在 IM 和文档中 | IM+Docs | 去重合并 |
| S7 | "今天天气不错" | IM | 不提取（反信号） |
| S8 | "可能用MySQL也可能用PostgreSQL" | IM | 不提取（不确定性） |
| S9 | IM 线程 5 条消息达成共识 | IM | 从线程上下文提取决策 |
| S10 | 存储两个冲突决策 | Storage | MemoryGraph 检测到冲突 |

### 2.5 对抗测试（边界和压力）

**Tag: `adversarial`**

| 测试 | 说明 |
|------|------|
| LLM 幻觉 | 输入模糊文本，验证不捏造事实 |
| XSS 注入 | IM 中 `<script>` 标签，验证安全存储 |
| Unicode 攻击 | 零宽字符、RTL 覆盖、emoji 序列 |
| 超长输入 | 1MB IM 消息，验证优雅截断 |
| 并发写入 | 50 并行 pipeline 写入，验证无数据损坏 |
| 网络故障 | 同步中途停止多维表格 API，验证回滚 |
| 探测器风暴 | 9 个探测器同时触发，验证工作池稳定性 |
| 时钟偏移 | 修改系统时钟，验证 SDR ID 生成 |
| 空多维表格 | 多维表格返回空，验证同步优雅处理 |
| 令牌过期 | 过期 ARK_API_KEY，验证熔断器和降级 |

### 2.6 长期运行测试

**Tag: `longhaul`**

- **24h 稳定性**：每 10 分钟轮询 9 个适配器，监控 goroutine 数、内存、错误率
- **1000 决策负载**：通过 MCP 工具自动创建 1000 个决策，测量时间、磁盘、多维表格行数
- **同步耐力**：每 5 分钟执行一次 Git↔多维表格同步，运行 2 小时，每次验证一致性

### 2.7 LLM 质量评估（离线标注）

建立人工标注的 Ground Truth 数据集：

| 类别 | 数量 | 示例 |
|------|------|------|
| 显式决策（中文） | 40 | "我们决定使用PostgreSQL" |
| 显式决策（英文） | 20 | "LGTM, approved" |
| 隐式决策 | 30 | "那就用方案A吧" |
| 非决策（状态更新） | 30 | "进度：已完成登录模块" |
| 非决策（问候） | 20 | "早上好" |
| 非决策（问题） | 20 | "这个方案怎么样？" |
| 非决策（不确定） | 20 | "可能用MySQL也行" |
| 非决策（信息分享） | 20 | "FYI，供参考" |
| 会议纪要含决策 | 20 | 含明确结论的纪要 |
| 文档评论含批准 | 20 | "/approve" |
| 混合/模糊 | 30 | 同时含决策和非决策信号 |
| 边界情况 | 30 | 极短、极长、纯 emoji、代码段 |

---

## 三、测试组织与执行

### 构建标签体系

| Tag | 包含的测试 | 依赖 | 运行频率 |
|-----|-----------|------|---------|
| 无 | 单元测试 | 无 | 每次提交 |
| `race` | 并发安全测试 | 无 | 每次提交 |
| `integration` | 真实 API 测试 | `ARK_API_KEY`, `LARK_*` | 每次 PR + 每日 |
| `bench` | 性能基准 | 无 | 每次发布 |
| `scenario` | 端到端流程 | `ARK_API_KEY`, `LARK_*` | 每日 |
| `adversarial` | 边界和压力 | `ARK_API_KEY`, `LARK_*` | 每周 |
| `longhaul` | 24h+ 稳定性 | `ARK_API_KEY`, `LARK_*` | 每周 |

### 执行命令

```bash
# 每次提交 - 单元测试
go test ./internal/... -v -count=1 -timeout=60s
go test ./internal/... -race -count=1 -timeout=120s
go vet ./...

# PR + 每日 - 集成测试
go test ./test/detector/... -tags=integration -v -count=1 -timeout=300s
go test ./test/llm_module/... -tags=integration -v -count=1 -timeout=300s
go test ./test/mcp/... -tags=integration -v -count=1 -timeout=300s

# 每周 - 对抗 + 长期
go test ./test/... -tags=adversarial -v -count=1 -timeout=300s
go test ./test/... -tags=longhaul -v -count=1 -timeout=86400s
```

---

## 四、质量门禁

| 门禁 | 条件 | 动作 |
|------|------|------|
| 单元测试通过 | 100% 通过 + 无竞态 | 允许合并 |
| 代码覆盖率 | `internal/` > 60% | 低于则警告 |
| 集成测试 | main 分支 100% 通过 | 阻断灰度部署 |
| LLM 精度下降 | 比基线 > 5% | 创建告警 issue |
| 性能回归 | 比基线 > 10% | 阻断发布 |
| 内存泄漏 | goroutine 每小时增长 > 10% | 停止 longhaul，通知 |

---

## 五、分阶段落地计划

| 阶段 | 任务 | 预估工作量 |
|------|------|-----------|
| **Phase 1: 单元测试地基** | 给 signal/detector、state_machine、llm/fallback、client、core/memory_graph、pipeline、decision/node、git/format、sync/sync 加白盒单元测试 | 5-8 天 |
| **Phase 2: 真值数据集** | 创建 testdata/ 目录：决策语料 500 条、分类 100 条、冲突 50 对、跨主题 50 条 | 3-5 天 |
| **Phase 3: 集成测试增强** | 整合现有探测器测试为参数化套件、增加上下文覆盖验证、标准化环境变量处理 | 2-3 天 |
| **Phase 4: 场景测试** | 实现 10 个真实端到端场景（发送→检测→断言→清理） | 3-4 天 |
| **Phase 5: 基准测试** | 添加 testing.B 基准，建立基线追踪 | 2 天 |
| **Phase 6: CI/CD 流水线** | GitHub Actions 工作流、凭据管理、覆盖率报告 | 1-2 天 |
| **Phase 7: 对抗测试** | 构造边界输入、注入错误模拟、模糊测试 | 2-3 天 |
| **Phase 8: 长期运行测试** | 24h 稳定性测试框架、goroutine/内存监控 | 2-3 天 |

---

## 六、架构改进建议（提高可测性）

1. **提取接口层**：为 `LarkCLI` 和 `LLM Client` 定义接口，使 9 个探测器和 LLM 调用可被 Mock，这是提升可测性最高杠杆的改进
2. **评分阈值可配置**：将 `computeFinalScore()` 中的硬编码权重（0.45/0.15/0.05/0.35）和反信号惩罚系数（0.5）改为可配置，支持网格搜索调优
3. **集成测试断言增强**：现有 `t.Logf`→`assert`/`require`，使测试真正能失败
4. **标准化构建标签**：统一使用 `integration`/`scenario`/`adversarial`/`longhaul`/`bench` 标签体系
5. **修复 Makefile**：`make test` 目标引用了不存在的 `./test/p1`-`./test/p4` 目录

---

## 七、关键文件清单

- `internal/signal/detector.go` — EnhancedDetector 多因子评分，最重要算法逻辑
- `internal/signal/engine.go` — SignalActivationEngine，集成测试核心
- `internal/llm/client.go` — LLM 客户端 + JSON 解析，需要健壮的单元测试
- `internal/lark-adapter/types.go` — Detector 接口 + Change 类型，所有适配器的契约
- `internal/core/memory_graph.go` — 内存图，并发安全和一致性测试重点
- `internal/sync/sync.go` — 双向同步，最难测的组件（双外部依赖）
- `internal/storage/git/format.go` — 决策序列化格式，往返测试重点
- `test/detector/` — 现有探测器集成测试，需要增强断言
- `test/llm_module/` — LLM 模块测试，有良好基础可扩展
- `test/mcp/` — MCP 协议测试，需要补充真实请求验证
