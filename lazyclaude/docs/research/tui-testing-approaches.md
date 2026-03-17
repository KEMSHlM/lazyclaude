# TUI Testing Approaches for gocui Inside tmux display-popup

## Summary

Testing a gocui TUI running inside a `tmux display-popup`, with scripted key inputs and output
assertions in CI (GitHub Actions), is feasible. The most practical approach is the
**tmux scripting pattern** (detached session + `send-keys` + `capture-pane`), which requires no
display server, works on GitHub Actions ubuntu runners out of the box, and can assert on raw
terminal cell content. A more robust Go-native alternative is to bypass tmux for the TUI
process itself and use a **PTY + vt100 emulator** (e.g., `ActiveState/termtest` or
`Netflix/go-expect` + `hinshun/vt10x`) to drive gocui directly, giving programmatic,
synchronous assertions. VHS is not suitable for assertion-based CI testing. `goexpect` is
archived and should not be used.

---

## 1. VHS (charmbracelet/vhs)

### What it is

VHS scripts terminal sessions as `.tape` files (commands: `Type`, `Sleep`, `Enter`,
`Screenshot`, etc.) and renders them as GIF/MP4. It uses a real PTY internally via a headless
terminal renderer.

### Testing capability

VHS has a `testing.go` in its source, but the testing mechanism is **golden file comparison
only**: you capture `.txt` or `.ascii` output and diff it against a stored snapshot. There are
no inline `assert` commands in the tape language. VHS cannot express "assert line 3 contains
'foo'" as a first-class operation.

- Golden file workflow: run tape, save `.txt` output to git, future runs diff against it.
- Screenshot output (`Screenshot foo.png`) supports visual snapshot diffs.
- CI integration via official `charmbracelet/vhs-action` GitHub Action.

### tmux interaction

VHS does not interact with an existing tmux session. It spawns its own PTY-backed virtual
terminal. You cannot point it at a running `display-popup`.

### Verdict for this use case

Not suitable. VHS is for demo recording and coarse golden-file regression, not for
programmatic assertion-based integration tests of a TUI process running inside tmux.

---

## 2. tmux Scripting (detached session + send-keys + capture-pane)

### Core pattern

```bash
# Start a detached tmux session (no display required)
tmux -f /dev/null new-session -d -s test -x 220 -y 50

# Launch TUI in the pane
tmux send-keys -t test 'your-tui-binary' Enter

# Wait for output (poll with timeout)
deadline=$((SECONDS + 5))
while ! tmux capture-pane -p -t test | grep -q 'Expected Text'; do
  [ $SECONDS -ge $deadline ] && { echo "timeout"; exit 1; }
  sleep 0.05
done

# Send a key
tmux send-keys -t test 'j' ''   # no Enter for single-key TUI input

# Assert on pane content
OUTPUT=$(tmux capture-pane -p -t test)
echo "$OUTPUT" | grep -q 'Expected Result' || { echo "assertion failed"; exit 1; }

# Cleanup
tmux kill-session -t test
```

### Headless / CI

tmux does not require a display server (X11, Wayland, etc.). It operates as a pure
terminal multiplexer over a PTY. On GitHub Actions ubuntu-latest runners, tmux is available in
the default image and runs without any additional setup. No `Xvfb` or display configuration
is needed.

Confirmed: tmux works on headless Linux VMs via SSH with no display server present.

### display-popup specifics

`tmux display-popup` runs a command in a floating overlay attached to the **current tmux
client**. In a test environment there is no "current client" in the traditional sense,
but you can:

1. Attach a dummy client: `tmux attach-session -t test -d` from a background process (fragile).
2. **Skip the popup layer for unit testing**: invoke the gocui binary directly in a pane
   rather than through `display-popup`. The popup is a presentation concern; the TUI behavior
   is what you want to test.
3. For integration tests that specifically validate the popup launch path, use a nested tmux
   approach: outer tmux session (test driver) starts inner tmux server (`tmux -L inner`),
   triggers `display-popup` inside it, then captures the inner pane.

### Limitations

- **Timing is inherently async**: `capture-pane` reads a snapshot; you must poll until the
  expected content appears. Race conditions require careful `sleep`/retry logic.
