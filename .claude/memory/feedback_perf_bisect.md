---
name: perf-debugging-approach
description: Performance debugging should use git bisect with binary builds, not code-level analysis
type: feedback
---

When lazyclaude feels slow, use git bisect to identify the exact commit rather than analyzing code diffs or adding perf logging. The bisect approach found the root cause (9bc6193 display-message cursor sync) in minutes, after hours of code analysis failed.

**Why:** Code-level analysis missed the real cause because the problematic `tmux display-message` call looked innocuous (~5ms), but it doubled every capture-pane cycle. The perf logging approach (output/layout/capture counters) was useful for ruling out event loop issues but couldn't identify the subprocess overhead.

**How to apply:** When the user reports sluggishness, immediately build checkpoint binary + HEAD binary for A/B comparison. If confirmed regression, bisect with `git checkout <hash> && go build -o ~/.local/bin/lazyclaude ./cmd/lazyclaude/`.
