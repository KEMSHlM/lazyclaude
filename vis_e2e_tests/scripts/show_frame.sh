#!/bin/bash
# VHS .txt のフレーム表示 + diff。
#
# 必要な変数:
#   TAPE_NAME — テープ名

PREV_FRAME="/tmp/prev_frame.txt"
CURR_FRAME="/tmp/curr_frame.txt"
: > "$PREV_FRAME"
FRAME_N=0

show_frame() {
    local block="$1"

    echo "$block" | sed '/^[[:space:]]*$/d' > "$CURR_FRAME"

    if [ -s "$PREV_FRAME" ] && diff -q "$PREV_FRAME" "$CURR_FRAME" >/dev/null 2>&1; then
        return
    fi

    FRAME_N=$((FRAME_N + 1))

    printf '\033[2J\033[H'
    printf '\033[1;36m[Frame %d] %s\033[0m\n' "$FRAME_N" "$TAPE_NAME"
    printf '\033[90m'
    printf '%0.s─' $(seq 1 80)
    printf '\033[0m\n'

    cat "$CURR_FRAME"

    printf '\033[90m'
    printf '%0.s─' $(seq 1 80)
    printf '\033[0m\n'

    if [ -s "$PREV_FRAME" ]; then
        local d
        d=$(diff "$PREV_FRAME" "$CURR_FRAME" 2>/dev/null || true)
        if [ -n "$d" ]; then
            printf '\033[90m--- diff (Frame %d -> %d) ---\033[0m\n' "$((FRAME_N - 1))" "$FRAME_N"
            echo "$d" | while IFS= read -r dline; do
                case "$dline" in
                    \<*) printf '\033[31m%s\033[0m\n' "$dline" ;;
                    \>*) printf '\033[32m%s\033[0m\n' "$dline" ;;
                    *)   printf '\033[90m%s\033[0m\n' "$dline" ;;
                esac
            done
            printf '\033[90m--- end diff ---\033[0m\n'
        fi
    fi
    cp "$CURR_FRAME" "$PREV_FRAME"
}

cleanup_frames() {
    rm -f "$PREV_FRAME" "$CURR_FRAME"
}
