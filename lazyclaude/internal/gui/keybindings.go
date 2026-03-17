package gui

import "github.com/jesseduffield/gocui"

// Binding represents a key binding with display metadata.
type Binding struct {
	Key             gocui.Key
	Rune            rune // for character keys (e.g., 'j', 'k', 'y')
	Description     string
	Handler         func() error
	DisplayOnScreen bool        // show in action bar
	Style           ActionStyle // color style for action bar
}

// MatchesKey returns true if this binding matches the given key event.
func (b *Binding) MatchesKey(key gocui.Key, ch rune) bool {
	if b.Rune != 0 {
		return ch == b.Rune
	}
	return key == b.Key
}

// Label returns a human-readable label for the key.
func (b *Binding) Label() string {
	if b.Rune != 0 {
		return string(b.Rune)
	}
	return KeyLabel(b.Key)
}

// KeyLabel returns a human-readable name for a gocui key.
func KeyLabel(key gocui.Key) string {
	switch key {
	case gocui.KeyEnter:
		return "enter"
	case gocui.KeyEsc:
		return "Esc"
	case gocui.KeySpace:
		return "space"
	case gocui.KeyTab:
		return "tab"
	case gocui.KeyArrowUp:
		return "up"
	case gocui.KeyArrowDown:
		return "down"
	case gocui.KeyCtrlC:
		return "ctrl+c"
	case gocui.KeyCtrlX:
		return "ctrl+x"
	default:
		return "?"
	}
}