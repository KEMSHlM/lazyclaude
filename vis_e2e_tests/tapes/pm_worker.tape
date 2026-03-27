Output outputs/pm_worker/pm_worker.gif
Output outputs/pm_worker/pm_worker.txt

Set Shell "bash"
Set Width 1800
Set Height 1200
Set FontSize 16
Set WaitTimeout 180s

# Ctrl+\ で lazyclaude popup 起動
Ctrl+\
Wait+Screen /No sessions/

# --- PM セッション起動 ---
Type "P"
Wait+Screen /\[PM\]/
Sleep 2s

# --- Worker セッション起動 (worktree + タスク) ---
Type "w"
Wait+Screen /Branch/
Sleep 500ms
Type "fix-readme"
Tab
Wait+Screen /Prompt/
Sleep 500ms
Type "Create a file /tmp/worker-done.txt with content 'task complete'. Then send a review request to PM using the message API."
Enter
Wait+Screen /\[W\]/
Sleep 3s

# --- Worker をフルスクリーンで確認 ---
Type "j"
Sleep 500ms
Enter
Sleep 10s

# tool permission が来たら承認 (1 = Yes)
Wait+Screen /Do you want/
Type "1"
Sleep 15s

# 追加の permission があれば承認
Type "1"
Sleep 15s

# --- フルスクリーン終了 ---
Ctrl+\
Sleep 2s

# --- PM を選択してフルスクリーン ---
Type "k"
Sleep 500ms
Enter
Wait+Screen /Sonnet\|Opus\|Claude/
Sleep 2s

# PM にレビューリクエストの確認を指示
Type "Poll for messages using the message API and review any pending requests." Enter
Sleep 20s

# tool permission が来たら承認
Type "1"
Sleep 15s

# --- フルスクリーン終了 → 終了 ---
Ctrl+\
Sleep 2s
Type "q" Sleep 1s
