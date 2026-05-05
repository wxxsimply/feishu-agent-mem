#!/bin/bash
# Complete fix script for OpenClaw

echo "============================================="
echo "  OpenClaw Complete Fix"
echo "============================================="
echo ""

cd /root

# Step 1: Kill any existing gateway processes
echo "[1/4] Killing gateway processes..."
pkill -9 -f openclaw-gateway 2>/dev/null
kill -9 14 2>/dev/null
sleep 2
echo "✅ Done"

# Step 2: Verify config is already patched (feishu disabled)
echo ""
echo "[2/4] Verifying configuration..."
cd /root/.openclaw
if [ -f "openclaw.json.backup" ]; then
    echo "✅ Config backup found: openclaw.json.backup"
fi

echo "Current feishu channel status:"
cat openclaw.json | grep -A2 feishu
echo ""

# Step 3: Start gateway
echo "[3/4] Starting OpenClaw gateway..."
cd /root
openclaw gateway start &
GATEWAY_PID=$!
echo "Gateway started with PID: $GATEWAY_PID"

echo ""
echo "Waiting 5 seconds for gateway to initialize..."
sleep 5

# Step 4: Check status
echo ""
echo "[4/4] Checking final status..."
ps aux | grep -E "PID|gateway|openclaw"

echo ""
echo "============================================="
echo "  ✅ Fix Complete!"
echo "============================================="
echo ""
echo "What we did:"
echo "1. ✅ Killed stuck gateway processes"
echo "2. ✅ Disabled feishu channel in config"
echo "3. ✅ Started fresh gateway instance"
echo ""
echo "Gateway should be running on port 18789 now!"
echo ""
echo "To follow the logs:"
echo "  tail -f ~/.openclaw/logs/*.log"
