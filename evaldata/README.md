# 真值数据集 (Ground Truth Dataset)

用于自动化评估 Agent 各模块的准确率、召回率和一致性。

## 目录结构

```
testdata/
├── corpus/           # 决策检测语料 (100条)
│   └── decisions.jsonl   # {id, source, text, is_decision, expected_topic, ...}
├── classification/   # 主题分类语料 (40条)
│   └── classification.jsonl  # {id, decision, expected_topic, confidence_lower_bound, ...}
├── conflicts/        # 冲突检测语料 (20对)
│   └── conflicts.jsonl      # {id, decision_a, decision_b, expected_contradiction_range, ...}
└── crosstopic/       # 跨主题检测语料 (15条)
    └── crosstopic.jsonl     # {id, title, decision, candidate_topics, expected_is_cross_topic, ...}
```

## 语料难度分布

| 难度 | corpus | classification | conflicts | crosstopic |
|------|--------|---------------|-----------|------------|
| easy | 47 | 40 | - | - |
| medium | 31 | - | - | - |
| hard | 22 | - | - | - |

## 格式说明

所有数据集使用 JSONL（每行一个 JSON 对象）格式。

### corpus/decisions.jsonl

决策检测语料，用于评测 `EnhancedDetector.Analyze()`：
- `id`: 唯一标识
- `source`: 来源 (im/doc/vc/task/calendar)
- `text`: 消息文本
- `is_decision`: 是否为决策
- `expected_topic`: 预期主题（可选）
- `expected_impact`: 预期影响级别（可选）
- `min_confidence`: 最低置信度阈值
- `difficulty`: 难度 (easy/medium/hard)
- `anti_signal_categories`: 反信号类别列表（可选）

### classification/classification.jsonl

主题分类语料，用于评测 `MemoryAgent.ClassifyTopic()`：
- `id`: 唯一标识
- `decision`: 决策文本
- `expected_topic`: 预期主题
- `confidence_lower_bound`: 最低置信度
- `difficulty`: 难度

### conflicts/conflicts.jsonl

冲突检测语料，用于评测 `ConflictResolver`：
- `id`: 唯一标识
- `decision_a`: 决策 A
- `decision_b`: 决策 B
- `expected_contradiction_range`: 预期矛盾分数范围 [min, max]
- `expected_type`: 冲突类型 (direct/partial/none)

### crosstopic/crosstopic.jsonl

跨主题检测语料，用于评测 `MemoryAgent.DetectCrossTopic()`：
- `id`: 唯一标识
- `title`: 决策标题
- `decision`: 决策内容
- `candidate_topics`: 候选主题列表
- `expected_is_cross_topic`: 是否跨主题
- `expected_affected_topics`: 预期影响的主题列表
