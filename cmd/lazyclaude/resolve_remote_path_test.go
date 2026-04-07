package main

import (
	"testing"

	"github.com/any-context/lazyclaude/internal/daemon"
	"github.com/stretchr/testify/assert"
)

func TestResolveRemotePath_NonLocalPath_FallbackDot(t *testing.T) {
	t.Parallel()
	a := &guiCompositeAdapter{
		cp:               daemon.NewCompositeProvider(nil, nil),
		localProjectRoot: "/local/project",
	}
	// When host is set and queryRemoteCWD returns empty, falls back to "."
	// because local paths are meaningless on the remote machine.
	assert.Equal(t, ".", a.resolveRemotePath("/home/user/other-project", "remote"))
}

func TestResolveRemotePath_DotPath_NoProvider_Passthrough(t *testing.T) {
	t.Parallel()
	a := &guiCompositeAdapter{
		cp:               daemon.NewCompositeProvider(nil, nil),
		localProjectRoot: "/local/project",
	}
	// "." path with no remote provider falls back to the original path.
	assert.Equal(t, ".", a.resolveRemotePath(".", "remote"))
}

func TestResolveRemotePath_LocalProjectRoot_NoProvider_FallbackDot(t *testing.T) {
	t.Parallel()
	a := &guiCompositeAdapter{
		cp:               daemon.NewCompositeProvider(nil, nil),
		localProjectRoot: "/local/project",
	}
	// localProjectRoot with no remote provider and host set falls back to ".".
	assert.Equal(t, ".", a.resolveRemotePath("/local/project", "remote"))
}
