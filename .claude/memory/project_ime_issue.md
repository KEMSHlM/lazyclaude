---
name: ime-cursor-issue-open
description: Japanese IME composition appears in fullscreen mode - unsolved, needs different approach
type: project
---

Japanese IME composition window appears over lazyclaude fullscreen content and auto-commits after certain characters. User uses Kitty terminal.

**What was tried and FAILED:**
- g.Cursor = false: IME still appears (Kitty ignores cursor hidden state for IME)
- g.Cursor = true + SetCursor to specific position: cursor goes outside view
- fmt.Fprint(os.Stdout, "\033[?25l"): flickers because gocui overrides every frame
- input-sink (separate 1x1 Editable view): cursor position didn't move IME due to gocui's x0+cx+1 calculation putting cursor off-screen
- Moving input-sink to various positions: IME still appeared at wrong place

**Why:** gocui couples cursor visibility with cursor positioning in its draw() function. tcell's draw() moves cursor during cell rendering, and the final cursor position (even when hidden) is where the IME appears. The root issue spans gocui/tcell/Kitty layers.

**How to apply:** This issue is still open. Solving it likely requires patching gocui to separate cursor positioning from visibility, or finding a Kitty-specific escape sequence to suppress IME.
