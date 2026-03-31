# 可視化 E2E テストカタログ

各 tape は人間の TUI 操作のみを記録する。
テスト都合（環境変数、モックデータ、サービス起動）は `entrypoint.sh` が処理する。

出力: `outputs/{name}/` に `.gif` + `.txt` + `.log`。

## テープ一覧

### smoke
基本的なシェル動作確認。Docker 環境が正しくセットアップされているか検証する。
- 前提: なし
- 操作: pwd, ls, echo, cat (エラーケース)
- 期待: 各コマンドの出力が正しく表示される

### hero
README 用デモ GIF。lazyclaude の主要機能を一連のフローで紹介する。
- 前提: Claude Code OAuth トークン (`vis_e2e_tests/.env`)
- 操作: Ctrl+\ で起動 → `n` でセッション作成 → Enter でフルスクリーン → Claude にファイル作成を依頼 → Ctrl+\ でフルスクリーン終了 → permission popup を Ctrl+y で承認 → `q` で終了
- 期待: セッション一覧、ライブプレビュー、フルスクリーン、permission popup の各機能が動作
- 必須: Claude Code トークン
- 更新: `make readme-gif` で GIF を生成し `docs/images/hero.gif` にコピー
