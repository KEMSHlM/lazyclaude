package gui

import "strings"

// SideTab represents a tab in the left side panel (lazygit TabView pattern).
type SideTab struct {
	Label string // display label in tab bar
	Name  string // view identifier
}

// SideTabs returns the side panel tabs for the main screen.
func SideTabs() []SideTab {
	return []SideTab{
		{Label: "Sessions", Name: "sessions"},
		{Label: "Server", Name: "server"},
	}
}

// TabBar renders the tab bar string for the side panel title.
// Active tab is indicated by brackets.
// Example: "[Sessions]  Server"
func TabBar(tabs []SideTab, activeIdx int) string {
	parts := make([]string, len(tabs))
	for i, tab := range tabs {
		if i == activeIdx {
			parts[i] = "[" + tab.Label + "]"
		} else {
			parts[i] = tab.Label
		}
	}
	return strings.Join(parts, "  ")
}
