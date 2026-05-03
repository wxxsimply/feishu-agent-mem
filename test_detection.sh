
#!/bin/bash
echo "=== Bash 测试检测流程 ==="

CHAT_ID="oc_202372b4148e01ae309636d99764bc9a"
LAST_CHECK=$(date -u -v-1H +"%Y-%m-%dT%H:%M:%SZ" | sed -e 's/Z/+08:00/')
LAST_CHECK=$(date -u -d "1 hour ago" +"%Y-%m-%dT%H:%M:%S+08:00")

echo "CHAT_ID=$CHAT_ID"
echo "LAST_CHECK=$LAST_CHECK"

echo -e "\n--- 1. 调用 lark-cli ---"
OUTPUT=$(lark-cli im +chat-messages-list \
    --chat-id "$CHAT_ID" \
    --as user \
    --start "$LAST_CHECK" \
    --page-size 10 2>&1)

echo "$OUTPUT"

echo -e "\n--- 2. 检查是否有 ok=true ---"
if echo "$OUTPUT" | grep -q '"ok": true'; then
    echo "✓ lark-cli 调用成功！"
else
    echo "✗ lark-cli 调用失败！"
fi

echo -e "\n--- 3. 检查是否有 messages ---"
MSG_COUNT=$(echo "$OUTPUT" | grep -o '"create_time":' | wc -l)
echo "找到 $MSG_COUNT 条消息"

echo -e "\n--- 完成 ---"
