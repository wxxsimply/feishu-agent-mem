#!/bin/bash
# 删除飞书多维表格显示增强字段

BASE_TOKEN="NnnMb5mWJaBJkXsHf69cIpfMn8b"
TABLE_ID="tblEBXkSxaqxnY6l"

echo "=== 开始删除公式字段 ==="

# 1. 删除显示_摘要卡片
echo ""
echo "1. 删除显示_摘要卡片 (fldxUfGh0M)..."
lark-cli base +field-delete \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" \
  --field-id "fldxUfGh0M" \
  --as user \
  --yes

# 2. 删除显示_创建时间
echo ""
echo "2. 删除显示_创建时间 (fld5khRGvS)..."
lark-cli base +field-delete \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" \
  --field-id "fld5khRGvS" \
  --as user \
  --yes

# 3. 删除显示_影响等级
echo ""
echo "3. 删除显示_影响等级 (fldb8DXRnX)..."
lark-cli base +field-delete \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" \
  --field-id "fldb8DXRnX" \
  --as user \
  --yes

# 4. 删除显示_状态
echo ""
echo "4. 删除显示_状态 (fldKKhr5zi)..."
lark-cli base +field-delete \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" \
  --field-id "fldKKhr5zi" \
  --as user \
  --yes

# 5. 删除显示_Git哈希
echo ""
echo "5. 删除显示_Git哈希 (fldgWpHkXl)..."
lark-cli base +field-delete \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" \
  --field-id "fldgWpHkXl" \
  --as user \
  --yes

# 6. 删除显示_执行人
echo ""
echo "6. 删除显示_执行人 (fld4QP6WxM)..."
lark-cli base +field-delete \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" \
  --field-id "fld4QP6WxM" \
  --as user \
  --yes

# 7. 删除显示_提交人
echo ""
echo "7. 删除显示_提交人 (fldQdii0wD)..."
lark-cli base +field-delete \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" \
  --field-id "fldQdii0wD" \
  --as user \
  --yes

echo ""
echo "=== 所有字段删除完成！==="
echo ""
echo "已恢复原始纯文本显示。"
