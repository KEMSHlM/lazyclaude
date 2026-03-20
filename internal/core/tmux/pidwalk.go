package tmux

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// FindWindowForPid walks the process tree of pid and matches against
// tmux pane PIDs to find the window containing the process.
func FindWindowForPid(ctx context.Context, c Client, pid int) (*WindowInfo, error) {
	panes, err := c.ListPanes(ctx, "")
	if err != nil {
		return nil, fmt.Errorf("list panes: %w", err)
	}
	return FindWindowForPidWithPanes(ctx, c, pid, panes, func(p int) (int, error) {
		return parentPid(ctx, p)
	})
}

// FindWindowForPidWithPanes is like FindWindowForPid but uses a pre-fetched
// pane list and a parentPid lookup function (for testability).
func FindWindowForPidWithPanes(
	ctx context.Context,
	c Client,
	pid int,
	panes []PaneInfo,
	getParent func(int) (int, error),
) (*WindowInfo, error) {
	panePidToWindow := make(map[int]string, len(panes))
	for _, p := range panes {
		panePidToWindow[p.PID] = p.Window
	}

	current := pid
	for i := 0; i < 20; i++ {
		if windowID, ok := panePidToWindow[current]; ok {
			return findWindowByID(ctx, c, windowID)
		}
		parent, err := getParent(current)
		if err != nil || parent <= 1 {
			break
		}
		current = parent
	}

	return nil, nil
}

func findWindowByID(ctx context.Context, c Client, windowID string) (*WindowInfo, error) {
	// List all windows across all sessions to find the one with this ID
	out, err := c.ShowMessage(ctx, windowID, "#{session_name}")
	if err != nil {
		return nil, fmt.Errorf("show-message for window %s: %w", windowID, err)
	}
	session := strings.TrimSpace(out)
	if session == "" {
		return nil, nil
	}

	windows, err := c.ListWindows(ctx, session)
	if err != nil {
		return nil, err
	}
	for _, w := range windows {
		if w.ID == windowID {
			return &w, nil
		}
	}
	return nil, nil
}

// parentPid returns the parent PID of a process using ps.
func parentPid(ctx context.Context, pid int) (int, error) {
	out, err := exec.CommandContext(ctx, "ps", "-o", "ppid=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}