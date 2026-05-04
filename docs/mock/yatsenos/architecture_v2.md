# Feishu Agentic Memory Engine - 架构 v2.0 方案

## 概述

本文档描述了将 mem-service 与各检测器进行进程分离的架构方案，以及改进的检测时间逻辑。

## 1. 整体架构设计

### 1.1 进程架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        mem-service                              │
│         (主进程 - 决策提取、存储、管理)                           │
├─────────────────────────────────────────────────────────────────┤
│                                                                  │
│  ┌───────────────────────────────────────────────────────────┐ │
│  │         WebSocket Server (双向通信)                       │ │
│  │  ┌────────┐  ┌────────┐  ┌────────┐  ┌────────┐         │ │
│  │  │ IM Det│  │ Wiki Det│  │ Doc Det│  │ Cal Det│  ...   │ │
│  │  └────┬───┘  └────┬───┘  └────┬───┘  └────┬───┘         │ │
│  └───────┼─────────────┼─────────────┼─────────────┼─────────────┘ │
└──────────┼─────────────┼─────────────┼─────────────┼───────────────┘
           │             │             │             │
           │             │             │             │
           ▼             ▼             ▼             ▼
       [独立进程]    [独立进程]    [独立进程]    [独立进程]
      lark-im-det  lark-wiki-det lark-doc-det lark-cal-det
```

### 1.2 进程职责

#### mem-service (主进程)
- 决策提取与处理
- 决策存储 (Git + Bitable)
- 内存索引管理
- 冲突检测与解决
- WebSocket 服务端
- 检测器状态管理
- 心跳管理

#### 检测器进程 (独立进程)
- 各检测器独立运行
- 独立的检测循环
- 与 mem-service 通过 WebSocket 通信
- 发送检测到的变更数据
- 发送心跳信号
- 接收 mem-service 的控制信号

---

## 2. 检测器循环逻辑改进

### 2.1 当前问题
目前检测器在一次检测后就结束进入下一个循环，对持续的变化不够敏感。

### 2.2 新逻辑

```go
// 检测器循环伪代码
func (detector Detector) Run() {
    var lastChangeTime time.Time
    var inBurstMode bool
    var lastCheck time.Time

    for {
        // 执行检测
        detectResult, changes := detector.Detect(lastCheck)
        
        if len(changes) > 0 {
            // 有新变更
            lastChangeTime = time.Now()
            inBurstMode = true
            lastCheck = detectResult.DetectedAt
            
            // 发送变更数据到 mem-service
            sendChanges(changes)
            
            // 快速循环 - 持续检测新变更
            time.Sleep(5 * time.Second)
        } else {
            if inBurstMode {
                // 检查是否已经 1 分钟没有变更
                if time.Since(lastChangeTime) > 1*time.Minute {
                    inBurstMode = false
                }
                time.Sleep(5 * time.Second)
            } else {
                // 正常检测间隔
                time.Sleep(detectorConfig.Interval)
            }
        }
    }
}
```

### 2.3 检测状态机

```
[初始状态]
    │
    ▼
[正常检测] ────┐
    │          │ 有新变更
    │          │
    │          │
    ▼          │
[突发模式] ◀────────┘
    │
    │ 1分钟无变更
    │
    ▼
[恢复正常检测]
```

**突发模式特点**:
- 检测间隔缩短为 5 秒
- 持续关注同一数据源的连续变更
- 快速响应用户操作

---

## 3. 配置文件 openclaw.yaml

### 3.1 配置文件结构

```yaml
# openclaw.yaml

# Mem-Service 配置
service:
  name: "feishu-memory-service"
  version: "2.0.0"
  ws_port: 8765        # WebSocket 服务端口
  heartbeat_interval: 30s  # mem-service 心跳间隔

