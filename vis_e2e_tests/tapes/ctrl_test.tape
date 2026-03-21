Output outputs/ctrl_test/ctrl_test.gif
Output outputs/ctrl_test/ctrl_test.txt

Set Shell "bash"
Set Width 1200
Set Height 600
Set FontSize 16
Set WaitTimeout 10s

# tmux 起動してバインド設定
Type "tmux new-session -d -s t && tmux bind-key -T root C-\\ display-message 'GOT_CTRL_BACKSLASH' && tmux attach -t t"
Enter
Sleep 2s

# Ctrl+\ を送信
Ctrl+\
Sleep 2s
