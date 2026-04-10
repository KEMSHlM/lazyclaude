package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/any-context/lazyclaude/internal/core/config"
	"github.com/any-context/lazyclaude/internal/core/tmux"
	"github.com/any-context/lazyclaude/internal/daemon"
	"github.com/any-context/lazyclaude/internal/session"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// This file contains end-to-end behaviour tests for command routing. Unlike
// routing_test.go (which asserts on arguments passed to a sessionCommander
// mock), these tests wire up the real SessionCommandService stack — real
// session.Manager, real CompositeProvider, real MirrorManager, real
// RemoteHostManager — and verify the final state of the session.Store
// after each command runs. The only fakes are the network-facing pieces
// (fakeRemoteSessionAPI for n/N remote plain-create, fakeSessionProvider
// for w/W/P/worker/g remote) and tmux.MockClient.
//
// The tests MUST NOT call t.Parallel(): chdirTemp mutates the process cwd to
// control the "no cursor, no pending" local-fallback path (which resolves
// via filepath.Abs(".")), and parallel execution would race on os.Chdir.

// --- fakeRemoteSessionAPI: n/N remote plain-create ---------------------------

// fakeRemoteSessionAPI replaces the SSH-backed *daemon.RemoteProvider that
// SessionCommandService.completeRemoteCreate invokes via remoteProviderFn.
// It only covers the plain-create path (n, N) plus d/R's remote delete and
// rename, which bypass the CompositeProvider and go through this type
// directly.
type fakeRemoteSessionAPI struct {
	// createCalls records the remote path passed to CreateSession so that
	// n/N tests can assert what was sent to the remote daemon after
	// resolveRemotePath translation.
	createCalls []string
	// deleteCalls records session ids passed to Delete for d remote.
	deleteCalls []string
	// renameCalls records the (id, newName) pairs passed to Rename for
	// R remote.
	renameCalls []renameCall
}

func (f *fakeRemoteSessionAPI) CreateSession(path string) (*daemon.SessionCreateResponse, error) {
	f.createCalls = append(f.createCalls, path)
	id := uuid.New().String()
	return &daemon.SessionCreateResponse{
		ID:         id,
		Name:       "remote-" + id[:4],
		Path:       path,
		TmuxWindow: "lc-" + id[:8],
	}, nil
}

func (f *fakeRemoteSessionAPI) Delete(id string) error {
	f.deleteCalls = append(f.deleteCalls, id)
	return nil
}

func (f *fakeRemoteSessionAPI) Rename(id, newName string) error {
	f.renameCalls = append(f.renameCalls, renameCall{ID: id, NewName: newName})
	return nil
}

var _ remoteSessionAPI = (*fakeRemoteSessionAPI)(nil)

// --- fakeSessionProvider: remote w/W/P/worker/g -------------------------------

// fakeSessionProvider satisfies daemon.SessionProvider and CWDQuerier. It is
// registered in CompositeProvider as the remote backend for host="AERO".
// Every role-session / worktree method records its arguments and then calls
// the PostCreateHook (= MirrorManager.CreateMirror) so that the integration
// test can observe the resulting remote mirror session in the local store.
// This mirrors how *daemon.RemoteProvider behaves in production.
type fakeSessionProvider struct {
	host       string
	postCreate daemon.PostCreateHook
	remoteCWD  string

	// Call records exposed to tests.
	worktreeCalls  []worktreeCall  // CreateWorktree
	resumeWTCalls  []worktreeCall  // ResumeWorktree
	listWTCalls    []string        // ListWorktrees projectRoot
	pmCalls        []string        // CreatePMSession projectRoot
	workerCalls    []worktreeCall  // CreateWorkerSession
	lazygitCalls   []string        // LaunchLazygit path
}

// --- daemon.SessionLister ---

func (f *fakeSessionProvider) HasSession(_ string) bool              { return false }
func (f *fakeSessionProvider) Host() string                          { return f.host }
func (f *fakeSessionProvider) Sessions() ([]daemon.SessionInfo, error) { return nil, nil }

// --- daemon.SessionMutator ---

func (f *fakeSessionProvider) Create(_ string) error     { return nil }
func (f *fakeSessionProvider) Delete(_ string) error     { return nil }
func (f *fakeSessionProvider) Rename(_, _ string) error  { return nil }
func (f *fakeSessionProvider) PurgeOrphans() (int, error) { return 0, nil }

// --- daemon.PreviewProvider (stubs) ---

func (f *fakeSessionProvider) CapturePreview(_ string, _, _ int) (*daemon.PreviewResponse, error) {
	return &daemon.PreviewResponse{}, nil
}
func (f *fakeSessionProvider) CaptureScrollback(_ string, _, _, _ int) (*daemon.ScrollbackResponse, error) {
	return &daemon.ScrollbackResponse{}, nil
}
func (f *fakeSessionProvider) HistorySize(_ string) (int, error) { return 0, nil }

// --- daemon.SessionActioner ---

