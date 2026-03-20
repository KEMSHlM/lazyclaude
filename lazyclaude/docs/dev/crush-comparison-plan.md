# lazyclaude vs crush 設計比較 & リファクタリング計画

**作成日**: 2026-03-20
**参照**: https://github.com/charmbracelet/crush

---

## 1. プロジェクト性質の違い

| 項目 | crush | lazyclaude |
|------|-------|-----------|
| **目的** | Claude Code のスタンドアロン TUI フロントエンド | tmux セッション管理 + permission popup プロキシ |
| **Claude との関係** | Claude API を直接呼ぶ (LLM クライアント) | Claude Code の外部ツール (MCP サーバー) |
| **TUI フレームワーク** | BubbleTea v2 (Elm architecture) | gocui (lazygit fork, 命令的) |
| **規模** | 283 Go files, 58 internal packages | 91 Go files, 8 internal packages |
| **永続化** | SQLite + sqlc コード生成 | JSON ファイル (state.json) |
| **MCP** | MCP クライアント (外部 MCP サーバーに接続) | MCP サーバー (Claude Code が接続してくる) |

crush は「Claude の TUI フロントエンド」、lazyclaude は「Claude Code セッションの管理 + tmux popup プロキシ」。
目的が異なるため、全ての設計を模倣する必要はない。

---

## 2. crush から学ぶべきアーキテクチャパターン

### 2.1 Service Interface パターン (優先度: 高)

**crush の設計:**
```go
// 各ドメインが Service interface を持つ
type SessionService interface {
    Create(ctx context.Context, ...) (*Session, error)
    Get(ctx context.Context, id string) (*Session, error)
    List(ctx context.Context) ([]*Session, error)
    Delete(ctx context.Context, id string) error
}

type MessageService interface { ... }
type PermissionService interface { ... }

// App が全 Service を注入
type App struct {
    Sessions    SessionService
    Messages    MessageService
    Permissions PermissionService
}
```

**lazyclaude の現状:**
- `session.Manager` は concrete struct で interface なし
- `server.Handler` は `tmux.Client` interface を使うが、他は直接依存
- テストで mock しにくい箇所がある

**改善案:**
```go
// internal/session/service.go
type Service interface {
    Create(ctx context.Context, name string, opts CreateOpts) (*Session, error)
    Get(id string) (*Session, error)
    List() []*Session
    Delete(ctx context.Context, id string) error
    Sync(ctx context.Context) error
}

// Manager が Service を実装
type Manager struct { ... }
var _ Service = (*Manager)(nil)
```

### 2.2 Pub/Sub イベントブローカー (優先度: 中)

**crush の設計:**
```go
// pubsub.Broker[T] — 型安全なイベント配信
broker := pubsub.NewBroker[SessionEvent]()
broker.Publish(SessionCreatedEvent{...})

// TUI 側で購読
sub := broker.Subscribe()
for event := range sub.Ch() { ... }
```

**lazyclaude の現状:**
- 通知キュー: ファイルベース (notify queue) + GUI ticker (100ms polling)
- Server → GUI: 直接の通信経路なし (ファイル経由)
- Control mode: tmux event callback → outputNotify channel

**改善案:**
ファイルベースの notify queue は SSH リモート対応に必須なため維持。
ただし、ローカルの Server → GUI 間には channel ベースの通知を追加:

```go
// internal/core/event/broker.go
type Broker[T any] struct {
    subscribers []chan T
    mu          sync.RWMutex
}

func (b *Broker[T]) Publish(event T) { ... }
func (b *Broker[T]) Subscribe() <-chan T { ... }
```

### 2.3 Dialog Interface (優先度: 中)

**crush の設計:**
```go
type Dialog interface {
    ID() string
    HandleMsg(msg tea.Msg) Action
    Draw(scr Screen, area Rectangle) *Cursor
}

type Overlay struct {
    dialogs []Dialog  // スタック
}
```

**lazyclaude の現状:**
- `PopupController` はスタック管理のみ
- Popup の描画は `app.go` の `renderPopup()` に集中
- 異なるタイプの popup (tool, diff, notification) の区別が弱い

**改善案:**
```go
// internal/gui/popup/popup.go
type Popup interface {
    ID() string
    Type() PopupType  // Tool, Diff, Notification
    Render(width, height int) []string
    HandleKey(key gocui.Key, ch rune) (Choice, bool)
}
```

### 2.4 App ライフサイクル管理 (優先度: 中)

