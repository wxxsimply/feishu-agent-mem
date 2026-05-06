#!/bin/bash
# 重新初始化 Git storage 目录

echo "=== 重新初始化 Git Storage ==="

GIT_DIR="./data"
BACKUP_ROOT="./data/backup"

if [ -d "$GIT_DIR" ]; then
    echo "1. 备份和清空当前目录..."
    mkdir -p "$BACKUP_ROOT"
    BACKUP_DIR="${BACKUP_ROOT}/$(date +%Y%m%d_%H%M%S)"

    # 先创建备份目录
    mkdir -p "$BACKUP_DIR"

    # 遍历 data 目录下的所有内容（除了 backup）
    for item in "$GIT_DIR"/* "$GIT_DIR"/.*; do
        # 跳过 . 和 .. 以及 backup 目录
        if [ "$(basename "$item")" = "." ] || [ "$(basename "$item")" = ".." ] || [ "$(basename "$item")" = "backup" ]; then
            continue
        fi
        if [ -e "$item" ]; then
            mv "$item" "$BACKUP_DIR/" 2>/dev/null || rm -rf "$item"
        fi
    done

    echo "   已清空 data 目录，备份到: $BACKUP_DIR"
fi

echo "2. 创建新目录..."
mkdir -p "$GIT_DIR"
cd "$GIT_DIR"

echo "3. 初始化 Git 仓库..."
git init

echo "4. 配置 Git 用户..."
git config user.name "feishu-mem"
git config user.email "feishu-mem@example.com"

echo "5. 创建基础目录结构..."
mkdir -p decisions conflicts archive

echo "6. 创建 L0_RULES.md..."
cat > L0_RULES.md << 'EOF'
# L0 核心规则

> 本文件中的规则不可被决策覆盖。违反 L0 规则的决策将被阻断。

## 规则列表

### L0-001: 数据安全
- 不得在决策文件中存储密码、密钥、Token 等敏感信息
- 数据库连接字符串使用环境变量引用

### L0-002: 向后兼容
- 涉及 API 变更的决策必须包含迁移方案
- 废弃 API 需保留至少一个版本周期

### L0-003: 审批流程
- impact_level 为 critical 的决策必须经过两人以上审批
- 涉及生产环境的决策必须包含回滚方案
EOF

echo "7. 创建 .gitignore..."
cat > .gitignore << 'EOF'
.DS_Store
__pycache__/
*.pyc
EOF

echo "8. 初始 commit..."
git add .
git commit -m "init: 初始化 OpenClaw 决策仓库"

echo ""
echo "=== Git Storage 初始化完成 ==="
git log --oneline
