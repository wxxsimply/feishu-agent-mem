#!/bin/bash
# 增强飞书多维表格显示效果的脚本

BASE_TOKEN="NnnMb5mWJaBJkXsHf69cIpfMn8b"
TABLE_ID="tblEBXkSxaqxnY6l"

echo "=== 开始创建公式字段 ==="

# 1. 创建显示_提交人
echo ""
echo "1. 创建显示_提交人..."
lark-cli base +field-create \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" \
  --json '{
    "type": "formula",
    "name": "显示_提交人",
    "description": "带图标的提交人显示",
    "expression": "IF(ISBLANK([proposer]), \"\", \"👤 \" & [proposer])"
  }' \
  --as user \
  --i-have-read-guide

# 2. 创建显示_执行人
echo ""
echo "2. 创建显示_执行人..."
lark-cli base +field-create \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" \
  --json '{
    "type": "formula",
    "name": "显示_执行人",
    "description": "带图标的执行人显示",
    "expression": "IF(ISBLANK([executor]), \"未分配\", \"👤 \" & [executor])"
  }' \
  --as user \
  --i-have-read-guide

# 3. 创建显示_Git哈希
echo ""
echo "3. 创建显示_Git哈希..."
lark-cli base +field-create \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" \
  --json '{
    "type": "formula",
    "name": "显示_Git哈希",
    "description": "等宽格式显示Git提交哈希",
    "expression": "IF(ISBLANK([git_commit_hash]), \"\", \"`\" & LEFT([git_commit_hash], 7) & \"`\")"
  }' \
  --as user \
  --i-have-read-guide

# 4. 创建显示_状态
echo ""
echo "4. 创建显示_状态..."
lark-cli base +field-create \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" \
  --json '{
    "type": "formula",
    "name": "显示_状态",
    "description": "带图标的状态标签",
    "expression": "IFS([status] = \"pending\", \"⏳ 待处理\", [status] = \"open\", \"🟢 开放\", [status] = \"in_progress\", \"🟡 进行中\", [status] = \"closed\", \"🔴 已关闭\", [status] = \"resolved\", \"✅ 已解决\", TRUE, [status])"
  }' \
  --as user \
  --i-have-read-guide

# 5. 创建显示_影响等级
echo ""
echo "5. 创建显示_影响等级..."
lark-cli base +field-create \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" \
  --json '{
    "type": "formula",
    "name": "显示_影响等级",
    "description": "带图标的影响等级",
    "expression": "IFS([impact_level] = \"critical\", \"🔴 Critical\", [impact_level] = \"high\", \"🟠 High\", [impact_level] = \"major\", \"🟡 Major\", [impact_level] = \"medium\", \"🟢 Medium\", [impact_level] = \"low\", \"🟢 Low\", [impact_level] = \"advisory\", \"ℹ️ Advisory\", TRUE, [impact_level])"
  }' \
  --as user \
  --i-have-read-guide

# 6. 创建显示_创建时间
echo ""
echo "6. 创建显示_创建时间..."
lark-cli base +field-create \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" \
  --json '{
    "type": "formula",
    "name": "显示_创建时间",
    "description": "格式化的创建时间",
    "expression": "[created_at]"
  }' \
  --as user \
  --i-have-read-guide

# 7. 创建显示_摘要卡片
echo ""
echo "7. 创建显示_摘要卡片..."
lark-cli base +field-create \
  --base-token "$BASE_TOKEN" \
  --table-id "$TABLE_ID" \
  --json '{
    "type": "formula",
    "name": "显示_摘要卡片",
    "description": "综合信息摘要",
    "expression": "显示_状态 & \" | \" & 显示_影响等级 & \" | \" & 显示_Git哈希"
  }' \
  --as user \
  --i-have-read-guide

echo ""
echo "=== 所有字段创建完成！==="
echo ""
echo "现在可以去飞书多维表格查看效果了。"
echo "建议创建一个新视图，只显示增强后的字段。"
