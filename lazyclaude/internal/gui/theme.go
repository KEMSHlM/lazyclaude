package gui

import "github.com/jesseduffield/gocui"

// Color constants for the TUI (Dracula-inspired palette).
var (
	ColorDefault = gocui.ColorDefault
	ColorGreen   = gocui.NewRGBColor(0x50, 0xFA, 0x7B)
	ColorRed     = gocui.NewRGBColor(0xFF, 0x55, 0x55)
	ColorYellow  = gocui.NewRGBColor(0xF1, 0xFA, 0x8C)
	ColorCyan    = gocui.NewRGBColor(0x8B, 0xE9, 0xFD)
	ColorDim     = gocui.NewRGBColor(0x62, 0x72, 0xA4)
	ColorWhite   = gocui.NewRGBColor(0xF8, 0xF8, 0xF2)

	// Diff-specific
	ColorDiffAdd = gocui.NewRGBColor(0x50, 0xFA, 0x7B)
	ColorDiffDel = gocui.NewRGBColor(0xFF, 0x55, 0x55)
)

// ActionStyle defines how an action key should be displayed.
type ActionStyle int

const (
	StyleDefault ActionStyle = iota
	StyleGreen               // accept/yes
	StyleRed                 // reject/no
	StyleYellow              // allow-all
)
