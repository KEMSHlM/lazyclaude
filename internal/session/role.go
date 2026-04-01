package session

import (
	_ "embed"
	"fmt"
)

// Role identifies the operational role of a session.
type Role string

const (
	// RoleNone is the zero value; represents a regular session with no PM/Worker role.
	// Used for backward compatibility with existing state.json files.
	RoleNone Role = ""

	// RolePM represents a Project Manager session that reviews Worker PRs.
	RolePM Role = "pm"

	// RoleWorker represents a Worker session that operates within a git worktree.
	RoleWorker Role = "worker"
)

// String returns a human-readable name. RoleNone returns "none".
func (r Role) String() string {
	if r == RoleNone {
		return "none"
	}
	return string(r)
}

// IsValid reports whether r is one of the defined Role constants.
func (r Role) IsValid() bool {
	return r == RoleNone || r == RolePM || r == RoleWorker
}

//go:embed prompts/pm.md
var pmSystemPrompt string

// BuildPMPrompt generates the system prompt injected into a PM session at launch.
// Uses lazyclaude CLI subcommands for server communication instead of raw curl.
// The template is loaded from prompts/pm.md.
func BuildPMPrompt(sessionID string, workerList string) string {
	return fmt.Sprintf(pmSystemPrompt,
		sessionID, // Session ID line
		sessionID, // msg send --from
		sessionID, // msg send --from (spawn)
		workerList,
	)
}

//go:embed prompts/worker.md
var workerRolePrompt string

// BuildWorkerPrompt generates the system prompt injected into a Worker session at launch.
// Uses lazyclaude CLI subcommands for server communication instead of raw curl.
// The template is loaded from prompts/worker.md.
func BuildWorkerPrompt(worktreePath, projectRoot, sessionID string) string {
	return fmt.Sprintf(workerRolePrompt,
		projectRoot,  // NEVER modify ... must remain untouched
		worktreePath, // Worktree path line
		sessionID,    // Session ID line
		sessionID,    // msg send --from (review_request)
		sessionID,    // msg send --from (spawn)
	)
}
