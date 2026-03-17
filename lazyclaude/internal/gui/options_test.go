package gui_test

import (
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/gui"
	"github.com/stretchr/testify/assert"
)

func TestRenderActionBar_Basic(t *testing.T) {
	t.Parallel()
	bindings := []gui.Binding{
		{Rune: 'y', Description: "Yes", DisplayOnScreen: true, Style: gui.StyleGreen},
		{Rune: 'a', Description: "Allow", DisplayOnScreen: true, Style: gui.StyleYellow},
		{Rune: 'n', Description: "No", DisplayOnScreen: true, Style: gui.StyleRed},
	}

	result := gui.RenderActionBar(bindings, 80)
	assert.Equal(t, "Yes: y  |  Allow: a  |  No: n", result)
}

func TestRenderActionBar_FiltersNonDisplay(t *testing.T) {
	t.Parallel()
	bindings := []gui.Binding{
		{Rune: 'y', Description: "Yes", DisplayOnScreen: true},
		{Rune: 'j', Description: "scroll down", DisplayOnScreen: false},
		{Rune: 'n', Description: "No", DisplayOnScreen: true},
	}

	result := gui.RenderActionBar(bindings, 80)
	assert.Equal(t, "Yes: y  |  No: n", result)
}

func TestRenderActionBar_Empty(t *testing.T) {
	t.Parallel()
	result := gui.RenderActionBar(nil, 80)
	assert.Equal(t, "", result)
}

func TestRenderActionBar_AllHidden(t *testing.T) {
	t.Parallel()
	bindings := []gui.Binding{
		{Rune: 'j', Description: "down", DisplayOnScreen: false},
	}
	result := gui.RenderActionBar(bindings, 80)
	assert.Equal(t, "", result)
}

func TestRenderActionBar_Truncate(t *testing.T) {
	t.Parallel()
	bindings := []gui.Binding{
		{Rune: 'y', Description: "Yes", DisplayOnScreen: true},
		{Rune: 'a', Description: "Allow always", DisplayOnScreen: true},
		{Rune: 'n', Description: "No", DisplayOnScreen: true},
	}

	result := gui.RenderActionBar(bindings, 20)
	assert.LessOrEqual(t, len(result), 20)
	assert.True(t, len(result) > 0)
}

func TestRenderActionBar_NoTruncateWhenFits(t *testing.T) {
	t.Parallel()
	bindings := []gui.Binding{
		{Rune: 'q', Description: "quit", DisplayOnScreen: true},
	}

	result := gui.RenderActionBar(bindings, 80)
	assert.Equal(t, "quit: q", result)
}