**crush の設計:**
```go
type App struct {
    cleanupFuncs []func(context.Context) error
}

func (a *App) Close(ctx context.Context) error {
    for _, fn := range a.cleanupFuncs {
        fn(ctx)
    }
}
```

**lazyclaude の現状:**
- `defer g.Close()` は root.go にある
- GC の停止は `gc.Stop()` を直接呼ぶ
- Server shutdown は signal handler で処理
- cleanup が分散している

**改善案:**
App struct に cleanup 関数を集約。

### 2.5 構造化された Config (優先度: 低)

**crush の設計:**
- `config.json` + `AGENTS.md` front matter + env vars
- 型安全な ConfigStore with hot reload

**lazyclaude の現状:**
- `config.Paths` (ディレクトリパスのみ)
- `PopupMode` enum
- 設定ファイルなし (全て env var)

**将来改善:**
`~/.config/lazyclaude/config.toml` で keybind override、テーマ等。
ただし現時点では premature — 機能が安定してから。

---

## 3. lazyclaude 固有の強み (維持すべき)

| 設計 | 理由 |
|------|------|
| **tmux 深度統合** | control mode, display-popup, pidwalk — crush にはない |
| **SSH reverse tunnel** | リモート Claude Code → ローカル popup — 独自設計 |
| **Popup queue (per-window)** | 同一ウィンドウの popup を逐次処理 — crush は単一 permission dialog |
| **Choice detection** | Claude Code の pane から選択肢数を推定 — crush は自前 dialog |
| **File-based notify** | SSH 越しでも動作する通知メカニズム |
| **gocui (lazygit fork)** | lazygit と同じ UX パターン — 変更する理由がない |

---

## 4. 採用しない crush の設計

| 設計 | 不採用理由 |
|------|-----------|
| **BubbleTea への移行** | gocui は lazygit 互換で安定。TUI フレームワーク変更はリスクが高すぎる |
| **SQLite 永続化** | セッション数は少ない (数十件)。JSON ファイルで十分 |
| **sqlc コード生成** | DB 不使用のため不要 |
| **MCP クライアント** | lazyclaude は MCP サーバー側。方向が逆 |
| **LLM プロバイダ抽象化** | Claude Code が LLM を扱う。lazyclaude は管理ツール |
| **96KB ui.go** | crush でも課題。lazyclaude は小さいファイルを維持 |

---

## 5. リファクタリング計画

### Phase R1: Service Interface 導入

**目標**: テスタビリティ向上、依存関係の明示化

| # | タスク | ファイル |
|---|--------|---------|
| R1.1 | `session.Service` interface 定義 | `internal/session/service.go` (新規) |
| R1.2 | `Manager` が `Service` を実装 | `internal/session/manager.go` (修正) |
| R1.3 | GUI の SessionProvider を `session.Service` に統一 | `internal/gui/app.go` (修正) |
| R1.4 | Server handler に Service 注入 | `internal/server/handler.go` (修正) |

### Phase R2: Popup Interface 整理

**目標**: popup 種類の明確化、描画ロジックの分離

| # | タスク | ファイル |
|---|--------|---------|
| R2.1 | `popup.Popup` interface 定義 | `internal/gui/popup/popup.go` (新規) |
| R2.2 | `ToolPopup`, `DiffPopup`, `NotificationPopup` 実装 | `internal/gui/popup/` (新規) |
| R2.3 | `PopupController` を interface ベースに変更 | `internal/gui/popup_controller.go` (修正) |
| R2.4 | `app.go` の `renderPopup()` を各 Popup に委譲 | `internal/gui/popup.go` (修正) |

### Phase R3: Event Channel 追加 (ローカル通知)

**目標**: file polling → channel 通知 (ローカル接続時)

| # | タスク | ファイル |
|---|--------|---------|
| R3.1 | `event.Broker[T]` 汎用イベントブローカー | `internal/core/event/broker.go` (新規) |
| R3.2 | Server が popup イベントを publish | `internal/server/server.go` (修正) |
| R3.3 | GUI が channel を subscribe (file polling と並行) | `internal/gui/app.go` (修正) |

### Phase R4: App ライフサイクル統合

**目標**: cleanup 処理の集約

| # | タスク | ファイル |
|---|--------|---------|
| R4.1 | `App.RegisterCleanup()` メソッド追加 | `internal/gui/app.go` (修正) |
| R4.2 | GC, Control mode, Server の cleanup を集約 | `cmd/lazyclaude/root.go` (修正) |

---

## 6. 優先度と依存関係

