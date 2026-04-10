package main

import (
	"testing"

	"github.com/any-context/lazyclaude/internal/gui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCommands records the arguments passed to sessionCommander methods so
// that routing tests can assert the OperationTarget each command receives.
// Only the fields exercised by a given test need be populated; unused methods
// return nil.
type mockCommands struct {
	createCalls           []OperationTarget
	createWorktreeCalls   []worktreeCall
	resumeWorktreeCalls   []worktreeCall
	listWorktreesCalls    []OperationTarget
	createPMCalls         []OperationTarget
	createWorkerCalls     []worktreeCall
	launchLazygitCalls    []OperationTarget
	deleteCalls           []string
	renameCalls           []renameCall
}

type worktreeCall struct {
	Target OperationTarget
	Name   string
	Prompt string
}

type renameCall struct {
	ID      string
	NewName string
}

func (m *mockCommands) Create(target OperationTarget) error {
	m.createCalls = append(m.createCalls, target)
	return nil
}

func (m *mockCommands) Delete(id string) error {
	m.deleteCalls = append(m.deleteCalls, id)
	return nil
}

func (m *mockCommands) Rename(id, newName string) error {
	m.renameCalls = append(m.renameCalls, renameCall{ID: id, NewName: newName})
	return nil
}

func (m *mockCommands) LaunchLazygit(target OperationTarget) error {
	m.launchLazygitCalls = append(m.launchLazygitCalls, target)
	return nil
}

func (m *mockCommands) CreateWorktree(target OperationTarget, name, prompt string) error {
	m.createWorktreeCalls = append(m.createWorktreeCalls, worktreeCall{Target: target, Name: name, Prompt: prompt})
	return nil
}

func (m *mockCommands) ResumeWorktree(target OperationTarget, wtPath, prompt string) error {
	m.resumeWorktreeCalls = append(m.resumeWorktreeCalls, worktreeCall{Target: target, Name: wtPath, Prompt: prompt})
	return nil
}

func (m *mockCommands) ListWorktrees(target OperationTarget) ([]gui.WorktreeInfo, error) {
	m.listWorktreesCalls = append(m.listWorktreesCalls, target)
	return nil, nil
}

func (m *mockCommands) CreatePMSession(target OperationTarget) error {
	m.createPMCalls = append(m.createPMCalls, target)
	return nil
}

func (m *mockCommands) CreateWorkerSession(target OperationTarget, name, prompt string) error {
	m.createWorkerCalls = append(m.createWorkerCalls, worktreeCall{Target: target, Name: name, Prompt: prompt})
	return nil
}

// Compile-time interface check.
var _ sessionCommander = (*mockCommands)(nil)

// cursorState describes the cursor position for a routing test case. It
// mirrors the output of App.CurrentSessionHost(): host plus an onNode flag
// that distinguishes "on a local node" (onNode=true, host="") from "no node
// selected" (onNode=false, host="").
type cursorState struct {
	Host   string
	OnNode bool
}

const (
	localProjPath  = "/Users/me/project"
	remoteProjPath = "/home/user/remote-project"
)

// newRoutingAdapter constructs a minimally-wired guiCompositeAdapter with a
// mockCommands injected as the sessionCommander. The adapter's host caches
// are pre-populated to simulate the state Sessions() would leave after a
// layout cycle.
func newRoutingAdapter(t *testing.T, cursor cursorState, pendingHost string) (*guiCompositeAdapter, *mockCommands) {
	t.Helper()
	mock := &mockCommands{}
	a := &guiCompositeAdapter{
		commands:         mock,
		pendingHost:      pendingHost,
		localProjectRoot: localProjPath,
	}
	a.cachedHost = cursor.Host
	a.cachedOnNode = cursor.OnNode
	return a, mock
}

// TestRouting_n_Create verifies the routing of `n` (CreateSession). The path
// is supplied by the caller (currentProjectRoot() in the app layer), and the
// adapter is responsible for picking the host via resolveHost().
func TestRouting_n_Create(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name         string
		cursor       cursorState
		pendingHost  string
		inputPath    string // path the app layer would pass
		expectHost   string
		expectPath   string
	}{
		{
			name:        "cursor on local node stays local even with pending remote",
			cursor:      cursorState{Host: "", OnNode: true},
			pendingHost: "AERO",
			inputPath:   localProjPath,
			expectHost:  "",
			expectPath:  localProjPath,
		},
		{
			name:        "cursor on remote node routes to that host",
			cursor:      cursorState{Host: "AERO", OnNode: true},
			pendingHost: "AERO",
			inputPath:   remoteProjPath,
			expectHost:  "AERO",
			expectPath:  remoteProjPath,
		},
		{
			name:        "no node selected falls back to pending remote",
			cursor:      cursorState{Host: "", OnNode: false},
			pendingHost: "AERO",
			inputPath:   ".",
			expectHost:  "AERO",
			expectPath:  ".",
		},
		{
			name:        "no node selected and no pending host stays local",
			cursor:      cursorState{Host: "", OnNode: false},
			pendingHost: "",
			inputPath:   ".",
			expectHost:  "",
			expectPath:  ".",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, mock := newRoutingAdapter(t, tc.cursor, tc.pendingHost)
			require.NoError(t, a.Create(tc.inputPath))
			require.Len(t, mock.createCalls, 1)
			assert.Equal(t, tc.expectHost, mock.createCalls[0].Host)
			assert.Equal(t, tc.expectPath, mock.createCalls[0].ProjectRoot)
		})
	}
}