func (f *fakeSessionProvider) SendChoice(_ string, _ int) error { return nil }
func (f *fakeSessionProvider) AttachSession(_ string) error     { return nil }

func (f *fakeSessionProvider) LaunchLazygit(path string) error {
	f.lazygitCalls = append(f.lazygitCalls, path)
	return nil
}

// --- daemon.WorktreeProvider ---

func (f *fakeSessionProvider) CreateWorktree(name, prompt, projectRoot string) error {
	f.worktreeCalls = append(f.worktreeCalls, worktreeCall{
		Target: OperationTarget{Host: f.host, ProjectRoot: projectRoot},
		Name:   name,
		Prompt: prompt,
	})
	resp := f.newResponse(name, filepath.Join(projectRoot, session.WorktreePathSegment, name), "worker")
	return f.postCreate(f.host, projectRoot, resp)
}

func (f *fakeSessionProvider) ResumeWorktree(worktreePath, prompt, projectRoot string) error {
	f.resumeWTCalls = append(f.resumeWTCalls, worktreeCall{
		Target: OperationTarget{Host: f.host, ProjectRoot: projectRoot},
		Name:   worktreePath,
		Prompt: prompt,
	})
	resp := f.newResponse(filepath.Base(worktreePath), worktreePath, "worker")
	return f.postCreate(f.host, projectRoot, resp)
}

func (f *fakeSessionProvider) ListWorktrees(projectRoot string) ([]daemon.WorktreeInfo, error) {
	f.listWTCalls = append(f.listWTCalls, projectRoot)
	return nil, nil
}

// --- daemon.RoleSessionProvider ---

func (f *fakeSessionProvider) CreatePMSession(projectRoot string) error {
	f.pmCalls = append(f.pmCalls, projectRoot)
	resp := f.newResponse("pm", projectRoot, "pm")
	return f.postCreate(f.host, projectRoot, resp)
}

func (f *fakeSessionProvider) CreateWorkerSession(name, prompt, projectRoot string) error {
	f.workerCalls = append(f.workerCalls, worktreeCall{
		Target: OperationTarget{Host: f.host, ProjectRoot: projectRoot},
		Name:   name,
		Prompt: prompt,
	})
	resp := f.newResponse(name, filepath.Join(projectRoot, session.WorktreePathSegment, name), "worker")
	return f.postCreate(f.host, projectRoot, resp)
}

// --- daemon.ConnectionAware ---

func (f *fakeSessionProvider) ConnectionState() daemon.ConnectionState {
	return daemon.Connected
}

// --- daemon.CWDQuerier ---

// QueryCWD returns the fake's configured remote working directory. Called
// from guiCompositeAdapter.resolveRemotePath when the caller passes "." or
// the local project root as the project path.
func (f *fakeSessionProvider) QueryCWD(_ context.Context) (string, error) {
	return f.remoteCWD, nil
}

// newResponse builds a synthetic SessionCreateResponse with the given name,
// path, and role. Used by CreateWorktree / CreatePMSession / etc. so that
// MirrorManager.CreateMirror has everything it needs to insert a session
// into the local store.
func (f *fakeSessionProvider) newResponse(name, path, role string) *daemon.SessionCreateResponse {
	id := uuid.New().String()
	return &daemon.SessionCreateResponse{
		ID:         id,
		Name:       name,
		Path:       path,
		TmuxWindow: "lc-" + id[:8],
		Role:       role,
	}
}

// Compile-time interface checks.
var (
	_ daemon.SessionProvider = (*fakeSessionProvider)(nil)
	_ daemon.CWDQuerier      = (*fakeSessionProvider)(nil)
)

// --- fakeIntegrationMirrorCreator -------------------------------------------
//
// The real MirrorManager.CreateMirror builds an SSH command and issues
// tmux.NewSession / NewWindow through the mock tmux client, which is fine
// for the store-state assertions. No extra fake is needed here; we use the
// real MirrorManager wired with tmux.MockClient.

// --- helpers -----------------------------------------------------------------

const integrationRemoteHost = "AERO"

// initGitRepo creates a git repo with an initial empty commit. Mirrors the
// helper in internal/session/manager_test.go so that the local w / worker
// tests can invoke real `git worktree add`. Uses plain `git init` (no
// --initial-branch flag) to stay compatible with older Git versions in CI.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "cmd %v: %s", args, out)
	}
}

// chdirTemp switches the process cwd to dir and restores the previous cwd
// via t.Cleanup. Integration tests call this so that filepath.Abs(".") and
// any other cwd-dependent lookups are deterministic. Because this mutates
// process-global state, tests using it MUST NOT call t.Parallel(). A
// failed restore is reported as a test error so that a subsequent test
// does not silently run in the wrong directory and produce a confusing
// failure far from the root cause.
func chdirTemp(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Errorf("chdirTemp restore to %q failed: %v", orig, err)
		}
	})
}

