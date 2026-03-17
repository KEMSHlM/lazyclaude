package integration_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTmux_DiffCommand_InPane(t *testing.T) {
	bin := ensureBinary(t)
	h := newTmuxHelper(t)

	h.startSession("diff-test", 120, 40)

	oldFile := testdataPath(t, "old.go")
	newFile := testdataPath(t, "new.go")

	// Run lazyclaude diff in the pane
	// Current stub prints "diff popup - not yet implemented" and exits
	h.sendKeys("diff-test",
		fmt.Sprintf("%s diff --window lc-test --old %s --new %s", bin, oldFile, newFile),
		"Enter")

	found := h.waitForText("diff-test", "diff popup", 5000000000) // 5s
	assert.True(t, found, "expected to see 'diff popup' in pane output")
}

func TestTmux_ToolCommand_InPane(t *testing.T) {
	bin := ensureBinary(t)
	h := newTmuxHelper(t)

	h.startSession("tool-test", 120, 40)

	h.sendKeys("tool-test",
		fmt.Sprintf("%s tool", bin),
		"Enter")

	found := h.waitForText("tool-test", "tool popup", 5000000000)
	assert.True(t, found, "expected to see 'tool popup' in pane output")
}

func TestTmux_ServerCommand_StartsAndStops(t *testing.T) {
	bin := ensureBinary(t)
	h := newTmuxHelper(t)

	h.startSession("server-test", 120, 40)

	// Start server in background
	h.sendKeys("server-test",
		fmt.Sprintf("%s server --port 0 &", bin),
		"Enter")

	// Give it time to start
	found := h.waitForText("server-test", "MCP server", 5000000000)
	require.True(t, found, "expected MCP server started message")

	// Kill it
	h.sendKeys("server-test", "kill %1", "Enter")
}

func TestTmux_HelpCommand_InPane(t *testing.T) {
	bin := ensureBinary(t)
	h := newTmuxHelper(t)

	h.startSession("help-test", 120, 40)

	h.sendKeys("help-test",
		fmt.Sprintf("%s --help", bin),
		"Enter")

	found := h.waitForText("help-test", "terminal UI", 5000000000)
	assert.True(t, found, "expected help text in pane output")
}
