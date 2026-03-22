---
name: feedback_vis_e2e_philosophy
description: vis_e2e_tests design philosophy - tape is human operations only, everything else is infrastructure
type: feedback
---

vis_e2e_tests の設計哲学。

## 原則

1. **tape は人間の操作のみ**: Type, Enter, Sleep, Ctrl。テスト都合（env var, mock data, setup）は一切書かない
2. **テスト都合は entrypoint.sh**: セットアップ、サービス起動、環境変数設定は全て entrypoint の case 文
3. **出力は agent が読める形式**: gif は人間用、.log はプレーンテキストで agent が読む。画面クリアは stderr、内容は stdout
4. **責務の分離**: entrypoint.sh（ライフサイクル）、show_frame.sh（表示ロジック）、watch_frames.sh（監視ループ）
5. **フレーム取得はコンテキスト最小化**: `awk '/\[Frame N\]/,/\[Frame M\]/{if(/\[Frame M\]/)exit; print}'` で該当フレームだけ抽出。Read ツールで全体を読まない

## Docker 構成

- remote (sshd) + vhs の2コンテナ。同一ネットワーク
- vhs コンテナの bash ラッパーで `VHS_AUTO_TMUX` 時に自動 `tmux attach`
- tmux セッションは entrypoint で事前作成（VHS ターミナルサイズに合わせる）

## VHS の制約

- `Set Shell` は固定名のみ（bash, zsh 等）。任意パス不可
- `Output .txt` は公式機能。各コマンド後のターミナル全画面スナップショットを `────` 区切りで出力
- コマンドの exit code 検知不可（Issue #653 未解決）
- `Type` + `Enter` + `Sleep` を1行にまとめるとスナップショット数が減る

**Why:** テストの信頼性と可読性。tape に都合を混ぜるとデバッグが困難になり、実際の操作と乖離する。agent が結果を読めないと自動修正ができない。

**How to apply:** 新しい tape を作るとき、操作以外を tape に書こうとしたら entrypoint に移す。ログ確認時は awk でフレーム抽出。
