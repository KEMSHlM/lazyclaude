# Plan: Bug 2 Phase 2 — Full SSH-backed MCP/plugin editing on remote

## Context

Phase 1 (`docs/dev/mcp-plugin-remote-plan.md`, merge `acd7de6`) は remote session で MCP/plugin 編集を **disable してプレースホルダー表示** するだけだった。ユーザーの本来の要求は **remote で実際に編集できる** こと。Phase 2 で full SSH-backed 実装に差し替える。

## Reference commits (未 merge、stg base の古い branch)

以下 commit が過去の試みの実装サンプル。daemon-arch ではアーキテクチャが異なるので cherry-pick はできないが、実装ロジック (SSH read/write, manager host field, 設定ファイルフォーマット処理) は参考になる:

- `0630493` feat: support MCP toggle for SSH remote sessions via SSH commands (fix-ssh-mcp-toggle branch)
  - Added `internal/mcp/ssh.go`: `sshReadFile` / `sshWriteFile` / `shellQuote` / `splitHostPort`
  - Added `Manager.host` + `SetHost`
  - Extracted `parseClaudeJSON` / `parseDeniedServers` / `buildDeniedJSON` helpers for local/SSH code path reuse
  - Refresh / ToggleDenied host-aware分岐
- `15fb347` fix: address security and error handling issues from code review
  - `shellQuote` の remotePath (injection 対策)
  - error handling
  - IPv6 `[::1]:22` 対応
- `e1e1178` fix: disable plugin management for SSH remote sessions (fix-ssh-plugins)
  - 注: これは plugin を **disable** する commit。plugin の SSH 実装は当時も未着手

Worker は `git show 0630493 -- internal/mcp/` と `git show 15fb347 -- internal/mcp/` を参考にする。

## Scope

### MCP: full SSH 対応
Remote session で MCP toggle (有効/無効) が SSH 経由で実 remote の `~/.claude.json` (user-level) と `<project>/.claude/settings.local.json` (project-level) を読み書きして動作する。

### Plugin: 本 Phase では引き続き Phase 1 の disable を維持
理由:
- 過去の fix-ssh-plugins branch にも plugin の SSH 実装は**存在しなかった** (`claude plugins install/uninstall` を SSH 越しに実行するのは CLI 依存が大きい)
- Plugin 操作は install/uninstall/update/toggle/refresh の 5 つあり、多くは marketplace からのダウンロードを伴う
- 今回は **MCP 優先、plugin は Phase 3 として別 plan に分離**
- ユーザー確認: 「MCP だけで十分 or plugin も必要か」は plan レビュー時点で確認

### Phase 1 で残すもの / 外すもの
- **残す**: `pluginState.remoteDisabled` / `mcpState.remoteDisabled` フラグ + render 側 placeholder (plugin 側に適用)
- **残す**: 7 つの plugin write entry point の `guardRemoteOp("Plugin editing")` (plugin は Phase 3 まで disable 維持)
- **外す**: `MCPToggleDenied` の `guardRemoteOp("MCP editing")` ガード (Phase 2 で動くようになるため)
- **外す**: `syncPluginProject` の `mcpState.remoteDisabled = true` 設定 (MCP 側は remote でも通常 Refresh する)
- **外す**: `renderMCPList` / `renderMCPPreview` の `mcpState.remoteDisabled` guard
- **残す**: `renderPluginPanel` / `renderPluginPreview` の PluginTabPlugins/PluginTabMarketplace の placeholder guard (plugin は継続 disable)

つまり **MCP は透過的に動かし、plugin は Phase 1 の disable を維持** する。

## Design

### Interface 拡張

ファイル: `internal/gui/mcp_state.go`

```go
type MCPProvider interface {
    SetProjectDir(dir string)
    SetHost(host string)  // NEW: "" for local, hostname for SSH
    Refresh(ctx context.Context) error
    Servers() []MCPItem
    ToggleDenied(ctx context.Context, name string) error
}
```

Plugin interface は**変更しない** (Phase 3 まで SetHost を追加しない)。

### Adapter wiring

ファイル: `cmd/lazyclaude/root.go`

```go
type mcpAdapter struct {
    mgr *mcp.Manager
}

func (a *mcpAdapter) SetHost(host string) {
    a.mgr.SetHost(host)
}
// ... existing methods forward as before
```

### Manager SSH 対応

ファイル: `internal/mcp/manager.go`

- `Manager` struct に `host string` field を追加
- `SetHost(host string)` method を追加
- `Refresh(ctx)`:
  - `host == ""` → 従来通り local file 読む
  - `host != ""` → SSH で `cat ~/.claude.json` と `cat <projectDir>/.claude/settings.local.json` を実行、結果を parse
