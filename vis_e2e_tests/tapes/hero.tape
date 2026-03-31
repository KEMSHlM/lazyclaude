Output outputs/hero/hero.gif
Output outputs/hero/hero.txt

Set Shell "bash"
Set Width 1800
Set Height 1200
Set FontSize 16
Set WaitTimeout 60s

# Launch lazyclaude via tmux plugin (Ctrl+\)
Ctrl+\
Wait+Screen /No sessions/
Sleep 1s

# Create a new Claude Code session
Type "n"
Wait+Screen /app/
Sleep 10s

# Enter fullscreen mode to interact with Claude
Enter
Wait+Screen /Sonnet/

# Ask Claude to create a file (triggers Write tool permission prompt)
Type "create a file /tmp/hello.txt with content 'Hello from lazyclaude!'" Enter

# Wait for Claude to process, then exit fullscreen to see popup
Sleep 5s
Ctrl+\

# Permission popup appears as a gocui overlay
Wait+Screen /C-y/
Sleep 2s

# Approve the permission with Ctrl+y
Ctrl+y
Sleep 3s

# Exit lazyclaude
Type "q"
Sleep 1s