// setupMCPFiles writes the port file and lock file that session.Manager
// expects when creating PM/worker sessions. Without these, the launcher
// script generation succeeds but the session-integration path that reads
// MCP credentials would fail (claudeEnv lookup). Only strictly required by
// PM and worker tests; cheap enough to write unconditionally.
func setupMCPFiles(t *testing.T, paths config.Paths) {
	t.Helper()
	const port = 19876
	const token = "integration-test-token"
	require.NoError(t, os.MkdirAll(filepath.Dir(paths.PortFile()), 0o755))
	require.NoError(t, os.WriteFile(paths.PortFile(), []byte(strconv.Itoa(port)), 0o600))
	require.NoError(t, os.MkdirAll(paths.IDEDir, 0o755))
	lockData, err := json.Marshal(map[string]string{"authToken": token})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(paths.LockFile(port), lockData, 0o600))
}

// waitUntilStoreSettled blocks until the background completeRemoteCreate
// goroutine (spawned by SessionCommandService.Create for remote hosts) has
// removed the "connecting..." placeholder and converged on the expected
// session count. A 5 second ceiling is generous — the goroutine does no
// real IO in these tests.
func waitUntilStoreSettled(t *testing.T, mgr *session.Manager, expected int) {
	t.Helper()
	require.Eventually(t, func() bool {
		sessions := mgr.Sessions()
		for _, s := range sessions {
			if s.Name == "connecting..." {
				return false
			}
		}
		return len(sessions) == expected
	}, 5*time.Second, 20*time.Millisecond, "store did not settle to %d sessions", expected)
}

// recordingTmux wraps tmux.MockClient and records every target passed to
// KillWindow. tmux.MockClient does not track KillWindow arguments, so
// without this wrapper the d tests cannot verify that the right window
// prefix (lc- for local, rm- for remote mirror) was killed.
type recordingTmux struct {
	*tmux.MockClient
	killedWindows []string
}

func (r *recordingTmux) KillWindow(ctx context.Context, target string) error {
	r.killedWindows = append(r.killedWindows, target)
	return r.MockClient.KillWindow(ctx, target)
}

// integrationStack holds every wired component the tests need. Built by
// newIntegrationStack for each test and torn down via t.TempDir / t.Cleanup.
type integrationStack struct {
	adapter    *guiCompositeAdapter
	svc        *SessionCommandService
	mgr        *session.Manager
	composite  *daemon.CompositeProvider
	mirrorMgr  *MirrorManager
	hostMgr    *RemoteHostManager
	fakeRemote *fakeRemoteSessionAPI
	fakeRP     *fakeSessionProvider
	tmuxMock   *recordingTmux

	localProj string // absolute path of the local git project (also the cwd)
}

// newIntegrationStack builds the full real stack (session.Manager,
// CompositeProvider, MirrorManager, SessionCommandService,
// guiCompositeAdapter) with tmux.MockClient and a fakeSessionProvider
// registered for integrationRemoteHost. The caller supplies the adapter's
// cursor/pendingHost state.
//
// Side effect: chdirs into the local project for the duration of the test.
func newIntegrationStack(t *testing.T, cursor cursorState, pendingHost string) *integrationStack {
	t.Helper()

	tmp := t.TempDir()
	paths := config.TestPaths(tmp)
	setupMCPFiles(t, paths)

	// Local project lives under the test's tmpdir so that it has a unique
	// absolute path per test and so filepath.Abs(".") is deterministic once
	// chdirTemp runs. On macOS, t.TempDir() returns a path under /var/
	// while the kernel reports the canonical /private/var/ after chdir —
	// capture os.Getwd() post-chdir so that expected and observed paths
	// agree.
	localProj := filepath.Join(tmp, "proj")
	require.NoError(t, os.MkdirAll(localProj, 0o755))
	initGitRepo(t, localProj)
	chdirTemp(t, localProj)
	canonProj, err := os.Getwd()
	require.NoError(t, err)
	localProj = canonProj

	store := session.NewStore(filepath.Join(paths.DataDir, "state.json"))
	tmuxMock := &recordingTmux{MockClient: tmux.NewMockClient()}
	mgr := session.NewManager(store, tmuxMock, paths, nil)

	localProv := &localDaemonProvider{mgr: mgr, tmux: tmuxMock}
	composite := daemon.NewCompositeProvider(localProv, nil)

	mirrorMgr := &MirrorManager{
		tmux:  tmuxMock,
		store: store,
	}

	// A no-op connectFn makes EnsureConnected a no-op for the remote host.
	// MarkConnected pre-populates the lazyConn entry so even the once.Do
	// is skipped. This keeps the host manager exercising its real routing
	// code without touching SSH.
	hostMgr := NewRemoteHostManager(func(_ string) error { return nil })
	hostMgr.MarkConnected(integrationRemoteHost)

	fakeRemote := &fakeRemoteSessionAPI{}
	// Post-create hook mirrors production: every remote CreateWorktree /
	// CreatePMSession / CreateWorkerSession must funnel through
	// MirrorManager.CreateMirror so that the resulting mirror session
	// lands in the local store where tests can observe it. We wire the
	// hook at construction (rather than assigning it afterwards) to
	// prevent a future reorder from registering the fake in
	// CompositeProvider before the hook is set and causing a nil panic.
	fakeRP := &fakeSessionProvider{
		host:      integrationRemoteHost,
		remoteCWD: "/remote/cwd",
		postCreate: func(host, path string, resp *daemon.SessionCreateResponse) error {
			return mirrorMgr.CreateMirror(host, path, resp)
		},
	}

	// Build the adapter first so its resolveRemotePath method can be
	// referenced by SessionCommandService.resolveRemotePathFn and its
	// readPendingHost method can be read by CreateAtPaneCWD.
	adapter := &guiCompositeAdapter{
		cp:               composite,
		localMgr:         mgr,
		paths:            paths,
		pendingHost:      pendingHost,
		localProjectRoot: localProj,
	}
	adapter.cachedHost = cursor.Host
	adapter.cachedOnNode = cursor.OnNode

	svc := &SessionCommandService{
		localMgr:            mgr,
		cp:                  composite,
		mirrors:             mirrorMgr,
		tmux:                tmuxMock,
		ensureConnectedFn:   hostMgr.EnsureConnected,
		resolveRemotePathFn: adapter.resolveRemotePath,
		remoteProviderFn: func(host string) remoteSessionAPI {
			if host == integrationRemoteHost {
				return fakeRemote
			}
			return nil
		},
	}
	adapter.commands = svc

	// Register the fake as the AERO remote. resolveRemotePath's CWDQuerier
	// lookup and CompositeProvider.providerForHost both read this map.
	composite.AddRemote(integrationRemoteHost, fakeRP)

	return &integrationStack{
		adapter:    adapter,
		svc:        svc,
		mgr:        mgr,
		composite:  composite,
		mirrorMgr:  mirrorMgr,
		hostMgr:    hostMgr,
		fakeRemote: fakeRemote,
		fakeRP:     fakeRP,
		tmuxMock:   tmuxMock,
		localProj:  localProj,
	}
}

