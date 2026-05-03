# 执行指南（多应用版）

## 0. 方案说明

本方案使用**8个独立的飞书应用**来模拟8个不同的人员，每个应用对应一个角色，不需要多个飞书用户账号。

### 两种方案对比

| 方案 | 飞书应用数量 | 飞书用户账号 | 配置方式 |
|-----|------------|------------|---------|
| **方案A: 1个应用 + 8个用户** | 1个 | 8个 | 同一应用用不同用户登录 |
| **方案B: 8个应用（本方案）** | 8个 | 1个或多个 | 每个角色用独立的应用 |

### 为什么选择多应用方案？

- ✅ **不需要多个飞书账号** - 只需要你自己的账号就可以
- ✅ **应用身份天然隔离** - 每个应用是独立的身份
- ❌ **需要在飞书后台创建8个应用** - 稍繁琐但可以接受

## 1. 概述

本指南说明如何通过**8个独立飞书应用**切换 lark-cli 配置目录的方式，使用不同应用身份发送模拟数据，每个应用对应一个角色，有独立的身份。

## 2. 配置方案

### 2.1 配置目录设计

为每个模拟角色创建独立的飞书应用和对应的配置目录：

```
~/.lark-cli-p1/  -> 陈卓远 [项目经理] (飞书应用 1)
~/.lark-cli-p2/  -> 林晓薇 [内核开发] (飞书应用 2)
~/.lark-cli-p3/  -> 赵一帆 [进程调度] (飞书应用 3)
~/.lark-cli-p4/  -> 王思齐 [用户态开发] (飞书应用 4)
~/.lark-cli-p5/  -> 李沐阳 [存储专家] (飞书应用 5)
~/.lark-cli-p6/  -> 周子涵 [测试专家] (飞书应用 6)
~/.lark-cli-p7/  -> 张若琳 [文档协调] (飞书应用 7)
~/.lark-cli-p8/  -> 刘劲松 [内存专家] (飞书应用 8)
```

### 2.2 角色与配置目录/应用映射表

| 编号 | 姓名 | 角色 | 配置目录 | 飞书应用 |
|-----|------|------|---------|---------|
| P1 | 陈卓远 | 项目经理/架构师 | `~/.lark-cli-p1/` | 应用 1 (YatSenOS-P1) |
| P2 | 林晓薇 | 内核开发 | `~/.lark-cli-p2/` | 应用 2 (YatSenOS-P2) |
| P3 | 赵一帆 | 进程与调度 | `~/.lark-cli-p3/` | 应用 3 (YatSenOS-P3) |
| P4 | 王思齐 | 用户态开发 | `~/.lark-cli-p4/` | 应用 4 (YatSenOS-P4) |
| P5 | 李沐阳 | 存储与文件系统 | `~/.lark-cli-p5/` | 应用 5 (YatSenOS-P5) |
| P6 | 周子涵 | 测试与质量 | `~/.lark-cli-p6/` | 应用 6 (YatSenOS-P6) |
| P7 | 张若琳 | 文档与协调 | `~/.lark-cli-p7/` | 应用 7 (YatSenOS-P7) |
| P8 | 刘劲松 | 内存管理 | `~/.lark-cli-p8/` | 应用 8 (YatSenOS-P8) |

## 3. 准备阶段

### 3.1 创建8个飞书应用

在飞书开发者后台创建8个应用：

1. 访问 https://open.feishu.cn/
2. 创建应用：
   - 应用1名称: YatSenOS-P1-陈卓远
   - 应用2名称: YatSenOS-P2-林晓薇
   - ...
   - 应用8名称: YatSenOS-P8-刘劲松
3. 为每个应用配置权限（相同的权限集）：
   - im:message
   - im:chat
   - contact:user.base:readonly
   - task:task
   - calendar:calendar
   - calendar:calendar.event
   - docs:document
   - wiki:wiki
   - base:app
   - sheets:spreadsheet
   - vc:meeting
   - minutes:minutes
4. 将所有8个应用都加入到测试群聊中

### 3.2 环境检查

1. 确认 lark-cli 已安装
2. 确认 .env 文件中的群聊 ID 配置正确
3. 确认8个应用都已创建并加入群聊