# 检测器配置
detectors:
  lark_im:
    enabled: true
    interval: 5s        # 正常检测间隔
    burst_interval: 5s   # 突发模式检测间隔
    burst_timeout: 1m   # 突发模式超时时间
    heartbeat_interval: 15s  # 检测器心跳间隔

  lark_doc:
    enabled: true
    interval: 30s
    burst_interval: 5s
    burst_timeout: 1m
    heartbeat_interval: 15s

  lark_wiki:
    enabled: true
    interval: 30s
    burst_interval: 5s
    burst_timeout: 1m
    heartbeat_interval: 15s

  lark_calendar:
    enabled: true
    interval: 1m
    burst_interval: 10s
    burst_timeout: 1m
    heartbeat_interval: 15s

  lark_task:
    enabled: true
    interval: 10s
    burst_interval: 5s
    burst_timeout: 1m
    heartbeat_interval: 15s

  lark_vc:
    enabled: true
    interval: 30s
    burst_interval: 5s
    burst_timeout: 1m
    heartbeat_interval: 15s

# 存储配置
storage:
  git_path: "./data/git"
  bitable_enabled: true
```

---

## 4. WebSocket 通信协议

### 4.1 通信模式

**双向通信**:
- 检测器 → mem-service: 变更数据、心跳
- mem-service → 检测器: 控制命令、心跳响应

### 4.2 消息类型

#### 4.2.1 检测器 → mem-service

##### 类型 1: 心跳消息
```json
{
  "type": "heartbeat",
  "detector": "lark_im",
  "version": "1.0.0",
  "timestamp": 1714800000,
  "status": {
    "state": "running",  // running | paused | error
    "last_check": "2026-05-04T11:00:00Z",
    "in_burst_mode": false
  }
}
```

##### 类型 2: 变更数据消息
```json
{
  "type": "detect_result",
  "detector": "lark_doc",
  "timestamp": 1714800000,
  "result": {
    "source": "lark_doc",
    "has_changes": true,
    "detected_at": "2026-05-04T11:00:00Z",
    "last_check": "2026-05-04T10:30:00Z",
    "changes": [
      {
        "type": "doc_updated",
        "entity_type": "doc",
        "entity_id": "doc_token_xxx",
        "summary": "文档内容更新: xxx",
        "timestamp": 1714800000,
        "content": "..."  // 可选，文档内容
      }
    ]
  }
}
```

##### 类型 3: 检测器注册消息
```json
{
  "type": "register",
  "detector": "lark_wiki",
  "version": "1.0.0",
  "config": {
    "interval": "30s",
    "burst_interval": "5s",
    "burst_timeout": "1m"
  }
}
```

#### 4.2.2 mem-service → 检测器

##### 类型 1: 心跳响应
```json
{
  "type": "heartbeat_ack",
  "server_time": 1714800000,
  "status": "ok"
}
```

##### 类型 2: 控制命令
```json
{
  "type": "control",
  "command": "pause",  // pause | resume | config_update | stop
  "timestamp": 1714800000,
  "payload": {
    "config": {
      "interval": "10s"
    }
  }
}
```

##### 类型 3: 全局状态同步
```json
{
  "type": "state_sync",
  "timestamp": 1714800000,
  "global_state": {
    "detectors": {
      "lark_im": {
        "state": "running",
        "last_heartbeat": 1714800000,
        "last_change": 1714799000
      }
    }
  }
}
```

---

## 5. 全局检测器状态管理

### 5.1 状态字段结构

```go
// 全局状态 (mem-service 维护)
type GlobalDetectorState struct {
    Detectors map[string]DetectorState `json:"detectors"`
    SyncAt time.Time `json:"sync_at"`
}

// 单个检测器状态
type DetectorState struct {
    State string `json:"state"`  // registered | running | paused | error | disconnected
    Version string `json:"version"`
    LastHeartbeat time.Time `json:"last_heartbeat"`
    LastCheck time.Time `json:"last_check"`
    LastChange time.Time `json:"last_change"`
    InBurstMode bool `json:"in_burst_mode"`
    TotalChanges int64 `json:"total_changes"`
    TotalErrors int64 `json:"total_errors"`
    Config DetectorConfig `json:"config"`
}

