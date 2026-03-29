package gui

import (
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/gui/keymap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterKeybindItems_EmptyQuery(t *testing.T) {
	t.Parallel()
	items := []keymap.ActionDef{
		{Action: keymap.ActionNewSession, Description: "Create new session", HintLabel: "new"},
		{Action: keymap.ActionDeleteSession, Description: "Delete session", HintLabel: "del"},
	}
	result := filterKeybindItems(items, "")
	assert.Equal(t, items, result)
}

func TestFilterKeybindItems_MatchDescription(t *testing.T) {
	t.Parallel()
	items := []keymap.ActionDef{
		{Action: keymap.ActionNewSession, Description: "Create new session", HintLabel: "new"},
		{Action: keymap.ActionDeleteSession, Description: "Delete session", HintLabel: "del"},
		{Action: keymap.ActionAttachSession, Description: "Attach to session", HintLabel: "attach"},
	}
	result := filterKeybindItems(items, "delete")
	require.Len(t, result, 1)
	assert.Equal(t, keymap.ActionDeleteSession, result[0].Action)
}

func TestFilterKeybindItems_MatchKey(t *testing.T) {
	t.Parallel()
	items := []keymap.ActionDef{
		{Action: keymap.ActionNewSession, Description: "Create new session",
			Bindings: []keymap.KeyBinding{{Rune: 'n'}}, HintLabel: "new"},
		{Action: keymap.ActionDeleteSession, Description: "Delete session",
			Bindings: []keymap.KeyBinding{{Rune: 'd'}}, HintLabel: "del"},
	}
	result := filterKeybindItems(items, "n")
	require.Len(t, result, 2) // "n" matches key "n" and "new" in description/label
}

func TestFilterKeybindItems_CaseInsensitive(t *testing.T) {
	t.Parallel()
	items := []keymap.ActionDef{
		{Action: keymap.ActionNewSession, Description: "Create new session"},
	}
	result := filterKeybindItems(items, "CREATE")
	require.Len(t, result, 1)
	assert.Equal(t, keymap.ActionNewSession, result[0].Action)
}

func TestFilterKeybindItems_NoMatch(t *testing.T) {
	t.Parallel()
	items := []keymap.ActionDef{
		{Action: keymap.ActionNewSession, Description: "Create new session"},
	}
	result := filterKeybindItems(items, "zzzzz")
	assert.Empty(t, result)
}

func TestFilterKeybindItems_MatchHintLabel(t *testing.T) {
	t.Parallel()
	items := []keymap.ActionDef{
		{Action: keymap.ActionNewSession, Description: "Create new session", HintLabel: "new"},
		{Action: keymap.ActionDeleteSession, Description: "Delete session", HintLabel: "del"},
	}
	result := filterKeybindItems(items, "del")
	require.Len(t, result, 1)
	assert.Equal(t, keymap.ActionDeleteSession, result[0].Action)
}

func TestPanelNameToScope(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		scope keymap.Scope
	}{
		{"sessions", keymap.ScopeSession},
		{"plugins", keymap.ScopePlugins},
		{"logs", keymap.ScopeLog},
		{"unknown", keymap.ScopeGlobal},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.scope, panelNameToScope(tt.name))
	}
}
