#!/bin/bash
# 测试添加单个机器人到单个群聊

# 安全加载.env文件
if [ -f ".env" ]; then
    P1_APP_ID=$(grep '^P1_APP_ID=' .env | cut -d= -f2)
    YATSENOS_CHAT_ID=$(grep '^YATSENOS_CHAT_ID=' .env | cut -d= -f2)
else
    echo "❌ 未找到.env文件"
    exit 1
fi

echo "🧪 测试添加单个机器人..."
echo ""
echo "机器人: P1 ($P1_APP_ID)"
echo "群聊: 总群聊 ($YATSENOS_CHAT_ID)"
echo ""
echo "请确保已登录用户身份："
echo "   lark-cli auth status"
echo ""
read -p "确认继续？(y/n): " confirm

if [ "$confirm" = "y" ]; then
    echo ""
    echo "执行..."
    lark-cli im chat.members create \
        --params "{\"chat_id\": \"$YATSENOS_CHAT_ID\", \"member_id_type\": \"app_id\"}" \
        --as user \
        --data "{\"id_list\": [\"$P1_APP_ID\"]}"
    echo ""
    echo "✅ 测试完成，请检查是否添加成功！"
fi
