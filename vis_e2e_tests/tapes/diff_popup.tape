Output outputs/diff_popup/diff_popup.gif
Output outputs/diff_popup/diff_popup.txt

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set WaitTimeout 10s

# diff ビューア起動
Type "lazyclaude diff --old /tmp/old.go --new /tmp/new.go"
Enter
Sleep 3s

# スクロール
Type "j"
Type "j"
Type "j"
Sleep 1s
Type "k"
Sleep 1s

# 承認
Type "y"
Sleep 1s