// findSessionByHost returns the first session matching the given host.
// Helper for assertions — tests create exactly one session each, so there
// is no ambiguity in practice.
func findSessionByHost(mgr *session.Manager, host string) *session.Session {
	for _, s := range mgr.Sessions() {
		if s.Host == host {
			return &s
		}
	}
	return nil
}

// findProjectByPathHost returns the project with the given path+host tuple.
// Host is derived from the project's sessions (PM or first worker) because
// session.Project has no Host field of its own.
func findProjectByPathHost(mgr *session.Manager, path, host string) *session.Project {
	for _, p := range mgr.Projects() {
		if p.Path != path {
			continue
		}
		if projectHostOf(p) == host {
			return &p
		}
	}
	return nil
}

// projectHostOf inspects a project's sessions and returns the host they
// carry. Mirrors session.projectHost (unexported in the session package).
func projectHostOf(p session.Project) string {
	if p.PM != nil && p.PM.Host != "" {
		return p.PM.Host
	}
	for _, s := range p.Sessions {
		if s.Host != "" {
			return s.Host
		}
	}
	return ""
}

// --- n (CreateSession) ×4 ----------------------------------------------------

// 1. n, cursor on local node, pendingHost=AERO → stays local (plan table row 1)
func TestIntegration_n_LocalCursor_StaysLocal(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: "", OnNode: true}, integrationRemoteHost)

	require.NoError(t, s.adapter.Create(s.localProj))

	assert.Empty(t, s.fakeRemote.createCalls, "local cursor must not invoke remote API")

	sessions := s.mgr.Sessions()
	require.Len(t, sessions, 1)
	assert.Empty(t, sessions[0].Host, "session must be local")
	assert.Equal(t, s.localProj, sessions[0].Path)

	proj := findProjectByPathHost(s.mgr, s.localProj, "")
	require.NotNil(t, proj, "local project must exist")
}

// 2. n, cursor on remote node (/remote/proj), pendingHost=AERO → routes to AERO
func TestIntegration_n_RemoteCursor_RoutesToRemote(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: integrationRemoteHost, OnNode: true}, integrationRemoteHost)

	const remoteProj = "/remote/proj"
	require.NoError(t, s.adapter.Create(remoteProj))
	waitUntilStoreSettled(t, s.mgr, 1)

	assert.Equal(t, []string{remoteProj}, s.fakeRemote.createCalls,
		"remote CreateSession must receive the cursor-provided remote path verbatim")

	proj := findProjectByPathHost(s.mgr, remoteProj, integrationRemoteHost)
	require.NotNil(t, proj, "remote project must exist")
	require.Len(t, proj.Sessions, 1)
	assert.Equal(t, integrationRemoteHost, proj.Sessions[0].Host)
}

