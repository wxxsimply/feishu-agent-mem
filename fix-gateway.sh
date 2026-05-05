#!/bin/bash
# Fix OpenClaw Gateway startup issue

echo "============================================="
echo "  Fixing OpenClaw Gateway"
echo "============================================="

cd /root

echo ""
echo "[1] Stopping any running gateway processes..."
pkill -9 openclaw-gateway 2>/dev/null
kill -9 14 2>/dev/null
sleep 2

echo ""
echo "[2] Checking for remaining gateway processes..."
PIDS=$(pgrep -f "openclaw-gateway")
if [ -n "$PIDS" ]; then
    echo "Killing PIDs: $PIDS"
    kill -9 $PIDS 2>/dev/null
    sleep 1
fi

echo ""
echo "[3] Current processes:"
ps aux | grep -E "PID|openclaw|gateway"

echo ""
echo "[4] Starting gateway..."
cd /root
openclaw gateway start &
GATEWAY_PID=$!

echo ""
echo "Waiting for gateway to start (5s)..."
sleep 5

echo ""
echo "[5] Checking final status:"
ps aux | grep -E "PID|openclaw|gateway"

echo ""
echo "============================================="
echo "  Done!"
echo "============================================="
echo ""
echo "If gateway still fails, check the logs at:"
echo "  ~/.openclaw/logs/"
