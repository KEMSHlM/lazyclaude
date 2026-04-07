package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/any-context/lazyclaude/internal/core/config"
	"github.com/any-context/lazyclaude/internal/core/model"
	"github.com/any-context/lazyclaude/internal/core/tmux"
	"github.com/any-context/lazyclaude/internal/daemon"
	"github.com/any-context/lazyclaude/internal/gui"
	"github.com/any-context/lazyclaude/internal/notify"
	"github.com/any-context/lazyclaude/internal/session"
	"github.com/google/uuid"
)

// guiCompositeAdapter wraps daemon.CompositeProvider to implement gui.SessionProvider.
// This bridges the daemon's type system (daemon.SessionInfo etc.) to the GUI's
// type system (gui.SessionItem etc.).
type guiCompositeAdapter struct {
	cp         *daemon.CompositeProvider
	localMgr   *session.Manager
	tmuxClient tmux.Client
	paths      config.Paths

	// windowActivityFn provides window->activity mapping from the App layer.
	windowActivityFn func() map[string]gui.WindowActivityEntry

	// cachedPending is refreshed once per layout cycle.
	cachedPending map[string]bool

	// Lazy remote connection: pendingHost is the default SSH host, initially set
	// at construction from DetectSSHHost() and updated by SetPendingHost after
	// successful connect-dialog connections. Protected by hostMu for thread safety.
	// RWMutex because reads (every operation) vastly outnumber writes (connect dialog).
	hostMu           sync.RWMutex
	pendingHost      string             // Default SSH host (updated after connect dialog)
	localProjectRoot string             // Local project root at startup (immutable after construction)
	connectFn        func(string) error // connectRemoteHost from root.go
	connectMu        sync.Mutex
	connecting       map[string]*lazyConn // one entry per host

	// onError reports errors to the GUI via showError. Wired in root.go.
	// lastErrorMsg deduplicates consecutive identical errors to avoid flooding
	// the GUI when Sessions() fails persistently (e.g. daemon unreachable).
	onError      func(msg string)
	lastErrorMsg string

	// guiUpdateFn triggers a GUI refresh from background goroutines.
	guiUpdateFn func() // triggers gui.Update (wired in root.go)
}

// Compile-time checks.
var _ gui.SessionProvider = (*guiCompositeAdapter)(nil)
var _ gui.HostAwareCreator = (*guiCompositeAdapter)(nil)

// SetPendingHost updates the default remote host. Called after a successful
// connection via the connect dialog so that subsequent operations route to
// the newly connected host.
func (a *guiCompositeAdapter) SetPendingHost(host string) {
	a.hostMu.Lock()
	defer a.hostMu.Unlock()
	debugLog("SetPendingHost: %q -> %q", a.pendingHost, host)
	a.pendingHost = host
}

// readPendingHost returns the current default remote host (thread-safe).
func (a *guiCompositeAdapter) readPendingHost() string {
	a.hostMu.RLock()
	defer a.hostMu.RUnlock()
	return a.pendingHost
}

// lazyConn ensures a remote host is connected exactly once.
// If the initial connect fails, subsequent callers see the cached error
// without retrying (connectRemoteHost leaves no side effects on failure).
type lazyConn struct {
	once sync.Once
	err  error
}

// markConnected records that a host has been successfully connected via an
// external path (e.g. the connect dialog). This populates the lazyConn cache
// so that ensureRemoteConnected skips the redundant connectFn call.
func (a *guiCompositeAdapter) markConnected(host string) {
	a.connectMu.Lock()
	defer a.connectMu.Unlock()
	if a.connecting == nil {
		a.connecting = make(map[string]*lazyConn)
	}
	lc := &lazyConn{}
	lc.once.Do(func() {}) // mark as completed with nil error
	a.connecting[host] = lc
	debugLog("markConnected: host=%q cached in lazyConn", host)
}

// ensureRemoteConnected lazily establishes a remote connection on first use.
// Returns nil if host is empty (local operation) or already connected.
// Uses sync.Once per host to guarantee exactly one connectFn call.
func (a *guiCompositeAdapter) ensureRemoteConnected(host string) error {
	debugLog("ensureRemoteConnected: host=%q connectFn=%v", host, a.connectFn != nil)
	if host == "" || a.connectFn == nil {
		return nil
	}

	a.connectMu.Lock()
	if a.connecting == nil {
		a.connecting = make(map[string]*lazyConn)
	}
	lc, ok := a.connecting[host]
	if !ok {
		lc = &lazyConn{}
		a.connecting[host] = lc
	}
	a.connectMu.Unlock()

	lc.once.Do(func() {
		debugLog("ensureRemoteConnected: calling connectFn for host=%q", host)
		lc.err = a.connectFn(host)
		debugLog("ensureRemoteConnected: connectFn result: %v", lc.err)
	})
	return lc.err
}

