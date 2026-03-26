package gui_test

import (
	"strings"
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/gui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// renderSessionListOutput calls the internal renderSessionList via a headless
// app layout cycle and captures the session-list view content.
// We use the exported RenderSessionListForTest helper exposed from render.go.
func TestRenderSessionList_PendingActivity_ShowsMagentaDiamond(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "worker", Status: "Running", Activity: "pending"},
	}
	out := gui.RenderSessionListForTest(items, 0)

	// Magenta diamond character should be present (◆)
	assert.Contains(t, out, "◆", "pending session should show diamond icon")
	// Magenta ANSI escape \x1b[35m
	assert.Contains(t, out, "\x1b[35m", "pending icon should be magenta")
	// Should NOT contain the green running circle
	assert.NotContains(t, out, "\x1b[32m●", "pending session should NOT show green running icon")
}

func TestRenderSessionList_Running_ShowsGreenCircle(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "app", Status: "Running", Activity: ""},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.Contains(t, out, "●", "running session should show filled circle")
	assert.Contains(t, out, "\x1b[32m", "running icon should be green")
}

func TestRenderSessionList_Dead_ActivityIgnored(t *testing.T) {
	// Even if Activity is set, Dead status takes priority
	items := []gui.SessionItem{
		{ID: "s1", Name: "dead-session", Status: "Dead", Activity: "pending"},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.Contains(t, out, "×", "dead session should show red cross icon")
	assert.NotContains(t, out, "\x1b[35m", "dead session should NOT show magenta pending icon")
}

func TestRenderSessionList_Orphan_ActivityIgnored(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "orphan-session", Status: "Orphan", Activity: "pending"},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.Contains(t, out, "○", "orphan session should show empty circle icon")
	assert.NotContains(t, out, "\x1b[35m", "orphan session should NOT show magenta pending icon")
}

func TestRenderSessionList_EmptyActivity_FallsBackToStatus(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "app", Status: "Running", Activity: ""},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.Contains(t, out, "●", "empty activity should fall back to status-based icon")
}

func TestRenderSessionList_MultipleSessions_OnlyPendingGetsDiamond(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "normal", Status: "Running", Activity: ""},
		{ID: "s2", Name: "blocked", Status: "Running", Activity: "pending"},
		{ID: "s3", Name: "dead-one", Status: "Dead", Activity: ""},
	}
	out := gui.RenderSessionListForTest(items, 0)

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	require.Len(t, lines, 3)

	assert.Contains(t, lines[0], "●", "first session (normal) should have green circle")
	assert.NotContains(t, lines[0], "◆", "first session should not have diamond")

	assert.Contains(t, lines[1], "◆", "second session (pending) should have diamond")
	assert.Contains(t, lines[1], "\x1b[35m", "second session diamond should be magenta")

	assert.Contains(t, lines[2], "×", "third session (dead) should have red cross")
}

func TestRenderSessionList_NoSessions_EmptyActivityFieldIsZeroValue(t *testing.T) {
	// Zero value of SessionItem.Activity is empty string (no pending)
	item := gui.SessionItem{ID: "s1", Name: "app", Status: "Running"}
	assert.Equal(t, "", item.Activity, "Activity zero value should be empty string")
}

func TestRenderSessionList_PMRole_ShowsPMBadge(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "pm-session", Status: "Running", Role: "pm"},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.Contains(t, out, "[PM]", "PM role session should show [PM] badge")
}

func TestRenderSessionList_PMRole_BadgeIsPurple(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "pm-session", Status: "Running", Role: "pm"},
	}
	out := gui.RenderSessionListForTest(items, 0)

	// FgPurple is \x1b[38;5;141m
	assert.Contains(t, out, "\x1b[38;5;141m", "PM badge should use purple color")
}

func TestRenderSessionList_NonPMRole_NoPMBadge(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "regular", Status: "Running", Role: ""},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.NotContains(t, out, "[PM]", "non-PM session should not show [PM] badge")
}

func TestRenderSessionList_WorkerRole_NoPMBadge(t *testing.T) {
	items := []gui.SessionItem{
		{ID: "s1", Name: "worker-session", Status: "Running", Role: "worker"},
	}
	out := gui.RenderSessionListForTest(items, 0)

	assert.NotContains(t, out, "[PM]", "worker role session should not show [PM] badge")
}

func TestSessionItem_RoleField_IsString(t *testing.T) {
	t.Parallel()
	item := gui.SessionItem{Role: "pm"}
	assert.Equal(t, "pm", item.Role)

	item2 := gui.SessionItem{}
	assert.Equal(t, "", item2.Role)
}
