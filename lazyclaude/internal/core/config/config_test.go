package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/KEMSHlM/lazyclaude/internal/core/config"
	"github.com/stretchr/testify/assert"
)

func TestDefaultPaths_UsesHomeDir(t *testing.T) {
	t.Parallel()
	p := config.DefaultPaths()

	home, _ := os.UserHomeDir()
	assert.Contains(t, p.IDEDir, filepath.Join(home, ".claude", "ide"))
	assert.Contains(t, p.DataDir, "lazyclaude")
}

func TestTestPaths_FullyIsolated(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	p := config.TestPaths(tmp)

	// All paths must be under the tmp directory
	assert.True(t, isUnder(p.IDEDir, tmp), "IDEDir should be under tmp")
	assert.True(t, isUnder(p.DataDir, tmp), "DataDir should be under tmp")
	assert.True(t, isUnder(p.RuntimeDir, tmp), "RuntimeDir should be under tmp")
}

func TestTestPaths_NoOverlapWithDefault(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	prod := config.DefaultPaths()
	test := config.TestPaths(tmp)

	// None of the test paths should equal production paths
	assert.NotEqual(t, prod.IDEDir, test.IDEDir)
	assert.NotEqual(t, prod.DataDir, test.DataDir)
	assert.NotEqual(t, prod.RuntimeDir, test.RuntimeDir)
}

func TestPaths_StateFile(t *testing.T) {
	t.Parallel()
	p := config.TestPaths("/tmp/test")
	assert.Equal(t, "/tmp/test/data/state.json", p.StateFile())
}

func TestPaths_PortFile(t *testing.T) {
	t.Parallel()
	p := config.TestPaths("/tmp/test")
	assert.Equal(t, "/tmp/test/run/lazyclaude-mcp.port", p.PortFile())
}

func TestPaths_ChoiceFile(t *testing.T) {
	t.Parallel()
	p := config.TestPaths("/tmp/test")
	assert.Equal(t, "/tmp/test/run/lazyclaude-choice-lc-abc.txt", p.ChoiceFile("lc-abc"))
}

func TestPaths_LockFile(t *testing.T) {
	t.Parallel()
	p := config.TestPaths("/tmp/test")
	assert.Equal(t, "/tmp/test/ide/7860.lock", p.LockFile(7860))
}

func isUnder(path, parent string) bool {
	rel, err := filepath.Rel(parent, path)
	if err != nil {
		return false
	}
	return rel != ".." && rel[:2] != ".."
}