// 检测器配置
type DetectorConfig struct {
    Enabled bool `json:"enabled"`
    Interval time.Duration `json:"interval"`
    BurstInterval time.Duration `json:"burst_interval"`
    BurstTimeout time.Duration `json:"burst_timeout"`
    HeartbeatInterval time.Duration `json:"heartbeat_interval"`
}
```

### 5.2 状态持久化
- 状态保存到 `outputs/detector_state.json`
- 定期快照（可选）
- 用于重启恢复

---

## 6. 目录结构变化

```
feishu-agent-mem/
├── cmd/
│   ├── mem-service/           # 主服务 (已存在)
│   │   └── main.go
│   ├── detector-lark-im/      # 新增: IM检测器独立进程
│   │   └── main.go
│   ├── detector-lark-doc/     # 新增: 文档检测器独立进程
│   │   └── main.go
│   ├── detector-lark-wiki/    # 新增: Wiki检测器独立进程
│   │   └── main.go
│   ├── detector-lark-calendar/# 新增: 日历检测器独立进程
│   │   └── main.go
│   ├── detector-lark-task/    # 新增: 任务检测器独立进程
│   │   └── main.go
│   └── detector-lark-vc/      # 新增: VC检测器独立进程
│       └── main.go
├── internal/
│   ├── detector/              # 新增: 检测器通用框架
│   │   ├── base.go
│   │   ├── wsclient.go
│   │   └── config.go
│   ├── ws/                   # 新增: WebSocket协议
│   │   ├── server.go
│   │   ├── client.go
│   │   └── protocol.go
│   └── ... (原有代码)
├── config/
│   └── openclaw.yaml         # 新增: 全局配置
└── outputs/
    └── detector_state.json    # 新增: 检测器状态
```

---

## 7. 部署方案

### 7.1 本地开发模式 (单进程模拟)

```bash
# 运行主服务
./bin/mem-service --config openclaw.yaml

# 运行检测器 (另一个终端)
./bin/detector-lark-im --config openclaw.yaml
```

### 7.2 生产模式 (多进程 + 进程管理)

```bash
# 使用 systemd, supervisor, 或者 docker compose 管理
```

**docker-compose.yaml 示例**:
```yaml
version: '3'
services:
  mem-service:
    build: .
    command: mem-service --config /config/openclaw.yaml
    volumes:
      - ./config:/config
      - ./data:/data
      - ./outputs:/outputs
    ports:
      - "8765:8765"

  detector-im:
    build: .
    command: detector-lark-im --config /config/openclaw.yaml
    depends_on:
      - mem-service

  detector-doc:
    build: .
    command: detector-lark-doc --config /config/openclaw.yaml
    depends_on:
      - mem-service
```

---

## 8. 迁移路径

### 8.1 Phase 1: 配置文件与日志 (当前状态)
- 添加 openclaw.yaml
- 改进检测器日志 (已完成)

### 8.2 Phase 2: 检测循环逻辑改进
- 修改检测器循环逻辑
- 添加突发模式
- 使用配置文件

### 8.3 Phase 3: WebSocket 与通信
- 实现 WebSocket server
- 实现 detector 端的通信层
- 实现协议编码/解码

### 8.4 Phase 4: 进程分离
- 拆分独立 detector 程序
- 实现心跳机制
- 实现状态管理

---

## 9. 总结

### 9.1 核心改进
1. **检测器分离**: 每个检测器独立进程，降低耦合
2. **改进的检测逻辑**: 突发模式 + 正常模式，更敏感的响应
3. **配置管理**: 统一的 openclaw.yaml 配置
4. **双向通信**: WebSocket，支持控制和状态同步
5. **状态管理**: 全局检测器状态字段，支持监控和恢复

### 9.2 后续可扩展
- 检测器健康检查与自动重启
- 动态配置更新
- 指标与监控
- 分布式部署 (多实例 mem-service)
