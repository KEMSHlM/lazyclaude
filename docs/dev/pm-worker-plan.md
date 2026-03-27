# Implementation Plan: PM-Worker System

## Overview

lazyclaude に PM (Project Manager) と Worker のロールシステムを導入する。
PM は Worker の PR をレビューする Claude Code セッション、Worker は既存の worktree 機能を拡張した Claude Code セッション。
MCP サーバーに REST API を追加して PM-Worker 間の非同期メッセージングを実現し、起動時のシステムプロンプト注入でロールと API の存在を Claude Code に通知する。

## Requirements

1. PM モード: Worker の PR レビューを行う Claude Code セッション
2. Worker モード: 既存 worktree 機能を使用した Claude Code セッション
3. main preview から PM/Worker モードの pane を起動
4. PM-Worker 間の通信 API (MCP サーバーに REST エンドポイント追加)
5. 起動時システムプロンプト注入 (ロール + API 情報)
6. セッション状態の可視化 (Pending 検出)

---

## Session Activity (状態の可視化)

### 状態モデル

| 状態 | 意味 | 検出方法 |
|------|------|----------|
| **Running** | 動作中。prompt 送信可能 | pane alive + PID > 0 (既存 SyncWithTmux) |
| **Pending** | 選択肢待ち。ユーザー操作が必要でブロック中 | PendingNotifications() の window マッチ (既存) |
| **Dead** | pane 死亡 | PaneInfo.Dead (既存) |
| **Orphan** | tmux window 消失 | 既存ロジック |

### 設計判断

- **Idle vs Busy の区別は不要**: Busy 中も prompt 送信可能であり、ブロック状態ではない
- **新しい hook は不要**: Pending の検出は既存の `PendingNotifications()` で実現済み
- **追加コスト最小**: SessionItem にフィールド追加 + render 変更のみ

### 実装

#### SessionItem に Activity 追加

**変更ファイル**: `internal/gui/app.go` (L58-67)

```go
type SessionItem struct {
    // ... 既存フィールド ...
    Activity string // "pending" or "" (empty = normal)
}
```

#### sessionAdapter で pending 判定

**変更ファイル**: `cmd/lazyclaude/root.go`

`Sessions()` メソッド (L228-243) で、PendingNotifications の window と各セッションの TmuxWindow を照合:

```go
func (a *sessionAdapter) Sessions() []gui.SessionItem {
    sessions := a.mgr.Sessions()
    pending := a.pendingWindowSet() // PendingNotifications() → map[window]bool

    items := make([]gui.SessionItem, len(sessions))
    for i, s := range sessions {
        activity := ""
        if s.Status == session.StatusRunning && pending[s.TmuxWindow] {
            activity = "pending"
        }
        items[i] = gui.SessionItem{
            // ... 既存 ...
            Activity: activity,
        }
    }
    return items
}

func (a *sessionAdapter) pendingWindowSet() map[string]bool {
    notifications := a.PendingNotifications()
    set := make(map[string]bool, len(notifications))
    for _, n := range notifications {
        set[n.Window] = true
    }
    return set
}
```

#### アイコン表示

**変更ファイル**: `internal/gui/presentation/style.go`

```go
IconPending = "\x1b[35m◆\x1b[0m" // magenta diamond (choice waiting)
```

**変更ファイル**: `internal/gui/render.go`

`renderSessionList` の icon 判定を拡張:

```go
var icon string
switch {
case item.Status == "Dead":
    icon = " " + presentation.IconDead
case item.Status == "Orphan":
    icon = " " + presentation.IconOrphan
case item.Activity == "pending":
    icon = " " + presentation.IconPending
case item.Status == "Running":
    icon = " " + presentation.IconRunning
case item.Status == "Detached":
    icon = " " + presentation.IconDetached
}
```

---

## Architecture

```
┌──────────────────────────────────────────────┐
│              lazyclaude TUI                   │
│                                              │
│  ┌──────┐   ┌────────┐   ┌────────────────┐  │
│  │ Main │   │ PM     │   │ Worker (x N)   │  │
│  │ Pane │   │ [PM]   │   │ [W] worktree   │  │
│  └──┬───┘   └───┬────┘   └───┬────────────┘  │
│     │           │             │               │
│     │    ┌──────▼─────────────▼──────┐        │
│     │    │     MCP Server            │        │
│     │    │  既存: /, /notify         │        │
│     │    │  新規: /msg/send          │        │
│     │    │        /msg/poll          │        │
│     │    │        /msg/sessions      │        │
│     │    └───────────────────────────┘        │
│     │                                        │
│     ├── [P] PM pane 起動                      │
│     └── [w] Worker pane 起動 (worktree)       │
└──────────────────────────────────────────────┘
```