```
R1 (Service Interface) ──→ R2 (Popup Interface)
                          ↘
                           R3 (Event Channel) ──→ R4 (App Lifecycle)
```

- R1 は独立して着手可能、最も影響が大きい
- R2 は R1 と並行可能だが、R1 完了後が理想
- R3 は R1, R2 と独立
- R4 は全体の仕上げ

### 推奨実施順序

1. **R1** (Service Interface) — テスタビリティ改善の基盤
2. **R2** (Popup Interface) — GUI コードの整理
3. **R3** (Event Channel) — パフォーマンス改善
4. **R4** (App Lifecycle) — 仕上げ

---

## 7. crush から取り入れたい具体的コード

### 7.1 Dialog スタック管理 (crush: `internal/ui/dialog/dialog.go`)

```go
// crush のアプローチ: ID ベースでダイアログを管理
func (o *Overlay) OpenDialog(d Dialog)
func (o *Overlay) CloseDialog(id string)
func (o *Overlay) BringToFront(id string)
```

lazyclaude の `PopupController` も同様のスタック管理を行うが、
ID ベースのアクセスと `BringToFront()` は取り入れる価値がある。

### 7.2 Split/Unified Diff 切替 (crush: `internal/ui/dialog/permissions.go`)

```go
// crush: diff 表示モードのトグル
type Permissions struct {
    diffSplitMode      *bool
    unifiedDiffContent string
    splitDiffContent   string
}
```

lazyclaude の diff popup は現在 unified のみ。
split diff の追加は将来検討 (P9+)。

### 7.3 Graceful Shutdown パターン (crush: `internal/app/app.go`)

```go
// crush: cleanup 関数を登録制にして順序保証
type App struct {
    cleanupFuncs []func(context.Context) error
}
```

---

## 8. メトリクス目標

| 指標 | 現在 | 目標 |
|------|------|------|
| Go ソースファイル数 | 43 | 50-55 (interface ファイル追加) |
| Interface 数 | 1 (tmux.Client) | 4+ (Session.Service, Popup, Event.Broker, ...) |
| テストカバレッジ (server) | 84.2% | 90%+ |
| テストカバレッジ (gui) | 43.8% | 60%+ (mock 可能な interface 増加で) |
| 最大ファイル行数 | ~400 | 400 以下維持 |

---

## 9. テスト設計の比較

### crush のテスト戦略

| 手法 | crush での使い方 | lazyclaude への適用 |
|------|-----------------|-------------------|
| **VCR (HTTP 録画/再生)** | 外部 API 呼び出しを YAML cassette に記録、再生テスト | MCP WebSocket テストに応用可能 |
| **Golden file テスト** | diff 表示の出力を `.golden` ファイルと比較 (32 パターン: layout x theme x option) | diff/tool popup の描画結果を golden file で検証 |
| **synctest (同期テスト)** | `synctest.Test()` + `synctest.Wait()` で goroutine タイミングを決定的に | PopupOrchestrator の queue テストに適用可能 |
| **goleak (goroutine leak 検出)** | `go.uber.org/goleak` でテスト終了時に goroutine 残存チェック | Control mode, GC, popup goroutine のリーク検出 |
| **Benchmark** | `BenchmarkBuildSummaryPrompt`, `BenchmarkRegexCache` | `BenchmarkCapturePaneANSI`, `BenchmarkDetectMaxOption` |
| **Table-driven** | 全パッケージで一貫して使用 | 使用中、維持 |
| **fakeEnv fixture** | 全依存を注入した統合テスト環境 | `testEnv` struct で tmux.Client, session.Service, config を一括セットアップ |

### lazyclaude に取り入れるべきテストパターン

**優先度: 高**

1. **Golden file テスト (presentation パッケージ)**
   - `presentation.FormatToolLines()` の出力を golden file で固定
   - `presentation.ParseUnifiedDiff()` + `FormatDiffLine()` の出力検証
   - 幅・テーマごとの組み合わせテスト
   ```go
   func TestFormatToolLines_Golden(t *testing.T) {
       output := FormatToolLines(td)
       golden.RequireEqual(t, []byte(strings.Join(output, "\n")))
   }
   ```

2. **goroutine leak 検出**
   - `PopupOrchestrator`, `ControlClient`, `GC` がリーク源になり得る
   ```go
   func TestMain(m *testing.M) {
       goleak.VerifyTestMain(m)
   }
   ```

