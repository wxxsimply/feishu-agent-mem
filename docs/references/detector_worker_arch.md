# Detector & Worker 架构设计

## 概述

本文档描述了检测器和决策生成器的进程分布设计，使用 chan 进行通信，避免阻塞。

## 架构图

```
┌─────────────┐
│  Detector   │
│ (IM/Vote etc)│
└──────┬──────┘
       │
       │ Detect Changes
       │
       │ ┌──────────────────┐
       │ │ DetectionJob     │
       └►│  - AdapterType   │
         │  - Change        │
         │  - ReceivedAt    │
         └────────┬─────────┘
                  │
                  │  chan
                  │
         ┌────────▼───────────┐
         │    WorkerPool      │
         │  ┌──────────────┐ │
         │  │  Worker-1    │ │
         │  ├──────────────┤ │
         │  │  Worker-2    │ │
         │  ├──────────────┤ │
         │  │  Worker-N    │ │
         │  └──────────────┘ │
         │  ┌──────────────┐ │
         │  │  LLM Calls   │ │
         │  └──────────────┘ │
         └────────┬───────────┘
                  │
                  │ DecisionResult
                  │
         ┌────────▼──────────┐
         │ ResultProcessor   │
         │  - Apply Mutation │
         │  - Git Sync       │
         │  - Bitable Sync   │
         └───────────────────┘
```

## 核心组件

### 1. DetectionJob (通信结构体)

```go
type DetectionJob struct {
    AdapterType AdapterType
    Change      larkadapter.Change
    ReceivedAt  time.Time
}
```

### 2. DecisionResult (结果结构体)

```go
type DecisionResult struct {
    Job       *DetectionJob
    Mutation  *DecisionMutation
    Processed time.Time
    Err       error
}
```

### 3. WorkerPool

- 管理多个工作协程
- 使用 chan 传递任务和结果
- 避免阻塞 Detector
- 统计：活跃 workers、处理数量、LLM 调用次数

### 4. ResultProcessor

- 独立协程处理结果
- 应用 Mutation 到 Pipeline
- 不阻塞 WorkerPool

## 特点

✅ **非阻塞 Detector**
- Detector 只负责检测和提交任务
- 立即返回，不等待决策生成

✅ **并行处理**
- 多个 Worker 同时处理不同消息
- LLM 调用并发执行

✅ **详细日志**
- LLM 调用前/后都有日志
- WorkerPool 统计信息
- Goroutine 数量监控

✅ **自适应 Worker 数量**
- 默认使用 CPU 核心数
- 最少 2 个 Worker

## 日志示例

```
[System] NumGoroutine: 1
[System] NumCPU: 8
[WorkerPool] Starting 8 workers...
[Worker-0] Started
[Worker-1] Started
[Detector] Starting cycle with 1 detectors
[Detector] IM: No changes detected
[SignalEngine] LLM available: true
[SignalEngine] Calling LLM for decision extraction...
========== LLM CALL START ==========
[LLM] Model: doubao-1-5-pro
[LLM] User prompt preview: [群聊] 张三: 决定采用新方案...
[LLM] Sending request to LLM API...
[LLM] LLM call succeeded in 876.543ms
========== LLM CALL END ==========
[ResultProcessor] Applying mutation: DEC-20260502150405-1
```
