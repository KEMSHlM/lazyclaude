#!/bin/bash
# Post a tool notification to the MCP server.
# Usage: post_notify.sh TOOL_NAME [WINDOW]
#
# Reads port and auth token from lock file automatically.
# Writes a pending-window file so the server can resolve the PID.
TOOL_NAME="${1:-Bash}"
WINDOW="${2:-@0}"

PORT_FILE="/tmp/lazyclaude-mcp.port"
for i in $(seq 1 30); do [ -f "$PORT_FILE" ] && break; sleep 0.1; done
if [ ! -f "$PORT_FILE" ]; then
    echo "FAIL: port file not found" >&2
    exit 1
fi
PORT=$(cat "$PORT_FILE")

LOCK_DIR="$HOME/.claude/ide"
LOCK_FILE=$(ls "$LOCK_DIR"/*.lock 2>/dev/null | head -1)
if [ -z "$LOCK_FILE" ]; then
    echo "FAIL: no lock file" >&2
    exit 1
fi
AUTH_TOKEN=$(node -e "console.log(JSON.parse(require('fs').readFileSync('$LOCK_FILE','utf8')).authToken)")

# Write pending-window so server can resolve PID -> window
echo "$WINDOW" > /tmp/lazyclaude-pending-window

PID=$$

# Phase 1: tool_info
curl -s -X POST \
    -H "Content-Type: application/json" \
    -H "X-Claude-Code-Ide-Authorization: $AUTH_TOKEN" \
    -d "{\"type\":\"tool_info\",\"pid\":$PID,\"tool_name\":\"$TOOL_NAME\",\"tool_input\":{\"command\":\"ls /tmp\"},\"cwd\":\"/tmp\"}" \
    "http://127.0.0.1:$PORT/notify"

sleep 0.3

# Phase 2: permission_prompt
curl -s -X POST \
    -H "Content-Type: application/json" \
    -H "X-Claude-Code-Ide-Authorization: $AUTH_TOKEN" \
    -d "{\"pid\":$PID,\"message\":\"Allow $TOOL_NAME?\"}" \
    "http://127.0.0.1:$PORT/notify"

echo ""
echo "NOTIFY_SENT tool=$TOOL_NAME port=$PORT window=$WINDOW"
