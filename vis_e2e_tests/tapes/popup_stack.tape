Output outputs/popup_stack/popup_stack.gif
Output outputs/popup_stack/popup_stack.txt

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set WaitTimeout 30s

# lazyclaude 起動
Type "lazyclaude"
Enter
Wait+Screen /No sessions/
Sleep 1s

# カスケード表示を待つ [3/3]
Wait+Screen /3\/3/
Sleep 1s

# 上矢印 → [2/3]
Up
Sleep 1s

# Esc → サスペンド
Escape
Sleep 1s

# p → 再表示
Type "p"
Sleep 1s

# y → 個別却下
Type "y"
Sleep 1s

# y → もう一つ却下
Type "y"
Sleep 1s

# 終了
Type "q"
Sleep 1s