### 3.3 初始化配置目录流程

**步骤1: 备份现有配置**

```bash
cp -r ~/.lark-cli ~/.lark-cli.backup
```

**步骤2: 为应用1（陈卓远）配置并保存**

```bash
# 清除配置，配置应用1
rm -rf ~/.lark-cli/*
lark-cli config init  # 输入应用1的 app_id/app_secret
# [完成配置...]
cp -r ~/.lark-cli ~/.lark-cli-p1
echo "P1 (陈卓远) 配置已保存"
```

**步骤3: 为应用2（林晓薇）配置并保存**

```bash
# 清除配置，配置应用2
rm -rf ~/.lark-cli/*
lark-cli config init  # 输入应用2的 app_id/app_secret
# [完成配置...]
cp -r ~/.lark-cli ~/.lark-cli-p2
echo "P2 (林晓薇) 配置已保存"
```

**步骤4: 重复以上步骤，为 P3-P8 配置**

```bash
# 应用3-8 依次配置
rm -rf ~/.lark-cli/*
lark-cli config init  # 输入对应应用的 app_id/app_secret
cp -r ~/.lark-cli ~/.lark-cli-p3  # 赵一帆
# ... 同理 p4-p8
```

### 3.4 验证所有配置

```bash
# 测试 P1 身份
lark-p1() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p1 lark-cli "$@"; }
lark-p1 auth status

# 测试 P2 身份
lark-p2() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p2 lark-cli "$@"; }
lark-p2 auth status
```

## 4. 执行阶段

### 4.1 便捷函数设置

在执行前，先设置便捷的 shell 函数：

```bash
lark-p1() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p1 lark-cli "$@"; }
lark-p2() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p2 lark-cli "$@"; }
lark-p3() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p3 lark-cli "$@"; }
lark-p4() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p4 lark-cli "$@"; }
lark-p5() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p5 lark-cli "$@"; }
lark-p6() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p6 lark-cli "$@"; }
lark-p7() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p7 lark-cli "$@"; }
lark-p8() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p8 lark-cli "$@"; }
```

### 4.2 执行顺序

按照以下顺序执行：

```
Phase 0 (W1-W2)
├── Day 1: 创建知识库 + 发送启动消息 + 创建任务清单
├── Day 2: 创建日程 + 发送环境搭建讨论
├── Day 3-5: 发送 Rust 培训讨论 + 创建文档
├── Day 6-7: 发送 UEFI 验证讨论 + 更新任务状态
└── Day 8: 阶段总结 + 更新知识库

Phase 1 (W2-W4)
├── Day 1: 创建设计文档 + 发送启动讨论
├── Day 2-5: 发送技术讨论 + 创建任务
├── Day 6-10: 发送代码审查讨论 + 更新进度
├── Day 11-14: 发送收尾讨论 + 更新知识库
└── Day 15: 阶段总结

... (Phase 2-8 类似)
```

### 4.3 每日执行流程

```
上午 (9:00-12:00)
├── 发送当日启动消息
├── 处理昨日遗留问题讨论
└── 发送技术讨论消息

下午 (13:00-18:00)
├── 发送进度同步消息
├── 发送代码审查消息
├── 创建/更新任务
└── 创建/更新文档

晚上 (19:00-23:00)
├── 发送闲聊消息
├── 发送问题讨论消息
└── 更新进度表
```

### 4.4 周五执行流程

```
周五 (16:00-18:00)
├── 发送周会通知
├── 创建视频会议记录
├── 发送周会纪要
├── 更新进度表
└── 更新知识库
```

### 4.5 阶段末执行流程

```
阶段末
├── 发送阶段总结消息
├── 创建阶段汇报会议
├── 更新所有任务状态
├── 更新进度仪表盘
├── 更新知识库结构
└── 归档会议纪要
```

## 5. 消息发送详细指南

### 5.1 使用对应应用身份发送消息

