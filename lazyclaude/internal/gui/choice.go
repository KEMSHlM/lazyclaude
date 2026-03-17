package gui

import (
	"fmt"
	"os"

	"github.com/KEMSHlM/lazyclaude/internal/core/config"
)

// Choice represents a user's selection in a diff/tool popup.
type Choice int

const (
	ChoiceAccept Choice = 1 // y — accept
	ChoiceAllow  Choice = 2 // a — allow always
	ChoiceReject Choice = 3 // n — reject
	ChoiceCancel Choice = 0 // Esc — cancel
)

// WriteChoiceFile writes the choice to a file for the MCP server to read.
func WriteChoiceFile(paths config.Paths, window string, choice Choice) error {
	path := paths.ChoiceFile(window)
	return os.WriteFile(path, []byte(fmt.Sprintf("%d\n", choice)), 0o600)
}

// ReadChoiceFile reads and removes the choice file.
func ReadChoiceFile(paths config.Paths, window string) (Choice, error) {
	path := paths.ChoiceFile(window)

	data, err := os.ReadFile(path)
	if err != nil {
		return ChoiceCancel, err
	}

	os.Remove(path)

	var val int
	if _, err := fmt.Sscanf(string(data), "%d", &val); err != nil {
		return ChoiceCancel, fmt.Errorf("parse choice: %w", err)
	}

	return Choice(val), nil
}
