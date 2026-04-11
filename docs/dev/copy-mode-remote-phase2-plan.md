# Plan: Remote fullscreen copy mode の scrollback を remote daemon 経由で取得 (Bug 3 Phase 2)

## Context

Phase 1 (`docs/dev/copy-mode-remote-plan.md`) の診断で仮説 A が確定:

- 再現実機の debug log で
  - `[remote] HistorySize: host="AERO" target="@440" histSize=0`
  - `[local]  HistorySize: host=""     target="@414" histSize=43100`
- Remote session の local mirror window は tmux pane scrollback buffer が空 (0 行)
- `CapturePaneANSIRange` が返す bytes は **現在画面に見える領域** のみ (約 8KB)
- 結果: fullscreen Ctrl+V で copy mode に入ると表示領域が固定され、過去の出力にさかのぼれない

## 設計上の根本原因

Mirror window の仕組み:
- Local tmux の window 内で `ssh -t host 'tmux attach'` プロセスが走る
- SSH セッションは remote tmux の **現在画面を描画しているだけ** で、remote tmux の scrollback buffer は local tmux からは見えない
- Remote tmux pane の実際の scrollback は remote 側にしか存在しない
- `capture-pane` を local mirror に対して実行すると、local tmux が SSH セッション上に描画した current viewport + local tmux が独自に蓄積した scrollback (ほぼ空) しか取れない

ローカルはこの問題を持たない:
- Local claude は lazyclaude tmux server の pane 内で **直接** 動作
- 出力は local tmux pane の scrollback buffer にそのまま蓄積 (実測 43100 行)
- `capture-pane -S -N` でフル履歴にアクセスできる

## 修正方針 (Approach 1)

Scrollback / history_size の取得を **remote daemon API 経由** に切り替える。Remote daemon の lazyclaude server は remote tmux server (`-L lazyclaude`) に直接アクセスできるので、そこで `capture-pane` / `show-message` を実行し HTTP で結果を返す。

`CapturePreview` は現状動いている (mirror window から現在画面を取得) ので **触らない**。scrollback 系 2 メソッドのみ routing を変更する。

### Host 分岐の最小化

- 新規 host 分岐は `CompositeProvider.CaptureScrollback/HistorySize` の 1 箇所に集約
- 「session.Host が非空なら remote provider、空なら local provider」という既存の routing pattern (CreateWorktree 等) と同じ方針
- `providerForSession(id)` は attach/delete 等で引き続き local を返す (Mirror 経由 runtime ops は保持)
- 新しい helper `providerForCapture(id)` を内部に作り、session を lookup して Host 見て分岐、**capture 系メソッドのみ** この helper 経由

## 実装ステップ

### Step 1: daemon API endpoints の追加

ファイル: `internal/daemon/api.go`

既に `ScrollbackRequest` / `ScrollbackResponse` / `HistorySizeResponse` は定義済 (L85-102)。`HistorySize` 用のリクエスト型だけ追加 (URL パス + id でも可だが既存 pattern に合わせる):

既存の `ScrollbackRequest` で `ID` と `Width/StartLine/EndLine` を持つので流用。HistorySize はパス `{id}` のみで request body 不要。

ファイル: `internal/daemon/server.go`

ルーティング追加 (L94 付近の `mux.HandleFunc(...)` リストに):
```go
mux.HandleFunc("POST /session/{id}/scrollback", s.withAuth(s.handleScrollback))
mux.HandleFunc("GET /session/{id}/history-size", s.withAuth(s.handleHistorySize))
```

ハンドラー実装 (新規追記):
```go
// handleScrollback captures scrollback for a session.
// Uses the local (remote daemon's) tmux server via capture-pane -S -E range.
func (s *Server) handleScrollback(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if id == "" {
        http.Error(w, "missing session id", http.StatusBadRequest)
        return
    }
    var req ScrollbackRequest
    if err := readJSON(w, r, &req); err != nil {
        return
    }
    sess := s.mgr.Store().FindByID(id)
    if sess == nil {
        http.Error(w, "session not found", http.StatusNotFound)
        return
    }
    target := sess.TmuxTarget()
    content, err := s.tmux.CapturePaneANSIRange(r.Context(), target, req.StartLine, req.EndLine)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadGateway)
        return
    }
    writeJSON(w, http.StatusOK, ScrollbackResponse{Content: content})
}

// handleHistorySize returns the pane's scrollback history size.
func (s *Server) handleHistorySize(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    if id == "" {
        http.Error(w, "missing session id", http.StatusBadRequest)
        return
    }
    sess := s.mgr.Store().FindByID(id)
    if sess == nil {
        http.Error(w, "session not found", http.StatusNotFound)
        return
    }
    target := sess.TmuxTarget()
    out, err := s.tmux.ShowMessage(r.Context(), target, "#{history_size}")
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadGateway)
        return
    }
    n, _ := strconv.Atoi(strings.TrimSpace(out))
    writeJSON(w, http.StatusOK, HistorySizeResponse{Lines: n})
}
```

