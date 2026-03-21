Output outputs/option_count/option_count.gif
Output outputs/option_count/option_count.txt

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

# 2択ポップアップ待機
Wait+Screen /Bash/
Sleep 1s

# 却下
Type "y"
Sleep 1s

# 3択ポップアップ待機
Wait+Screen /Write/
Sleep 1s

# 却下
Type "y"
Sleep 1s

# 終了
Type "q"
Sleep 1s
