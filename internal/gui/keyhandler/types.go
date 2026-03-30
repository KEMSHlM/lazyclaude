package keyhandler

import "github.com/jesseduffield/gocui"

// HandlerResult indicates whether a handler consumed the key event.
type HandlerResult int

const (
	Unhandled HandlerResult = iota
	Handled
)

// KeyEvent wraps a gocui key event for handler dispatch.
type KeyEvent struct {
	Key  gocui.Key
	Rune rune
	Mod  gocui.Modifier
}
