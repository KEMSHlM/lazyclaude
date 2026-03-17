package component_test

import (
	"testing"
	"time"

	"github.com/ActiveState/termtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDiffPopup_BinaryStarts(t *testing.T) {
	bin := ensureBinary(t)
	oldFile := testdataPath(t, "old.go")
	newFile := testdataPath(t, "new.go")
	window := "test-diff-component"

	cp, err := termtest.New(termtest.Options{
		CmdName:        bin,
		Args:           []string{"diff", "--window", window, "--old", oldFile, "--new", newFile},
		DefaultTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	defer cp.Close()

	// Current stub prints "diff popup - not yet implemented"
	out, err := cp.Expect("diff popup")
	require.NoError(t, err)
	assert.Contains(t, out, "diff popup")

	// Cleanup: choice file location depends on runtime config, safe to skip in stub test
}

func TestToolPopup_BinaryStarts(t *testing.T) {
	bin := ensureBinary(t)

	cp, err := termtest.New(termtest.Options{
		CmdName:        bin,
		Args:           []string{"tool"},
		DefaultTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	defer cp.Close()

	out, err := cp.Expect("tool popup")
	require.NoError(t, err)
	assert.Contains(t, out, "tool popup")
}

func TestHelpOutput_Component(t *testing.T) {
	bin := ensureBinary(t)

	cp, err := termtest.New(termtest.Options{
		CmdName:        bin,
		Args:           []string{"--help"},
		DefaultTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	defer cp.Close()

	out, err := cp.Expect("terminal UI")
	require.NoError(t, err)
	assert.Contains(t, out, "lazyclaude")
}

func TestVersionOutput_Component(t *testing.T) {
	bin := ensureBinary(t)

	cp, err := termtest.New(termtest.Options{
		CmdName:        bin,
		Args:           []string{"--version"},
		DefaultTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	defer cp.Close()

	out, err := cp.Expect("lazyclaude version")
	require.NoError(t, err)
	assert.Contains(t, out, "lazyclaude")
}
