package gui

import (
	"bytes"
	"fmt"

	"github.com/KEMSHlM/lazyclaude/internal/gui/presentation"
	"github.com/KEMSHlM/lazyclaude/internal/session"
)

// RenderSessionListForTest renders the session list to a string buffer for testing.
// It replicates the logic of renderSessionList without requiring a gocui.View.
func RenderSessionListForTest(items []SessionItem, cursor int) string {
	if len(items) == 0 {
		return ""
	}

	var buf bytes.Buffer
	for i, item := range items {
		var icon string
		switch {
		case item.Status == "Dead":
			icon = " " + presentation.IconDead
		case item.Status == "Orphan":
			icon = " " + presentation.IconOrphan
		case item.Activity == "pending":
			icon = " " + presentation.IconPending
		case item.Status == "Running":
			icon = " " + presentation.IconRunning
		case item.Status == "Detached":
			icon = " " + presentation.IconDetached
		}

		name := item.Name
		if item.Host != "" {
			name = presentation.FgPurple + item.Host + presentation.Reset + ":" + name
		}
		if session.IsWorktreePath(item.Path) {
			name = presentation.IconWorktree + " " + name
		}
		if item.Role == "pm" {
			name = presentation.IconPM + " " + name
		}

		_ = i
		fmt.Fprintf(&buf, "%-20s%s\n", name, icon)
	}
	return buf.String()
}
