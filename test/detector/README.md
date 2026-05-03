# 检测器测试套件

本目录包含针对飞书各个检测器的测试文件，使用真实的飞书环境进行测试。

## 测试文件列表

| 文件 | 测试内容 |
|------|----------|
| `detector_im_test.go` | IM 消息检测器测试 |
| `detector_vc_test.go` | 视频会议检测器测试 |
| `detector_doc_test.go` | 云文档检测器测试 |
| `detector_calendar_test.go` | 日程检测器测试 |
| `detector_task_test.go` | 任务检测器测试 |
| `detector_wiki_test.go` | 知识库检测器测试 |
| `detector_integration_test.go` | 多检测器集成测试 |
| `detector_live_test.go` | 实时测试（需要 build tag） |
| `detector_concurrent_test.go` | 并发测试（定时检测 + 状态模拟） |
| `test_utils.go` | 测试工具函数 |

## 快速开始

### 前置条件

1. 已配置好飞书环境（`.env` 文件或环境变量）
2. `lark-cli` 已正确配置并可以访问飞书 API（可选，用于增强测试）

## 测试方式

### 方式1: 基础测试脚本

```bash
# 运行所有测试
./test/detector/run_tests.sh

# 运行单个检测器测试
./test/detector/run_tests.sh im
./test/detector/run_tests.sh vc
./test/detector/run_tests.sh doc
./test/detector/run_tests.sh calendar
./test/detector/run_tests.sh task
./test/detector/run_tests.sh wiki

# 仅运行集成测试
./test/detector/run_tests.sh integration
```

### 方式2: 场景测试脚本

```bash
# 仅检测现有数据
./test/detector/run_scenario_test.sh detect-only

# 测试完整链路 (Detector → Emitter)
./test/detector/run_scenario_test.sh full-chain

# 并行测试
./test/detector/run_scenario_test.sh parallel

# 完整测试套件
./test/detector/run_scenario_test.sh full

# 实时测试（需要 lark-cli）
./test/detector/run_scenario_test.sh live

# 查看 lark-cli 命令演示
./test/detector/run_scenario_test.sh demo
```

### 方式3: 并发测试（推荐用于交互测试）⭐

启动两个并行进程：一个定时检测，一个由你手动模拟状态变化！

```bash
# 交互式菜单选择
./test/detector/run_concurrent_test.sh menu

# 直接测试单个检测器
./test/detector/run_concurrent_test.sh im
./test/detector/run_concurrent_test.sh vc
./test/detector/run_concurrent_test.sh docs
./test/detector/run_concurrent_test.sh calendar
./test/detector/run_concurrent_test.sh task
./test/detector/run_concurrent_test.sh wiki

# 测试所有检测器一起运行
./test/detector/run_concurrent_test.sh all
```

**并发测试说明：**

- 🔍 **进程1 (检测器)**: 每 30 秒自动运行一次检测
- 🎮 **进程2 (模拟器)**: 你在飞书中手动操作
- ⏱️ **测试时长**: 2 分钟
- 📊 **实时反馈**: 可以立即看到检测器是否捕捉到你的操作！

### 方式4: 使用 `go test` 直接运行

```bash
# 运行所有检测器测试
go test -v ./test/detector/...

# 运行单个检测器测试
go test -v ./test/detector -run TestIMDetector
go test -v ./test/detector -run TestVCDetector
go test -v ./test/detector -run TestDocDetector
go test -v ./test/detector -run TestCalendarDetector
go test -v ./test/detector -run TestTaskDetector
go test -v ./test/detector -run TestWikiDetector

# 运行集成测试
go test -v ./test/detector -run TestAllDetectorsTogether
go test -v ./test/detector -run TestDetectorsParallel
go test -v ./test/detector -run TestDetectorEmitterChain

# 运行实时测试（需要 lark-cli）
go test -v -tags=live ./test/detector -run TestLive

# 运行并发测试（推荐）
go test -v -tags=concurrent ./test/detector -run TestConcurrentIM
go test -v -tags=concurrent ./test/detector -run TestConcurrentAll
```

## 测试说明

### 单检测器测试

每个检测器测试包含两个子测试：

1. **Baseline Detection（基线检测）**：使用 `ExtractDetect` 函数运行完整检测
2. **Incremental Detection（增量检测）**：直接调用 `Detect` 方法，模拟过去1小时的增量检测

### 集成测试

集成测试包含三个测试：

1. **TestAllDetectorsTogether**：顺序运行所有检测器，确保它们可以独立工作
2. **TestDetectorsParallel**：并行运行所有检测器，验证没有资源竞争或冲突
3. **TestDetectorEmitterChain**：测试检测器 -> Emitter 的完整链路

### 实时测试（Live Tests）

使用 `live` build tag 的测试会尝试：
- 验证 lark-cli 是否可用
- 使用真实的飞书 API 进行测试
- 测试完整的检测链

