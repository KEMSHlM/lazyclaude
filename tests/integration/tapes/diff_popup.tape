Output outputs/diff_popup.gif

Require lazyclaude

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set WaitTimeout 10s

# Create test files
Hide
Type@0ms "bash /app/tests/integration/tapes/helpers/create_diff_files.sh"
Enter
Show
Sleep 500ms

# Run diff popup
Sleep 500ms
Type "lazyclaude diff --old /tmp/old.go --new /tmp/new.go"
Enter
Sleep 3s

# Scroll down
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms

# Scroll up
Type "k"
Sleep 500ms

# Accept
Type "y"
Sleep 1s