必要 import: `strconv`, `strings` (既存の import に追加)。

**注**: `Server.mgr.Store().FindByID(id)` と `sess.TmuxTarget()` は Bug 1 で merge 済の Session.TmuxTarget() を流用する。Remote daemon の `s.tmux` は remote tmux server client なので、同じ helper が remote でも valid target を返す。

### Step 2: ClientAPI に capture methods を追加

ファイル: `internal/daemon/client.go`

`ClientAPI` interface に追加:
```go
// --- Capture ---

// CaptureScrollback retrieves a range of scrollback lines for a session.
// Used by the fullscreen copy mode for remote sessions where the local
// mirror window's tmux buffer is empty.
CaptureScrollback(ctx context.Context, req ScrollbackRequest) (*ScrollbackResponse, error)

// HistorySize returns the number of scrollback lines for a session.
HistorySize(ctx context.Context, id string) (int, error)
```

### Step 3: HTTPClient 実装

ファイル: `internal/daemon/http_client.go`

`ClientAPI` 実装側に 2 メソッド追加:
```go
func (c *HTTPClient) CaptureScrollback(ctx context.Context, req ScrollbackRequest) (*ScrollbackResponse, error) {
    path := "/session/" + req.ID + "/scrollback"
    var resp ScrollbackResponse
    if err := c.post(ctx, path, req, &resp); err != nil {
        return nil, fmt.Errorf("capture scrollback: %w", err)
    }
    return &resp, nil
}

func (c *HTTPClient) HistorySize(ctx context.Context, id string) (int, error) {
    path := "/session/" + id + "/history-size"
    var resp HistorySizeResponse
    if err := c.get(ctx, path, &resp); err != nil {
        return 0, fmt.Errorf("history size: %w", err)
    }
    return resp.Lines, nil
}
```

既存の helper (`c.post`, `c.get`) を流用。既存 HTTPClient の他メソッド (CreateSession 等) と同じ pattern。

### Step 4: RemoteProvider の stub を実装に差し替え

ファイル: `internal/daemon/remote_provider.go:325-334`

```go
// CaptureScrollback retrieves scrollback via the remote daemon API. This is
// the fullscreen copy-mode path for remote sessions; the mirror window's
// local tmux buffer does not contain the remote tmux's historical scrollback,
// so we ask the remote daemon to run capture-pane on its own tmux server.
func (rp *RemoteProvider) CaptureScrollback(id string, width, startLine, endLine int) (*ScrollbackResponse, error) {
    client, err := rp.conn.Client()
    if err != nil {
        return nil, fmt.Errorf("capture scrollback: %w", err)
    }
    return client.CaptureScrollback(context.Background(), ScrollbackRequest{
        ID:        id,
        Width:     width,
        StartLine: startLine,
        EndLine:   endLine,
    })
}

// HistorySize returns the remote tmux pane's scrollback length via the
// daemon API. Same rationale as CaptureScrollback.
func (rp *RemoteProvider) HistorySize(id string) (int, error) {
    client, err := rp.conn.Client()
    if err != nil {
        return 0, fmt.Errorf("history size: %w", err)
    }
    return client.HistorySize(context.Background(), id)
}
```

**注**:
- `CapturePreview` は error stub のまま保持 (mirror 経由で動いているから触らない)
- 既存の client 取得 pattern (`rp.conn.Client()`) に従う
- `fmt.Errorf` で context wrap

### Step 5: CompositeProvider の routing を変更

ファイル: `internal/daemon/composite_provider.go:218-234`

```go
// CaptureScrollback captures scrollback. Remote sessions go through the
// remote daemon API because the local mirror window's tmux buffer does
// not contain the remote tmux's historical scrollback. Local sessions
// still use the local provider.
func (c *CompositeProvider) CaptureScrollback(id string, width, startLine, endLine int) (*ScrollbackResponse, error) {
    p := c.providerForCapture(id)
    if p == nil {
        return nil, fmt.Errorf("no provider found for session %q", id)
    }
    return p.CaptureScrollback(id, width, startLine, endLine)
}

// HistorySize returns scrollback size. Remote sessions go through the
// remote daemon API for the same reason as CaptureScrollback.
func (c *CompositeProvider) HistorySize(id string) (int, error) {
    p := c.providerForCapture(id)
    if p == nil {
        return 0, fmt.Errorf("no provider found for session %q", id)
    }
    return p.HistorySize(id)
}
```

