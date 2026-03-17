package context

import (
	"fmt"
	"sync"
)

// Manager manages a stack of contexts (lazygit ContextMgr pattern).
type Manager struct {
	mu    sync.Mutex
	stack []Context
}

// NewManager creates an empty context manager.
func NewManager() *Manager {
	return &Manager{}
}

// Push activates a new context on top of the stack.
// The previous context (if any) receives OnBlur.
func (m *Manager) Push(c Context) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.stack) > 0 {
		m.stack[len(m.stack)-1].OnBlur()
	}
	m.stack = append(m.stack, c)
	c.OnFocus()
}

// Pop removes and returns the top context.
// The context below (if any) receives OnFocus.
func (m *Manager) Pop() (Context, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.stack) == 0 {
		return nil, fmt.Errorf("context stack is empty")
	}

	top := m.stack[len(m.stack)-1]
	top.OnBlur()
	m.stack = m.stack[:len(m.stack)-1]

	if len(m.stack) > 0 {
		m.stack[len(m.stack)-1].OnFocus()
	}
	return top, nil
}

// Current returns the active (top) context, or nil.
func (m *Manager) Current() Context {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.stack) == 0 {
		return nil
	}
	return m.stack[len(m.stack)-1]
}

// Depth returns the number of contexts on the stack.
func (m *Manager) Depth() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.stack)
}

// Replace replaces the top context without triggering OnBlur/OnFocus on lower contexts.
func (m *Manager) Replace(c Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.stack) == 0 {
		return fmt.Errorf("context stack is empty")
	}

	m.stack[len(m.stack)-1].OnBlur()
	m.stack[len(m.stack)-1] = c
	c.OnFocus()
	return nil
}