# 多维表格模块 (lark-base) 运作计划

## 1. 模块概述

lark-base 用于创建项目数据表，跟踪 Bug、任务看板、进度仪表盘等结构化数据。

## 2. 多维表格设计

### 2.1 Bug 跟踪表

**创建时间**: Phase 1 第 1 周

**表名**: YatSenOS Bug 跟踪

**字段设计**:

| 字段名 | 类型 | 说明 |
|-------|------|------|
| Bug ID | 自动编号 | 自动生成 |
| 标题 | 文本 | Bug 简要描述 |
| 描述 | 多行文本 | Bug 详细描述 |
| 所属阶段 | 单选 | Phase 0-8 |
| 所属模块 | 单选 | bootloader/interrupt/process/user/storage/memory/misc |
| 严重程度 | 单选 | Critical/Major/Minor/Trivial |
| 状态 | 单选 | Open/In Progress/Resolved/Closed/Won't Fix |
| 报告人 | 人员 | 发现 Bug 的人 |
| 负责人 | 人员 | 修复 Bug 的人 |
| 发现日期 | 日期 | Bug 发现时间 |
| 解决日期 | 日期 | Bug 解决时间 |
| 复现步骤 | 多行文本 | 如何复现 |
| 解决方案 | 多行文本 | 如何解决 |

### 2.2 任务看板表

**创建时间**: Phase 0 第 1 周

**表名**: YatSenOS 任务看板

**字段设计**:

| 字段名 | 类型 | 说明 |
|-------|------|------|
| 任务 ID | 自动编号 | 自动生成 |
| 任务名称 | 文本 | 任务简要描述 |
| 所属阶段 | 单选 | Phase 0-8 |
| 所属议题 | 文本 | 议题名称 |
| 负责人 | 人员 | 任务负责人 |
| 状态 | 单选 | Backlog/In Progress/Review/Done/Blocked |
| 优先级 | 单选 | P0/P1/P2/P3 |
| 开始日期 | 日期 | 任务开始时间 |
| 截止日期 | 日期 | 任务截止时间 |
| 完成日期 | 日期 | 任务完成时间 |
| 关联任务 | 关联 | 关联到其他任务 |
| 备注 | 多行文本 | 其他说明 |

### 2.3 进度仪表盘

**创建时间**: Phase 0 第 1 周

**表名**: YatSenOS 进度仪表盘

**字段设计**:

| 字段名 | 类型 | 说明 |
|-------|------|------|
| 阶段 | 单选 | Phase 0-8 |
| 计划开始日期 | 日期 | 阶段计划开始时间 |
| 计划结束日期 | 日期 | 阶段计划结束时间 |
| 实际开始日期 | 日期 | 阶段实际开始时间 |
| 实际结束日期 | 日期 | 阶段实际结束时间 |
| 完成度 | 数字(百分比) | 阶段完成百分比 |
| 状态 | 单选 | Not Started/In Progress/Completed/Delayed |
| 负责人 | 人员 | 阶段主要负责人 |
| 风险 | 多行文本 | 当前风险和问题 |

## 3. 多维表格创建时机

### 3.1 Phase 0

- 创建任务看板表
- 创建进度仪表盘表
- 初始化阶段数据

### 3.2 Phase 1

- 创建 Bug 跟踪表
- 开始记录 Bug

### 3.3 每个阶段

- 更新进度仪表盘
- 更新任务看板状态
- 记录新发现的 Bug

### 3.4 Phase 8

- 最终统计和分析
- 生成项目报告

## 4. 使用 lark-cli 操作多维表格

### 4.1 创建多维表格

```bash
lark-cli base +create --name "YatSenOS Bug 跟踪" --as user
```

### 4.2 创建字段

```bash
lark-cli base +field --app-token $APP_TOKEN --table-id $TABLE_ID --field-name "标题" --type text --as user
```

### 4.3 添加记录

```bash
lark-cli base +record --app-token $APP_TOKEN --table-id $TABLE_ID --fields '{"标题":"ELF 加载失败","严重程度":"Critical","状态":"Open"}' --as user
```

### 4.4 更新记录

```bash
lark-cli base +record --app-token $APP_TOKEN --table-id $TABLE_ID --record-id $RECORD_ID --fields '{"状态":"Resolved","解决方案":"修复了页表映射"}' --as user
```

### 4.5 查询记录

```bash
lark-cli base +record --app-token $APP_TOKEN --table-id $TABLE_ID --filter '{"状态":"Open"}' --as user
```

## 5. 多维表格与群聊的联动

### 5.1 Bug 记录触发

当群聊中出现以下消息时，触发 Bug 记录：
- "这里有个bug" -> 创建 Bug 记录
- "测试没过" -> 创建 Bug 记录
- "这个边界条件没处理" -> 创建 Bug 记录

### 5.2 状态更新触发

当群聊中出现以下消息时，更新 Bug 状态：
- "这个bug修了" -> 更新 Bug 状态为 Resolved
- "测试通过了" -> 更新 Bug 状态为 Closed

### 5.3 进度更新

每个阶段结束时，更新进度仪表盘：
```
P7 张若琳: Phase 1 进度已更新到多维表格
P7 张若琳: 完成度：85%，状态：In Progress
P7 张若琳: 有两个 Bug 待修复
```