```bash
# P1 (陈卓远) 发消息
lark-p1 im +messages-send --chat-id $CHAT_ID --text "我觉得用 ELF 加载的方式比较稳妥"

# P2 (林晓薇) 发消息
lark-p2 im +messages-send --chat-id $CHAT_ID --text "这里需要特别注意 Cr0 寄存器的写保护位"

# P3 (赵一帆) 发消息
lark-p3 im +messages-send --chat-id $CHAT_ID --text "我不同意用 FIFO，太简单了"

# P4 (王思齐) 发消息
lark-p4 im +messages-send --chat-id $CHAT_ID --text "行，搞定了"

# P5 (李沐阳) 发消息
lark-p5 im +messages-send --chat-id $CHAT_ID --text "PR 提了"

# P6 (周子涵) 发消息
lark-p6 im +messages-send --chat-id $CHAT_ID --text "这里有个 bug"

# P7 (张若琳) 发消息
lark-p7 im +messages-send --chat-id $CHAT_ID --text "大家注意一下，本周五是阶段 deadline ~"

# P8 (刘劲松) 发消息
lark-p8 im +messages-send --chat-id $CHAT_ID --text "根据论文，这个可以用 Slab 分配器"

# 多行消息使用 $'...' 格式
lark-p2 im +messages-send --chat-id $CHAT_ID --text $'我分析了一下 ELF 加载的流程，有三个问题需要讨论：\n 1. load_segment 的权限位设置...\n 2. 栈空间的分配方式...\n 3. 页表映射的粒度选择...'
```

### 5.2 消息内容参考

参考 `04-chat-messages.md` 中的消息模板，根据当前阶段和议题选择合适的消息。

### 5.3 消息时间控制

- 使用 `sleep` 命令控制消息间隔（建议 30-60 秒）
- 模拟真实的消息间隔，不要一次性发送大量消息
- 注意消息时间要符合学生的作息
- 可以使用环境变量或脚本变量来控制当前"模拟日期"

## 6. 任务创建详细指南

### 6.1 创建任务清单（可以用任意角色）

```bash
# 用 P1 身份创建任务清单
lark-p1 task tasklists create --summary "YatSenOS - Phase 0: 基础设施搭建"
```

### 6.2 创建任务

```bash
lark-p1 task +create --summary "Rust 环境配置" --description "配置 QEMU + Rust nightly toolchain，能编译运行 UEFI 程序" --due "2026-03-08T18:00:00+08:00"
```

### 6.3 创建子任务

```bash
lark-p1 task +create --summary "安装 QEMU" --parent $PARENT_TASK_GUID
lark-p1 task +create --summary "配置 Rust toolchain" --parent $PARENT_TASK_GUID
```

### 6.4 更新任务状态

```bash
lark-p1 task +update --guid $TASK_GUID --completed true
```

## 7. 日程创建详细指南

### 7.1 创建日程

```bash
lark-p1 calendar +create --summary "YatSenOS Phase 0 启动会" --start "2026-03-03T14:00:00+08:00" --end "2026-03-03T15:00:00+08:00"
```

### 7.2 添加参会人

```bash
# 注意：应用身份可能无法正常邀请用户，这个步骤可选
lark-p1 calendar events patch --event-id $EVENT_ID --add-attendees "$OPEN_ID2,$OPEN_ID3..."
```

## 8. 文档创建详细指南

### 8.1 创建文档

```bash
# 用 P7 (文档协调) 身份创建
lark-p7 docs +create --api-version v2 --title "YatSenOS 开发环境指南" --content '<h1>开发环境指南</h1><p>本文档说明如何配置 YatSenOS 的开发环境</p><h2>1. 环境要求</h2><p>- QEMU 8.0+</p><p>- Rust nightly</p>'
```

### 8.2 更新文档

```bash
lark-p7 docs +update --api-version v2 --doc $DOC_TOKEN --command append --content '<h2>2. 配置步骤</h2><p>...新增内容...</p>'
```

## 9. 知识库操作详细指南

### 9.1 创建知识空间

```bash
lark-p7 wiki spaces create --name "YatSenOS 项目知识库"
```

### 9.2 创建节点

```bash
lark-p7 wiki +node-create --space-id $SPACE_ID --parent $PARENT_TOKEN --title "开发环境指南"
```