// 3. n, no cursor, pendingHost=AERO → path "." resolved via remoteCWD
func TestIntegration_n_NoCursor_FallsBackToPending(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, integrationRemoteHost)

	// App layer would pass filepath.Abs("."); "." also matches localProj
	// because chdirTemp put us there. Either form exercises the CWDQuerier
	// path — we pick "." to match the most common runtime case.
	require.NoError(t, s.adapter.Create("."))
	waitUntilStoreSettled(t, s.mgr, 1)

	assert.Equal(t, []string{s.fakeRP.remoteCWD}, s.fakeRemote.createCalls,
		"remote CreateSession must receive the remote CWD returned by QueryCWD")

	proj := findProjectByPathHost(s.mgr, s.fakeRP.remoteCWD, integrationRemoteHost)
	require.NotNil(t, proj, "remote project at remote CWD must exist")
	require.Len(t, proj.Sessions, 1)
}

// 4. n, no cursor, pendingHost="" → stays local at the cwd project path
func TestIntegration_n_NoCursor_NoPending_StaysLocal(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, "")

	require.NoError(t, s.adapter.Create(s.localProj))

	assert.Empty(t, s.fakeRemote.createCalls)

	proj := findProjectByPathHost(s.mgr, s.localProj, "")
	require.NotNil(t, proj)
	require.Len(t, proj.Sessions, 1)
}

// --- N (CreateAtPaneCWD) ×4 --------------------------------------------------

// 5. N, cursor on local node, pendingHost=AERO → still routes to pending remote
func TestIntegration_N_LocalCursor_RoutesToPending(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: "", OnNode: true}, integrationRemoteHost)

	require.NoError(t, s.adapter.CreateAtPaneCWD())
	waitUntilStoreSettled(t, s.mgr, 1)

	assert.Equal(t, []string{s.fakeRP.remoteCWD}, s.fakeRemote.createCalls,
		"N must bypass resolveHost() and route to pendingHost even with a local cursor")

	proj := findProjectByPathHost(s.mgr, s.fakeRP.remoteCWD, integrationRemoteHost)
	require.NotNil(t, proj)
	require.Len(t, proj.Sessions, 1)
}

// 6. N, cursor on remote node, pendingHost=AERO → pendingHost + remoteCWD
func TestIntegration_N_RemoteCursor_RoutesToPending(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: integrationRemoteHost, OnNode: true}, integrationRemoteHost)

	require.NoError(t, s.adapter.CreateAtPaneCWD())
	waitUntilStoreSettled(t, s.mgr, 1)

	assert.Equal(t, []string{s.fakeRP.remoteCWD}, s.fakeRemote.createCalls)

	proj := findProjectByPathHost(s.mgr, s.fakeRP.remoteCWD, integrationRemoteHost)
	require.NotNil(t, proj)
}

// 7. N, no cursor, pendingHost=AERO → pendingHost + remoteCWD
func TestIntegration_N_NoCursor_RoutesToPending(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, integrationRemoteHost)

	require.NoError(t, s.adapter.CreateAtPaneCWD())
	waitUntilStoreSettled(t, s.mgr, 1)

	assert.Equal(t, []string{s.fakeRP.remoteCWD}, s.fakeRemote.createCalls)
	proj := findProjectByPathHost(s.mgr, s.fakeRP.remoteCWD, integrationRemoteHost)
	require.NotNil(t, proj)
}

// 8. N, no cursor, pendingHost="" → stays local at the pane cwd
func TestIntegration_N_NoCursor_NoPending_StaysLocal(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, "")

	require.NoError(t, s.adapter.CreateAtPaneCWD())

	assert.Empty(t, s.fakeRemote.createCalls)

	// localDaemonProvider.Create translates "." to filepath.Abs(".") =
	// localProj, so the resulting project is at localProj.
	proj := findProjectByPathHost(s.mgr, s.localProj, "")
	require.NotNil(t, proj)
	require.Len(t, proj.Sessions, 1)
}

// --- w (CreateWorktree) ×4 ---------------------------------------------------

// 9. w, cursor on local node → git worktree add under localProj
func TestIntegration_w_LocalCursor_StaysLocal(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: "", OnNode: true}, integrationRemoteHost)

	require.NoError(t, s.adapter.CreateWorktree("feat-local", "do it", s.localProj))

	assert.Empty(t, s.fakeRP.worktreeCalls, "remote provider must not be invoked")
	wtPath := filepath.Join(s.localProj, session.WorktreePathSegment, "feat-local")
	info, err := os.Stat(wtPath)
	require.NoError(t, err, "git worktree add must have created the directory")
	assert.True(t, info.IsDir())

	proj := findProjectByPathHost(s.mgr, s.localProj, "")
	require.NotNil(t, proj)
	require.Len(t, proj.Sessions, 1)
	assert.Equal(t, wtPath, proj.Sessions[0].Path)
	assert.Equal(t, session.RoleWorker, proj.Sessions[0].Role)
}

