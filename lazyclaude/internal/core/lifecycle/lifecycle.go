// Package lifecycle provides a centralized cleanup registration mechanism.
// Cleanup functions are run in reverse (LIFO) order when Close is called.
package lifecycle

import (
	"fmt"
	"sync"
)

// entry holds a named cleanup function.
type entry struct {
	name string
	fn   func()
}

// Lifecycle collects cleanup functions and runs them in reverse order on Close.
// All methods are safe to call concurrently.
type Lifecycle struct {
	mu      sync.Mutex
	entries []entry
	closed  bool
}

// New returns a new, empty Lifecycle.
func New() *Lifecycle {
	return &Lifecycle{}
}

// Register adds a cleanup function. name is used for logging only.
// If the Lifecycle has already been closed, the function is not registered.
// Register is safe to call concurrently.
func (lc *Lifecycle) Register(name string, fn func()) {
	lc.mu.Lock()
	defer lc.mu.Unlock()

	if lc.closed {
		return
	}
	lc.entries = append(lc.entries, entry{name: name, fn: fn})
}

// Close runs all registered cleanup functions in reverse registration order.
// It is idempotent: subsequent calls after the first are no-ops.
// A panic inside a cleanup function is recovered and logged to stderr;
// remaining cleanup functions still run.
// Close is safe to call concurrently.
func (lc *Lifecycle) Close() {
	lc.mu.Lock()
	if lc.closed {
		lc.mu.Unlock()
		return
	}
	// Snapshot the entries and mark closed while holding the lock so that
	// concurrent Register calls after Close are rejected immediately.
	snapshot := make([]entry, len(lc.entries))
	copy(snapshot, lc.entries)
	lc.closed = true
	lc.mu.Unlock()

	// Run in reverse (LIFO) order without holding the lock.
	for i := len(snapshot) - 1; i >= 0; i-- {
		runCleanup(snapshot[i])
	}
}

// Len returns the number of registered cleanup functions.
func (lc *Lifecycle) Len() int {
	lc.mu.Lock()
	defer lc.mu.Unlock()
	return len(lc.entries)
}

// runCleanup calls e.fn, recovering from any panic so that subsequent
// cleanup functions still execute.
func runCleanup(e entry) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("lifecycle: cleanup %q panicked: %v\n", e.name, r)
		}
	}()
	e.fn()
}