## 10. 简化：辅助脚本

### 10.1 设置脚本

创建 `scripts/setup-mock-identities.sh`:

```bash
#!/bin/bash
# 设置所有模拟角色的 lark-cli 快捷函数

echo "正在设置飞书模拟角色快捷函数..."

lark-p1() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p1 lark-cli "$@"; }
lark-p2() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p2 lark-cli "$@"; }
lark-p3() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p3 lark-cli "$@"; }
lark-p4() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p4 lark-cli "$@"; }
lark-p5() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p5 lark-cli "$@"; }
lark-p6() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p6 lark-cli "$@"; }
lark-p7() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p7 lark-cli "$@"; }
lark-p8() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p8 lark-cli "$@"; }

export -f lark-p1 lark-p2 lark-p3 lark-p4 lark-p5 lark-p6 lark-p7 lark-p8

echo "✅ 设置完成！现在可以用 lark-p1 到 lark-p8 来切换角色了"
echo ""
echo "测试命令："
echo "  lark-p1 auth status  # 查看 P1 身份"
echo "  lark-p2 auth status  # 查看 P2 身份"
```

### 10.2 初始化8个应用的脚本

创建 `scripts/init-mock-apps.sh`:

```bash
#!/bin/bash
# 初始化8个飞书应用的配置目录

echo "🟢 开始初始化8个飞书应用配置..."
echo ""

# 备份现有配置
echo "备份现有配置..."
cp -r ~/.lark-cli ~/.lark-cli.backup

# 应用1 - P1 陈卓远
echo ""
echo "=========================="
echo "应用1 - P1 陈卓远"
echo "=========================="
read -p "请输入应用1的 app_id: " APP_ID1
read -p "请输入应用1的 app_secret: " APP_SECRET1
rm -rf ~/.lark-cli/*
expect -c "
spawn lark-cli config init
expect \"App ID:\"
send \"$APP_ID1\r\"
expect \"App Secret:\"
send \"$APP_SECRET1\r\"
expect \"Brand (feishu/lark):\"
send \"feishu\r\"
expect \"Language (zh/en):\"
send \"zh\r\"
expect eof
"
cp -r ~/.lark-cli ~/.lark-cli-p1
echo "✅ P1 配置保存到 ~/.lark-cli-p1"

# 应用2 - P2 林晓薇
echo ""
echo "=========================="
echo "应用2 - P2 林晓薇"
echo "=========================="
read -p "请输入应用2的 app_id: " APP_ID2
read -p "请输入应用2的 app_secret: " APP_SECRET2
rm -rf ~/.lark-cli/*
expect -c "
spawn lark-cli config init
expect \"App ID:\"
send \"$APP_ID2\r\"
expect \"App Secret:\"
send \"$APP_SECRET2\r\"
expect \"Brand (feishu/lark):\"
send \"feishu\r\"
expect \"Language (zh/en):\"
send \"zh\r\"
expect eof
"
cp -r ~/.lark-cli ~/.lark-cli-p2
echo "✅ P2 配置保存到 ~/.lark-cli-p2"

# ... 同理 P3-P8，可以后续补充

echo ""
echo "✅ 完成！"
echo ""
echo "请确保所有8个应用都已添加到测试群聊中"
echo "然后运行 source scripts/setup-mock-identities.sh 加载快捷函数"
```

### 10.3 消息发送辅助脚本

创建 `scripts/send-mock-message.sh`:

```bash
#!/bin/bash
# 模拟发送消息的辅助脚本

ROLE="$1"
CHAT_ID="$2"
TEXT="$3"

if [ -z "$ROLE" ] || [ -z "$CHAT_ID" ] || [ -z "$TEXT" ]; then
    echo "用法: $0 <角色> <群聊ID> <消息内容>"
    echo "示例: $0 p1 oc_xxx \"大家好\""
    exit 1
fi

case "$ROLE" in
    p1) LARK_CLI_CONFIG_DIR=~/.lark-cli-p1 lark-cli im +messages-send --chat-id "$CHAT_ID" --text "$TEXT" ;;
    p2) LARK_CLI_CONFIG_DIR=~/.lark-cli-p2 lark-cli im +messages-send --chat-id "$CHAT_ID" --text "$TEXT" ;;
    p3) LARK_CLI_CONFIG_DIR=~/.lark-cli-p3 lark-cli im +messages-send --chat-id "$CHAT_ID" --text "$TEXT" ;;
    p4) LARK_CLI_CONFIG_DIR=~/.lark-cli-p4 lark-cli im +messages-send --chat-id "$CHAT_ID" --text "$TEXT" ;;
    p5) LARK_CLI_CONFIG_DIR=~/.lark-cli-p5 lark-cli im +messages-send --chat-id "$CHAT_ID" --text "$TEXT" ;;
    p6) LARK_CLI_CONFIG_DIR=~/.lark-cli-p6 lark-cli im +messages-send --chat-id "$CHAT_ID" --text "$TEXT" ;;
    p7) LARK_CLI_CONFIG_DIR=~/.lark-cli-p7 lark-cli im +messages-send --chat-id "$CHAT_ID" --text "$TEXT" ;;
    p8) LARK_CLI_CONFIG_DIR=~/.lark-cli-p8 lark-cli im +messages-send --chat-id "$CHAT_ID" --text "$TEXT" ;;
    *) echo "未知角色: $ROLE"; exit 1 ;;
esac
```

## 11. 注意事项

### 11.1 配置目录管理

- 妥善保管所有8个配置目录
- 定期备份配置，避免意外丢失
- 可以使用符号链接或 git 管理配置

### 11.2 飞书应用管理

- 确保所有8个应用都已加入测试群聊
- 确保所有应用都有相同的权限配置
- 建议给应用起容易识别的名字（YatSenOS-P1-陈卓远）

### 11.3 消息真实性

- 避免过于完美的对话
- 包含打字错误、重复发送等细节
- 包含闲聊和废话
- 包含冲突和争论

### 11.4 时间合理性

- 消息时间要符合学生作息
- 避免凌晨 3 点的技术讨论
- 周末消息量减少
- 临近 deadline 消息量增加

### 11.5 渐进式

- 知识和技术讨论要体现学习曲线
- 不要一开始就全知全能
- 问题和 bug 要真实
- 解决方案要合理

### 11.6 冲突适度

- 冲突要真实但不激烈
- 最终都要有解决方案
- 不要有伤害性的言论
- 体现团队协作

### 11.7 废话比例

- 约 15-20% 的消息是非技术性的闲聊
- 闲聊内容要真实（食堂、考试、段子等）
- 闲聊时间要合理（周五下午、阶段性突破后等）

## 12. 执行检查清单

### 12.1 准备阶段检查清单

- [ ] 8个飞书应用已在飞书开发者后台创建
- [ ] 8个应用都已配置相同的权限
- [ ] 8个应用都已添加到测试群聊
- [ ] 8个配置目录已初始化完成 (~/.lark-cli-p1 ~ ~/.lark-cli-p8)
- [ ] 便捷函数已测试
- [ ] .env 文件中群聊 ID 配置正确

### 12.2 Phase 0 检查清单

- [ ] 知识库已创建（用 P7 身份）
- [ ] Phase 0 群聊消息已发送（20-30条）
- [ ] Phase 0 任务已创建（用 P1 身份）
- [ ] Phase 0 日程已创建
- [ ] Phase 0 文档已创建
- [ ] Phase 0 知识库结构已建立

### 12.3 Phase 1-8 检查清单

- [ ] 阶段群聊消息已发送（50-100条）
- [ ] 阶段任务已创建并更新状态
- [ ] 阶段日程已创建
- [ ] 阶段文档已创建
- [ ] 知识库已更新
- [ ] 进度表已更新
- [ ] 阶段总结已完成

### 12.4 最终检查清单

- [ ] 所有阶段群聊消息完整
- [ ] 所有任务状态正确
- [ ] 所有日程安排完整
- [ ] 所有文档内容完整
- [ ] 知识库结构最终版正确
- [ ] 进度表数据完整
- [ ] Bug 跟踪表数据完整
- [ ] 会议纪要和妙记完整
