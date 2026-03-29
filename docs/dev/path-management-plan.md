# Path Management Refactoring Plan

## 要件の整理

### 現状の問題

1. **`filepath.Abs(".")` の散在**: パス解決が 5 箇所で個別に `filepath.Abs(".")` を呼んでいる
   - `app_actions.go:311` (StartPMSession)
   - `app_actions.go:345` (SelectWorktree)
   - `keybindings.go:149` (worktree confirm)
   - `keybindings.go:294` (worktree resume)
   - `cmd/lazyclaude/root.go:397` (sessionAdapter.Create)

2. **CWD の意味が曖昧**: lazyclaude は `display-popup -d "${PANE_CWD:-.}"` で起動されるため、プロセスの CWD = ユーザーのペインの CWD。しかし worktree 内セッションからの操作では CWD が worktree パスになる可能性がある。

3. **Project root の推定が毎回**: `session.InferProjectRoot(abs)` が各アクションで毎回呼ばれる。

4. **`n` (new session) のパスが常に `.`**: SSH 以外は `path := "."` がハードコードされており、カーソル位置のプロジェクトを考慮しない。

5. **Plugin パス判定が worktree 非対応**: `syncPluginProject()` がセッションの `Path` をそのまま使うが、worktree パスだと plugin 設定が project root のものと一致しない。

### 目標

- **親 (Project) にパス管理責務を集約**: Project が `projectRoot` を持ち、子セッションは親のパスを参照する
- **`n` キーの動作変更**: 現在のカーソル位置のプロジェクトの `projectRoot` で新セッションを作成
- **CWD 移動キーの新設**: 別のディレクトリで新セッションを作る機能 (現在の `n` の代替)
- **不要な重複コードの削除**
- **Plugin パスの worktree 対応**

---

## 現在のキーマッピング (Sessions パネル)

| Key | Action | 説明 |
|-----|--------|------|
| `n` | CreateSession | 新セッション (CWD) |
| `d` | DeleteSession | セッション削除 |
| `a` | AttachSession | tmux attach |
| `g` | LaunchLazygit | lazygit 起動 |
| `enter` | EnterFullScreen / ToggleProject | フルスクリーン |
| `r` | EnterFullScreen | フルスクリーン |
| `R` | StartRename | 名前変更 |
| `w` | StartWorktreeInput | worktree 作成 |
| `W` | SelectWorktree | worktree 選択 |
| `D` | PurgeOrphans | orphan 削除 |
| `P` | StartPMSession | PM セッション |
| `h/l` | Collapse/Expand | ツリー折りたたみ |
| `1/2/3` | SendKeyToPane | キー送信 |

---

## 設計方針

### 1. Project がパスの SSOT (Single Source of Truth)

現在の `Session.Path` はそのまま維持するが、`projectRoot` の算出は `Project.Path` から行う。

GUI 層では `filepath.Abs(".")` の代わりに、**カーソル位置のノードから Project を辿って** `projectRoot` を取得する。

```
currentProjectRoot() string
  -> currentNode() から ProjectNode/SessionNode を判定
  -> ProjectNode: node.Project.Path
  -> SessionNode: InferProjectRoot(node.Session.Path)
  -> fallback: filepath.Abs(".")  (セッションが 0 のとき)
```

### 2. `n` キーの動作変更

**現在**: CWD (`"."`) で新セッション作成
**変更後**: カーソル位置のプロジェクトの `projectRoot` で新セッション作成

これにより、複数プロジェクトを管理している場合に正しいプロジェクト配下にセッションが作られる。

### 3. CWD で新セッションを作る機能 — キー相談

新しい CWD でセッションを作る機能 (従来の `n` 相当) にキーを割り当てる必要がある。

**候補:**
- `N` (Shift+n): "New session at CWD" — `n` と対になり直感的
- `c` (create): 未使用だが lazygit では `c` = commit
- `o` (open): 未使用

**提案: `N`** — `n` = "new in project", `N` = "new at CWD" が最も自然。

### 4. Plugin パスの worktree 対応

`syncPluginProject()` で `InferProjectRoot()` を適用:

```go
// Before
projectPath = node.Session.Path

// After
projectPath = InferProjectRoot(node.Session.Path)
```

### 5. filepath.Abs(".") の集約

`currentProjectRoot()` メソッドに集約。各アクション (`StartPMSession`, `SelectWorktree`, worktree confirm/resume) はこのメソッドを使う。

---

## 実装フェーズ

### Phase 1: `currentProjectRoot()` の導入

**ファイル**: `internal/gui/app_actions.go`

- `currentProjectRoot() string` メソッドを追加
- カーソル位置から Project パスを取得、fallback は `filepath.Abs(".")`
- `StartPMSession`, `SelectWorktree` の `filepath.Abs(".")` + `InferProjectRoot` を置換

### Phase 2: `n` キーの動作変更

**ファイル**: `internal/gui/app_actions.go`

- `CreateSession()` を変更: `path := "."` の代わりに `currentProjectRoot()` を使用
- SSH の場合はそのまま (`DetectRemotePath()` を優先)
- `sessionAdapter.Create` の `filepath.Abs(".")` も不要に (絶対パスが渡される)

### Phase 3: `N` キーの追加 (CWD で新セッション)

**ファイル**:
- `internal/gui/keyhandler/actions.go` — `CreateSessionAtCWD()` 追加
- `internal/gui/keyhandler/sessions.go` — `'N'` ハンドラ追加
- `internal/gui/app_actions.go` — `CreateSessionAtCWD()` 実装
- `internal/gui/keybindings.go` — `'N'` をルーンリストに追加

### Phase 4: Plugin パスの worktree 対応

**ファイル**: `internal/gui/app_actions.go`

- `syncPluginProject()` で `InferProjectRoot()` を適用

### Phase 5: worktree dialog のパス集約

**ファイル**: `internal/gui/keybindings.go`

- worktree confirm (line 149) と worktree resume (line 294) の `filepath.Abs(".")` を `currentProjectRoot()` に置換
- ただし keybindings.go のハンドラは `*gocui.Gui, *gocui.View` シグネチャなので、`a.currentProjectRoot()` として呼ぶ

### Phase 6: 不要コードの削除

- `sessionAdapter.Create` の `filepath.Abs(".")` 分岐を削除 (Phase 2 で絶対パスが渡されるため)
- 各所の `filepath.Abs(".")` + `InferProjectRoot()` の重複パターンを削除

### Phase 7: テスト

- `currentProjectRoot()` のユニットテスト
- `syncPluginProject` の worktree パス対応テスト
- 既存テストが壊れていないことの確認

---

## リスク

| リスク | 影響 | 対策 |
|--------|------|------|
| セッション 0 の状態で `n` を押した場合 | Project が存在しない | fallback: `filepath.Abs(".")` |
| SSH セッションからの `n` | リモートパス検出 | SSH 検出ロジックは変更しない |
| worktree 内で `n` → project root で作成 | 意図通り | worktree からの新規は `w` を使う |

---

## 相談事項

1. **`N` キーで CWD 新セッション** — この割り当てで良いか?
2. **`n` の動作変更** — カーソル位置の project root で新セッション作成に変更して良いか?
3. **他に検討すべきキー変更はあるか?**
