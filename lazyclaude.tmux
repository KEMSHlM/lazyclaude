#!/usr/bin/env bash
# lazyclaude TPM plugin entry point.
# 1. Runs `lazyclaude setup` (MCP server + Claude Code hooks)
# 2. Registers tmux keybindings from @claude-* options
#
# Configurable options (set in tmux.conf / plugins.conf):
#   @claude-launch-key    key to launch lazyclaude TUI (default: space)
#   @claude-suppress-keys space-separated keys to disable inside lazyclaude session

BINARY="$(command -v lazyclaude 2>/dev/null)"

if [ -z "$BINARY" ]; then
    echo "lazyclaude: binary not found in PATH" >&2
    exit 1
fi

# Capture the directory containing lazyclaude (and likely claude).
LAUNCH_BIN_DIR="$(dirname "$BINARY")"

# Run Go setup (MCP server + Claude Code hooks)
"$BINARY" setup

# Read tmux options
launch_key=$(tmux show-option -gqv @claude-launch-key 2>/dev/null)
suppress_keys=$(tmux show-option -gqv @claude-suppress-keys 2>/dev/null)

launch_key="${launch_key:-space}"

# Register keybindings — toggle: close if open, open if closed.
tmux bind-key "$launch_key" display-popup -B -w 80% -h 80% -d "#{pane_current_path}" -E "LAZYCLAUDE_HOST_TMUX=\$TMUX env -u TMUX PATH='$LAUNCH_BIN_DIR':\$PATH LAZYCLAUDE_POPUP_MODE=tmux $BINARY"

# Suppress specified keys inside lazyclaude session.
# Skip if the key is already wrapped with our if-shell guard (prevents
# exponential nesting on repeated source).
for key in $suppress_keys; do
    current=$(tmux list-keys -T prefix 2>/dev/null | awk -v k="$key" '$4 == k {$1=$2=$3=$4=""; sub(/^[[:space:]]+/,""); print}')
    case "$current" in
        *lazyclaude*) continue ;;  # already wrapped
    esac
    if [ -n "$current" ]; then
        tmux bind-key "$key" if -F '#{==:#{session_name},lazyclaude}' '' "$current"
    fi
done
