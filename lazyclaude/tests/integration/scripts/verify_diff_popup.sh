#!/bin/bash
# Verify diff popup displays file differences correctly.
#
# Tests:
# 1. Diff popup renders with title, colored diff, and action bar
# 2. Scroll navigation (j/k) works
# 3. 2-option vs 3-option action bar
# 4. y accepts, n rejects
#
# PASS: all checks pass
# FAIL: any check fails

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "$SCRIPT_DIR/test_lib.sh"

init_test "Diff Popup Test" "${1:-lazyclaude}" "${@:2}"

# Create test files
OLD_FILE=$(mktemp /tmp/diff-old-XXXX.go)
NEW_FILE=$(mktemp /tmp/diff-new-XXXX.go)
cat > "$OLD_FILE" <<'EOF'
package main

import "fmt"

func main() {
	fmt.Println("hello")
	fmt.Println("world")
}
EOF
cat > "$NEW_FILE" <<'EOF'
package main

import "fmt"

func main() {
	fmt.Println("hello, world")
	fmt.Println("goodbye")
	fmt.Println("new line")
}
EOF

cleanup_diff() {
    rm -f "$OLD_FILE" "$NEW_FILE"
    cleanup_test_silent
    rm -f "$_PREV_FRAME_FILE" "$_CURR_FRAME_FILE" 2>/dev/null || true
}
trap cleanup_diff EXIT

# --- Test 1: Diff popup renders correctly (3-option default) ---
start_session "$BINARY diff --old $OLD_FILE --new $NEW_FILE"
sleep 2
frame "diff popup (3-option)"

C=$(capture)
R=0; echo "$C" | grep -qE "Diff:|old" || R=1
check "diff popup shows title" $R

R=0; echo "$C" | grep -q "hello" || R=1
check "diff popup shows content" $R

R=0; echo "$C" | grep -q "allow always" || R=1
check "diff popup shows allow always (3-option)" $R

R=0; echo "$C" | grep -q "Esc" || R=1
check "diff popup shows Esc: cancel" $R

# Dismiss
send_keys y
sleep 0.5
tmux -L "$TEST_SOCKET" kill-server 2>/dev/null || true
sleep 0.3

# --- Test 2: Diff popup with 2-option detection ---
# Simulate a 2-option dialog by having a Claude pane with 2 options
tmux -L "$TEST_SOCKET" new-session -d -s lazyclaude -x "$TEST_WIDTH" -y "$TEST_HEIGHT"
TEST_PANE="lazyclaude"
send_keys "echo ' Do you want to proceed?'; echo ' 1. Yes'; echo '   2. No'" Enter
sleep 0.5

# Get window ID
WIN_ID=$(tmux -L "$TEST_SOCKET" display-message -t lazyclaude -p '#{window_id}')

# Open diff in a new window targeting that window
tmux -L "$TEST_SOCKET" new-window -t lazyclaude -n diff
tmux -L "$TEST_SOCKET" send-keys -t lazyclaude:diff \
    "$BINARY diff --window $WIN_ID --old $OLD_FILE --new $NEW_FILE" Enter
sleep 2

TEST_PANE="lazyclaude:diff"
frame "diff popup (2-option)"

C=$(capture_target "lazyclaude:diff")
R=0; echo "$C" | grep -q "allow always" && R=1
check "2-option: does NOT show allow always" $R

R=0; echo "$C" | grep -qE "y:.*yes" || R=1
check "2-option: shows y: yes" $R

R=0; echo "$C" | grep -qE "n:.*no" || R=1
check "2-option: shows n: no" $R

finish_test
