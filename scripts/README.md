# Slack 数据集测试脚本

## 快速开始

### 完整自动化测试

运行完整的测试流程：

```bash
cd scripts
./test-lark-im.sh
```

这个脚本会：
1. 解析 100 条测试数据
2. 先发送一条消息确认 lark-cli 正常工作
3. 重启 mem-service 和 detector-lark-im
4. 发送 100 条消息到测试群
5. 显示日志输出

### 分步测试

#### 1. 解析测试数据

```bash
cd scripts
python3 parse-slack-data.py ../chat-data/Software-related-Slack-Chats-with-Disentangled-Conversations/data/pythondev/2018/merged-pythondev-help.xml 100 ../outputs/test-messages.json
```

#### 2. 测试单条消息发送

```bash
python3 send-test-messages.py ../outputs/test-messages.json oc_096c0cd1dfe93cb2f1264e59490946d2 0 1
```

#### 3. 发送多条消息

```bash
python3 send-test-messages.py ../outputs/test-messages.json oc_f382174f2ab17aa2ebefba72834df0b2 0 100
```

## 配置说明

| 环境变量 | 说明 |
|---------|------|
| `LARK_CHAT_IDS` | 决策卡片推送群 (oc_096c0cd1dfe93cb2f1264e59490946d2) |
| `LARK_DETECT_CHAT_IDS` | 消息检测群 (oc_f382174f2ab17aa2ebefba72834df0b2) |

## 测试流程

1. 运行 `./test-lark-im.sh` 启动完整测试
2. 观察日志输出，确认检测器正常工作
3. 检查是否有决策被提取

## 相关文件

- `parse-slack-data.py` - 解析 XML 数据集
- `send-test-messages.py` - 发送测试消息
- `test-lark-im.sh` - 完整测试流程
