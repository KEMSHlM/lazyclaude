package gui

import (
	"fmt"
	"strings"
)

// RenderActionBar builds the action bar string from visible bindings.
// Returns a formatted string like "Yes: y  |  Allow: a  |  No: n"
func RenderActionBar(bindings []Binding, maxWidth int) string {
	var parts []string
	for _, b := range bindings {
		if !b.DisplayOnScreen {
			continue
		}
		parts = append(parts, fmt.Sprintf("%s: %s", b.Description, b.Label()))
	}
	if len(parts) == 0 {
		return ""
	}

	line := strings.Join(parts, "  |  ")

	if maxWidth > 0 && len(line) > maxWidth {
		line = truncateToWidth(line, maxWidth)
	}
	return line
}

func truncateToWidth(s string, maxWidth int) string {
	if maxWidth <= 3 {
		return "..."
	}
	runes := []rune(s)
	if len(runes) <= maxWidth {
		return s
	}
	return string(runes[:maxWidth-3]) + "..."
}