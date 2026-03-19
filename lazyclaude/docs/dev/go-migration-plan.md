# lazyclaude — Standalone Claude Code TUI

**作成日**: 2026-03-17
**改訂日**: 2026-03-20 (v5: P6 SSH 完了、E2E 全フロー検証済み)

---

## コンセプト

lazygit が git の standalone TUI であるように、**lazyclaude** は Claude Code の standalone TUI。

### 2 層アーキテクチャ

| 層                          | 何ができるか                                                            | tmux 必要? |
| --------------------------- | ----------------------------------------------------------------------- | ---------- |
| **lazyclaude (standalone)** | セッション管理 TUI、diff/tool ビューア、MCP サーバー                    | Yes        |
| **lazyclaude.tmux (拡張)**  | keybind 登録、作業中ペインへの自動 popup 割り込み、MCP サーバー自動起動 | TPM        |

### popup の実現方式 (設計変更)

当初計画では `tmux display-popup` で別プロセスとして popup を表示する予定だった。
実装では **gocui オーバーレイ popup** を採用:

```
Claude Code → MCP server → notification queue (file-based)
                                    ↓
                        GUI ticker (100ms) polling
                                    ↓
                        PopupController.Push() → cascade overlay
                                    ↓
                        User choice → choice file → MCP server → Claude Code
```

---

## アーキテクチャ概要

### 起動モデル

```
lazyclaude                    # メイン TUI (セッション一覧 + プレビュー + full-screen)
lazyclaude server             # MCP サーバーデーモン
lazyclaude diff <args>        # diff ビューア (サブプロセスモード)
lazyclaude tool <args>        # tool 確認ビューア (サブプロセスモード)
lazyclaude setup              # tmux keybind + hook 登録 (未実装)
```

### 全体構成図 (ローカル + リモート)

```
┌─────────────────────────────────────────────────────────────────────┐
│  lazyclaude (local)                                                 │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌───────────────────────────┐  │
│  │  Main TUI    │  │  MCP Server  │  │  Popup Viewers            │  │
│  │  (gocui)     │  │  (WS + HTTP) │  │  lazyclaude diff/tool     │  │
│  │  session mgr │  │  /notify     │  │  (gocui subprocess)       │  │
│  └──────┬───────┘  └──────┬───────┘  └───────────────────────────┘  │
│         │                 │                                         │
│  ┌──────┴─────────────────┴──────────────────────────────────────┐  │
│  │  Core: tmux client | config (env overrides) | notify queue    │  │
│  │        EnsureServer (TCP health check) | pending-window       │  │
│  └───────────────────────────────────────────────────────────────┘  │
└──────────┬─────────────────┬────────────────────────────────────────┘
           │                 │
     ┌─────┴─────┐    ┌─────┴──────────────────────────────────────┐
     │   tmux    │    │  SSH reverse tunnel (-R PORT:localhost:PORT) │
     └───────────┘    └─────┬──────────────────────────────────────┘
                            │
                    ┌───────┴───────────────────────────┐
                    │  remote host                       │
                    │  Claude Code → hooks → /notify     │
                    │  reads ~/.claude/ide/PORT.lock      │
                    │  connects to ws://localhost:PORT    │
                    └────────────────────────────────────┘
```

---

## 技術スタック

| ライブラリ             | 用途                                   |
| ---------------------- | -------------------------------------- |
| `jesseduffield/gocui`  | TUI フレームワーク (lazygit 同一 fork) |
| `nhooyr.io/websocket`  | MCP WebSocket サーバー                 |
| `spf13/cobra`          | CLI サブコマンド                       |
| `charmbracelet/x/ansi` | ANSI カラー処理                        |
| stdlib                 | HTTP, JSON, exec, crypto, net          |

---

## パッケージ設計 (現在の実装)

