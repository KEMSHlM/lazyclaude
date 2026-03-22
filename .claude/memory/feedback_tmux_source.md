---
name: tmux-source-no-reset
description: tmux source does not reset keybindings, only adds/overwrites
type: feedback
---

`tmux source ~/.config/tmux/tmux.conf` does NOT reset keybindings. It only adds or overwrites. Old bindings from previous plugin loads persist.

**Why:** tmux has no "reset all bindings" command. `source` re-runs the config which adds bindings on top of existing ones.

**How to apply:** To fully reset keybindings, the tmux server must be restarted. When debugging keybinding issues, use `tmux list-keys | grep <key>` to check what's actually bound, and `tmux unbind-key` to remove stale bindings manually.
