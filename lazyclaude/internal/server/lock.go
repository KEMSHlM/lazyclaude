package server

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

// LockFile represents the contents of an IDE lock file.
type LockFile struct {
	PID       int    `json:"pid"`
	AuthToken string `json:"authToken"`
	Transport string `json:"transport"`
}

// LockManager handles IDE lock file lifecycle.
type LockManager struct {
	ideDir string
}

// NewLockManager creates a lock manager.
// ideDir is typically ~/.claude/ide/
func NewLockManager(ideDir string) *LockManager {
	return &LockManager{ideDir: ideDir}
}

// Write creates a lock file at <ideDir>/<port>.lock.
func (m *LockManager) Write(port int, token string) error {
	if err := os.MkdirAll(m.ideDir, 0o700); err != nil {
		return fmt.Errorf("create ide dir: %w", err)
	}

	lock := LockFile{
		PID:       os.Getpid(),
		AuthToken: token,
		Transport: "ws",
	}
	data, err := json.Marshal(lock)
	if err != nil {
		return fmt.Errorf("marshal lock: %w", err)
	}

	path := m.lockPath(port)
	return os.WriteFile(path, data, 0o600)
}

// Read reads a lock file.
func (m *LockManager) Read(port int) (*LockFile, error) {
	data, err := os.ReadFile(m.lockPath(port))
	if err != nil {
		return nil, err
	}
	var lock LockFile
	if err := json.Unmarshal(data, &lock); err != nil {
		return nil, fmt.Errorf("parse lock: %w", err)
	}
	return &lock, nil
}

// Remove deletes a lock file.
func (m *LockManager) Remove(port int) error {
	return os.Remove(m.lockPath(port))
}

// Exists checks if a lock file exists for a port.
func (m *LockManager) Exists(port int) bool {
	_, err := os.Stat(m.lockPath(port))
	return err == nil
}

func (m *LockManager) lockPath(port int) string {
	return filepath.Join(m.ideDir, strconv.Itoa(port)+".lock")
}