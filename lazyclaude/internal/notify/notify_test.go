package notify_test

import (
	"testing"
	"time"

	"github.com/KEMSHlM/lazyclaude/internal/notify"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWrite_Read_RoundTrip(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	n := notify.ToolNotification{
		ToolName:  "Bash",
		Input:     `{"command":"rm -rf /"}`,
		CWD:       "/home/user",
		Window:    "lc-abc12345",
		Timestamp: time.Now().Truncate(time.Second),
	}

	err := notify.Write(dir, n)
	require.NoError(t, err)

	got, err := notify.Read(dir)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, n.ToolName, got.ToolName)
	assert.Equal(t, n.Input, got.Input)
	assert.Equal(t, n.CWD, got.CWD)
	assert.Equal(t, n.Window, got.Window)
}

func TestRead_NoFile_ReturnsNil(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	got, err := notify.Read(dir)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestRead_DeletesFileAfterRead(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	n := notify.ToolNotification{ToolName: "Write", Window: "lc-123"}
	require.NoError(t, notify.Write(dir, n))

	// First read succeeds
	got, err := notify.Read(dir)
	require.NoError(t, err)
	require.NotNil(t, got)

	// Second read returns nil (file deleted)
	got, err = notify.Read(dir)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestIsDiff_True(t *testing.T) {
	t.Parallel()
	n := notify.ToolNotification{ToolName: "Write", OldFilePath: "/tmp/test.go"}
	assert.True(t, n.IsDiff())
}

func TestIsDiff_False(t *testing.T) {
	t.Parallel()
	n := notify.ToolNotification{ToolName: "Bash"}
	assert.False(t, n.IsDiff())
}

func TestWrite_Read_DiffNotification(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	n := notify.ToolNotification{
		ToolName:    "Write",
		OldFilePath: "/home/user/main.go",
		NewContents: "package main\n\nfunc main() {}\n",
		Window:      "lc-abc",
	}
	require.NoError(t, notify.Write(dir, n))

	got, err := notify.Read(dir)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, got.IsDiff())
	assert.Equal(t, "/home/user/main.go", got.OldFilePath)
	assert.Equal(t, "package main\n\nfunc main() {}\n", got.NewContents)
}

func TestWrite_OverwritesPrevious(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	n1 := notify.ToolNotification{ToolName: "Bash", Window: "lc-111"}
	n2 := notify.ToolNotification{ToolName: "Write", Window: "lc-222"}

	require.NoError(t, notify.Write(dir, n1))
	require.NoError(t, notify.Write(dir, n2))

	got, err := notify.Read(dir)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "Write", got.ToolName)
	assert.Equal(t, "lc-222", got.Window)
}
