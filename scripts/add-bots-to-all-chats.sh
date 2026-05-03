#!/bin/bash
# 将P1-P8八个应用添加到10个YATSENOS群聊中

echo "============================================"
echo "  YATSENOS 批量添加机器人到群聊"
echo "============================================"
echo ""

# 安全加载.env文件
if [ -f ".env" ]; then
    # 读取应用ID
    P1_APP_ID=$(grep '^P1_APP_ID=' .env | cut -d= -f2)
    P2_APP_ID=$(grep '^P2_APP_ID=' .env | cut -d= -f2)
    P3_APP_ID=$(grep '^P3_APP_ID=' .env | cut -d= -f2)
    P4_APP_ID=$(grep '^P4_APP_ID=' .env | cut -d= -f2)
    P5_APP_ID=$(grep '^P5_APP_ID=' .env | cut -d= -f2)
    P6_APP_ID=$(grep '^P6_APP_ID=' .env | cut -d= -f2)
    P7_APP_ID=$(grep '^P7_APP_ID=' .env | cut -d= -f2)
    P8_APP_ID=$(grep '^P8_APP_ID=' .env | cut -d= -f2)

    # 读取群聊ID
    YATSENOS_CHAT_ID=$(grep '^YATSENOS_CHAT_ID=' .env | cut -d= -f2)
    YATSENOS_LAB0_CHAT_ID=$(grep '^YATSENOS_LAB0_CHAT_ID=' .env | cut -d= -f2)
    YATSENOS_LAB1_CHAT_ID=$(grep '^YATSENOS_LAB1_CHAT_ID=' .env | cut -d= -f2)
    YATSENOS_LAB2_CHAT_ID=$(grep '^YATSENOS_LAB2_CHAT_ID=' .env | cut -d= -f2)
    YATSENOS_LAB3_CHAT_ID=$(grep '^YATSENOS_LAB3_CHAT_ID=' .env | cut -d= -f2)
    YATSENOS_LAB4_CHAT_ID=$(grep '^YATSENOS_LAB4_CHAT_ID=' .env | cut -d= -f2)
    YATSENOS_LAB5_CHAT_ID=$(grep '^YATSENOS_LAB5_CHAT_ID=' .env | cut -d= -f2)
    YATSENOS_LAB6_CHAT_ID=$(grep '^YATSENOS_LAB6_CHAT_ID=' .env | cut -d= -f2)
    YATSENOS_LAB7_CHAT_ID=$(grep '^YATSENOS_LAB7_CHAT_ID=' .env | cut -d= -f2)
    YATSENOS_LAB8_CHAT_ID=$(grep '^YATSENOS_LAB8_CHAT_ID=' .env | cut -d= -f2)

    echo "✅ 已加载.env文件"
else
    echo "❌ 未找到.env文件"
    exit 1
fi

# 收集8个应用的app_id
BOT_APP_IDS=(
    "$P1_APP_ID"
    "$P2_APP_ID"
    "$P3_APP_ID"
    "$P4_APP_ID"
    "$P5_APP_ID"
    "$P6_APP_ID"
    "$P7_APP_ID"
    "$P8_APP_ID"
)

# 10个群聊
CHAT_IDS=(
    "$YATSENOS_CHAT_ID"
    "$YATSENOS_LAB0_CHAT_ID"
    "$YATSENOS_LAB1_CHAT_ID"
    "$YATSENOS_LAB2_CHAT_ID"
    "$YATSENOS_LAB3_CHAT_ID"
    "$YATSENOS_LAB4_CHAT_ID"
    "$YATSENOS_LAB5_CHAT_ID"
    "$YATSENOS_LAB6_CHAT_ID"
    "$YATSENOS_LAB7_CHAT_ID"
    "$YATSENOS_LAB8_CHAT_ID"
)

# 群聊名称对应（便于显示）
CHAT_NAMES=(
    "总群聊"
    "LAB0群"
    "LAB1群"
    "LAB2群"
    "LAB3群"
    "LAB4群"
    "LAB5群"
    "LAB6群"
    "LAB7群"
    "LAB8群"
)

echo ""
echo "📋 配置信息："
echo "   机器人数量: ${#BOT_APP_IDS[@]}"
echo "   群聊数量: ${#CHAT_IDS[@]}"
echo ""

# 显示8个应用的app_id
echo "🤖 机器人列表："
for i in {0..7}; do
    echo "   P$((i+1)): ${BOT_APP_IDS[$i]}"
done
echo ""

# 显示10个群聊
echo "💬 群聊列表："
for i in {0..9}; do
    echo "   ${CHAT_NAMES[$i]}: ${CHAT_IDS[$i]}"
done
echo ""

# 确认操作
read -p "⚠️  即将把8个机器人添加到10个群聊，确认继续？(y/n): " confirm
if [ "$confirm" != "y" ]; then
    echo "❌ 已取消"
    exit 0
fi
echo ""

# 函数：将机器人添加到群聊
add_bots_to_chat() {
    local chat_id="$1"
    local chat_name="$2"
    shift 2
    local bot_ids=("$@")

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "📱 正在处理群聊: $chat_name"
    echo "🆔 群聊ID: $chat_id"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

    # 每次最多添加5个机器人，分两批
    local batch1=("${bot_ids[@]:0:5}")
    local batch2=("${bot_ids[@]:5:3}")

    for batch_num in 1 2; do
        local ids=()
        if [ "$batch_num" -eq 1 ]; then
            ids=("${batch1[@]}")
        elif [ "$batch_num" -eq 2 ] && [ "${#batch2[@]}" -gt 0 ]; then
            ids=("${batch2[@]}")
        else
            continue
        fi

        echo ""
        echo "📦 批次 $batch_num/2，添加 ${#ids[@]} 个机器人..."
        echo "   机器人: ${ids[*]}"

        # 转换为JSON列表
        local id_list_json
        id_list_json=$(printf '%s\n' "${ids[@]}" | jq -R . | jq -s 'map(select(. != ""))')

        # 构建 params
        local params_json="{\"chat_id\": \"$chat_id\", \"member_id_type\": \"app_id\"}"

        # 执行添加
        local add_result
        add_result=$(lark-cli im chat.members create \
            --params "$params_json" \
            --as user \
            --data "{\"id_list\": $id_list_json}" 2>&1)

        local exit_code=$?

        if [ "$exit_code" -eq 0 ]; then
            echo "✅ 批次 $batch_num 添加成功"
            echo "$add_result"
        else
            echo "❌ 批次 $batch_num 添加失败"
            echo "$add_result"
        fi

        sleep 1
    done

    echo ""
    echo "✅ 群聊 $chat_name 处理完成"
    echo ""
}

# 遍历所有群聊，添加机器人
for i in {0..9}; do
    add_bots_to_chat "${CHAT_IDS[$i]}" "${CHAT_NAMES[$i]}" "${BOT_APP_IDS[@]}"
    sleep 2
done

echo ""
echo "============================================"
echo "🎉 全部完成！"
echo "============================================"
echo ""
echo "建议: 现在可以为每个机器人初始化配置目录了"
