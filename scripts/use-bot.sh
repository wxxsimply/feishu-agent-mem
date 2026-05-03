#!/bin/bash
# 切换到指定机器人的配置（用mv方式）

if [ -z "$1" ]; then
    echo "用法: source scripts/use-bot.sh [p1|p2|p3|p4|p5|p6|p7|p8]"
    echo "      ./scripts/use-bot.sh [p1|p2|...|p8]"
    echo ""
    echo "注意: 如果要在当前shell直接使用，需要用source"
    exit 1
fi

# 转换为小写
BOT=$(echo "$1" | tr '[:upper:]' '[:lower:]')

# 检查当前配置
if [ -d "$HOME/.lark-cli" ] && [ ! -L "$HOME/.lark-cli" ]; then
    # 如果当前不是链接，先备份
    if [ ! -d "$HOME/.lark-cli.backup" ]; then
        mv "$HOME/.lark-cli" "$HOME/.lark-cli.backup"
    else
        rm -rf "$HOME/.lark-cli"
    fi
fi

# 目标配置目录
TARGET_DIR="$HOME/.lark-cli-${BOT}"

if [ ! -d "$TARGET_DIR" ]; then
    echo "❌ 配置目录不存在: $TARGET_DIR"
    exit 1
fi

# 创建符号链接（或者直接mv）
# 我们用mv而不是ln，这样更可靠
rm -rf "$HOME/.lark-cli"
cp -r "$TARGET_DIR" "$HOME/.lark-cli"

echo "✅ 已切换到 $BOT"
echo "   配置目录: $TARGET_DIR"
echo ""
lark-cli auth status
