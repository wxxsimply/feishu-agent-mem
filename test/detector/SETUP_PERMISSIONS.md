# Lark CLI 权限设置指南

## 当前权限问题

测试显示缺少以下权限：
- `search:docs:read` - 搜索文档权限

## 增加权限的方法

### 方法1：重新登录并添加所需权限

```bash
# 重新登录并添加权限
lark-cli auth login --scope "search:docs:read im:chat:read im:message:read vc:meeting:read calendar:event:read task:task:read wiki:wiki:read"
```

### 方法2：添加多个常用权限

```bash
# IM 相关
lark-cli auth login --scope "im:chat:read im:message:read im:message:write"

# VC 相关
lark-cli auth login --scope "vc:meeting:read minutes:minute:read"

# Docs 相关
lark-cli auth login --scope "search:docs:read doc:document:read"

# Calendar 相关
lark-cli auth login --scope "calendar:event:read calendar:calendar:read"

# Task 相关
lark-cli auth login --scope "task:task:read task:task:write"

# Wiki 相关
lark-cli auth login --scope "wiki:wiki:read wiki:space:read"
```

### 方法3：一次性添加所有常用权限（推荐）

```bash
lark-cli auth login --scope "im:chat:read im:message:read im:message:write vc:meeting:read minutes:minute:read search:docs:read doc:document:read calendar:event:read calendar:calendar:read task:task:read task:task:write wiki:wiki:read wiki:space:read"
```

## 查看当前权限

```bash
# 查看当前身份和权限
lark-cli auth whoami
```

## 权限登录流程

1. 运行上述命令后，会显示一个验证 URL
2. 在浏览器中打开该 URL
3. 使用你的飞书账号登录并授权
4. 等待命令完成即可

## 权限说明

| 权限 | 用途 |
|------|------|
| `im:chat:read` | 读取聊天信息 |
| `im:message:read` | 读取消息内容 |
| `vc:meeting:read` | 读取会议信息 |
| `minutes:minute:read` | 读取妙记内容 |
| `search:docs:read` | 搜索文档 |
| `doc:document:read` | 读取文档内容 |
| `calendar:event:read` | 读取日程信息 |
| `task:task:read` | 读取任务信息 |
| `wiki:wiki:read` | 读取知识库内容 |

## 验证权限是否设置成功

```bash
# 尝试运行测试
cd /Users/halllo/openclaw-workspace/feishu-agent-mem
./test/detector/run_concurrent_test.sh vc
```

或者直接测试某个命令：
```bash
lark-cli docs +search
lark-cli im chats
lark-cli vc +search
lark-cli calendar +agenda
lark-cli task +get-my-tasks
lark-cli wiki spaces list
```