// TestRouting_N_CreateAtPaneCWD verifies the routing of `N`
// (CreateSessionAtCWD). Unlike `n`, this command is pane-based: it must use
// pendingHost regardless of cursor state so that the pane's CWD semantics are
// preserved. A previous bug used resolveHost() here, causing a local cursor
// to override the pane's remote host.
func TestRouting_N_CreateAtPaneCWD(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		cursor      cursorState
		pendingHost string
		expectHost  string
	}{
		{
			name:        "cursor on local node still routes to pending remote",
			cursor:      cursorState{Host: "", OnNode: true},
			pendingHost: "AERO",
			expectHost:  "AERO",
		},
		{
			name:        "cursor on remote node routes to pending remote",
			cursor:      cursorState{Host: "AERO", OnNode: true},
			pendingHost: "AERO",
			expectHost:  "AERO",
		},
		{
			name:        "no node selected routes to pending remote",
			cursor:      cursorState{Host: "", OnNode: false},
			pendingHost: "AERO",
			expectHost:  "AERO",
		},
		{
			name:        "no node selected and no pending host stays local",
			cursor:      cursorState{Host: "", OnNode: false},
			pendingHost: "",
			expectHost:  "",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, mock := newRoutingAdapter(t, tc.cursor, tc.pendingHost)
			require.NoError(t, a.CreateAtPaneCWD())
			require.Len(t, mock.createCalls, 1)
			assert.Equal(t, tc.expectHost, mock.createCalls[0].Host)
			// N always passes "." — the actual remote CWD translation
			// happens in SessionCommandService via resolveRemotePathFn.
			assert.Equal(t, ".", mock.createCalls[0].ProjectRoot)
		})
	}
}

// TestRouting_CursorBasedCommands verifies commands that follow the same
// cursor-based host routing as `n`: w (CreateWorktree), P (CreatePMSession),
// g (LaunchLazygit). All three should resolve host the same way via
// resolveTarget() and delegate to the matching sessionCommander method.
func TestRouting_CursorBasedCommands(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		cursor      cursorState
		pendingHost string
		inputPath   string
		expectHost  string
	}{
		{
			name:        "local cursor stays local",
			cursor:      cursorState{Host: "", OnNode: true},
			pendingHost: "AERO",
			inputPath:   localProjPath,
			expectHost:  "",
		},
		{
			name:        "remote cursor routes remote",
			cursor:      cursorState{Host: "AERO", OnNode: true},
			pendingHost: "AERO",
			inputPath:   remoteProjPath,
			expectHost:  "AERO",
		},
		{
			name:        "no cursor falls back to pending",
			cursor:      cursorState{Host: "", OnNode: false},
			pendingHost: "AERO",
			inputPath:   ".",
			expectHost:  "AERO",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// w: CreateWorktree
			a, mock := newRoutingAdapter(t, tc.cursor, tc.pendingHost)
			require.NoError(t, a.CreateWorktree("wt-name", "wt-prompt", tc.inputPath))
			require.Len(t, mock.createWorktreeCalls, 1)
			assert.Equal(t, tc.expectHost, mock.createWorktreeCalls[0].Target.Host, "w: host")
			assert.Equal(t, tc.inputPath, mock.createWorktreeCalls[0].Target.ProjectRoot, "w: path")
			assert.Equal(t, "wt-name", mock.createWorktreeCalls[0].Name)
			assert.Equal(t, "wt-prompt", mock.createWorktreeCalls[0].Prompt)

			// P: CreatePMSession
			a, mock = newRoutingAdapter(t, tc.cursor, tc.pendingHost)
			require.NoError(t, a.CreatePMSession(tc.inputPath))
			require.Len(t, mock.createPMCalls, 1)
			assert.Equal(t, tc.expectHost, mock.createPMCalls[0].Host, "P: host")
			assert.Equal(t, tc.inputPath, mock.createPMCalls[0].ProjectRoot, "P: path")

			// g: LaunchLazygit
			a, mock = newRoutingAdapter(t, tc.cursor, tc.pendingHost)
			require.NoError(t, a.LaunchLazygit(tc.inputPath))
			require.Len(t, mock.launchLazygitCalls, 1)
			assert.Equal(t, tc.expectHost, mock.launchLazygitCalls[0].Host, "g: host")
			assert.Equal(t, tc.inputPath, mock.launchLazygitCalls[0].ProjectRoot, "g: path")
		})
	}
}

// TestRouting_SessionBoundCommands verifies commands whose routing is bound
// to an existing session (d, R). The adapter's cursor/pending host state must
// not influence them — they forward the session id unchanged to the command
// service, which then consults session.Host internally.
func TestRouting_SessionBoundCommands(t *testing.T) {
	t.Parallel()
	// Cursor on local node with a pending remote — a particularly
	// adversarial state, to confirm routing does not leak into d/R.
	a, mock := newRoutingAdapter(t, cursorState{Host: "", OnNode: true}, "AERO")

	require.NoError(t, a.Delete("sess-id"))
	require.Len(t, mock.deleteCalls, 1)
	assert.Equal(t, "sess-id", mock.deleteCalls[0])

	require.NoError(t, a.Rename("sess-id", "new-name"))
	require.Len(t, mock.renameCalls, 1)
	assert.Equal(t, renameCall{ID: "sess-id", NewName: "new-name"}, mock.renameCalls[0])
}