---

## Design Decisions

### 1. メッセージストア: ファイルベース

- 既存パターンとの一貫性 (`notify.Enqueue`, `choice.ReadFile` がファイルベース)
- MCP サーバー再起動でもメッセージが残る
- 場所: `{RuntimeDir}/lazyclaude-messages/{id}.json`

### 2. Role の表現

- `Session.Role` フィールドを追加 (`json:"role,omitempty"` で後方互換)
- 空文字列 `""` = 従来のロールなしセッション

### 3. 通信方式

- Claude Code の Bash ツールから `curl` で REST API を呼ぶ
- ポーリング方式 (PM/Worker が定期的に `/msg/poll` を呼ぶ)
- プロンプト内に curl コマンド例を埋め込み

### 4. プロンプト注入

- 既存 `writeWorktreeLauncher()` パターンを再利用
- `--append-system-prompt` でロール情報 + API 情報を注入
- API ドキュメントは `docs/api/pm-worker-api.md` に配置し、プロンプトからパスを参照

### 5. Worker 起動: 既存 worktree ダイアログの拡張

- 新規キー `P` = PM 起動
- Worker 起動は既存の `w` (worktree dialog) を拡張して role=worker で起動

---

## Implementation Phases

### Phase 1: Session Activity (Pending 検出 + 表示)

新しい hook やエンドポイントは不要。既存の仕組みを UI に反映するだけ。

| File | Change |
|------|--------|
| `internal/gui/app.go` | `SessionItem` に `Activity string` 追加 |
| `internal/gui/presentation/style.go` | `IconPending` 追加 |
| `internal/gui/render.go` | `renderSessionList` で Activity に基づくアイコン切り替え |
| `cmd/lazyclaude/root.go` | `sessionAdapter.Sessions()` で pending 判定 |

### Phase 2: Role 型とセッション拡張

| File | Change |
|------|--------|
| `internal/session/role.go` (新規) | `Role` 型定義 (`RoleNone`, `RolePM`, `RoleWorker`) |
| `internal/session/store.go` | `Session` に `Role Role` フィールド追加 |
| `internal/session/role.go` | `BuildPMPrompt()`, `BuildWorkerPrompt()` プロンプトビルダー |

### Phase 3: メッセージ API

| File | Change |
|------|--------|
| `internal/server/message.go` (新規) | `Message` 型 + `MessageStore` (ファイルベース永続化) |
| `internal/server/handler_msg.go` (新規) | `/msg/send`, `/msg/poll`, `/msg/sessions` ハンドラ |
| `internal/server/server.go` | `msgStore` フィールド追加 + mux エンドポイント登録 |

**エンドポイント:**

| Method | Path | Description |
|--------|------|-------------|
| `POST /msg/send` | メッセージ送信 | `{"from":"<id>","to":"<id>","type":"review_request","body":"..."}` |
| `GET /msg/poll?session=<id>` | 未読メッセージ取得 | 宛先が一致するメッセージを返す |
| `GET /msg/sessions` | セッション一覧 | PM/Worker が互いを発見する用 |

**Message 構造:**

```go
type Message struct {
    ID        string    `json:"id"`
    From      string    `json:"from"`
    To        string    `json:"to"`
    Type      string    `json:"type"`  // "review_request", "review_response", "status", "done"
    Body      string    `json:"body"`
    CreatedAt time.Time `json:"created_at"`
    Read      bool      `json:"read"`
}
```

### Phase 4: PM/Worker セッション起動

| File | Change |
|------|--------|
| `internal/session/manager.go` | `CreatePMSession()` 追加, `launchWorktreeSession()` に role 引数追加, `CreateWorkerSession()` 追加 |
| `internal/gui/app.go` | `SessionProvider` に `CreatePMSession()`, `CreateWorkerSession()` 追加 |
| `cmd/lazyclaude/root.go` | `sessionAdapter` に PM/Worker メソッド実装, `Sessions()` で Role マッピング |

**CreatePMSession:**
- worktree なし、projectRoot で動作
- `sess.Role = RolePM`, `sess.Name = "pm"`
- `BuildPMPrompt()` → `writeWorktreeLauncher()` → tmux window 作成
- 同一プロジェクトに PM は 1 つのみ (重複チェック)

**CreateWorkerSession:**
- 既存 `CreateWorktree` と同ロジック + `role = RoleWorker`
- `BuildWorkerPrompt()` で worktree 隔離 + API 情報を注入

