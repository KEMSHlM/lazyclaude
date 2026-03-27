package presentation_test

import (
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/gui/presentation"
	"github.com/stretchr/testify/assert"
)

func TestIconPending_ContainsDiamond(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconPending, "◆", "IconPending should contain diamond character")
}

func TestIconPending_IsMagenta(t *testing.T) {
	t.Parallel()
	// Magenta ANSI escape: \x1b[35m
	assert.Contains(t, presentation.IconPending, "\x1b[35m", "IconPending should use magenta color")
}

func TestIconPending_ResetsColor(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconPending, "\x1b[0m", "IconPending should reset color after diamond")
}

func TestIconPending_IsDistinctFromDetached(t *testing.T) {
	t.Parallel()
	// IconDetached is gray diamond, IconPending should be magenta diamond
	// They share the diamond character but differ in color
	assert.NotEqual(t, presentation.IconPending, presentation.IconDetached,
		"IconPending and IconDetached should differ (different colors)")
}

func TestIconPM_Exists(t *testing.T) {
	t.Parallel()
	assert.NotEmpty(t, presentation.IconPM, "IconPM should be defined")
}

func TestIconPM_ContainsPMLabel(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconPM, "[PM]", "IconPM should contain [PM] label")
}

func TestIconPM_IsPurple(t *testing.T) {
	t.Parallel()
	// FgPurple is \x1b[38;5;141m
	assert.Contains(t, presentation.IconPM, "\x1b[38;5;141m", "IconPM should use purple color")
}

func TestIconPM_ResetsColor(t *testing.T) {
	t.Parallel()
	assert.Contains(t, presentation.IconPM, "\x1b[0m", "IconPM should reset color after label")
}

func TestIconPM_IsDistinctFromWorktree(t *testing.T) {
	t.Parallel()
	assert.NotEqual(t, presentation.IconPM, presentation.IconWorktree,
		"IconPM and IconWorktree should be different icons")
}
