package gui_test

import (
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/gui"
	"github.com/jesseduffield/gocui"
	"github.com/stretchr/testify/assert"
)

func TestBinding_MatchesKey_Rune(t *testing.T) {
	t.Parallel()
	b := gui.Binding{Rune: 'y'}

	assert.True(t, b.MatchesKey(0, 'y'))
	assert.False(t, b.MatchesKey(0, 'n'))
	assert.False(t, b.MatchesKey(gocui.KeyEnter, 0))
}

func TestBinding_MatchesKey_SpecialKey(t *testing.T) {
	t.Parallel()
	b := gui.Binding{Key: gocui.KeyEnter}

	assert.True(t, b.MatchesKey(gocui.KeyEnter, 0))
	assert.False(t, b.MatchesKey(gocui.KeyEsc, 0))
	assert.False(t, b.MatchesKey(0, 'y'))
}

func TestBinding_Label_Rune(t *testing.T) {
	t.Parallel()
	b := gui.Binding{Rune: 'j'}
	assert.Equal(t, "j", b.Label())
}

func TestBinding_Label_SpecialKey(t *testing.T) {
	t.Parallel()
	tests := []struct {
		key  gocui.Key
		want string
	}{
		{gocui.KeyEnter, "enter"},
		{gocui.KeyEsc, "Esc"},
		{gocui.KeySpace, "space"},
		{gocui.KeyCtrlC, "ctrl+c"},
		{gocui.KeyCtrlX, "ctrl+x"},
	}
	for _, tt := range tests {
		b := gui.Binding{Key: tt.key}
		assert.Equal(t, tt.want, b.Label())
	}
}