package server

import (
	"sync"
	"time"
)

// ConnState holds per-connection state.
type ConnState struct {
	PID    int
	Window string
}

// PendingTool stores tool info from PreToolUse hooks with a TTL.
type PendingTool struct {
	ToolName string
	Input    string
	CWD      string
	Expiry   time.Time
}

// State manages shared server state across connections.
type State struct {
	mu          sync.RWMutex
	connections map[string]*ConnState // connID -> state
	pidToWindow map[int]string        // pid -> window ID
	pending     map[string]PendingTool // window -> pending tool info
}

// NewState creates an empty State.
func NewState() *State {
	return &State{
		connections: make(map[string]*ConnState),
		pidToWindow: make(map[int]string),
		pending:     make(map[string]PendingTool),
	}
}

// SetConn registers or updates a connection's state.
func (s *State) SetConn(connID string, cs *ConnState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.connections[connID] = cs
	if cs.PID > 0 && cs.Window != "" {
		s.pidToWindow[cs.PID] = cs.Window
	}
}

// GetConn returns a connection's state.
func (s *State) GetConn(connID string) *ConnState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.connections[connID]
}

// RemoveConn removes a connection's state.
func (s *State) RemoveConn(connID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if cs, ok := s.connections[connID]; ok {
		delete(s.pidToWindow, cs.PID)
	}
	delete(s.connections, connID)
}

// WindowForPID returns the cached window for a PID.
func (s *State) WindowForPID(pid int) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pidToWindow[pid]
}

const pendingTTL = 15 * time.Second

// SetPending stores pending tool info for a window with a TTL.
func (s *State) SetPending(window string, tool PendingTool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tool.Expiry = time.Now().Add(pendingTTL)
	s.pending[window] = tool
}

// SetPendingWithExpiry stores pending tool info with an explicit expiry (for testing).
func (s *State) SetPendingWithExpiry(window string, tool PendingTool, expiry time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tool.Expiry = expiry
	s.pending[window] = tool
}

// GetPending retrieves and removes pending tool info (if not expired).
func (s *State) GetPending(window string) (PendingTool, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tool, ok := s.pending[window]
	if !ok {
		return PendingTool{}, false
	}
	delete(s.pending, window)
	if time.Now().After(tool.Expiry) {
		return PendingTool{}, false
	}
	return tool, true
}

// ConnCount returns the number of active connections.
func (s *State) ConnCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.connections)
}