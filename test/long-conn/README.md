# 飞书长连接测试

使用 `lark-cli event +subscribe` 通过 WebSocket 长连接监听飞书消息事件。

## 前置准备

1. 确保已安装并配置 `lark-cli`

```bash
lark-cli auth status
```

2. 配置 `.env` 文件中的群聊 ID

```env
LARK_CHAT_IDS=oc_xxxxxxxxxxxxx
```

## 使用方法

### 方式一：直接运行 lark-cli

```bash
lark-cli event +subscribe --event-types im.message.message_receive_v1
```

### 方式二：运行测试程序

```bash
cd test/long-conn
go run main.go
```

## 功能说明

- 通过 WebSocket 长连接实时接收飞书事件
- 过滤显示 `.env` 中配置的群聊消息
- 解析并显示消息详情（发送者、内容、提及等）
- 按 Ctrl+C 退出

## 事件类型

本程序主要关注：
- `im.message.message_receive_v1` - 接收消息事件

## 参考文档

- 飞书事件订阅配置: https://open.feishu.cn/document/server-docs/event-subscription-guide/event-subscription-configure-/request-url-configuration-case
- lark-cli 文档: https://github.com/larksuite/cli
