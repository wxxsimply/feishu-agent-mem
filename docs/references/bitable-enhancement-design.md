# 飞书多维表格显示增强设计

## 需求概述

在不改变现有字段结构的前提下，通过新增**公式字段**来增强显示效果：

1. **proposer/executor** - 添加可点击的联系人标签
2. **git_commit_hash** - 用等宽格式显示（反引号包裹）
3. **topic/status/impact_level** - 用状态标签/文本增强区分
4. **created_at** - 格式化时间显示

## 方案设计

### 方式：新增公式字段

根据飞书多维表格的公式能力，我们将创建以下新的公式字段来增强显示：

| 新字段名 | 说明 | 依赖原字段 |
|---------|------|-----------|
| `显示_提交人` | 带链接的提交人标签 | `proposer` |
| `显示_执行人` | 带链接的执行人标签 | `executor` |
| `显示_Git哈希` | 等宽显示的 commit hash | `git_commit_hash` |
| `显示_状态` | 状态标签化显示 | `status` |
| `显示_影响等级` | 影响等级标签化 | `impact_level` |
| `显示_创建时间` | 美化的时间格式 | `created_at` |
| `显示_摘要卡片` | 综合所有信息的摘要 | 多个字段 |

## 公式字段实现

### 1. 显示_提交人

```formula
HYPERLINK("https://www.feishu.cn/people?key=" & ENCODEURL([proposer]), "👤 " & [proposer])
```

### 2. 显示_执行人

```formula
HYPERLINK("https://www.feishu.cn/people?key=" & ENCODEURL([executor]), "👤 " & [executor])
```

### 3. 显示_Git哈希

```formula
"`" & [git_commit_hash] & "`"
```

### 4. 显示_状态

```formula
IFS(
  [status] = "open", "🟢 开放",
  [status] = "in_progress", "🟡 进行中",
  [status] = "closed", "🔴 已关闭",
  [status] = "resolved", "✅ 已解决",
  TRUE, [status]
)
```

### 5. 显示_影响等级

```formula
IFS(
  [impact_level] = "critical", "🔴 Critical",
  [impact_level] = "high", "🟠 High",
  [impact_level] = "medium", "🟡 Medium",
  [impact_level] = "low", "🟢 Low",
  TRUE, [impact_level]
)
```

### 6. 显示_创建时间

```formula
TEXT([created_at], "YYYY-MM-DD HH:mm")
```

### 7. 显示_摘要卡片（可选）

```formula
"📌 " & [title] & "
" & "显示_状态" & " | " & "显示_影响等级" & "
" & "👤 提交人: " & [proposer] & " | 👤 执行人: " & IF(ISBLANK([executor]), "未分配", [executor])
```

## 实现步骤

### 步骤 1：获取当前表结构

使用 `+field-list` 获取当前字段列表，确认字段名准确。

### 步骤 2：创建公式字段

通过 `+field-create` 逐一创建上述公式字段。

### 步骤 3：配置视图

创建或更新视图，隐藏原始字段，显示增强后的字段。

## 注意事项

1. **不修改现有字段** - 所有增强都通过新增公式字段实现
2. **字段名匹配** - 公式中的字段名必须与实际字段名完全一致
3. **权限** - 需要有表编辑权限才能创建字段
4. **用户身份** - 建议使用 `--as user` 身份操作

## 附：lark-cli 命令示例

### 获取表结构

```bash
lark-cli base +table-get \
  --base-token <base_token> \
  --table-id <table_id> \
  --as user
```

### 获取字段列表

```bash
lark-cli base +field-list \
  --base-token <base_token> \
  --table-id <table_id> \
  --as user
```

### 创建公式字段

```bash
lark-cli base +field-create \
  --base-token <base_token> \
  --table-id <table_id> \
  --json '{
    "type": "formula",
    "name": "显示_提交人",
    "expression": "HYPERLINK(\"https://www.feishu.cn/people?key=\" & ENCODEURL([proposer]), \"👤 \" & [proposer])"
  }' \
  --as user \
  --i-have-read-guide
```

## 备选方案：富文本字段

如果公式字段的能力不足，可以考虑：

1. 创建一个**富文本字段**（`richtext` 类型）
2. 在写入数据时，通过代码生成 Markdown 格式的富文本内容
3. 这种方式需要修改 `bitable.go` 的写入逻辑

这个方案的好处是可以支持更丰富的格式，但需要修改现有代码。