// 10. w, cursor on remote node → fake CreateWorktree with the remote project path
func TestIntegration_w_RemoteCursor_RoutesToRemote(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: integrationRemoteHost, OnNode: true}, integrationRemoteHost)

	const remoteProj = "/remote/proj"
	require.NoError(t, s.adapter.CreateWorktree("feat-r", "do it", remoteProj))

	require.Len(t, s.fakeRP.worktreeCalls, 1)
	assert.Equal(t, remoteProj, s.fakeRP.worktreeCalls[0].Target.ProjectRoot)
	assert.Equal(t, "feat-r", s.fakeRP.worktreeCalls[0].Name)

	proj := findProjectByPathHost(s.mgr, remoteProj, integrationRemoteHost)
	require.NotNil(t, proj)
	require.Len(t, proj.Sessions, 1)
	assert.Equal(t, integrationRemoteHost, proj.Sessions[0].Host)
}

// 11. w, no cursor, pendingHost=AERO → resolveRemotePath("." → remoteCWD), fake receives remoteCWD
func TestIntegration_w_NoCursor_FallsBackToPending(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, integrationRemoteHost)

	require.NoError(t, s.adapter.CreateWorktree("feat-cwd", "do it", "."))

	require.Len(t, s.fakeRP.worktreeCalls, 1)
	assert.Equal(t, s.fakeRP.remoteCWD, s.fakeRP.worktreeCalls[0].Target.ProjectRoot,
		"remote worktree projectRoot must be the resolved remote CWD")
	proj := findProjectByPathHost(s.mgr, s.fakeRP.remoteCWD, integrationRemoteHost)
	require.NotNil(t, proj)
	require.Len(t, proj.Sessions, 1)
}

// 12. w, no cursor, no pending → local git worktree add at localProj
func TestIntegration_w_NoCursor_NoPending_StaysLocal(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, "")

	require.NoError(t, s.adapter.CreateWorktree("feat-12", "do it", s.localProj))

	assert.Empty(t, s.fakeRP.worktreeCalls)
	wtPath := filepath.Join(s.localProj, session.WorktreePathSegment, "feat-12")
	_, err := os.Stat(wtPath)
	require.NoError(t, err)

	proj := findProjectByPathHost(s.mgr, s.localProj, "")
	require.NotNil(t, proj)
	require.Len(t, proj.Sessions, 1)
	assert.Equal(t, wtPath, proj.Sessions[0].Path)
}

// --- W (ListWorktrees) ×4 ----------------------------------------------------

// 13. W, cursor on local node → real git worktree list on localProj (empty)
func TestIntegration_W_LocalCursor_StaysLocal(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: "", OnNode: true}, integrationRemoteHost)

	items, err := s.adapter.ListWorktrees(s.localProj)
	require.NoError(t, err)

	assert.Empty(t, s.fakeRP.listWTCalls, "local W must not route through fake remote provider")
	// A fresh git repo has no worktrees under .lazyclaude/worktrees; the
	// parser filters to that prefix and so returns an empty slice.
	assert.Empty(t, items)
}

// 14. W, cursor on remote node → fake.listWTCalls carries the remote project path
func TestIntegration_W_RemoteCursor_RoutesToRemote(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: integrationRemoteHost, OnNode: true}, integrationRemoteHost)

	const remoteProj = "/remote/proj"
	_, err := s.adapter.ListWorktrees(remoteProj)
	require.NoError(t, err)

	assert.Equal(t, []string{remoteProj}, s.fakeRP.listWTCalls)
}

// 15. W, no cursor, pendingHost=AERO → resolveRemotePath translates "." → remoteCWD
func TestIntegration_W_NoCursor_FallsBackToPending(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, integrationRemoteHost)

	_, err := s.adapter.ListWorktrees(".")
	require.NoError(t, err)

	assert.Equal(t, []string{s.fakeRP.remoteCWD}, s.fakeRP.listWTCalls)
}

// 16. W, no cursor, no pending → real git worktree list (empty on fresh repo)
func TestIntegration_W_NoCursor_NoPending_StaysLocal(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, "")

	items, err := s.adapter.ListWorktrees(s.localProj)
	require.NoError(t, err)

	assert.Empty(t, s.fakeRP.listWTCalls)
	assert.Empty(t, items)
}

// --- P (CreatePMSession) ×4 --------------------------------------------------

// 17. P, cursor on local node → localDaemonProvider.CreatePMSession
func TestIntegration_P_LocalCursor_StaysLocal(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: "", OnNode: true}, integrationRemoteHost)

	require.NoError(t, s.adapter.CreatePMSession(s.localProj))

	assert.Empty(t, s.fakeRP.pmCalls)
	proj := findProjectByPathHost(s.mgr, s.localProj, "")
	require.NotNil(t, proj)
	require.NotNil(t, proj.PM, "local PM session must be stored under project.PM")
	assert.Equal(t, session.RolePM, proj.PM.Role)
}

