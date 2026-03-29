Output outputs/plugin_mode/plugin_mode.gif
Output outputs/plugin_mode/plugin_mode.txt

Set Shell "bash"
Set Width 1800
Set Height 1200
Set FontSize 16
Set WaitTimeout 60s

# ============================
# Plugin Management E2E Test
# ============================
# entrypoint.sh が事前に example-plugin をインストール済み

# 1. 起動: Sessions / Plugins / Logs の3パネル表示
Ctrl+\
Wait+Screen /No sessions/
Sleep 2s

# 2. Tab → Plugins にフォーカス (Installed タブ)
#    右パネルがプラグイン詳細に変化
#    example-plugin が [E] で表示されるはず
Tab
Sleep 3s

# 3. e で disable → [D] に変わる
Type "e"
Sleep 3s

# 4. e で enable → [E] に戻る
Type "e"
Sleep 3s

# 5. ] で Marketplace タブに切替
#    マーケットプレイスのプラグイン一覧が表示される
Type "]"
Sleep 3s

# 6. j で下にナビゲーション
Type "j" Sleep 500ms
Type "j" Sleep 500ms
Type "j" Sleep 500ms

# 7. i でインストール (カーソル位置のプラグイン)
Type "i"
Sleep 5s

# 8. [ で Installed タブに戻る
#    新しくインストールしたプラグインが追加されている
Type "["
Sleep 3s

# 9. d でアンインストール (カーソル位置のプラグイン)
Type "d"
Sleep 3s

# 10. Tab → Sessions に戻り、右パネルがセッションプレビューに復帰
Tab
Sleep 1s
Tab
Sleep 2s

# 11. 終了
Type "q" Sleep 1s