- **Raw terminal content**: `capture-pane -p` returns the rendered visible cell grid, including
  ANSI escape artifacts if `-e` flag is not used. Use `-e` to get escape sequences, or plain
  `-p` for rendered text.
- **Pane size matters**: gocui layout depends on terminal dimensions; set `-x` and `-y` when
  creating the session to match expected layout.
- `capture-pane` has a known issue where it can miss the first line in some versions
  (tmux/tmux issue #1663). Use `-S 0` to start from line 0 explicitly.

### Key flags

| Flag | Purpose |
|------|---------|
| `tmux -f /dev/null` | Use empty config (test isolation) |
| `new-session -d` | Detached, no controlling terminal needed |
| `-x 220 -y 50` | Set pane dimensions for predictable layout |
| `send-keys -t pane ''` | Send keys without Enter (for TUI navigation) |
| `capture-pane -p -t pane` | Print pane content to stdout |
| `capture-pane -e` | Include ANSI escape sequences |
| `capture-pane -S -` | Capture full history, not just visible |

---

## 3. expect / goexpect

### goexpect (google/goexpect)

- Spawns processes with a real PTY, matches output with regexes, sends input strings.
- **Archived on 2023-02-07. No longer maintained. Read-only.**
- Do not use for new projects.

### Netflix/go-expect

- Active fork/alternative. Provides `Console` type that multiplexes stdin/stdout through a PTY.
- Can be combined with `hinshun/vt10x` or `ActiveState/vt10x` to interpret ANSI escape codes
  and match against rendered terminal cell content rather than raw bytes.
- API: `console.SendLine("text")`, `console.ExpectString("expected output")`.
- Suitable for driving CLI tools; works for simple TUI apps but lacks synchronization
  primitives for complex event-driven UIs (no "wait until render is complete" hook).

### ActiveState/termtest

The most production-ready Go library for this class of problem.

- Built on top of Netflix/go-expect + vt100 terminal emulation.
- Maintains a virtual terminal state (scroll buffer, cell grid) that mirrors what a user would
  see in a real terminal.
- Handles terminal line wrapping correctly for `Expect()` matching.
- Developed for CI testing (ActiveState's state tool) and explicitly supports headless
  environments.
- Cross-platform: Linux, macOS, Windows (ConPTY on Windows).
- API: `term.SendLine(str)`, `term.Expect(str)`, `term.ExpectExitCode(n)`.
- Does not require tmux; drives the process directly via PTY.

**Limitation for this use case**: termtest drives a process it spawns directly. It cannot
attach to a gocui process already running inside a `tmux display-popup`. You would need to
invoke the gocui binary directly, not through tmux.

---

## 4. Other Approaches

### script / unbuffer / faketty (PTY allocation in CI)

`script -q -c "command" /dev/null` allocates a PTY for a command that would otherwise see a
non-TTY stdin. `unbuffer` (from `expect` package) does the same. These are useful when a
process checks `isatty()` and behaves differently without a terminal, but they do not provide
assertion capabilities on their own.

### creack/pty (Go PTY package)

The `creack/pty` package is the standard Go library for allocating Unix PTYs. It lets you:
- Start a command with `pty.Start(cmd)` and get read/write access to the PTY file descriptor.
- Read raw bytes from the PTY output and write keystrokes to the PTY input.

Used by go-expect, termtest, and others internally. For test code you would layer a terminal
emulator (vt10x) on top to interpret escape sequences.

### hinshun/vt10x + creack/pty (DIY approach)

Combining these two gives a pure-Go TUI test harness:
1. Start gocui binary with `pty.Start()`.
2. Pipe PTY output into a `vt10x.VT` instance which maintains the virtual screen state.
3. Query cell content via `vt10x.State.Cell(x, y)`.
4. Write keystrokes to the PTY to drive the TUI.

This is essentially what `termtest` does under the hood. Building it yourself gives maximum
control but requires significant boilerplate.

### tcell SimulationScreen (lazygit's approach)

lazygit uses tcell's `SimulationScreen` for integration tests. tcell (the library underlying
lazygit) has a built-in fake screen implementation that runs entirely in-process without a
real terminal. Tests call the `GuiDriver` interface (`PressKey()`, `CurrentContext()`) which
injects synthetic events into the event loop.

**This approach requires the TUI to be built on tcell.** gocui uses its own rendering layer
(also built on termbox-go historically). Unless gocui exposes a simulation/testing interface,
this pattern is not directly applicable without forking or wrapping gocui itself.

### microsoft/tui-test

A Node.js-based TUI testing framework using PTY + xterm.js (same renderer as VSCode).

- API: `terminal.write("text")`, `terminal.getByText("expected")`, `toMatchSnapshot()`.
- Auto-wait before assertions.
- Works in CI (GitHub Actions).
- Language-agnostic: can test any binary, including Go TUI apps.
- Drawback: requires Node.js in the test environment; the test code is written in
  JavaScript/TypeScript, not Go.

---

## 5. Virtual Framebuffer (Xvfb)

tmux does not need Xvfb. Xvfb is for X11 GUI applications (GTK, Qt, etc.). Terminal
applications running in tmux operate over PTY and have no dependency on a display server.
Setting `TERM=xterm` or `TERM=xterm-256color` in the environment is sufficient for CI. No
`Xvfb` setup is needed.

---

## Feasibility Assessment: gocui TUI in tmux display-popup, CI (GitHub Actions)

| Approach | Works in GH Actions | Drives display-popup | Assertion style | Go-native | Recommended |
|----------|--------------------|-----------------------|-----------------|-----------|-------------|
| tmux send-keys + capture-pane | Yes (tmux pre-installed) | Yes, with workarounds | String/regex grep on pane text | Shell/bash | Yes (simplest) |
| ActiveState/termtest | Yes (no display needed) | No (drives process directly) | Synchronous Expect() | Yes | Yes (for direct gocui testing) |
| Netflix/go-expect + vt10x | Yes | No | Regex match on terminal content | Yes | Yes (more control) |
| microsoft/tui-test | Yes | No (external process) | getByText, snapshot | No (Node.js) | Maybe (if Node.js acceptable) |
| VHS | Yes (vhs-action) | No | Golden file diff only | No | No |
| goexpect | Yes | No | Regex | Yes | No (archived) |
| tcell SimulationScreen | Yes | N/A (in-process) | Programmatic state | Yes | Only if migrating to tcell |

### Recommended architecture

For a project that needs to test the full stack including the tmux display-popup launch path:

1. **Unit/component tests**: Drive gocui directly using `creack/pty` + `vt10x` (or
   `ActiveState/termtest`). No tmux involvement. Tests run as standard `go test` in CI.
   Assert on rendered cell content.

2. **Integration tests**: Use the tmux scripting pattern. In CI, install tmux (or use the
   pre-installed version on ubuntu-latest), start a detached test session, invoke the popup
   path, use `capture-pane` to assert on visible output, use `send-keys` for input. Implement
   a `wait_for_text` shell function with a timeout to avoid polling races.

3. **Do not use display-popup in tests if it can be avoided.** The popup is a presentation
   layer. The gocui application logic can be invoked directly in a pane. Test the popup launch
   separately with a minimal smoke test.

---

## Key References

- charmbracelet/vhs: https://github.com/charmbracelet/vhs
- VHS testing.go (golden file comparison): https://github.com/charmbracelet/vhs/blob/main/testing.go
- ActiveState/termtest (cross-platform PTY test library): https://github.com/ActiveState/termtest
- Netflix/go-expect: https://github.com/Netflix/go-expect
- google/goexpect (archived): https://github.com/google/goexpect
- creack/pty: https://github.com/creack/pty
- hinshun/vt10x: https://github.com/hinshun/vt10x
- ActiveState/vt10x: https://github.com/ActiveState/vt10x
- microsoft/tui-test: https://github.com/microsoft/tui-test
- lazygit integration test README: https://github.com/jesseduffield/lazygit/blob/master/pkg/integration/README.md
- Jesse Duffield - Lessons Learned Revamping Lazygit's Integration Tests: https://jesseduffield.com/IntegrationTests/
- Waleed Khan - Testing TUI Apps: https://blog.waleedkhan.name/testing-tui-apps/
- Tao of tmux - Scripting: https://tao-of-tmux.readthedocs.io/en/latest/manuscript/10-scripting.html
- tmux capture-pane first line bug: https://github.com/tmux/tmux/issues/1663