func (a *guiCompositeAdapter) RefreshPendingFrom(notifications []*model.ToolNotification) {
	a.cachedPending = pendingWindowSet(notifications)
}

func (a *guiCompositeAdapter) Sessions() []gui.SessionItem {
	sessions, err := a.cp.Sessions()
	if err != nil {
		msg := fmt.Sprintf("Session list error: %v", err)
		if a.onError != nil && msg != a.lastErrorMsg {
			a.lastErrorMsg = msg
			a.onError(msg)
		}
		return nil
	}
	a.lastErrorMsg = "" // clear on success so next error is reported
	items := make([]gui.SessionItem, len(sessions))
	activity := a.getWindowActivity()
	for i, s := range sessions {
		items[i] = daemonInfoToGUIItem(s, a.cachedPending, activity)
	}
	return items
}

func (a *guiCompositeAdapter) getWindowActivity() map[string]gui.WindowActivityEntry {
	if a.windowActivityFn != nil {
		return a.windowActivityFn()
	}
	return nil
}

func (a *guiCompositeAdapter) Projects() []gui.ProjectItem {
	projects := a.localMgr.Projects()
	activity := a.getWindowActivity()
	items := buildProjectItems(projects, a.cachedPending, activity)

	// Merge remote sessions as separate projects.
	remoteSessions, err := a.cp.Sessions()
	if err != nil {
		return items
	}
	// Group remote sessions by host+path into projects.
	type remoteProject struct {
		host     string
		path     string
		sessions []gui.SessionItem
	}
	rpMap := make(map[string]*remoteProject)
	for _, s := range remoteSessions {
		if s.Host == "" {
			continue // local session, already in projects
		}
		key := s.Host + ":" + s.Path
		rp, ok := rpMap[key]
		if !ok {
			rp = &remoteProject{host: s.Host, path: s.Path}
			rpMap[key] = rp
		}
		rp.sessions = append(rp.sessions, daemonInfoToGUIItem(s, a.cachedPending, activity))
	}
	for _, rp := range rpMap {
		name := rp.path
		if idx := strings.LastIndex(name, "/"); idx >= 0 && idx < len(name)-1 {
			name = name[idx+1:]
		}
		items = append(items, gui.ProjectItem{
			ID:       "remote-" + rp.host + "-" + rp.path,
			Name:     name,
			Path:     rp.path,
			Host:     rp.host,
			Expanded: true,
			Sessions: rp.sessions,
		})
	}
	return items
}

func (a *guiCompositeAdapter) ToggleProjectExpanded(projectID string) {
	a.localMgr.ToggleProjectExpanded(projectID)
}

func (a *guiCompositeAdapter) Create(path string) error {
	return a.createWithHost(path, a.readPendingHost())
}

// CreateWithHost creates a session on the specified host. If host is empty,
// falls back to pendingHost (same behavior as Create).
func (a *guiCompositeAdapter) CreateWithHost(path, host string) error {
	if host == "" {
		host = a.readPendingHost()
	}
	return a.createWithHost(path, host)
}

