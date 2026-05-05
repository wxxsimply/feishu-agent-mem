#!/bin/bash
# Simple script to start OpenClaw gateway (in container)

echo "============================================="
echo "  Starting OpenClaw Gateway"
echo "============================================="
echo ""

cd /root

# Kill any existing processes
echo "[1/3] Killing any running gateway..."
pkill -9 -f openclaw-gateway 2>/dev/null
kill -9 14 2>/dev/null
sleep 2
echo "✅ Done"

# Check status
echo ""
echo "[2/3] Current processes:"
ps aux | grep -E "PID|gateway|openclaw"
echo ""

# Start gateway
echo "[3/3] Starting OpenClaw gateway..."
openclaw gateway start &
GATEWAY_PID=$!
echo "Gateway started with PID: $GATEWAY_PID"
echo ""
echo "Waiting 5 seconds..."
sleep 5

echo ""
echo "============================================="
echo "  ✅ Done!"
echo "============================================="
echo ""
echo "Check final status:"
ps aux | grep -E "PID|gateway"
echo ""
echo "If it's running, you can access on http://localhost:18789"
echo ""
echo "Follow logs:"
echo "  tail -f ~/.openclaw/logs/*.log"
echo ""
echo "Note: The feishu module warning is normal and won't block gateway from starting!"