```
lazyclaude/
├── cmd/lazyclaude/
│   ├── main.go                 エントリポイント
│   ├── root.go                 cobra root (引数なし → メイン TUI)
│   ├── server.go               `lazyclaude server` サブコマンド
│   ├── diff.go                 `lazyclaude diff` サブコマンド
│   ├── tool.go                 `lazyclaude tool` サブコマンド
│   └── setup.go                `lazyclaude setup` (未実装 placeholder)
│
├── cmd/mock-claude-client/
│   └── main.go                 E2E テスト用 Claude Code シミュレーター
│
├── internal/
│   ├── core/
│   │   ├── tmux/
│   │   │   ├── client.go       interface Client (9 methods)
│   │   │   ├── types.go        ClientInfo, WindowInfo, PaneInfo
│   │   │   ├── exec.go         ExecClient (subprocess)
│   │   │   ├── control.go      ControlClient (persistent socket)
│   │   │   ├── pidwalk.go      PID → window 解決
│   │   │   └── mock.go         テスト用 MockClient
│   │   └── config/
│   │       └── config.go       Paths (env override 対応)
│   │
│   ├── server/
│   │   ├── server.go           HTTP/WebSocket + /notify (pending-window fallback)
│   │   ├── handler.go          MCP リクエストハンドラ (ide_connected pending-window)
│   │   ├── ensure.go           EnsureServer (TCP health check + auto-start)
│   │   ├── jsonrpc.go          JSON-RPC 2.0 プロトコル
│   │   ├── state.go            接続状態, PID→Window マッピング
│   │   └── lock.go             IDE lock ファイル管理
│   │
│   ├── gui/
│   │   ├── choice/             Choice 型 + ファイル I/O
│   │   ├── keymap/             AppState, KeyAction, KeyBinding, Registry
│   │   ├── presentation/       diff/tool/session フォーマット
│   │   ├── app.go              App struct, NewApp, Run, event loop
│   │   ├── state.go            transition(), enterFullScreen, forwardKey
│   │   ├── keybindings.go      gocui キー登録 (state-aware)
│   │   ├── layout.go           レイアウト + SideTab/TabBar
│   │   ├── popup.go            popup rendering + App 委譲
│   │   ├── popup_controller.go PopupController (独立テスト可能)
│   │   └── input.go            InputForwarder + inputEditor
│   │
│   ├── session/
│   │   ├── store.go            JSON 永続化 (state.json)
│   │   ├── manager.go          セッション CRUD + tmux 同期 + SSH session
│   │   ├── ssh.go              SSH コマンド構築 (reverse tunnel + lock file)
│   │   └── gc.go               バックグラウンド GC (orphan 検出)
│   │
│   └── notify/
│       └── notify.go           通知キュー (file-based, timestamp 順)
│
├── tests/
│   ├── cli/                    CLI 出力テスト (termtest)
│   ├── integration/
│   │   ├── scripts/            E2E bash スクリプト
│   │   │   ├── verify_popup_stack.sh      popup スタック検証
│   │   │   ├── verify_remote_popup.sh     リモート Claude → popup 全フロー
│   │   │   ├── verify_key_order.sh        キー順序検証
│   │   │   └── measure_latency.sh         レイテンシ計測
│   │   ├── fullscreen_test.go  フルスクリーン + popup E2E
│   │   ├── popup_test.go       diff/tool サブコマンド E2E
│   │   ├── server_test.go      MCP サーバー E2E (binary lifecycle)
│   │   ├── ssh_test.go         SSH E2E (mock client + tunnel)
│   │   └── tmux_test.go        tmux ヘルパー
│   └── testdata/               テスト用データファイル
│
├── Dockerfile.test             Docker テスト環境
├── Dockerfile.ssh-test         SSH 2 コンテナテスト (keygen → remote/local)
├── docker-compose.ssh.yml      SSH テスト orchestration
├── Makefile
├── go.mod
└── go.sum
```

---

## Phase 進捗一覧