// createWithHost is the shared implementation for Create and CreateWithHost.
func (a *guiCompositeAdapter) createWithHost(path, host string) error {
	debugLog("createWithHost: path=%q host=%q", path, host)
	if host == "" {
		// Local: synchronous (existing behavior).
		return a.cp.Create(path, "")
	}

	// Remote: optimistic creation. Add a placeholder to the local store
	// immediately so it appears in the sidebar, then attempt connection
	// and session creation in the background. The path is resolved to the
	// remote CWD after the connection is established.
	placeholder := session.Session{
		ID:        uuid.New().String(),
		Name:      a.localMgr.Store().GenerateName(host),
		Path:      path,
		Host:      host,
		Status:    session.StatusRunning,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	a.localMgr.Store().Add(placeholder, "")
	if err := a.localMgr.Store().Save(); err != nil {
		return fmt.Errorf("save placeholder: %w", err)
	}

	go a.completeRemoteCreate(placeholder.ID, path, host)
	return nil
}

// completeRemoteCreate runs in a background goroutine to finish the
// optimistic session creation. On failure it marks the placeholder as
// dead and stores the error message. On success it maps the placeholder
// to the real remote session for preview routing.
func (a *guiCompositeAdapter) completeRemoteCreate(placeholderID, localPath, host string) {
	debugLog("completeRemoteCreate: placeholderID=%q localPath=%q host=%q", placeholderID, localPath, host)
	if err := a.ensureRemoteConnected(host); err != nil {
		debugLog("completeRemoteCreate: ensureRemoteConnected failed: %v", err)
		a.failPlaceholder(placeholderID, fmt.Sprintf("Connection failed: %v", err))
		return
	}

	// Resolve the local path to the remote CWD now that the connection exists.
	remotePath := a.resolveRemotePath(localPath, host)
	debugLog("completeRemoteCreate: resolveRemotePath input=%q output=%q", localPath, remotePath)

	if err := a.cp.Create(remotePath, host); err != nil {
		debugLog("completeRemoteCreate: cp.Create failed: %v", err)
		a.failPlaceholder(placeholderID, fmt.Sprintf("Session creation failed: %v", err))
		return
	}
	debugLog("completeRemoteCreate: cp.Create succeeded")

	// Remove the placeholder -- the real remote session will be shown
	// via CompositeProvider.Sessions() from the daemon.
	debugLog("completeRemoteCreate: removing placeholder %s, remote session visible via daemon", placeholderID[:8])
	a.localMgr.Store().Remove(placeholderID)
	_ = a.localMgr.Store().Save()
	a.triggerGUIUpdate()
}

// failPlaceholder marks a placeholder session as dead and creates a tmux error
// window so that preview, fullscreen, and visual mode all work normally.
func (a *guiCompositeAdapter) failPlaceholder(id, msg string) {
	a.localMgr.Store().SetStatus(id, session.StatusDead)

	// Create a tmux window that displays the error message.
	// This makes the error visible via normal pane capture (preview/fullscreen).
	// The message is passed via environment variable to avoid shell injection
	// (error messages may contain newlines, quotes, or control characters).
	sess := a.localMgr.Store().FindByID(id)
	if sess != nil && a.tmuxClient != nil {
		windowName := sess.WindowName()
		const errCmd = "echo 'lazyclaude: session launch failed'; echo; echo \"$LAZYCLAUDE_ERR_MSG\"; echo; echo 'Press Enter to close'; read"
		abs, err := filepath.Abs(".")
		if err != nil {
			abs = "."
		}
		ctx := context.Background()
		if err := a.tmuxClient.NewWindow(ctx, tmux.NewWindowOpts{
			Session:  "lazyclaude",
			Name:     windowName,
			Command:  errCmd,
			StartDir: abs,
			Env:      map[string]string{"LAZYCLAUDE_ERR_MSG": msg},
		}); err != nil {
			if a.onError != nil {
				a.onError(fmt.Sprintf("create error window: %v", err))
			}
		} else {
			a.localMgr.Store().SetTmuxWindow(id, "lazyclaude:"+windowName)
		}
	}

	if err := a.localMgr.Store().Save(); err != nil && a.onError != nil {
		a.onError(fmt.Sprintf("save store: %v", err))
	}
	if a.onError != nil {
		a.onError(msg)
	}
	a.triggerGUIUpdate()
}

// triggerGUIUpdate schedules a GUI refresh if the callback is wired.
func (a *guiCompositeAdapter) triggerGUIUpdate() {
	if a.guiUpdateFn != nil {
		a.guiUpdateFn()
	}
}

// resolveRemotePath maps a local path to the remote daemon's CWD when
// creating the first session on an SSH host. Once remote sessions exist,
// currentProjectRoot() returns the correct remote path from the session
// tree, so the provided path is returned unchanged.
//
// The remote CWD is obtained via the daemon GET /cwd API. This requires
// the remote connection to be established first (call ensureRemoteConnected
// before this method).
func (a *guiCompositeAdapter) resolveRemotePath(path, host string) string {
	debugLog("resolveRemotePath: input=%q host=%q", path, host)
	// Always query the remote daemon for its CWD when the host is set.
	// Local paths (from currentProjectRoot fallback) are meaningless on
	// the remote machine.
	remoteCWD := a.queryRemoteCWD(host)
	if remoteCWD != "" {
		debugLog("resolveRemotePath: output=%q (from queryRemoteCWD)", remoteCWD)
		return remoteCWD
	}
	// Fallback: use "." so the daemon uses its own CWD.
	if host != "" {
		debugLog("resolveRemotePath: output=%q (fallback dot)", ".")
		return "."
	}
	debugLog("resolveRemotePath: output=%q (passthrough)", path)
	return path
}

// cwdQueryTimeout is the maximum time to wait for a remote CWD query.
const cwdQueryTimeout = 10 * time.Second

// queryRemoteCWD fetches the working directory from a connected remote daemon.
// Returns "" if the query fails (caller should fall back to the original path).
func (a *guiCompositeAdapter) queryRemoteCWD(host string) string {
	debugLog("queryRemoteCWD: host=%q", host)
	provider := a.cp.RemoteProvider(host)
	debugLog("queryRemoteCWD: provider=%v", provider != nil)
	if provider == nil {
		return ""
	}
	querier, ok := provider.(daemon.CWDQuerier)
	debugLog("queryRemoteCWD: implements CWDQuerier=%v", ok)
	if !ok {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), cwdQueryTimeout)
	defer cancel()
	cwd, err := querier.QueryCWD(ctx)
	debugLog("queryRemoteCWD: cwd=%q err=%v", cwd, err)
	if err != nil {
		return ""
	}
	return cwd
}