### 并发测试（Concurrent Tests）⭐

使用 `concurrent` build tag 的测试会启动两个并行进程：

**进程1: 检测器**
- 每 30 秒自动运行一次检测
- 持续 2 分钟
- 实时输出检测结果

**进程2: 自动模拟器**
- 使用 lark-cli 自动检查各个模块状态
- 模拟真实的状态变化场景
- VC 模块为例：自动检查会议列表和妙记

**并发测试函数：**
- `TestConcurrentIM` - IM 检测器并发测试
- `TestConcurrentVC` - VC 检测器并发测试
- `TestConcurrentDocs` - Docs 检测器并发测试
- `TestConcurrentCalendar` - Calendar 检测器并发测试
- `TestConcurrentTask` - Task 检测器并发测试
- `TestConcurrentWiki` - Wiki 检测器并发测试
- `TestConcurrentAll` - 所有检测器同时运行测试

## 使用 lark-cli 模拟状态变化

虽然测试主要是被动检测已有数据，但你可以手动使用 lark-cli 创建状态变化来测试检测器：

### 1. 发送测试消息（测试 IM 检测器）

```bash
# 使用 lark-cli 发送一条包含决策关键词的消息
# 然后运行 IM 检测器测试
```

### 2. 创建测试文档（测试 Docs 检测器）

```bash
# 创建一个标题含决策关键词的文档
# 然后运行 Docs 检测器测试
```

### 3. 创建测试任务（测试 Task 检测器）

```bash
# 创建一个新任务或更新任务状态
# 然后运行 Task 检测器测试
```

### 4. 创建测试日程（测试 Calendar 检测器）

```bash
# 创建一个含"评审"、"决策"等关键词的日程
# 然后运行 Calendar 检测器测试
```

## 测试输出示例

```
=== RUN   TestAllDetectorsTogether
    detector_integration_test.go:16: 开始运行所有检测器集成测试...
    detector_integration_test.go:17: 共 6 个检测器
=== RUN   TestAllDetectorsTogether/IM
    detector_integration_test.go:25: 测试 IM 检测器...
    detector_integration_test.go:38: IM 检测结果: HasChanges=true, Changes=2
    detector_integration_test.go:41:   [1] [new_text] message: 用户A: 我们决定采用方案B
=== RUN   TestAllDetectorsTogether/VC
    detector_integration_test.go:25: 测试 VC 检测器...
...
```

## 测试工作流建议

### 开发阶段快速测试

1. 修改检测器代码
2. 运行单个检测器测试验证
3. 运行集成测试确保没有破坏其他功能

```bash
# 快速迭代测试
./test/detector/run_tests.sh im
./test/detector/run_tests.sh integration
```

### 功能验证测试（推荐使用并发测试！）⭐

1. 启动并发测试
2. 在飞书中手动操作
3. 实时观察检测器是否捕捉到变化

```bash
# 交互式测试 IM 检测器
./test/detector/run_concurrent_test.sh im

# 测试所有检测器
./test/detector/run_concurrent_test.sh all
```

**并发测试流程示例：**

```
终端1: 启动 VC 测试
$ ./test/detector/run_concurrent_test.sh vc

========================================
  飞书检测器并发测试
========================================

VC 检测器并发测试
  - 定时检测: 每 30s
  - 自动模拟: 使用 lark-cli 检查会议状态
  - 测试时长: 2 分钟
========================================
[VC] 🔍 检测器进程已启动
[VC] 🤖 VC 自动模拟器进程已启动

[VC] ━━━━ 第 1 轮检测 ━━━━
[VC] 检测结果: HasChanges=false, Changes=0

[VC] ━━━━ 模拟操作 #1 ━━━━
[VC] 📋 检查会议列表...
[VC] ✓ 获取到 VC 数据: 1523 bytes
[VC] 📋 检查妙记列表...
[VC] ✓ 获取到妙记数据

[VC] ━━━━ 第 2 轮检测 ━━━━
[VC] 检测结果: HasChanges=true, Changes=2
[VC]   ✓ [1] meeting_ended - 周会已结束
[VC]   ✓ [2] meeting_minutes_available - 周会会议纪要生成 ✓
```

### 完整功能验证

1. 手动在飞书中创建一些变化（消息、文档、任务等）
2. 运行完整测试套件

```bash
./test/detector/run_scenario_test.sh full
```

## 注意事项

1. **真实环境测试**：这些测试使用真实的飞书 API，会产生实际的 API 调用
2. **时间范围**：默认检测过去1-24小时内的变更
3. **速率限制**：注意飞书 API 的速率限制，避免频繁运行测试
4. **权限要求**：确保配置的飞书账号有足够权限访问各个功能模块
5. **lark-cli 可选**：lark-cli 用于增强测试，但不是必需的

