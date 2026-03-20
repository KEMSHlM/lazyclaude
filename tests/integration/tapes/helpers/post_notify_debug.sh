#!/bin/bash
# Debug version: post notification and show server log
TOOL_NAME="${1:-Bash}"
WINDOW="${2:-@0}"

PORT_FILE="/tmp/lazyclaude-mcp.port"
for i in $(seq 1 30); do [ -f "$PORT_FILE" ] && break; sleep 0.1; done
PORT=$(cat "$PORT_FILE")

LOCK_DIR="$HOME/.claude/ide"
LOCK_FILE=$(ls "$LOCK_DIR"/*.lock 2>/dev/null | head -1)
AUTH_TOKEN=$(node -e "console.log(JSON.parse(require('fs').readFileSync('$LOCK_FILE','utf8')).authToken)")

echo "$WINDOW" > /tmp/lazyclaude-pending-window

PID=$$
echo "PORT=$PORT AUTH=${AUTH_TOKEN:0:8}... PID=$PID WINDOW=$WINDOW"

R1=$(curl -s -w '\n%{http_code}' -X POST \
    -H "Content-Type: application/json" \
    -H "X-Claude-Code-Ide-Authorization: $AUTH_TOKEN" \
    -d "{\"type\":\"tool_info\",\"pid\":$PID,\"tool_name\":\"$TOOL_NAME\",\"tool_input\":{\"command\":\"ls /tmp\"},\"cwd\":\"/tmp\"}" \
    "http://127.0.0.1:$PORT/notify")
echo "Phase1: $R1"

sleep 0.5

R2=$(curl -s -w '\n%{http_code}' -X POST \
    -H "Content-Type: application/json" \
    -H "X-Claude-Code-Ide-Authorization: $AUTH_TOKEN" \
    -d "{\"pid\":$PID,\"message\":\"Allow $TOOL_NAME?\"}" \
    "http://127.0.0.1:$PORT/notify")
echo "Phase2: $R2"