- `ToggleDenied(ctx, name)`:
  - `host == ""` → 従来通り
  - `host != ""` → SSH で read-modify-write:
    1. `cat <path>` で現在の settings.local.json を取得
    2. deniedMcpjsonServers リストを更新
    3. base64 + `echo ... | base64 -d > <path>` で書き戻し

### SSH ヘルパー

ファイル: `internal/mcp/ssh.go` (新規、`0630493` / `15fb347` の実装を参考)

```go
package mcp

// sshReadFile reads a remote file via SSH and returns its content.
// Returns ("", nil) if the file does not exist (distinguish from connection failure).
func sshReadFile(ctx context.Context, host, remotePath string) (string, error) {
    cmd := exec.CommandContext(ctx, "ssh", host, fmt.Sprintf("cat %s 2>/dev/null", shellQuote(remotePath)))
    out, err := cmd.Output()
    if err != nil {
        // Exit 255 = SSH connection failure, exit 1 = file not found.
        if exitErr, ok := err.(*exec.ExitError); ok {
            if exitErr.ExitCode() == 255 {
                return "", fmt.Errorf("ssh %s: %w", host, err)
            }
            return "", nil // file not found, treat as empty
        }
        return "", err
    }
    return string(out), nil
}

// sshWriteFile writes content to a remote file via SSH.
// Uses base64 encoding to avoid shell quoting issues with JSON content.
func sshWriteFile(ctx context.Context, host, remotePath, content string) error {
    encoded := base64.StdEncoding.EncodeToString([]byte(content))
    cmd := exec.CommandContext(ctx, "ssh", host,
        fmt.Sprintf("mkdir -p %s && echo %s | base64 -d > %s",
            shellQuote(filepath.Dir(remotePath)),
            encoded,
            shellQuote(remotePath)))
    return cmd.Run()
}

// shellQuote wraps a string in single quotes, escaping any existing single quotes.
func shellQuote(s string) string {
    return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
```

**Security**: `shellQuote` で remotePath を必ず escape、base64 で content の injection を回避。

### 既存の local path の保持

`parseClaudeJSON` / `parseDeniedServers` / `buildDeniedJSON` を抽出しておけば、local と SSH の両方で同じ JSON 処理を共有できる (`0630493` でこの refactor が既に行われている、参考)。

### GUI wiring

ファイル: `internal/gui/app_actions.go` `syncPluginProject`

```go
func (a *App) syncPluginProject() {
    if a.plugins == nil {
        return
    }
    node := a.currentNode()
    if node == nil {
        // clearRemoteDisabled (plugin only)
        a.pluginState.remoteDisabled = false
        return
    }

    host, isRemote := a.isRemoteNodeSelected()

    // Plugin: remote時はPhase 1と同様disable
    a.pluginState.remoteDisabled = isRemote

    // MCP: SetHost で remote 対応、通常 Refresh を実行
    if a.mcpServers != nil {
        if isRemote {
            a.mcpServers.SetHost(host)
        } else {
            a.mcpServers.SetHost("")
        }
        // MCP は常に Refresh (local でも remote でも動作する)
    }

    // ... existing projectPath resolution & Refresh ...
    // 注: plugin の Refresh は isRemote=true 時は skip (plugin は disable 維持)
}
```

`guardRemoteOp("MCP editing")` を `MCPToggleDenied` から削除。`guardRemoteOp("Plugin editing")` は 7 つの plugin entry point に残したまま。

### Render layer

- `renderPluginPanel` / `renderPluginPreview`: `PluginTabPlugins` / `PluginTabMarketplace` case は `pluginState.remoteDisabled` で placeholder を表示 (既存の挙動を残す)
- `PluginTabMCP` は `renderMCPList` / `renderMCPPreview` に dispatch (これは Phase 1 の通り)
- `renderMCPList` / `renderMCPPreview`: `mcpState.remoteDisabled` ガードを **削除** (MCP は remote でも通常表示)
- `renderMCPPreview` の `mcpState.remoteDisabled` 分岐も削除

## 実装ステップ

### Step 1: SSH ヘルパー追加
`internal/mcp/ssh.go` 新規作成 (`sshReadFile`, `sshWriteFile`, `shellQuote`, `splitHostPort` if needed for IPv6).

### Step 2: Manager に SSH 分岐を追加
`internal/mcp/manager.go`:
- `host string` field
- `SetHost(host string)` method
- `Refresh`: host 分岐
- `ToggleDenied`: host 分岐

JSON 処理を helper 化 (`parseClaudeJSON`, `parseDeniedServers`, `buildDeniedJSON`) して local / SSH 両方で再利用。

