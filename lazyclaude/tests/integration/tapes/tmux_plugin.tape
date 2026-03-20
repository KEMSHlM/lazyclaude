Output outputs/tmux_plugin.gif
Output outputs/tmux_plugin.txt

Require lazyclaude
Require tmux
Require node
Require curl

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set WaitTimeout 15s

# 1. Start tmux on "lazyclaude" socket (matches server default)
Sleep 500ms
Type "tmux -L lazyclaude new-session -s claude -x 80 -y 24"
Enter
Sleep 1s

# 2. Run lazyclaude setup (TPM plugin entry point)
Sleep 500ms
Type "lazyclaude setup"
Enter
Sleep 3s

# 3. Split pane (bottom = request sender)
Ctrl+b
Type "\""
Sleep 1s

# 4. Get window ID and send notification
Sleep 500ms
Type "WIN=$(tmux -L lazyclaude display-message -t claude:0.0 -p '#{window_id}') && bash /app/tests/integration/tapes/helpers/post_notify.sh Bash $WIN"
Enter
Sleep 5s

# 5. Popup should appear via display-popup. Type 'y' to accept.
Type "y"
Sleep 2s

# 6. Show final state
Ctrl+b
Up
Sleep 1s