func (a *guiCompositeAdapter) Delete(id string) error {
	return a.cp.Delete(id)
}

func (a *guiCompositeAdapter) Rename(id, newName string) error {
	return a.cp.Rename(id, newName)
}

func (a *guiCompositeAdapter) PurgeOrphans() (int, error) {
	return a.cp.PurgeOrphans()
}

func (a *guiCompositeAdapter) CapturePreview(id string, width, height int) (gui.PreviewResult, error) {
	resp, err := a.cp.CapturePreview(id, width, height)
	if err != nil || resp == nil {
		return gui.PreviewResult{}, err
	}
	return gui.PreviewResult{
		Content: resp.Content,
		CursorX: resp.CursorX,
		CursorY: resp.CursorY,
	}, nil
}

func (a *guiCompositeAdapter) CaptureScrollback(id string, width, startLine, endLine int) (gui.PreviewResult, error) {
	resp, err := a.cp.CaptureScrollback(id, width, startLine, endLine)
	if err != nil || resp == nil {
		return gui.PreviewResult{}, err
	}
	return gui.PreviewResult{Content: resp.Content}, nil
}

func (a *guiCompositeAdapter) HistorySize(id string) (int, error) {
	return a.cp.HistorySize(id)
}

func (a *guiCompositeAdapter) PendingNotifications() []*model.ToolNotification {
	notifications, err := notify.ReadAll(a.paths.RuntimeDir)
	if err != nil || len(notifications) == 0 {
		return nil
	}
	return notifications
}

func (a *guiCompositeAdapter) SendChoice(window string, c gui.Choice) error {
	return a.cp.SendChoice(window, int(c))
}

func (a *guiCompositeAdapter) AttachSession(id string) error {
	return a.cp.AttachSession(id)
}

func (a *guiCompositeAdapter) LaunchLazygit(path string) error {
	return a.cp.LaunchLazygit(path, "")
}

func (a *guiCompositeAdapter) CreateWorktree(name, prompt, projectRoot string) error {
	return a.createWorktreeWithHost(name, prompt, projectRoot, a.readPendingHost())
}

func (a *guiCompositeAdapter) CreateWorktreeWithHost(name, prompt, projectRoot, host string) error {
	if host == "" {
		host = a.readPendingHost()
	}
	return a.createWorktreeWithHost(name, prompt, projectRoot, host)
}

func (a *guiCompositeAdapter) createWorktreeWithHost(name, prompt, projectRoot, host string) error {
	debugLog("createWorktreeWithHost: name=%q host=%q", name, host)
	if err := a.ensureRemoteConnected(host); err != nil {
		return err
	}
	if host != "" {
		projectRoot = a.resolveRemotePath(projectRoot, host)
	}
	return a.cp.CreateWorktree(name, prompt, projectRoot, host)
}

func (a *guiCompositeAdapter) ResumeWorktree(worktreePath, prompt, projectRoot string) error {
	return a.resumeWorktreeWithHost(worktreePath, prompt, projectRoot, a.readPendingHost())
}

func (a *guiCompositeAdapter) ResumeWorktreeWithHost(worktreePath, prompt, projectRoot, host string) error {
	if host == "" {
		host = a.readPendingHost()
	}
	return a.resumeWorktreeWithHost(worktreePath, prompt, projectRoot, host)
}

