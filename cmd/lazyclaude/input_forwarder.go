package main

import (
	"fmt"
	"sync"

	"github.com/any-context/lazyclaude/internal/daemon"
	"github.com/any-context/lazyclaude/internal/gui"
)

// compositeInputForwarder routes key forwarding to local tmux or remote daemon
// based on the current fullscreen session context. When host is empty, keys are
// forwarded via the local tmux client. When host is set, keys are routed through
// the RemoteProvider (direct socket with daemon API fallback).
type compositeInputForwarder struct {
	local gui.InputForwarder       // local tmux send-keys
	cp    *daemon.CompositeProvider // for finding the remote provider

	mu        sync.RWMutex
	sessionID string // current fullscreen session ID
	host      string // empty for local sessions
}

// Compile-time checks.
var _ gui.InputForwarder = (*compositeInputForwarder)(nil)
var _ gui.SessionContextSetter = (*compositeInputForwarder)(nil)

func newCompositeInputForwarder(local gui.InputForwarder, cp *daemon.CompositeProvider) *compositeInputForwarder {
	return &compositeInputForwarder{local: local, cp: cp}
}

// SetSessionContext updates the forwarding target. Called when entering/exiting fullscreen.
func (f *compositeInputForwarder) SetSessionContext(sessionID, host string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	debugLog("compositeInputForwarder.SetSessionContext: sessionID=%q host=%q", sessionID, host)
	f.sessionID = sessionID
	f.host = host
}

func (f *compositeInputForwarder) ForwardKey(target string, key string) error {
	f.mu.RLock()
	host := f.host
	sid := f.sessionID
	f.mu.RUnlock()

	if host == "" {
		return f.local.ForwardKey(target, key)
	}
	rp := f.remoteProvider(host)
	if rp == nil {
		return fmt.Errorf("no remote provider for host %q", host)
	}
	return rp.SendKeys(sid, key)
}

func (f *compositeInputForwarder) ForwardLiteral(target string, text string) error {
	f.mu.RLock()
	host := f.host
	sid := f.sessionID
	f.mu.RUnlock()

	if host == "" {
		return f.local.ForwardLiteral(target, text)
	}
	rp := f.remoteProvider(host)
	if rp == nil {
		return fmt.Errorf("no remote provider for host %q", host)
	}
	return rp.SendKeysLiteral(sid, text)
}

func (f *compositeInputForwarder) ForwardPaste(target string, text string) error {
	f.mu.RLock()
	host := f.host
	sid := f.sessionID
	f.mu.RUnlock()

	if host == "" {
		return f.local.ForwardPaste(target, text)
	}
	rp := f.remoteProvider(host)
	if rp == nil {
		return fmt.Errorf("no remote provider for host %q", host)
	}
	return rp.PasteToPane(sid, text)
}

// remoteProvider returns the concrete RemoteProvider for the given host.
func (f *compositeInputForwarder) remoteProvider(host string) *daemon.RemoteProvider {
	sp := f.cp.RemoteProvider(host)
	if sp == nil {
		debugLog("compositeInputForwarder.remoteProvider: no provider for host=%q", host)
		return nil
	}
	rp, ok := sp.(*daemon.RemoteProvider)
	if !ok {
		debugLog("compositeInputForwarder.remoteProvider: provider for host=%q is %T, not *RemoteProvider", host, sp)
		return nil
	}
	return rp
}
