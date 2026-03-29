# Keybind Help (`?` key) -- Telescope Style

## Overview

`?` キーで Telescope スタイルのキーバインドヘルプを表示する。
左ペイン: fzf フィルタリングリスト、右ペイン: ドキュメントプレビュー。

## Requirements

1. `?` で現在のモード（Sessions, Plugins, Logs）のキーバインドヘルプを表示
2. FullScreen モードでは不要
3. 現在のモードの全キーバインド表示（HintLabel 空のナビキーも含む）
4. Telescope レイアウト: 左ペイン = fzf リスト、右ペイン = ドキュメントプレビュー
5. ドキュメントは埋め込み markdown ファイル。ActionDef から `DocSection` でセクション紐付け
6. 通知ポップアップ到着でヘルプは suspend、dismiss 後に復帰
7. fzf 入力フィールドへのフォーカス復帰を保証

## Architecture Changes

| File | Change |
|------|--------|
| `keymap/types.go` | `Description`, `DocSection` を ActionDef に追加 |
| `keymap/registry.go` | `BindingsForScopeTab()` 追加、Description 設定 |
| `keymap/doc.go` (new) | embed で keybinds.md を読み込み、セクション検索 |
| `keymap/keybinds.md` (new) | Markdown ドキュメント |
| `dialog.go` | `DialogKeybindHelp` 追加、ヘルプ用 state フィールド |
| `keybind_help.go` (new) | ヘルプオーバーレイ layout + rendering + フィルタ |
| `keybindings.go` | `?` 登録 + ヘルプ view 用バインディング |
| `app_actions.go` | `ShowKeybindHelp()` / `CloseKeybindHelp()` |
| `keyhandler/actions.go` | AppActions に `ShowKeybindHelp()` 追加 |
| `keyhandler/global.go` | `ActionShowKeybindHelp` ハンドル |

## Phases

### Phase 1: Data Layer

**1.1** `types.go` に `Description` と `DocSection` フィールド追加

**1.2** `registry.go` に `BindingsForScopeTab()` 追加。全 ActionDef に Description 設定

**1.3** `doc.go` + `keybinds.md` 作成。`//go:embed` でドキュメント埋め込み、セクション抽出関数

### Phase 2: Dialog Integration

**2.1** `dialog.go` に `DialogKeybindHelp` 追加。HelpItems, HelpCursor, HelpFilter 等のフィールド

### Phase 3: Keybind Registration + Dispatch

**3.1** `ActionShowKeybindHelp` を types.go に追加

**3.2** Registry に `?` バインディング登録

**3.3** AppActions に `ShowKeybindHelp()` 追加

**3.4** GlobalHandler で dispatch

**3.5** gocui keybindings に `?` 登録

### Phase 4: Help Overlay Layout + Rendering

**4.1** `keybind_help.go` 新規作成
- gocui views: `keybind-help-input`, `keybind-help-list`, `keybind-help-preview`, `keybind-help-hint`
- オーバーレイ: 80% 幅, 70% 高さ, 中央配置
- 左ペイン 40%, 右ペイン 60%
- フィルタリング: Description, HintLabel, キー表示で case-insensitive substring match

**4.2** `ShowKeybindHelp()` 実装 (app_actions.go)
- 現在の scope + tab から全バインディング取得
- Global scope も追加

**4.3** ヘルプ view のキーバインディング登録
- Esc: 閉じる, j/k: カーソル移動
- `helpInputEditor`: キー入力ごとにフィルタ実行

### Phase 5: Popup Suspension

**5.1** 通知到着時にヘルプを suspend、dismiss 後に resume
- `showToolPopup()` でヘルプ active 時に suspend
- popup 全 dismiss 後に resume + フォーカス復帰

**5.2** `layoutMain` のフォーカス優先順位に統合

### Phase 6: Tests

- doc section parser テスト
- filter logic テスト
- BindingsForScopeTab テスト
- dialog lifecycle テスト
- mock_actions 更新

## Risks

- フォーカス復帰: `dialogFocusView()` 既存パターンで対応
- global rune 衝突: Editable view が focus 時は view-specific binding 優先
- `?` 既存衝突: なし