### Step 3: Interface 拡張と adapter
`internal/gui/mcp_state.go`: `MCPProvider.SetHost` 追加
`cmd/lazyclaude/root.go`: `mcpAdapter.SetHost` 実装

### Step 4: GUI wiring
`internal/gui/app_actions.go`:
- `syncPluginProject` で MCP 側は SetHost 経由で remote 対応、plugin 側は remoteDisabled 維持
- `MCPToggleDenied` の `guardRemoteOp` 削除
- `MCPRefresh` の `guardRemoteOp` 削除
- plugin 側の 5 つの guard (Install/Uninstall/Toggle/Update/Refresh) は **残す**

### Step 5: Render 修正
`internal/gui/render_mcp.go`:
- `renderMCPList` / `renderMCPPreview` の `mcpState.remoteDisabled` guard を削除

`internal/gui/render_plugins.go`:
- PluginTabMCP dispatch は無変更 (そのまま `renderMCPList` / `renderMCPPreview` に委譲)
- PluginTabPlugins / PluginTabMarketplace の placeholder guard は **残す**

### Step 6: Tests
`internal/mcp/ssh_test.go` 新規: `shellQuote`, `splitHostPort` の unit test
`internal/mcp/manager_test.go`: host 設定時の Refresh / ToggleDenied は mock SSH で検証 (SSH 実行は避けて `sshCommand` フィールドを injection 可能にする等の refactor が必要な可能性)
`internal/gui/plugin_remote_disabled_test.go`: 既存 test の修正 (MCP 側は remote でも動作するよう更新、plugin 側は Phase 1 挙動維持)

### Step 7: Verification
1. go build ./... clean
2. go vet ./... clean
3. go test -race 全 PASS
4. /go-review → CRITICAL/HIGH ゼロ
5. /codex --enable-review-gate → APPROVED
6. 手動検証 (要ユーザー):
   - Remote session で MCP toggle → 実 remote host の settings.local.json が更新される
   - Remote session で MCP list が remote host の設定を表示する
   - Local session の MCP 動作 (regression)
   - Remote session で plugin 操作 → 引き続き "Plugin editing on remote is not supported" メッセージ
   - Local session の plugin 動作 (regression)

## Out of Scope

- Plugin の SSH 対応 (Phase 3 で扱う。`claude plugins` CLI を SSH 経由で実行する必要があり、install/uninstall/update/toggle/refresh の 5 エントリポイントそれぞれに対応する)
- Phase 3 の plugin 完全対応
- 既存の Bug 3 / Bug 4 に関わる変更
- APIVersion bump (daemon wire protocol は変更しない)

## Files Changed

| ファイル | 変更 |
|---------|------|
| `internal/mcp/ssh.go` (新規) | sshReadFile / sshWriteFile / shellQuote |
| `internal/mcp/manager.go` | host field, SetHost, Refresh/ToggleDenied の SSH 分岐, helper 化 |
| `internal/mcp/config.go` | parseClaudeJSON / buildDeniedJSON helper (既存があれば流用) |
| `internal/gui/mcp_state.go` | MCPProvider interface に SetHost 追加 |
| `cmd/lazyclaude/root.go` | mcpAdapter.SetHost 実装 |
| `internal/gui/app_actions.go` | syncPluginProject の MCP 側 SetHost 対応、MCPToggleDenied/MCPRefresh の guard 削除 |
| `internal/gui/render_mcp.go` | remoteDisabled guard 削除 |
| `internal/gui/render_plugins.go` | (無変更) |
| `internal/gui/plugin_remote_disabled_test.go` | MCP 関連 test を update |
| `internal/mcp/ssh_test.go` (新規) | SSH helper unit test |

## Risk Assessment

- **Low**: MCP manager 変更は既存の local path を保持しつつ SSH path を追加するだけ
- **Medium**: SSH command 実行は mock しづらい。`sshCommand` / `sshRunner` interface を injection する refactor が必要
- **Medium**: `~/.claude.json` が remote で既に存在するか、format が想定通りかは user 環境依存。conservative な JSON 処理 (unknown field 無視、invalid JSON 時は空 parse) で対応
- **Low**: Plugin 側は Phase 1 の挙動をそのまま残すので regression 懸念は小

## Open Questions

1. Plugin も Phase 2 で対応すべきか (コード量大、CLI 依存強) — PM は MCP 優先・plugin 延期を提案、ユーザー最終判断
2. `~/.claude.json` が remote host 上に存在しない場合、作成するか error とするか — worker 実装時に conservative に作成方針で進めてよいか
