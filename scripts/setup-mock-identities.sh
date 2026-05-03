#!/bin/bash
# 设置YATSENOS模拟角色的便捷函数

echo "============================================"
echo "  YATSENOS 设置模拟角色快捷函数"
echo "============================================"
echo ""

# 存储当前配置备份（如果存在且不是链接）
if [ -d "$HOME/.lark-cli" ] && [ ! -L "$HOME/.lark-cli" ]; then
    if [ ! -d "$HOME/.lark-cli.backup" ]; then
        mv "$HOME/.lark-cli" "$HOME/.lark-cli.backup"
        echo "💾 已备份当前配置到 ~/.lark-cli.backup"
    fi
fi

# 定义便捷函数
lark-use-p1() { rm -rf ~/.lark-cli && cp -r ~/.lark-cli-p1 ~/.lark-cli 2>/dev/null && echo "✅ P1" && lark-cli auth status; }
lark-use-p2() { rm -rf ~/.lark-cli && cp -r ~/.lark-cli-p2 ~/.lark-cli 2>/dev/null && echo "✅ P2" && lark-cli auth status; }
lark-use-p3() { rm -rf ~/.lark-cli && cp -r ~/.lark-cli-p3 ~/.lark-cli 2>/dev/null && echo "✅ P3" && lark-cli auth status; }
lark-use-p4() { rm -rf ~/.lark-cli && cp -r ~/.lark-cli-p4 ~/.lark-cli 2>/dev/null && echo "✅ P4" && lark-cli auth status; }
lark-use-p5() { rm -rf ~/.lark-cli && cp -r ~/.lark-cli-p5 ~/.lark-cli 2>/dev/null && echo "✅ P5" && lark-cli auth status; }
lark-use-p6() { rm -rf ~/.lark-cli && cp -r ~/.lark-cli-p6 ~/.lark-cli 2>/dev/null && echo "✅ P6" && lark-cli auth status; }
lark-use-p7() { rm -rf ~/.lark-cli && cp -r ~/.lark-cli-p7 ~/.lark-cli 2>/dev/null && echo "✅ P7" && lark-cli auth status; }
lark-use-p8() { rm -rf ~/.lark-cli && cp -r ~/.lark-cli-p8 ~/.lark-cli 2>/dev/null && echo "✅ P8" && lark-cli auth status; }
lark-use-main() { rm -rf ~/.lark-cli && [ -d ~/.lark-cli.backup ] && cp -r ~/.lark-cli.backup ~/.lark-cli && echo "✅ 主配置"; }

echo "✅ 快捷函数已设置："
echo ""
echo "   lark-use-p1  # 切换到 P1 (陈卓远)"
echo "   lark-use-p2  # 切换到 P2 (林晓薇)"
echo "   lark-use-p3  # 切换到 P3 (赵一帆)"
echo "   lark-use-p4  # 切换到 P4 (王思齐)"
echo "   lark-use-p5  # 切换到 P5 (李沐阳)"
echo "   lark-use-p6  # 切换到 P6 (周子涵)"
echo "   lark-use-p7  # 切换到 P7 (张若琳)"
echo "   lark-use-p8  # 切换到 P8 (刘劲松)"
echo "   lark-use-main # 恢复主配置"
echo ""
echo "💡 使用方式："
echo "   先 source 这个脚本"
echo "   然后执行 lark-use-p1"
echo ""
echo "然后直接用 lark-cli 发送消息就可以了！"
echo ""
