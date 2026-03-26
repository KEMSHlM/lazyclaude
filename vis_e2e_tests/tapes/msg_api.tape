Output outputs/msg_api/msg_api.gif
Output outputs/msg_api/msg_api.txt

Set Shell "bash"
Set Width 1800
Set Height 1200
Set FontSize 16
Set WaitTimeout 60s

# Ctrl+\ で lazyclaude popup 起動
Ctrl+\
Wait+Screen /No sessions/

# PM セッション起動
Type "P"
Wait+Screen /\[PM\]/
Sleep 2s

# Worker (worktree) セッション起動
Type "w"
Wait+Screen /Branch/
Sleep 500ms
Type "test-api"
Tab
Wait+Screen /Prompt/
Sleep 500ms
Type "wait for instructions"
Enter
Wait+Screen /\[W\]/
Sleep 5s

# entrypoint.sh のバックグラウンドスクリプトが API テストを実行
# 結果は outputs/msg_api/api-test.log に書き出される
Sleep 15s

# q で終了
Type "q" Sleep 1s
