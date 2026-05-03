#!/bin/bash
# 清空 Bitable 的脚本

echo "=== 清空 Bitable ==="

BASE_TOKEN="NnnMb5mWJaBJkXsHf69cIpfMn8b"
TABLE_ID="tblEBXkSxaqxnY6l"

echo "Base Token: $BASE_TOKEN"
echo "Table ID: $TABLE_ID"

# 1. 先列出所有 records 获取 record_ids
echo "1. 正在获取所有 record IDs..."
records=$(mktemp /tmp/bitable-records.XXXXXX)

lark-cli base +record-list \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" > "$records"

echo "2. 解析 record_id_list..."
record_ids=$(cat "$records" | python3 -c "
import sys, json
data = json.load(sys.stdin)
if 'data' in data and 'record_id_list' in data['data']:
    print('\n'.join(data['data']['record_id_list']))
")

if [ -z "$record_ids" ]; then
    echo "没有找到 records，或者无法解析！"
    echo "原始响应："
    cat "$records"
    rm -f "$records"
    exit 1
fi

echo "3. 找到了 $(echo "$record_ids" | wc -l | awk '{print $1}') 条 records，准备删除..."

# 4. 删除每个 record
count=0
for record_id in $record_ids; do
    if [ -z "$record_id" ]; then
        continue
    fi
    count=$((count + 1))
    echo "[$count/$record_count] 删除 record_id: $record_id..."
    lark-cli base +record-delete \
      --base-token "$BASE_TOKEN" \
      --table-id "$TABLE_ID" \
      --record-id "$record_id" \
      --yes 2>&1
    sleep 0.1  # 避免触发速率限制
done

# 5. 清理
rm -f "$records"

echo
echo "=== 完成！共删除了 $count 条 records ==="
