#!/bin/bash
# 设置YATSENOS模拟角色的便捷函数

echo "============================================"
echo "  YATSENOS 设置模拟角色快捷函数"
echo "============================================"
echo ""

# 定义便捷函数
lark-p1() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p1 lark-cli "$@"; }
lark-p2() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p2 lark-cli "$@"; }
lark-p3() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p3 lark-cli "$@"; }
lark-p4() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p4 lark-cli "$@"; }
lark-p5() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p5 lark-cli "$@"; }
lark-p6() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p6 lark-cli "$@"; }
lark-p7() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p7 lark-cli "$@"; }
lark-p8() { LARK_CLI_CONFIG_DIR=~/.lark-cli-p8 lark-cli "$@"; }

# 导出函数（给bash/zsh使用）
if [ -n "$ZSH_VERSION" ]; then
    export -f lark-p1 lark-p2 lark-p3 lark-p4 lark-p5 lark-p6 lark-p7 lark-p8
elif [ -n "$BASH_VERSION" ]; then
    export -f lark-p1 lark-p2 lark-p3 lark-p4 lark-p5 lark-p6 lark-p7 lark-p8
fi

echo "✅ 快捷函数已设置："
echo ""
echo "   lark-p1 - 陈卓远 (项目经理)"
echo "   lark-p2 - 林晓薇 (内核开发)"
echo "   lark-p3 - 赵一帆 (进程调度)"
echo "   lark-p4 - 王思齐 (用户态开发)"
echo "   lark-p5 - 李沐阳 (存储专家)"
echo "   lark-p6 - 周子涵 (测试专家)"
echo "   lark-p7 - 张若琳 (文档协调)"
echo "   lark-p8 - 刘劲松 (内存专家)"
echo ""
echo "🧪 测试命令："
echo "   lark-p1 auth status"
echo "   lark-p2 auth status"
echo ""
echo "💬 发送消息示例："
echo "   lark-p1 im +messages-send --chat-id oc_xxxxxx --text \"大家好\""
echo ""
echo "✅ 完成！"