| Phase | 目標 | 状態 | 備考 |
|-------|------|------|------|
| **P0** | プロジェクト初期化 + PoC | **完了** | go mod, cobra, Dockerfile.test |
| **P1** | コア層 (tmux + config) | **完了** | Client interface, ExecClient, ControlClient, MockClient, pidwalk |
| **P2** | MCP サーバー | **完了** | WebSocket, JSON-RPC, handler, state, lock, pending-window |
| **P3** | TUI フレームワーク | **完了** | App, layout, keybindings, state machine, KeyRegistry |
| **P4** | メイン画面 | **完了** | セッション一覧, プレビュー, full-screen, cursor sync, MCP 自動起動 |
| **P5** | Diff / Tool Popup | **完了** | diff.go, tool.go, popup stack, cascade, dismiss |
| **P5+** | Popup 拡張 | **完了** | 通知キュー, popup stack, suspend/reopen, Y accept-all |
| **P6** | SSH + リモートセッション | **完了** | SSH コマンド構築, reverse tunnel, Docker 2 コンテナ E2E, 実 Claude popup 検証 |
| **P7** | lazyclaude.tmux 拡張 | **未着手** | setup.go は placeholder |
| **P8** | 配布・CI/CD | **未着手** | goreleaser, GitHub Actions なし |

---

## Phase 6: SSH + リモートセッション (完了)

| # | タスク | 状態 |
|---|--------|------|
| P6.1 | SSH reverse tunnel 構築 | **完了** |
| P6.2 | リモート lock ファイル管理 | **完了** |
| P6.3 | Docker SSH テスト基盤 | **完了** |
| P6.4 | pending-window (リモート PID→window 解決) | **完了** |
| P6.5 | 実 Claude Code リモート popup E2E | **完了** |

### SSH 実装の詳細

```
lazyclaude (local)                          remote host
┌─────────────────────┐                    ┌──────────────────────┐
│ MCP Server :PORT    │◄── SSH -R PORT ────│ Claude Code          │
│  handleNotify:      │    reverse tunnel   │ hooks → /notify      │
│  pending-window     │                    │ reads ~/.claude/ide/ │
│  fallback           │                    │ connects to :PORT    │
│                     │                    │                      │
│ GUI polls notify    │                    │                      │
└─────────────────────┘                    └──────────────────────┘
```

- `buildSSHCommand()`: SSH + PTY + reverse tunnel + keepalive
- `buildRemoteCommand()`: lock file 作成 → claude 起動 → trap で lock 削除
- `splitHostPort()`: `user@host:port` を分離 (IPv6 対応)
- `Manager.readMCPInfo()`: port file + lock file から MCP 接続情報を取得
- `pending-window`: `ide_connected` / `handleNotify` 両方でリモート PID を解決
- `verify_remote_popup.sh`: 実 Claude Code でツール実行 → popup 表示まで検証

---

## Phase 7: lazyclaude.tmux 拡張 (次の開発)

| # | タスク | 優先度 | 成果物 |
|---|--------|--------|--------|
| P7.1 | lazyclaude.tmux エントリポイント | 高 | `lazyclaude.tmux` (shell, TPM 用) |
| P7.2 | `lazyclaude setup` サブコマンド | 高 | `cmd/lazyclaude/setup.go` |
| P7.3 | server --ensure (idempotent 起動) | 中 | `cmd/lazyclaude/server.go` 拡張 |
| P7.4 | Claude hooks 自動設定 | 高 | `internal/core/config/hooks.go` |

### P7 の設計方針

現在の JS 版 `tmux-claude.tmux` が行っていること:
1. TPM プラグインとして tmux にロードされる
2. keybind を登録 (Prefix + C で Claude popup 起動)
3. MCP サーバーを起動 (`mcp-server.js`)
4. Claude Code hooks を `~/.claude/settings.json` に設定

Go 版では:
- `lazyclaude.tmux` shell スクリプト → `lazyclaude setup` を呼ぶ
- `lazyclaude setup` が keybind 登録 + hooks 設定 + server 起動を一括実行
- `EnsureServer` は既に実装済み

---

## Phase 8: 配布・CI/CD (未着手)

