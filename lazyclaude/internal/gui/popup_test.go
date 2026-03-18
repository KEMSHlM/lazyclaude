package gui

import (
	"testing"
	"time"

	"github.com/KEMSHlM/lazyclaude/internal/notify"
	"github.com/stretchr/testify/assert"
)

func TestApp_HasPopup(t *testing.T) {
	t.Parallel()
	app := &App{}
	assert.False(t, app.hasPopup())

	app.showToolPopup(&notify.ToolNotification{
		ToolName: "Bash",
		Window:   "lc-test",
	})
	assert.True(t, app.hasPopup())
}

func TestApp_DismissPopup_ClearsPendingTool(t *testing.T) {
	t.Parallel()
	app := &App{}
	app.showToolPopup(&notify.ToolNotification{
		ToolName:  "Write",
		Window:    "lc-test",
		Timestamp: time.Now(),
	})

	app.dismissPopup(ChoiceAccept)
	assert.False(t, app.hasPopup())
	assert.Nil(t, app.pendingTool)
}

func TestApp_DismissPopup_NopWhenNoPopup(t *testing.T) {
	t.Parallel()
	app := &App{}
	// Should not panic
	app.dismissPopup(ChoiceCancel)
	assert.False(t, app.hasPopup())
}

func TestApp_ShowToolPopup_SetsFields(t *testing.T) {
	t.Parallel()
	app := &App{}
	n := &notify.ToolNotification{
		ToolName: "Edit",
		Input:    `{"file_path":"/tmp/test.go"}`,
		CWD:      "/home/user",
		Window:   "lc-abc",
	}
	app.showToolPopup(n)

	assert.Equal(t, "Edit", app.pendingTool.ToolName)
	assert.Equal(t, "lc-abc", app.pendingTool.Window)
}
