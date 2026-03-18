# Issue: Normal Mode Navigation (hjkl cursor + scrollback)

## Current State

- Normal mode exists (Ctrl+\ to enter, i to return to insert)
- q exits full-screen, popup y/a/n works
- j/k are no-op in normal mode (keybinding fires but does nothing)
- h/l are not bound in normal mode
- Mouse scroll works in insert mode only (SetOrigin on captured content)

## Problem: capture-pane Limitation

`capture-pane -ep` captures only the **visible pane content** (~70 lines).
Adding `-S -` captures the full scrollback buffer, but:
- Returns thousands of lines on long sessions
- gocui re-renders the entire buffer every frame (Clear + Fprint)
- Makes the TUI extremely slow (every capture + render processes all lines)

This makes vim-style hjkl cursor movement + scrollback viewing impractical
with the current capture-pane approach.

## Proposed Solution

### Option A: On-demand scrollback (recommended)

1. **Insert mode**: capture-pane without `-S` (visible only, fast)
2. **Normal mode enter**: capture-pane with `-S -1000` ONCE, cache the full content
3. **Normal mode hjkl**: move cursor/scroll within the cached content (no subprocess)
4. **Normal mode exit (i)**: discard cached scrollback, return to fast capture

This separates the fast insert-mode path from the full-content normal-mode path.
The cached scrollback is only fetched once on mode entry, not every frame.

### Option B: tmux copy-mode integration

Send tmux into copy-mode when entering normal mode:
```
tmux copy-mode -t lazyclaude:<window>
```
Then forward hjkl as tmux copy-mode navigation keys.
tmux handles scrollback natively. Exit copy-mode on `i` or `q`.

Downside: capture-pane of copy-mode output may differ from normal output.

### Option C: Virtual terminal emulator

Parse the raw pane output (via pipe-pane or control mode %output) through
a Go terminal emulator library (e.g., go-vt10x). Maintain our own scrollback
buffer in memory. No subprocess per render.

Downside: Complex implementation, must handle all ANSI escape sequences.

## Scope

- hjkl cursor movement with visible cursor
- Scrollback viewing (past output)
- Mouse scroll in normal mode
- Future: V mode (visual selection + copy)

## Related

- `docs/dev/popup-redesign-plan.md` Phase 3.6
- `memory/project_visual_mode.md`