**launchWorktreeSession の拡張:**
- `role Role` 引数を追加
- `role == RoleWorker`: `BuildWorkerPrompt()` を使用
- `role == RoleNone`: 既存の `BuildWorktreePrompt()` を使用 (後方互換)

### Phase 5: GUI 統合

| File | Change |
|------|--------|
| `internal/gui/keyhandler/actions.go` | `AppActions` に `StartPMSession()` 追加 |
| `internal/gui/app_actions.go` | `StartPMSession()` 実装 |
| `internal/gui/keyhandler/sessions.go` | `P` キーハンドリング + OptionsBar 更新 |
| `internal/gui/keybindings.go` | runes に `'P'` 追加 |
| `internal/gui/render.go` | セッション名に [PM] / [W] バッジ表示 |

### Phase 6: API ドキュメント + テスト

| File | Change |
|------|--------|
| `docs/api/pm-worker-api.md` (新規) | Claude Code 向け API ドキュメント |
| `internal/session/role_test.go` (新規) | Role 型 + プロンプトビルダーのテスト |
| `internal/server/message_test.go` (新規) | MessageStore のテスト |
| `internal/server/handler_msg_test.go` (新規) | API エンドポイントのテスト |
| `internal/session/store_test.go` | Role フィールドの後方互換テスト |

---

## File Change Summary

### 新規ファイル (5 + テスト 3)

| File | Lines (est.) | Purpose |
|------|-------------|---------|
| `internal/session/role.go` | ~80 | Role 型 + プロンプトビルダー |
| `internal/server/message.go` | ~150 | MessageStore (ファイルベース) |
| `internal/server/handler_msg.go` | ~120 | REST API ハンドラ |
| `docs/api/pm-worker-api.md` | ~80 | Claude Code 向け API ドキュメント |
| テストファイル (3) | ~300 | role_test, message_test, handler_msg_test |

### 変更ファイル (10)

| File | Changes |
|------|---------|
| `internal/session/store.go` | Session に `Role` フィールド追加 |
| `internal/session/manager.go` | `CreatePMSession`, `CreateWorkerSession`, `launchWorktreeSession` に role 引数追加 |
| `internal/server/server.go` | `msgStore` フィールド + mux エンドポイント登録 |
| `internal/gui/app.go` | `SessionProvider` に 2 メソッド追加, `SessionItem` に Activity/Role 追加 |
| `internal/gui/app_actions.go` | `StartPMSession()` 実装 |
| `internal/gui/presentation/style.go` | `IconPending` 追加 |
| `internal/gui/render.go` | Activity ベースのアイコン切り替え + Role バッジ |
| `internal/gui/keyhandler/actions.go` | `AppActions` に `StartPMSession()` 追加 |
| `internal/gui/keyhandler/sessions.go` | `P` キーハンドリング + OptionsBar 更新 |
| `cmd/lazyclaude/root.go` | `sessionAdapter` に PM/Worker メソッド + Activity/Role マッピング |

---

## Risks & Mitigations

| Risk | Severity | Mitigation |
|------|----------|------------|
| プロンプト内 auth token の露出 | Medium | launcher スクリプト自己削除。将来的にファイルベースのトークン渡しを検討 |
| MCP サーバー未起動時の PM/Worker 作成 | Medium | `readMCPInfo()` 失敗時に明確なエラーメッセージ |
| メッセージファイル肥大化 | Low | セッション削除時にメッセージも削除 |
| `launchWorktreeSession` の role 引数追加 | Low | RoleNone をデフォルトとし既存動作を維持 |
| `P` キーの衝突 | Low | 大文字 `P` は KeyRegistry に未登録 (確認済み) |

---

## Implementation Order

```
Phase 1 (Activity)  →  Phase 2 (Role)  →  Phase 3 (API)  →  Phase 4 (起動)  →  Phase 5 (GUI)  →  Phase 6 (Docs+Test)
```

Phase 1 は独立して完了・マージ可能。Phase 2-5 は PM/Worker 機能のコア。
各 Phase 完了時にユニットテストを追加。

## Success Criteria

- [ ] Pending セッションにマゼンタ ◆ アイコンが表示される
- [ ] `Session.Role` フィールドが追加され、既存 state.json との後方互換性維持
- [ ] PM セッションが `P` キーで起動、PM プロンプト確認可能
- [ ] Worker セッションが worktree ダイアログから起動、Worker プロンプト確認可能
- [ ] `/msg/send` と `/msg/poll` が正しく動作 (curl テスト)
- [ ] PM/Worker 間で curl 経由のメッセージ送受信が可能
- [ ] セッションリストに [PM] / [W] バッジ表示
- [ ] ユニットテスト 80%+ カバレッジ
