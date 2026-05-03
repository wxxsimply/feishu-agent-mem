#!/bin/bash
# 为P1-P8八个应用分别初始化配置目录并登录用户身份

echo "============================================"
echo "  YATSENOS 初始化配置并登录所有机器人"
echo "============================================"
echo ""

# 安全加载.env文件 - 只加载特定的变量
if [ -f ".env" ]; then
    # 直接从.env读取我们需要的变量
    P1_APP_ID=$(grep '^P1_APP_ID=' .env | cut -d= -f2)
    P1_APP_SECRET=$(grep '^P1_APP_SECRET=' .env | cut -d= -f2)
    P2_APP_ID=$(grep '^P2_APP_ID=' .env | cut -d= -f2)
    P2_APP_SECRET=$(grep '^P2_APP_SECRET=' .env | cut -d= -f2)
    P3_APP_ID=$(grep '^P3_APP_ID=' .env | cut -d= -f2)
    P3_APP_SECRET=$(grep '^P3_APP_SECRET=' .env | cut -d= -f2)
    P4_APP_ID=$(grep '^P4_APP_ID=' .env | cut -d= -f2)
    P4_APP_SECRET=$(grep '^P4_APP_SECRET=' .env | cut -d= -f2)
    P5_APP_ID=$(grep '^P5_APP_ID=' .env | cut -d= -f2)
    P5_APP_SECRET=$(grep '^P5_APP_SECRET=' .env | cut -d= -f2)
    P6_APP_ID=$(grep '^P6_APP_ID=' .env | cut -d= -f2)
    P6_APP_SECRET=$(grep '^P6_APP_SECRET=' .env | cut -d= -f2)
    P7_APP_ID=$(grep '^P7_APP_ID=' .env | cut -d= -f2)
    P7_APP_SECRET=$(grep '^P7_APP_SECRET=' .env | cut -d= -f2)
    P8_APP_ID=$(grep '^P8_APP_ID=' .env | cut -d= -f2)
    P8_APP_SECRET=$(grep '^P8_APP_SECRET=' .env | cut -d= -f2)
    echo "✅ 已加载.env文件"
else
    echo "❌ 未找到.env文件"
    exit 1
fi

# 配置目录和应用信息
declare -a BOT_NAMES=(
    "P1"
    "P2"
    "P3"
    "P4"
    "P5"
    "P6"
    "P7"
    "P8"
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
echo "📋 即将初始化 ${#BOT_APP_IDS[@]} 个机器人配置："
for i in {0..7}; do
    bot_num=$((i+1))
    echo "   P$bot_num ${BOT_NAMES[$i]}: ${BOT_APP_IDS[$i]}"
done
echo ""

# 备份现有配置
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

echo ""
echo "============================================"
echo "📝 说明："
echo "   现在需要你为每个机器人分别登录授权"
echo "   每次会打开浏览器进行授权"
echo "   可以用同一个飞书账号为所有应用授权"
echo "============================================"
echo ""
read -p "准备好了吗？(y/n): " ready
if [ "$ready" != "y" ]; then
    if [ -n "$backup_dir" ]; then
        mv "$backup_dir" "$HOME/.lark-cli"
    fi
    echo "❌ 已取消"
    exit 0
fi

# 函数：初始化单个应用配置并登录
init_and_login_bot() {
    local bot_num="$1"
    local bot_name="$2"
    local app_id="$3"
    local app_secret="$4"
    local config_dir="$5"

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "🤖 P$bot_num $bot_name"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📌 App ID: $app_id"
    echo "📂 Config: $config_dir"
    echo ""

    # 清空当前配置
    rm -rf "$HOME/.lark-cli"

    # 1. 配置应用
    echo "⏳ 正在配置应用..."
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

    if [ ! -d "$HOME/.lark-cli" ]; then
        echo "❌ 配置失败"
        return 1
    fi

    echo "✅ 应用配置完成"

    # 2. 登录用户身份
    echo ""
    echo "🔐 现在需要登录用户身份"
    echo "💡 提示：可以用同一个飞书账号为所有应用授权"
    echo ""
    read -p "按回车开始登录（或 q 跳过这个）：" -r action

    if [ "$action" = "q" ] || [ "$action" = "Q" ]; then
        echo "⏭️ 跳过P$bot_num"
        # 保存已配置的（未登录）
        mkdir -p "$config_dir"
        cp -r "$HOME/.lark-cli"/* "$config_dir/"
        return 0
    fi

    # 执行登录
    lark-cli auth login --domain "im,contact,task,calendar,docs,wiki,base,sheets,vc,minutes"

    # 验证登录状态
    echo ""
    echo "🧪 验证登录状态..."
    lark-cli auth status

    # 保存配置
    mkdir -p "$config_dir"
    cp -r "$HOME/.lark-cli"/* "$config_dir/"
    echo "✅ 配置已保存到 $config_dir"

    sleep 1
}

# 遍历处理8个应用
for i in {0..7}; do
    bot_num=$((i+1))
    config_dir="$HOME/.lark-cli-p$bot_num"
    init_and_login_bot "$bot_num" "${BOT_NAMES[$i]}" "${BOT_APP_IDS[$i]}" "${BOT_APP_SECRETS[$i]}" "$config_dir"

    echo ""
    read -p "继续下一个 (y/n): " -n 1 -r
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        break
    fi
done

# 恢复备份
echo ""
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
if [ -n "$backup_dir" ]; then
    echo "🔄 恢复原始配置..."
    rm -rf "$HOME/.lark-cli"
    mv "$backup_dir" "$HOME/.lark-cli"
else
    rm -rf "$HOME/.lark-cli"
fi

echo ""
echo "✅ 全部完成！"
echo ""
echo "📂 生成的配置目录："
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
echo "💡 下一步：运行 source scripts/setup-mock-identities.sh"
echo ""
