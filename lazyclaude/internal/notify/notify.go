package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ToolNotification represents a pending tool permission request from Claude Code.
type ToolNotification struct {
	ToolName    string    `json:"tool_name"`
	Input       string    `json:"input"`
	CWD         string    `json:"cwd,omitempty"`
	Window      string    `json:"window"`
	Timestamp   time.Time `json:"timestamp"`
	OldFilePath string    `json:"old_file_path,omitempty"` // set for Edit/Write diff
	NewContents string    `json:"new_contents,omitempty"`  // set for Edit/Write diff
}

// IsDiff returns true if this notification contains diff information.
func (n *ToolNotification) IsDiff() bool {
	return n.OldFilePath != ""
}

const notifyFileName = "lazyclaude-pending.json"

// FilePath returns the notification file path for a runtime directory.
func FilePath(runtimeDir string) string {
	return filepath.Join(runtimeDir, notifyFileName)
}

// Write atomically writes a notification to the runtime directory.
func Write(runtimeDir string, n ToolNotification) error {
	if err := os.MkdirAll(runtimeDir, 0o700); err != nil {
		return fmt.Errorf("create runtime dir: %w", err)
	}
	data, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	return os.WriteFile(FilePath(runtimeDir), data, 0o600)
}

// Read atomically claims and reads the notification file. Returns nil if none exists.
// Uses rename to prevent TOCTOU races between concurrent readers.
func Read(runtimeDir string) (*ToolNotification, error) {
	src := FilePath(runtimeDir)
	tmp := src + ".reading"
	if err := os.Rename(src, tmp); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("claim notification: %w", err)
	}
	defer os.Remove(tmp)

	data, err := os.ReadFile(tmp)
	if err != nil {
		return nil, fmt.Errorf("read notification: %w", err)
	}

	var n ToolNotification
	if err := json.Unmarshal(data, &n); err != nil {
		return nil, fmt.Errorf("parse notification: %w", err)
	}
	return &n, nil
}
