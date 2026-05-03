#!/bin/bash
# 为P1-P8八个应用初始化lark-cli配置目录

echo "============================================"
echo "  YATSENOS 初始化机器人配置目录"
echo "============================================"
echo ""

# 加载.env文件
if [ -f ".env" ]; then
    export $(grep -v '^#' .env | xargs)
    echo "✅ 已加载.env文件"
else
    echo "❌ 未找到.env文件"
    exit 1
fi

# 配置目录和应用信息
declare -a BOT_NAMES=(
    "陈卓远"
    "林晓薇"
    "赵一帆"
    "王思齐"
    "李沐阳"
    "周子涵"
    "张若琳"
    "刘劲松"
)

declare -a BOT_APP_IDS=(
    "$P1_APP_ID"
    "$P2_APP_ID"
    "$P3_APP_ID"
    "$P4_APP_ID"
    "$P5_APP_ID"
    "$P6_APP_ID"
    "$P7_APP_ID"
    "$P8_APP_ID"
)

declare -a BOT_APP_SECRETS=(
    "$P1_APP_SECRET"
    "$P2_APP_SECRET"
    "$P3_APP_SECRET"
    "$P4_APP_SECRET"
    "$P5_APP_SECRET"
    "$P6_APP_SECRET"
    "$P7_APP_SECRET"
    "$P8_APP_SECRET"
)

echo ""
echo "📋 即将为8个机器人初始化配置目录："
for i in {0..7}; do
    bot_num=$((i+1))
    echo "   P$bot_num ${BOT_NAMES[$i]}: ${BOT_APP_IDS[$i]}"
done
echo ""

# 备份现有配置（如果有）
if [ -d "$HOME/.lark-cli" ]; then
    backup_dir="$HOME/.lark-cli.backup.$(date +%Y%m%d%H%M%S)"
    echo "💾 备份现有配置到 $backup_dir"
    mv "$HOME/.lark-cli" "$backup_dir"
fi

echo ""
read -p "⚠️  确认继续？(y/n): " confirm
if [ "$confirm" != "y" ]; then
    if [ -n "$backup_dir" ]; then
        echo "🔄 恢复备份..."
        mv "$backup_dir" "$HOME/.lark-cli"
    fi
    echo "❌ 已取消"
    exit 0
fi

# 函数：初始化单个应用
init_bot_config() {
    local bot_num="$1"
    local bot_name="$2"
    local app_id="$3"
    local app_secret="$4"
    local config_dir="$5"

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🤖 正在初始化: P$bot_num $bot_name"
    echo "📍 配置目录: $config_dir"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    # 清空当前配置
    rm -rf "$HOME/.lark-cli"

    # 用 expect 自动配置
    echo "⏳ 正在配置..."
    expect <<EOF
spawn lark-cli config init
expect "App ID:"
send "$app_id\r"
expect "App Secret:"
send "$app_secret\r"
expect "Brand (feishu/lark):"
send "feishu\r"
expect "Language (zh/en):"
send "zh\r"
expect eof
EOF

    if [ -d "$HOME/.lark-cli" ]; then
        # 复制到目标目录
        mkdir -p "$config_dir"
        cp -r "$HOME/.lark-cli"/* "$config_dir/"
        echo "✅ P$bot_num 配置已保存到 $config_dir"

        # 测试验证
        echo "🧪 验证配置..."
        LARK_CLI_CONFIG_DIR="$config_dir" lark-cli auth status
    else
        echo "❌ 配置失败"
    fi

    sleep 1
}

# 遍历初始化8个应用
for i in {0..7}; do
    bot_num=$((i+1))
    config_dir="$HOME/.lark-cli-p$bot_num"
    init_bot_config "$bot_num" "${BOT_NAMES[$i]}" "${BOT_APP_IDS[$i]}" "${BOT_APP_SECRETS[$i]}" "$config_dir"
done

# 最后，恢复备份或者给用户提示
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "✅ 全部完成！"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo ""
echo "💡 配置目录："
for i in {0..7}; do
    bot_num=$((i+1))
    config_dir="$HOME/.lark-cli-p$bot_num"
    if [ -d "$config_dir" ]; then
        echo "   ✅ $config_dir (P$bot_num ${BOT_NAMES[$i]})"
    else
        echo "   ❌ $config_dir"
    fi
done
echo ""
echo "💡 接下来你可以："
echo "   1. 设置便捷函数：source scripts/setup-mock-identities.sh"
echo "   2. 测试身份：lark-p1 auth status"
echo ""

# 恢复备份（如果有）
if [ -n "$backup_dir" ]; then
    echo "🔄 恢复原始配置..."
    rm -rf "$HOME/.lark-cli"
    mv "$backup_dir" "$HOME/.lark-cli"
    echo "✅ 已恢复原始配置"
fi
