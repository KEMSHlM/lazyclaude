package model_test

import (
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/core/model"
	"github.com/stretchr/testify/assert"
)

func TestIsDiff_True(t *testing.T) {
	t.Parallel()
	n := model.ToolNotification{ToolName: "Write", OldFilePath: "/tmp/test.go"}
	assert.True(t, n.IsDiff())
}

func TestIsDiff_False(t *testing.T) {
	t.Parallel()
	n := model.ToolNotification{ToolName: "Bash"}
	assert.False(t, n.IsDiff())
}

func TestActivityState_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		state model.ActivityState
		want  string
	}{
		{model.ActivityUnknown, ""},
		{model.ActivityRunning, "running"},
		{model.ActivityNeedsInput, "needs_input"},
		{model.ActivityIdle, "idle"},
		{model.ActivityError, "error"},
		{model.ActivityDead, "dead"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, tt.state.String())
	}
}

func TestActivityState_ZeroValue(t *testing.T) {
	t.Parallel()
	var s model.ActivityState
	assert.Equal(t, model.ActivityUnknown, s)
	assert.Equal(t, "", s.String())
}
