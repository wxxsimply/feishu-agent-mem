#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
cd "$PROJECT_ROOT"

# Configuration
DATA_PATH="$PROJECT_ROOT/chat-data/Software-related-Slack-Chats-with-Disentangled-Conversations/data/pythondev/2018/merged-pythondev-help.xml"
MESSAGES_JSON="$PROJECT_ROOT/outputs/test-messages.json"
PUSH_CHAT_ID="oc_096c0cd1dfe93cb2f1264e59490946d2"
TEST_CHAT_ID="oc_f382174f2ab17aa2ebefba72834df0b2"
LOG_FILE="$PROJECT_ROOT/logs/mem-service-test.log"

echo "========================================"
echo "  Lark-IM 测试流程"
echo "========================================"
echo ""

# Step 1: Parse test data
echo "[1/6] 解析测试数据..."
mkdir -p "$(dirname "$MESSAGES_JSON")"
python3 "$SCRIPT_DIR/parse-slack-data.py" "$DATA_PATH" 100 "$MESSAGES_JSON"
echo ""

# Step 2: Send first test message to verify lark-cli works
echo "[2/6] 发送第一条测试消息..."
python3 "$SCRIPT_DIR/send-test-messages.py" "$MESSAGES_JSON" "$PUSH_CHAT_ID" 0 1
echo ""
read -p "按回车确认消息发送成功，继续下一步..."
echo ""

# Step 3: Stop mem-service
echo "[3/6] 停止所有 mem-service 进程..."
pkill -f mem-service || true
pkill -f detector-lark-im || true
sleep 2
echo "已停止"
echo ""

# Step 4: Rebuild and restart
echo "[4/6] 重新编译并启动 mem-service..."
go build -o bin/mem-service ./cmd/mem-service
go build -o bin/detector-lark-im ./cmd/detector-lark-im

mkdir -p "$(dirname "$LOG_FILE")"

echo "启动 mem-service (后台运行，日志输出到 $LOG_FILE)..."
rm -f "$LOG_FILE"
nohup ./bin/mem-service > "$LOG_FILE" 2>&1 &
MEM_SERVICE_PID=$!
echo "mem-service PID: $MEM_SERVICE_PID"

echo "等待 3 秒让服务启动..."
sleep 3
echo ""

# Step 5: Start detector-lark-im (long connection mode)
echo "[5/6] 启动 detector-lark-im (长连接模式)..."
nohup ./bin/detector-lark-im >> "$LOG_FILE" 2>&1 &
DETECTOR_PID=$!
echo "detector-lark-im PID: $DETECTOR_PID"
echo "等待 2 秒..."
sleep 2
echo ""

# Step 6: Send test messages to TEST_CHAT_ID
echo "[6/6] 发送 100 条测试消息到 $TEST_CHAT_ID..."
python3 "$SCRIPT_DIR/send-test-messages.py" "$MESSAGES_JSON" "$TEST_CHAT_ID" 0 100
echo ""

echo "========================================"
echo "  测试已启动！"
echo "========================================"
echo ""
echo "观察日志输出："
echo "  tail -f $LOG_FILE"
echo ""
echo "进程管理："
echo "  mem-service PID: $MEM_SERVICE_PID"
echo "  detector-lark-im PID: $DETECTOR_PID"
echo ""
echo "停止测试："
echo "  kill $MEM_SERVICE_PID $DETECTOR_PID"
echo ""
echo "按 Ctrl+C 可停止观察，后台进程会继续运行"

# Keep script running and show logs
tail -f "$LOG_FILE"
