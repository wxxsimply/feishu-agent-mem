# Lark-IM 检测器测试报告（v2）

## 测试时间
2026-05-06 23:50 ~ 2026-05-07 00:10

## 测试数据

### 数据集 1：Slack 技术问答
- 来源：Software-related Slack Chats (pythondev-help)
- 消息数：100 条
- 不同用户：21 人
- 内容：Python 技术问答（append/extend、virtualenv、BeautifulSoup 等）
- 不含决策信号，用于验证检测器基础功能

### 数据集 2：Yatsenos OS 项目模拟聊天
- 来源：`docs/mock/yatsenos/0x02/tasks.md`, `0x03/tasks.md`
- 消息数：100 条
- 不同用户：20 人
- 内容类型：
  - ✅ 决策信息：APIC 方案选型、bitflags 方案、500Hz 时钟、缓冲区 256、FIFO 调度器、AtomicU64
  - ⚡ 冲突决策：XAPIC vs x2APIC、100Hz vs 1000Hz、PID 起点、页表切换顺序、ahash 争议
  - 🔁 重复决策：时钟中断栈 ×3、IRQ 编号 ×2、ProcessManager init ×2、页表克隆 ×2
  - 🗣️ 噪声决策：食堂、羽毛球、奶茶、QEMU 起不来、IDE 崩了、真机测试

## 测试步骤

1. **清理状态**：重置 `outputs/detect_state.json`
2. **编译**：`go build -o bin/mem-service-im ./cmd/mem-service/main.go`
3. **启动**：`nohup bin/mem-service-im > logs/app.log 2>&1 &`
4. **发送消息**：通过 `cmd/send-messages` 调用 21 个 webhook 机器人发消息到 `oc_f382174f2ab17aa2ebefba72834df0b2`
5. **观察日志**：检测 10 秒等待检测器处理

## 测试结果

### 消息处理

| 指标 | 结果 |
|------|------|
| 消息发送 | 100/100 成功 |
| 检测器轮询 | 正常，每 5s 轮询 |
| 噪声过滤 | 正常过滤纯链接/短消息 |
| 消息聚合 | 按时间窗口聚合为讨论组 |
| WorkerPool | 正常处理所有任务 |

### 决策提取（核心测试）

| 指标 | 数值 |
|------|------|
| LLM 决策提取次数 | 28 次 |
| 不同决策标题 | 14 个 |
| 实际不重复决策 | ~5 个 |
| 高置信度决策 (score≥0.70) | 检测到 XAPIC、bitflags、页表克隆等 |
| 低置信度决策 (score<0.50) | 检测到噪声/闲聊类 |

### LLM 提取的决策示例

```
✅ 确定APIC实现方案：先实现XAPIC基础功能，IOAPIC留接口后续迭代
✅ APIC寄存器定义采用bitflags和bit_field库
✅ 寄存器定义采用bitflags方案，register offset统一使用enum
✅ 确定页表克隆方案：仅clone根节点而非完整树
```

## 修复的 Bug

| Bug | 修复位置 | 说明 |
|-----|---------|------|
| `last_detected` 被设为未来时间 | `lark_im.go` | `parseMessageTime` 改用 `time.ParseInLocation` |
| `lastCheck` 未来时间导致空返回 | `mem-service/main.go` | 增加 future time 检查 |
| create_time 精确到分钟被跳过 | `lark_im.go` | cutoff 增加 60 秒缓冲 |
| 重复推送决策卡片 | `push/push.go` | 添加 content hash 去重（5min） |
| 轮询周期重复提取决策 | `signal/engine.go` | `findSimilarDecision` 增加 IM 消息内容重叠匹配 |

## 决策去重流程

```
轮询检测 → 消息聚合 → EnhancedDetector(多因子分析)
  → findSimilarDecision(标题/内容重叠匹配)
    → evaluateDedupAction(LLM 判断 skip/update/conflict)
      → Pipeline.ApplyMutation → PushEngine.PushDecisionCard
        → canPush(SDRID:24h) → canPushContent(hash:5min)
          → sendToFeishu(卡片消息)
```

## 已知问题

1. 同一条消息在连续轮询中会被重复检测（burst模式每5s一次）
2. LLM 提取同一决策时措辞略有差异（如"先做XAPIC" vs "先实现XAPIC"）
3. 内容 hash 去重不够精确，后续可考虑 LLM 语义去重

## 结论

lark-im 检测器完整流程跑通：**轮询检测 → 噪声过滤 → 消息聚合 → 多因子决策信号检测 → LLM 决策提取 → 去重 → 卡片推送**。模拟项目交流消息中成功提取了 APIC 方案选型、bitflags 方案、页表克隆等真实决策。