3. **統合テスト fixture (`testEnv`)**
   - crush の `fakeEnv` パターンを採用
   ```go
   type testEnv struct {
       tmux    *tmux.MockClient
       session session.Service  // mock
       paths   config.Paths
       tmpDir  string
   }
   func newTestEnv(t *testing.T) *testEnv { ... }
   ```

**優先度: 中**

4. **Benchmark for クリティカルパス**
   - `DetectMaxOption()` (ANSI パース)
   - `CapturePaneANSI()` (tmux 呼び出し)
   - `PopupOrchestrator` のスループット

### lazyclaude の現状テストとの差分

| 項目 | crush | lazyclaude | Gap |
|------|-------|-----------|-----|
| Golden file | diffview 全パターン | なし | 導入すべき |
| goroutine leak | goleak 標準使用 | なし | 導入すべき |
| Benchmark | 主要パスに配置 | なし | 導入すべき |
| VCR/録画 | 外部 API 全て | なし | 中優先度 |
| Table-driven | 全パッケージ | 使用中 | OK |
| t.TempDir() | 全テスト | 使用中 | OK |
| t.Parallel() | 一部使用 | 未使用 | 検討 |
| E2E (Docker) | なし (VCR で代替) | VHS tape + bash scripts | lazyclaude の方が充実 |

---

## 10. Keymap 設計の比較

### crush の keymap アーキテクチャ

```
KeyMap struct (宣言的定義)
    ├── Editor { SendMessage, Newline, OpenEditor, ... }
    ├── Chat { Up, Down, Cancel, Copy, ... }
    ├── Initialize { Yes, No, Enter, Switch }
    └── Global { Quit, Help, Commands, Models, Sessions }

Dispatch: handleKeyPressMsg()
    1. Quit (最高優先)
    2. Dialog.HandleMsg() (ダイアログがあれば)
    3. State 分岐 (uiOnboarding / uiInitialize / uiChat)
    4. Focus 分岐 (uiFocusEditor / uiFocusMain)
    5. key.Matches() で個別処理
    6. handleGlobalKeys() (フォールバック)
```

### lazyclaude の keymap アーキテクチャ

```
KeyRegistry (集中管理)
    ├── StateMain { Enter, Tab, ... }
    ├── StateFullInsert { Ctrl+\, ... }
    ├── StateFullNormal { i, j, k, q, ... }
    └── Global { ... }

Dispatch: gocui.SetKeybinding()
    1. View-specific bindings
    2. Editor.Edit() (Editable view のみ)
    3. Global bindings (rune は Editable view ではスキップ)
```

### 設計比較

| 項目 | crush | lazyclaude | 評価 |
|------|-------|-----------|------|
| **定義方式** | `KeyMap` struct + `key.NewBinding()` | `KeyRegistry` + `ActionDef` | lazyclaude の方がドキュメント性が高い |
| **State 依存** | dispatch 時に `switch m.state` | 定義時に state を指定 | lazyclaude の方が明示的 |
| **Dialog keys** | Dialog ごとに独自 `localKeyMap` | 同一 Registry (view-specific handler) | crush の方が良い — dialog の独立性が高い |
| **Action 型** | `Action` interface (型安全 enum) | `Choice` enum + 直接処理 | crush の方が拡張性高い |
| **Help 表示** | `key.WithHelp()` で各 Binding に付与 | Registry から一覧生成可能 | 同等 |

### lazyclaude に取り入れるべき keymap パターン

**優先度: 高**

1. **Dialog ごとの独自 keymap**
   - 現状: popup の y/n/a は global binding で処理
   - 改善: popup 種類ごとに keymap を定義、popup 表示中は global binding を無効化
   ```go
   type ToolPopupKeyMap struct {
       Accept key.Binding  // y
       Allow  key.Binding  // a (3-option のみ)
       Reject key.Binding  // n
       Cancel key.Binding  // Esc
   }
   ```

2. **Action メッセージパターン**
   - Dialog が `Action` を返し、呼び出し元が解釈
   - popup の結果処理を popup 内部から分離
   ```go
   type PopupAction interface{}
   type AcceptAction struct{ Window string }
   type RejectAction struct{ Window string }
   ```

---

## 11. UI 設計の比較

### crush の UI アーキテクチャ

```
+---------------------+
| Header              |  (4 rows)
+------------+--------+
| Chat Area  | Side   |  (動的高さ)
| (lazy list)| bar    |  (幅 30)
|            |        |
+------------+--------+
| Pills (todos)       |  (可変)
+---------------------+
| Editor (textarea)   |  (5 rows)
+---------------------+
| Status / Help       |  (1 row)
+---------------------+
```