// 18. P, cursor on remote node → fake.pmCalls[0] == /remote/proj, mirror stored
func TestIntegration_P_RemoteCursor_RoutesToRemote(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: integrationRemoteHost, OnNode: true}, integrationRemoteHost)

	const remoteProj = "/remote/proj"
	require.NoError(t, s.adapter.CreatePMSession(remoteProj))

	assert.Equal(t, []string{remoteProj}, s.fakeRP.pmCalls)

	proj := findProjectByPathHost(s.mgr, remoteProj, integrationRemoteHost)
	require.NotNil(t, proj)
	require.NotNil(t, proj.PM)
	assert.Equal(t, integrationRemoteHost, proj.PM.Host)
	assert.Equal(t, session.RolePM, proj.PM.Role)
}

// 19. P, no cursor, pendingHost=AERO → remoteCWD resolution + mirror stored
func TestIntegration_P_NoCursor_FallsBackToPending(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, integrationRemoteHost)

	require.NoError(t, s.adapter.CreatePMSession("."))

	assert.Equal(t, []string{s.fakeRP.remoteCWD}, s.fakeRP.pmCalls)

	proj := findProjectByPathHost(s.mgr, s.fakeRP.remoteCWD, integrationRemoteHost)
	require.NotNil(t, proj)
	require.NotNil(t, proj.PM)
}

// 20. P, no cursor, no pending → local PM at localProj
func TestIntegration_P_NoCursor_NoPending_StaysLocal(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, "")

	require.NoError(t, s.adapter.CreatePMSession(s.localProj))

	assert.Empty(t, s.fakeRP.pmCalls)
	proj := findProjectByPathHost(s.mgr, s.localProj, "")
	require.NotNil(t, proj)
	require.NotNil(t, proj.PM)
}

// --- worker (CreateWorkerSession) ×4 -----------------------------------------

// 21. worker, cursor on local node → local worktree session with role=worker
func TestIntegration_worker_LocalCursor_StaysLocal(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: "", OnNode: true}, integrationRemoteHost)

	require.NoError(t, s.adapter.CreateWorkerSession("work-21", "task", s.localProj))

	assert.Empty(t, s.fakeRP.workerCalls)

	proj := findProjectByPathHost(s.mgr, s.localProj, "")
	require.NotNil(t, proj)
	require.Len(t, proj.Sessions, 1)
	assert.Equal(t, session.RoleWorker, proj.Sessions[0].Role)
	wtPath := filepath.Join(s.localProj, session.WorktreePathSegment, "work-21")
	assert.Equal(t, wtPath, proj.Sessions[0].Path)
}

// 22. worker, cursor on remote node → fake.workerCalls[0] == /remote/proj
func TestIntegration_worker_RemoteCursor_RoutesToRemote(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: integrationRemoteHost, OnNode: true}, integrationRemoteHost)

	const remoteProj = "/remote/proj"
	require.NoError(t, s.adapter.CreateWorkerSession("work-22", "task", remoteProj))

	require.Len(t, s.fakeRP.workerCalls, 1)
	assert.Equal(t, remoteProj, s.fakeRP.workerCalls[0].Target.ProjectRoot)
	assert.Equal(t, "work-22", s.fakeRP.workerCalls[0].Name)

	proj := findProjectByPathHost(s.mgr, remoteProj, integrationRemoteHost)
	require.NotNil(t, proj)
	require.Len(t, proj.Sessions, 1)
	assert.Equal(t, session.RoleWorker, proj.Sessions[0].Role)
}

// 23. worker, no cursor, pendingHost=AERO → resolveRemotePath translates "." → remoteCWD
func TestIntegration_worker_NoCursor_FallsBackToPending(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, integrationRemoteHost)

	require.NoError(t, s.adapter.CreateWorkerSession("work-23", "task", "."))

	require.Len(t, s.fakeRP.workerCalls, 1)
	assert.Equal(t, s.fakeRP.remoteCWD, s.fakeRP.workerCalls[0].Target.ProjectRoot)

	proj := findProjectByPathHost(s.mgr, s.fakeRP.remoteCWD, integrationRemoteHost)
	require.NotNil(t, proj)
	require.Len(t, proj.Sessions, 1)
}

// 24. worker, no cursor, no pending → local worker at localProj
func TestIntegration_worker_NoCursor_NoPending_StaysLocal(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, "")

	require.NoError(t, s.adapter.CreateWorkerSession("work-24", "task", s.localProj))

	assert.Empty(t, s.fakeRP.workerCalls)

	proj := findProjectByPathHost(s.mgr, s.localProj, "")
	require.NotNil(t, proj)
	require.Len(t, proj.Sessions, 1)
	assert.Equal(t, session.RoleWorker, proj.Sessions[0].Role)
}

// --- g (LaunchLazygit) ×2 — remote only --------------------------------------