新規 helper:
```go
// providerForCapture returns the provider that should serve scrollback /
// history-size queries for the given session. Remote sessions MUST go
// through their RemoteProvider because the local mirror window only has
// the current viewport — the remote tmux's historical scrollback is not
// replicated to the local pane. Local sessions use the local provider
// which reads the local tmux pane's own buffer.
//
// Note: preview capture is NOT routed here. CapturePreview still uses the
// mirror window because the current viewport is exactly what the user
// sees and the mirror renders it correctly.
//
// If the session has Host != "" but no matching remote provider is
// registered (e.g. the remote disconnected), returns nil so the caller
// emits "no provider found" which the GUI surfaces as a capture error.
func (c *CompositeProvider) providerForCapture(sessionID string) SessionProvider {
    // Local store is the authoritative source for session metadata including
    // Host. Look up the session and dispatch by Host.
    sess := c.local.Session(sessionID) // NEW helper, see Step 6
    if sess == nil {
        // Fall back to the existing providerForSession for robustness; if
        // not found there either the caller returns a not-found error.
        return c.providerForSession(sessionID)
    }
    if sess.Host == "" {
        return c.local
    }
    c.mu.RLock()
    defer c.mu.RUnlock()
    if rp, ok := c.remotes[sess.Host]; ok {
        return rp
    }
    return nil
}
```

**Host 分岐の局所化**: このファイル/関数に限定。providerForSession は変更せず、capture 専用の routing を追加する形。`if sess.Host == ""` 分岐は 1 箇所 (providerForCapture 内部) のみ。

### Step 6: localDaemonProvider に Session lookup helper を追加

`providerForCapture` が `c.local.Session(id)` を呼ぶので、`localDaemonProvider` に Session lookup を実装する。これは既存の `HasSession` の拡張。

ファイル: `cmd/lazyclaude/local_provider.go`

```go
// Session returns the session with the given ID from the local store, or
// nil if not found. Used by CompositeProvider.providerForCapture to
// dispatch capture ops by host.
func (p *localDaemonProvider) Session(id string) *session.Session {
    return p.mgr.Store().FindByID(id)
}
```

ファイル: `internal/daemon/composite_provider.go` の `SessionProvider` interface にも追加:

```go
type SessionProvider interface {
    // ... existing methods ...

    // Session returns the full Session object for the given ID, or nil if
    // not found. Used by composite provider for host-aware routing of
    // capture ops.
    Session(id string) *session.Session
}
```

**注**: ここで `session.Session` に依存することになる。`internal/daemon` は `internal/session` を import している? 要確認、worker が実装時に cyclic import にならないか検証する。

もし cyclic import リスクがある場合は代替案:
- `LocalSession(id) (host string, ok bool)` のような最小 API にする
- もしくは SessionProvider でなく `CompositeProvider` が直接 `localDaemonProvider` の concrete type を知る

Worker は Step 5 実装前に import cycle を確認すること。

### Step 7: Unit / integration tests

#### 7-a: daemon server handlers
ファイル: `internal/daemon/server_capture_test.go` (新規)

- TestServer_HandleScrollback_Success: mock tmux で capture-pane を stub、POST /session/{id}/scrollback に valid リクエスト、期待する content を返す
- TestServer_HandleScrollback_SessionNotFound: 存在しない id → 404
- TestServer_HandleHistorySize_Success: mock tmux で show-message を stub、GET /session/{id}/history-size → 期待する数値
- TestServer_HandleHistorySize_SessionNotFound: 404

#### 7-b: HTTPClient methods
ファイル: `internal/daemon/http_client_capture_test.go` (新規)

- httptest.Server で mock サーバー立てて CaptureScrollback / HistorySize のリクエスト/レスポンスを検証

#### 7-c: RemoteProvider methods
ファイル: `internal/daemon/remote_provider_test.go` (既存に追記)

- TestRemoteProvider_CaptureScrollback: mock ClientAPI で stub、RemoteProvider.CaptureScrollback が client にリクエスト転送することを assert
- TestRemoteProvider_HistorySize: 同様

#### 7-d: CompositeProvider routing
ファイル: `internal/daemon/composite_provider_test.go` (既存に追記)