- **Responsive**: width < 120 or height < 30 でコンパクトモード
- **Lazy rendering**: 可視アイテムのみ描画
- **Animation optimization**: 不可視アニメーションを一時停止
- **Render cache**: 幅変更時のみ再描画

### lazyclaude の UI アーキテクチャ

```
+------------------+------------------+
| Session List     | Preview          |  (メイン画面)
| (左パネル)       | (右パネル)       |
+------------------+------------------+
| Tab Bar                             |
+-------------------------------------+

フルスクリーン:
+-------------------------------------+
| Claude Code pane (tmux embed)       |
| + Popup overlay (cascade)           |
+-------------------------------------+
```

### 設計比較

| 項目 | crush | lazyclaude | 評価 |
|------|-------|-----------|------|
| **レイアウト計算** | `image.Rectangle` で動的計算 | gocui の `SetView()` で座標指定 | crush の方が柔軟 |
| **Responsive** | ブレークポイントでコンパクト化 | なし (固定レイアウト) | crush の方が良い |
| **Lazy rendering** | 可視アイテムのみ | 全アイテム描画 | crush の方が大規模向き |
| **Render cache** | 幅変更時のみ再描画 | 毎フレーム全描画 | crush の方が効率的 |
| **Preview** | なし (直接 chat) | tmux capture-pane でプレビュー | lazyclaude 固有 |
| **Popup** | Dialog stack (Overlay) | Popup cascade (PopupController) | 異なるアプローチ、両方有効 |

### lazyclaude に取り入れるべき UI パターン

**優先度: 高**

1. **Render cache (描画キャッシュ)**
   - セッション一覧の描画を幅変更時のみ再計算
   - プレビュー capture-pane の結果を TTL 付きキャッシュ (既に部分実装済み)
   ```go
   type cachedRender struct {
       content string
       width   int
       height  int
   }
   ```

2. **Responsive ブレークポイント**
   - ターミナル幅が小さい場合にプレビューパネルを非表示化
   - popup のサイズを画面サイズに応じて調整 (既に部分実装済み)

**優先度: 中**

3. **Rectangle ベースのレイアウト計算**
   - gocui の `SetView()` を直接呼ぶ代わりに、`Layout` struct で事前計算
   - テスタビリティ向上 (レイアウト計算を pure function に)
   ```go
   type Layout struct {
       SessionList image.Rectangle
       Preview     image.Rectangle
       TabBar      image.Rectangle
       Popup       image.Rectangle
   }
   func ComputeLayout(width, height int, hasPopup bool) Layout { ... }
   ```

---

## 12. リファクタリング計画 (更新)

### 全 Phase 一覧

| Phase | 内容 | 優先度 | 主な参考元 |
|-------|------|--------|-----------|
| **R1** | Service Interface 導入 | 高 | crush: Service pattern |
| **R2** | Popup Interface + Dialog keymap | 高 | crush: Dialog interface, local keymap |
| **R3** | テスト基盤強化 | 高 | crush: golden file, goleak, benchmark |
| **R4** | Event Channel (ローカル通知) | 中 | crush: pubsub.Broker |
| **R5** | UI 改善 (cache, responsive) | 中 | crush: render cache, breakpoints |
| **R6** | App ライフサイクル統合 | 低 | crush: cleanupFuncs |

### 依存関係

```
R1 (Service Interface) ─┬→ R2 (Popup + Dialog keymap)
                         └→ R3 (テスト基盤)
                              ↓
                         R4 (Event Channel)
                              ↓
                         R5 (UI 改善)
                              ↓
                         R6 (App Lifecycle)
```

---

## 結論

crush は Charm 社の高品質な TUI アプリケーション。
lazyclaude に適用すべきパターンは以下の 6 領域:

1. **Service Interface** — テスタビリティの基盤
2. **Dialog Interface + local keymap** — popup の関心分離
3. **Golden file + goleak + benchmark** — テスト品質の底上げ
4. **Pub/Sub Event** — ファイル polling の補完
5. **Render cache + responsive** — UI 効率化
6. **Cleanup 集約** — ライフサイクル管理

lazyclaude は tmux プラグインという異なる性質を持つため、
crush の全てを模倣するのではなく、**テスタビリティ** と **関心の分離** に
焦点を当てた段階的リファクタリングが適切。

lazyclaude 固有の強み (tmux 深度統合, SSH reverse tunnel, file-based notify,
display-popup orchestration) は維持する。