func (a *guiCompositeAdapter) resumeWorktreeWithHost(worktreePath, prompt, projectRoot, host string) error {
	debugLog("resumeWorktreeWithHost: wtPath=%q host=%q", worktreePath, host)
	if err := a.ensureRemoteConnected(host); err != nil {
		return err
	}
	if host != "" {
		projectRoot = a.resolveRemotePath(projectRoot, host)
	}
	return a.cp.ResumeWorktree(worktreePath, prompt, projectRoot, host)
}

func (a *guiCompositeAdapter) ListWorktrees(projectRoot string) ([]gui.WorktreeInfo, error) {
	return a.listWorktreesWithHost(projectRoot, a.readPendingHost())
}

func (a *guiCompositeAdapter) ListWorktreesWithHost(projectRoot, host string) ([]gui.WorktreeInfo, error) {
	if host == "" {
		host = a.readPendingHost()
	}
	return a.listWorktreesWithHost(projectRoot, host)
}

func (a *guiCompositeAdapter) listWorktreesWithHost(projectRoot, host string) ([]gui.WorktreeInfo, error) {
	debugLog("listWorktreesWithHost: projectRoot=%q host=%q", projectRoot, host)
	if err := a.ensureRemoteConnected(host); err != nil {
		return nil, err
	}
	if host != "" {
		projectRoot = a.resolveRemotePath(projectRoot, host)
	}
	items, err := a.cp.ListWorktrees(projectRoot, host)
	if err != nil {
		return nil, err
	}
	result := make([]gui.WorktreeInfo, len(items))
	for i, item := range items {
		result[i] = gui.WorktreeInfo{Name: item.Name, Path: item.Path, Branch: item.Branch}
	}
	return result, nil
}

func (a *guiCompositeAdapter) CreatePMSession(projectRoot string) error {
	return a.createPMSessionWithHost(projectRoot, a.readPendingHost())
}

func (a *guiCompositeAdapter) CreatePMSessionWithHost(projectRoot, host string) error {
	if host == "" {
		host = a.readPendingHost()
	}
	return a.createPMSessionWithHost(projectRoot, host)
}

func (a *guiCompositeAdapter) createPMSessionWithHost(projectRoot, host string) error {
	debugLog("createPMSessionWithHost: projectRoot=%q host=%q", projectRoot, host)
	if err := a.ensureRemoteConnected(host); err != nil {
		return err
	}
	if host != "" {
		projectRoot = a.resolveRemotePath(projectRoot, host)
	}
	return a.cp.CreatePMSession(projectRoot, host)
}

func (a *guiCompositeAdapter) CreateWorkerSession(name, prompt, projectRoot string) error {
	return a.createWorkerSessionWithHost(name, prompt, projectRoot, a.readPendingHost())
}

func (a *guiCompositeAdapter) CreateWorkerSessionWithHost(name, prompt, projectRoot, host string) error {
	if host == "" {
		host = a.readPendingHost()
	}
	return a.createWorkerSessionWithHost(name, prompt, projectRoot, host)
}

func (a *guiCompositeAdapter) createWorkerSessionWithHost(name, prompt, projectRoot, host string) error {
	debugLog("createWorkerSessionWithHost: name=%q host=%q", name, host)
	if err := a.ensureRemoteConnected(host); err != nil {
		return err
	}
	if host != "" {
		projectRoot = a.resolveRemotePath(projectRoot, host)
	}
	return a.cp.CreateWorkerSession(name, prompt, projectRoot, host)
}

// daemonInfoToGUIItem converts daemon.SessionInfo to gui.SessionItem.
func daemonInfoToGUIItem(s daemon.SessionInfo, pending map[string]bool, windowActivity map[string]gui.WindowActivityEntry) gui.SessionItem {
	activity := model.ActivityUnknown
	toolName := ""

	if s.Status == "running" {
		if wa, ok := windowActivity[s.TmuxWindow]; ok {
			activity = wa.State
			toolName = wa.ToolName
		}
	}

	if s.Status == "running" && pending[s.TmuxWindow] {
		activity = model.ActivityNeedsInput
	}

	return gui.SessionItem{
		ID:         s.ID,
		Name:       s.Name,
		Path:       s.Path,
		Host:       s.Host,
		Status:     s.Status,
		Flags:      s.Flags,
		TmuxWindow: s.TmuxWindow,
		Activity:   activity,
		ToolName:   toolName,
		Role:       s.Role,
	}
}
