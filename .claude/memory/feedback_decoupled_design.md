---
name: feedback_decoupled_design
description: Loosely coupled design required - no hacks, no nested quoting, each layer has one job
type: feedback
---

All components must be loosely coupled. No hacks, no duct-tape solutions.

**Why:** User became very frustrated with accumulating hacky implementations (nested quoting, multiple failed SSH command approaches, mixing concerns between shell and Go). Quick fixes that pile up create unmaintainable code.

**How to apply:**
- Shell handles shell concerns (env vars, process detection). Go handles logic.
- When crossing boundaries (shell/Go, local/remote), use clean serialization (base64, files) instead of nested quoting.
- SSH remote commands: write plain bash script to file, base64-encode it for SSH. Never nest shell.Quote() inside quoted strings.
- Tests must simulate human operations (ssh into remote, press keys) not inject raw env vars.
- `scripts/lazyclaude-launch.sh` is the single entry point for both tmux plugin and standalone.