| # | タスク | 成果物 |
|---|--------|--------|
| P8.1 | goreleaser 設定 | `.goreleaser.yml` |
| P8.2 | GitHub Actions CI | `.github/workflows/ci.yml` |
| P8.3 | GitHub Actions release | `.github/workflows/release.yml` |
| P8.4 | golangci-lint 設定 | `.golangci.yml` |
| P8.5 | Homebrew Formula | `homebrew-lazyclaude/` |
| P8.6 | README.md | standalone + tmux 拡張 両方 |

---

## 将来の機能 (P9+)

| 機能 | 備考 |
|------|------|
| 設定ファイル (~/.config/lazyclaude/config.toml) | テーマ、keybind override |
| ヘルプ popup (?) | keybind 一覧表示 |
| Keybinding ユーザーカスタマイズ | `keymap.Registry.LoadOverrides()` |
| Visual mode (V) | フルスクリーンでテキスト選択 |

---

## テスト戦略 (現在の実装)

| 層 | 対象 | 手法 | 実行環境 |
|----|------|------|----------|
| L1: Unit | core, server, session, gui | `go test` + mock | Host + Docker |
| L2: GUI headless | TUI 状態遷移 + keybinding | gocui headless (SimulationScreen) | Host + Docker |
| L3: CLI | サブコマンド出力 | termtest (PTY emulation) | Docker |
| L4: Integration | tmux popup + MCP + choice | tmux scripting (send-keys + capture-pane) | Docker |
| L5: E2E scripts | popup stack, IME, latency | bash (tmux + capture-pane) | Docker |
| L6: Server E2E | MCP binary lifecycle + WS + notify | Go binary + WebSocket client | Host + Docker |
| L7: SSH E2E | SSH tunnel + mock client | docker-compose 2 コンテナ | Docker only |
| L8: Remote Claude E2E | 実 Claude Code → popup 表示 | docker-compose + .env (OAuth) | Docker only |

### 実行コマンド

```bash
make test          # ユニットテスト (ホスト)
make test-unit     # internal/ のみ
make test-e2e      # TUI + Server E2E (Docker)
make test-ssh      # SSH mock E2E (Docker Compose)
make test-ssh-e2e  # 実 Claude popup E2E (Docker Compose + .env)
```

### テストカバレッジ (2026-03-20 v3)

| パッケージ | カバレッジ |
|-----------|-----------|
| core/config | 93.8% |
| gui/keymap | 91.7% |
| gui/presentation | 91.6% |
| gui/choice | 90.9% |
| server | 84.2% |
| session | 80.3% |
| notify | 78.8% |
| gui (main) | 43.8% |
| core/tmux | 35.5% |

---

## コードメトリクス (2026-03-20 v3)

| 指標 | 値 |
|------|-----|
| Go ソースファイル数 | 43 |
| Go テストファイル数 | 37 |
| 総テスト関数数 | 276 |
| TUI E2E テスト数 | 12 (Go) + 9 (bash) |
| Server E2E テスト数 | 6 (Go binary) |
| SSH E2E テスト数 | 6 (Docker Compose) |
| Remote Claude E2E | 1 (verify_remote_popup.sh) |

---

## リスクと対策

| リスク | 影響 | 状態 | 対策 |
|--------|------|------|------|
| gocui が tmux display-popup で動作しない | P5 | **解決** | display-popup は不使用、gocui overlay で実装 |
| Claude Code WebSocket 互換性 | P2 | **解決** | MCP サーバー動作確認済み |
| SSH reverse tunnel タイミング | P6 | **解決** | buildSSHCommand + Docker 2 コンテナ E2E |
| リモート PID→window 解決 | P6 | **解決** | pending-window file (ide_connected + handleNotify) |
| Claude Code install.sh Cloudflare block | P6 | **解決** | GCS 直接ダウンロード (arch 自動検出) |
| goroutine リーク | P2 | **対策済み** | context.Context + done channel パターン |
| メイン TUI ↔ full-screen 切り替え | P4 | **解決** | AppState state machine + transition() |
| IME 入力の順序破壊 | P4 | **解決** | serial key forwarding queue (buffered channel) |
| Popup の notification loss | P5 | **解決** | file-based queue (nanosecond timestamp) |
