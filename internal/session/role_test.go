package session_test

import (
	"strings"
	"testing"

	"github.com/any-context/lazyclaude/internal/session"
)

func TestRole_String(t *testing.T) {
	t.Parallel()
	tests := []struct {
		role session.Role
		want string
	}{
		{session.RoleNone, "none"},
		{session.RolePM, "pm"},
		{session.RoleWorker, "worker"},
		{session.Role("unknown"), "unknown"},
	}
	for _, tt := range tests {
		got := tt.role.String()
		if got != tt.want {
			t.Errorf("Role(%q).String() = %q, want %q", string(tt.role), got, tt.want)
		}
	}
}

func TestRole_IsValid(t *testing.T) {
	t.Parallel()
	valid := []session.Role{
		session.RoleNone,
		session.RolePM,
		session.RoleWorker,
	}
	for _, r := range valid {
		if !r.IsValid() {
			t.Errorf("Role(%q).IsValid() = false, want true", string(r))
		}
	}

	invalid := []session.Role{
		session.Role("admin"),
		session.Role("PM"),
		session.Role("Worker"),
		session.Role("unknown"),
	}
	for _, r := range invalid {
		if r.IsValid() {
			t.Errorf("Role(%q).IsValid() = true, want false", string(r))
		}
	}
}

func TestBuildPMPrompt_ContainsRequiredFields(t *testing.T) {
	t.Parallel()
	prompt := session.BuildPMPrompt("sess-abc123", "worker-1, worker-2")

	cases := []struct {
		desc    string
		snippet string
	}{
		{"sessionID", "sess-abc123"},
		{"worker list", "worker-1, worker-2"},
		{"sessions CLI", "lazyclaude sessions"},
		{"msg send CLI", "lazyclaude msg send"},
		{"role description", "PM"},
		{"review criteria correctness", "correctness"},
		{"review criteria tests", "test"},
		{"review criteria security", "security"},
		{"push delivery notice", "delivered directly"},
	}
	for _, tc := range cases {
		if !strings.Contains(prompt, tc.snippet) {
			t.Errorf("BuildPMPrompt missing %s: want %q in prompt", tc.desc, tc.snippet)
		}
	}
}

func TestBuildPMPrompt_NoPollInstructions(t *testing.T) {
	t.Parallel()
	prompt := session.BuildPMPrompt("sess-xyz", "")
	if strings.Contains(prompt, "/msg/poll") {
		t.Error("BuildPMPrompt should not contain /msg/poll (push-based, no polling needed)")
	}
}

func TestBuildPMPrompt_EmptyWorkerList(t *testing.T) {
	t.Parallel()
	prompt := session.BuildPMPrompt("sess-xyz", "")
	// Should still contain the CLI commands
	if !strings.Contains(prompt, "lazyclaude sessions") {
		t.Error("BuildPMPrompt with empty worker list should still contain lazyclaude sessions")
	}
}

func TestBuildPMPrompt_UsesCLINotCurl(t *testing.T) {
	t.Parallel()
	prompt := session.BuildPMPrompt("sess-pm", "")

	// Must use CLI subcommands, not raw curl
	if strings.Contains(prompt, "curl -s") {
		t.Error("prompt should use lazyclaude CLI, not curl")
	}
	if strings.Contains(prompt, "$PORT") {
		t.Error("prompt should not contain $PORT (CLI handles discovery)")
	}
	if strings.Contains(prompt, "$TOKEN") {
		t.Error("prompt should not contain $TOKEN (CLI handles discovery)")
	}
}

func TestBuildWorkerPrompt_ContainsRequiredFields(t *testing.T) {
	t.Parallel()
	prompt := session.BuildWorkerPrompt(
		"/project/.claude/worktrees/feat-x",
		"/project",
		"sess-worker-99",
	)

	cases := []struct {
		desc    string
		snippet string
	}{
		{"worktree path", "/project/.claude/worktrees/feat-x"},
		{"project root", "/project"},
		{"sessionID", "sess-worker-99"},
		{"sessions CLI", "lazyclaude sessions"},
		{"msg send CLI", "lazyclaude msg send"},
		{"isolation instruction", "NEVER modify"},
		{"role description", "Worker"},
		{"review request instruction", "review"},
		{"push delivery notice", "delivered directly"},
	}
	for _, tc := range cases {
		if !strings.Contains(prompt, tc.snippet) {
			t.Errorf("BuildWorkerPrompt missing %s: want %q in prompt", tc.desc, tc.snippet)
		}
	}
}

func TestBuildWorkerPrompt_NoPollInstructions(t *testing.T) {
	t.Parallel()
	prompt := session.BuildWorkerPrompt(
		"/project/.claude/worktrees/feat-x",
		"/project",
		"sess-worker-99",
	)
	if strings.Contains(prompt, "/msg/poll") {
		t.Error("BuildWorkerPrompt should not contain /msg/poll (push-based, no polling needed)")
	}
}

func TestBuildWorkerPrompt_PathIsolation(t *testing.T) {
	t.Parallel()
	worktree := "/home/user/project/.claude/worktrees/my-task"
	root := "/home/user/project"
	prompt := session.BuildWorkerPrompt(worktree, root, "id-1")

	if !strings.Contains(prompt, worktree) {
		t.Errorf("BuildWorkerPrompt missing worktree path %q", worktree)
	}
	if !strings.Contains(prompt, root) {
		t.Errorf("BuildWorkerPrompt missing project root %q", root)
	}
}

func TestBuildWorkerPrompt_UsesCLINotCurl(t *testing.T) {
	t.Parallel()
	prompt := session.BuildWorkerPrompt(
		"/project/.claude/worktrees/feat-x",
		"/project",
		"sess-worker-99",
	)

	if strings.Contains(prompt, "curl -s") {
		t.Error("prompt should use lazyclaude CLI, not curl")
	}
	if strings.Contains(prompt, "$PORT") {
		t.Error("prompt should not contain $PORT (CLI handles discovery)")
	}
	if strings.Contains(prompt, "$TOKEN") {
		t.Error("prompt should not contain $TOKEN (CLI handles discovery)")
	}
}

func TestBuildWorkerPrompt_SessionIDInFromFlag(t *testing.T) {
	t.Parallel()
	prompt := session.BuildWorkerPrompt(
		"/project/.claude/worktrees/feat-x",
		"/project",
		"sess-worker-99",
	)

	// The --from flag should contain the session ID
	if !strings.Contains(prompt, "--from sess-worker-99") {
		t.Error("worker prompt should contain --from <session-id> in msg send examples")
	}
}

func TestBuildPMPrompt_SessionIDInFromFlag(t *testing.T) {
	t.Parallel()
	prompt := session.BuildPMPrompt("sess-pm-42", "workers")

	if !strings.Contains(prompt, "--from sess-pm-42") {
		t.Error("PM prompt should contain --from <session-id> in msg send examples")
	}
}