- TestCompositeProvider_CaptureScrollback_LocalSession: session.Host="" → localProvider が呼ばれる
- TestCompositeProvider_CaptureScrollback_RemoteSession: session.Host="AERO" → remoteProvider が呼ばれる
- TestCompositeProvider_HistorySize_LocalSession: 同様 local
- TestCompositeProvider_HistorySize_RemoteSession: 同様 remote
- TestCompositeProvider_CaptureScrollback_RemoteSessionNoProvider: session.Host="GHOST" but no registered remote → error
- TestCompositeProvider_CapturePreview_StillUsesLocal: session.Host="AERO" でも preview は local 経由を確認 (regression guard)

### Step 8: 検証

1. `go build ./...` clean
2. `go vet ./...` clean
3. `go test -race ./internal/daemon/... ./cmd/lazyclaude/... ./internal/gui/...` 全 PASS
4. `/go-review` → CRITICAL/HIGH ゼロ
5. `/codex --enable-review-gate` → APPROVED
6. **手動検証** (要ユーザー):
   - [ ] Local session で fullscreen Ctrl+V → 従来通り scrollback 表示 (regression なし)
   - [ ] Remote session で fullscreen Ctrl+V → **remote tmux のフル scrollback** にアクセスできる
   - [ ] Remote session の preview (fullscreen でない通常表示) が regression なく動く
   - [ ] Remote session の attach / delete / send-keys が regression なく動く
   - [ ] Remote connection が切れた状態で Ctrl+V → "no provider" 的なエラー

## Out of Scope

- `CapturePreview` の routing 変更 (現状動いているので触らない)
- `ToolNotification` (permission popup) の remote 対応 (Bug 5 相当、別 plan)
- Bug 4 (activity state) — 並行中
- Diagnostic logging (Phase 1) の revert — 本 plan merge 時に必ず revert する (diag-copy-mode-remote branch を破棄)

## Files Changed

| ファイル | 変更 |
|---------|------|
| `internal/daemon/api.go` | (既存の ScrollbackRequest/ScrollbackResponse/HistorySizeResponse を流用、追加なし。必要なら comment 更新) |
| `internal/daemon/server.go` | `/session/{id}/scrollback`, `/session/{id}/history-size` のルート追加、handleScrollback / handleHistorySize 実装 |
| `internal/daemon/client.go` | ClientAPI に CaptureScrollback / HistorySize 追加 |
| `internal/daemon/http_client.go` | HTTPClient.CaptureScrollback / HistorySize 実装 |
| `internal/daemon/remote_provider.go` | CaptureScrollback / HistorySize を error stub から実装に差し替え (CapturePreview は stub のまま) |
| `internal/daemon/composite_provider.go` | `providerForCapture` helper 追加、`CaptureScrollback` / `HistorySize` の routing をそれ経由に変更。`SessionProvider` interface に `Session(id)` 追加 |
| `cmd/lazyclaude/local_provider.go` | `localDaemonProvider.Session(id)` 実装 |
| `internal/daemon/server_capture_test.go` (新規) | handler unit test |
| `internal/daemon/http_client_capture_test.go` (新規) | client unit test |
| `internal/daemon/remote_provider_test.go` | CaptureScrollback / HistorySize test 追加 |
| `internal/daemon/composite_provider_test.go` | providerForCapture routing test 追加 |

## Risk Assessment

- **Low**: capture ops は既に interface 化されていて、routing 変更と実装追加のみ
- **Low**: `CapturePreview` を触らないので現在動いている mirror 経由の preview は regression なし
- **Medium**: `SessionProvider` interface に `Session(id)` を追加すると既存の stub / mock 実装も更新が必要。事前に grep で影響範囲確認
- **Medium**: `internal/daemon` → `internal/session` の import cycle の可能性。既に `SessionInfo` はあるが `*session.Session` を interface に露出する場合は要検証。cycle になったら `LocalSessionHost(id) (host string, ok bool)` のような最小 API に変更
- **Low**: Daemon API endpoint の security/auth は既存の `s.withAuth` を使うので追加リスクなし

## Open Questions

1. `SessionProvider.Session(id)` が import cycle を招く場合の代替 API: `LocalSessionHost(id)` のような最小化で十分か、あるいは `*daemon.SessionInfo` 返しにするか
2. Daemon server handler で `capture-pane` の `-ep` flag を付けるか: 既存の `CapturePaneANSIRange` は `-e` ANSI 付きを返すはずなので、Worker が実装時に確認
3. `ScrollbackResponse` に `CursorX/CursorY` field があるが scrollback では使わないので空のまま。将来 preview も routing する時のために placeholder として残す

## Dependencies

- Bug 1 (attach) の `Session.TmuxTarget()` helper は merge 済 (c56b85c base 以降)。本 plan は TmuxTarget に依存
- Bug 3 Phase 1 の diagnostic logging (diag-copy-mode-remote branch) は本 plan merge 前に revert / branch 破棄すること
