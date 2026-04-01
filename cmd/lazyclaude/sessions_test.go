package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/any-context/lazyclaude/internal/server"
)

func TestPrintSessionsTable_Empty(t *testing.T) {
	var buf bytes.Buffer
	if err := printSessionsTable(&buf, nil, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No sessions found") {
		t.Errorf("expected 'No sessions found', got %q", buf.String())
	}
}

func TestPrintSessionsTable_Normal(t *testing.T) {
	sessions := []server.SessionInfo{
		{ID: "abc123", Name: "pm", Role: "pm", Status: "Running", Path: "/home/user/project", Window: "@1"},
		{ID: "def456", Name: "feat-a", Role: "worker", Status: "Running", Path: "/home/user/project/.claude/worktrees/feat-a", Window: "@2"},
	}

	var buf bytes.Buffer
	if err := printSessionsTable(&buf, sessions, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "ID") {
		t.Error("missing ID header")
	}
	if !strings.Contains(output, "NAME") {
		t.Error("missing NAME header")
	}
	// Should show absolute paths
	if !strings.Contains(output, "/home/user/project") {
		t.Error("expected absolute path in output")
	}
	if !strings.Contains(output, "/home/user/project/.claude/worktrees/feat-a") {
		t.Error("expected absolute worktree path in output")
	}
	// WINDOW should NOT appear in non-verbose mode
	if strings.Contains(output, "WINDOW") {
		t.Error("WINDOW should not appear in non-verbose mode")
	}
}

func TestPrintSessionsTable_Verbose(t *testing.T) {
	sessions := []server.SessionInfo{
		{ID: "abc12345", Name: "pm", Role: "pm", Status: "Running", Path: "/home/user/project", Window: "@1"},
	}

	var buf bytes.Buffer
	if err := printSessionsTable(&buf, sessions, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	output := buf.String()

	if !strings.Contains(output, "abc12345") {
		t.Error("expected ID in verbose output")
	}
	if !strings.Contains(output, "@1") {
		t.Error("expected WINDOW in verbose output")
	}
	if !strings.Contains(output, "/home/user/project") {
		t.Error("expected absolute path in verbose output")
	}
}
