package keymap

import (
	_ "embed"
	"strings"
)

//go:embed keybinds.md
var keybindDocRaw string

// DocSection extracts the content under a "## sectionName" heading
// from the embedded keybinds.md. Returns empty string if not found.
func DocSection(section string) string {
	header := "## " + section
	idx := strings.Index(keybindDocRaw, header)
	if idx < 0 {
		return ""
	}

	// Skip the header line itself.
	start := idx + len(header)
	if start < len(keybindDocRaw) && keybindDocRaw[start] == '\n' {
		start++
	}

	// Find the next "## " heading or end of file.
	rest := keybindDocRaw[start:]
	end := strings.Index(rest, "\n## ")
	if end < 0 {
		return strings.TrimSpace(rest)
	}
	return strings.TrimSpace(rest[:end])
}