// 25. g, cursor on remote node → fake.lazygitCalls[0] == /remote/proj
func TestIntegration_g_RemoteCursor_RoutesToRemote(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: integrationRemoteHost, OnNode: true}, integrationRemoteHost)

	const remoteProj = "/remote/proj"
	require.NoError(t, s.adapter.LaunchLazygit(remoteProj))

	assert.Equal(t, []string{remoteProj}, s.fakeRP.lazygitCalls)
}

// 26. g, no cursor, pendingHost=AERO → resolveRemotePath translates "." → remoteCWD
func TestIntegration_g_NoCursor_FallsBackToPending(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, integrationRemoteHost)

	require.NoError(t, s.adapter.LaunchLazygit("."))

	assert.Equal(t, []string{s.fakeRP.remoteCWD}, s.fakeRP.lazygitCalls)
}

// --- d (Delete) ×2 -----------------------------------------------------------

// 27. d, local session → store.Remove + tmux KillWindow(lc-), no remote API
func TestIntegration_d_LocalSession(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, "")

	// Create a local session to delete.
	require.NoError(t, s.adapter.Create(s.localProj))
	require.Len(t, s.mgr.Sessions(), 1)
	sess := s.mgr.Sessions()[0]

	require.NoError(t, s.adapter.Delete(sess.ID))

	assert.Empty(t, s.fakeRemote.deleteCalls, "remote API must not be invoked for local session")
	assert.Empty(t, s.mgr.Sessions(), "local store must be empty after delete")

	// tmux mock should have received a KillWindow for the local window.
	assertKillWindowContains(t, s.tmuxMock, "lc-")
}

// 28. d, remote session → store.Remove + fakeRemote.Delete + tmux KillWindow(rm-)
func TestIntegration_d_RemoteSession(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: integrationRemoteHost, OnNode: true}, integrationRemoteHost)

	// Create a remote session via the full stack so we have a mirror
	// window and a Host-bearing store entry.
	const remoteProj = "/remote/proj"
	require.NoError(t, s.adapter.Create(remoteProj))
	waitUntilStoreSettled(t, s.mgr, 1)
	sess := s.mgr.Sessions()[0]
	require.Equal(t, integrationRemoteHost, sess.Host)

	// Reset KillWindow history so we only see the delete-time kill.
	s.tmuxMock.killedWindows = nil

	require.NoError(t, s.adapter.Delete(sess.ID))

	assert.Equal(t, []string{sess.ID}, s.fakeRemote.deleteCalls,
		"remote API Delete must be called with the session id")
	assert.Empty(t, s.mgr.Sessions(), "local store must be empty after delete")

	// Mirror window prefix is rm- per session.MirrorWindowName.
	assertKillWindowContains(t, s.tmuxMock, "rm-")
}

// --- R (Rename) ×2 -----------------------------------------------------------

// 29. R, local session → store.Name updated, no remote API
func TestIntegration_R_LocalSession(t *testing.T) {
	s := newIntegrationStack(t, cursorState{}, "")

	require.NoError(t, s.adapter.Create(s.localProj))
	require.Len(t, s.mgr.Sessions(), 1)
	sess := s.mgr.Sessions()[0]

	require.NoError(t, s.adapter.Rename(sess.ID, "renamed-local"))

	assert.Empty(t, s.fakeRemote.renameCalls)
	updated := s.mgr.Store().FindByID(sess.ID)
	require.NotNil(t, updated)
	assert.Equal(t, "renamed-local", updated.Name)
}

// 30. R, remote session → fakeRemote.Rename + store.Name updated
func TestIntegration_R_RemoteSession(t *testing.T) {
	s := newIntegrationStack(t, cursorState{Host: integrationRemoteHost, OnNode: true}, integrationRemoteHost)

	const remoteProj = "/remote/proj"
	require.NoError(t, s.adapter.Create(remoteProj))
	waitUntilStoreSettled(t, s.mgr, 1)
	sess := s.mgr.Sessions()[0]
	require.Equal(t, integrationRemoteHost, sess.Host)

	require.NoError(t, s.adapter.Rename(sess.ID, "renamed-remote"))

	assert.Equal(t, []renameCall{{ID: sess.ID, NewName: "renamed-remote"}}, s.fakeRemote.renameCalls)
	updated := s.mgr.Store().FindByID(sess.ID)
	require.NotNil(t, updated)
	assert.Equal(t, "renamed-remote", updated.Name)
}

// --- tmux assertion helper ---------------------------------------------------

// assertKillWindowContains asserts that recordingTmux recorded at least
// one KillWindow whose target contains the given substring. Used by d
// tests to verify that the correct window prefix (lc- vs rm-) was killed.
func assertKillWindowContains(t *testing.T, rec *recordingTmux, substr string) {
	t.Helper()
	for _, target := range rec.killedWindows {
		if strings.Contains(target, substr) {
			return
		}
	}
	t.Fatalf("expected KillWindow target containing %q, got %v", substr, rec.killedWindows)
}
