# 通讯录模块 (lark-contact) 运作计划

## 1. 模块概述

lark-contact 用于查找项目成员信息，获取 open_id 等标识符。这是其他模块操作的基础。

## 2. 使用场景

### 2.1 初始化阶段

在 Phase 0 开始时，需要获取所有模拟成员的 open_id，用于后续的任务分配、日程邀请等操作。

### 2.2 日常使用

- 任务分配时获取负责人 open_id
- 日程邀请时获取参会人 open_id
- 知识库成员管理时获取成员 open_id

## 3. 成员信息查询

### 3.1 查询成员 open_id

```bash
lark-cli contact +search-user --query "成员姓名" --as user
```

### 3.2 获取成员详情

```bash
lark-cli api GET /open-apis/contact/v3/users/$OPEN_ID --params '{"user_id_type":"open_id"}' --as user
```

## 4. 模拟成员 open_id 映射

在实际执行时，需要先通过 lark-cli 查询真实用户的 open_id。以下是映射表（执行时填写）：

| 编号 | 姓名(模拟) | open_id (待填写) | 说明 |
|-----|-----------|-----------------|------|
| P1 | 陈卓远 | | 项目经理 |
| P2 | 林晓薇 | | 内核开发 |
| P3 | 赵一帆 | | 进程与调度 |
| P4 | 王思齐 | | 用户态开发 |
| P5 | 李沐阳 | | 存储开发 |
| P6 | 周子涵 | | 测试 |
| P7 | 张若琳 | | 文档协调 |
| P8 | 刘劲松 | | 内存管理 |

## 5. 执行前准备

### 5.1 第一步：查询所有成员 open_id

在开始模拟数据生成之前，需要先查询所有成员的 open_id：

```bash
# 查询每个成员的 open_id
lark-cli contact +search-user --query "陈卓远" --as user
lark-cli contact +search-user --query "林晓薇" --as user
lark-cli contact +search-user --query "赵一帆" --as user
lark-cli contact +search-user --query "王思齐" --as user
lark-cli contact +search-user --query "李沐阳" --as user
lark-cli contact +search-user --query "周子涵" --as user
lark-cli contact +search-user --query "张若琳" --as user
lark-cli contact +search-user --query "刘劲松" --as user
```

### 5.2 第二步：更新映射表

将查询到的 open_id 更新到映射表中，供后续操作使用。

### 5.3 第三步：验证权限

确保 bot 有足够的权限来操作各个模块。

## 6. 注意事项

1. **身份选择**: 大部分操作使用 `--as user` 身份
2. **权限检查**: 确保用户有权限访问各个群聊和模块
3. **open_id 有效期**: open_id 在同一租户内是稳定的
4. **批量操作**: 可以先查询一次，缓存 open_id 后批量使用
