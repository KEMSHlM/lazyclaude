package gui_test

import (
	"testing"

	"github.com/any-context/lazyclaude/internal/gui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildTreeNodes_Empty(t *testing.T) {
	t.Parallel()
	nodes := gui.BuildTreeNodes(nil, nil)
	assert.Empty(t, nodes)
}

func TestBuildTreeNodes_SingleProjectExpanded(t *testing.T) {
	t.Parallel()
	projects := []gui.ProjectItem{
		{
			ID:       "proj-1",
			Name:     "lazyclaude",
			Expanded: true,
			Sessions: []gui.SessionItem{
				{ID: "pm-1", Name: "pm", Role: "pm", Status: "Running"},
				{ID: "s1", Name: "feat-auth", Status: "Running", ParentID: "pm-1"},
				{ID: "s2", Name: "fix-bug", Status: "Detached", ParentID: "pm-1"},
			},
		},
	}

	nodes := gui.BuildTreeNodes(projects, nil)
	require.Len(t, nodes, 4) // project + PM + 2 sessions

	assert.Equal(t, gui.ProjectNode, nodes[0].Kind)
	assert.Equal(t, "proj-1", nodes[0].ProjectID)
	assert.Equal(t, "lazyclaude", nodes[0].Project.Name)

	assert.Equal(t, gui.SessionNode, nodes[1].Kind)
	assert.Equal(t, "pm-1", nodes[1].Session.ID)
	assert.Equal(t, 1, nodes[1].Depth)

	assert.Equal(t, gui.SessionNode, nodes[2].Kind)
	assert.Equal(t, "feat-auth", nodes[2].Session.Name)
	assert.Equal(t, 2, nodes[2].Depth)

	assert.Equal(t, gui.SessionNode, nodes[3].Kind)
	assert.Equal(t, "fix-bug", nodes[3].Session.Name)
	assert.Equal(t, 2, nodes[3].Depth)
}

func TestBuildTreeNodes_CollapsedProject(t *testing.T) {
	t.Parallel()
	projects := []gui.ProjectItem{
		{
			ID:       "proj-1",
			Name:     "lazyclaude",
			Expanded: false,
			Sessions: []gui.SessionItem{
				{ID: "s1", Name: "feat-auth"},
			},
		},
	}

	nodes := gui.BuildTreeNodes(projects, nil)
	require.Len(t, nodes, 1, "collapsed project shows only project row")
	assert.Equal(t, gui.ProjectNode, nodes[0].Kind)
}

func TestBuildTreeNodes_MultipleProjects(t *testing.T) {
	t.Parallel()
	projects := []gui.ProjectItem{
		{
			ID:       "proj-1",
			Name:     "lazyclaude",
			Expanded: true,
			Sessions: []gui.SessionItem{
				{ID: "s1", Name: "main"},
			},
		},
		{
			ID:       "proj-2",
			Name:     "my-api",
			Expanded: false,
			Sessions: []gui.SessionItem{
				{ID: "s2", Name: "app"},
			},
		},
	}

	nodes := gui.BuildTreeNodes(projects, nil)
	require.Len(t, nodes, 3) // proj-1 + session + proj-2 (collapsed)

	assert.Equal(t, gui.ProjectNode, nodes[0].Kind)
	assert.Equal(t, "lazyclaude", nodes[0].Project.Name)

	assert.Equal(t, gui.SessionNode, nodes[1].Kind)
	assert.Equal(t, "main", nodes[1].Session.Name)

	assert.Equal(t, gui.ProjectNode, nodes[2].Kind)
	assert.Equal(t, "my-api", nodes[2].Project.Name)
}

func TestBuildTreeNodes_NoPM(t *testing.T) {
	t.Parallel()
	projects := []gui.ProjectItem{
		{
			ID:       "proj-1",
			Name:     "app",
			Expanded: true,
			Sessions: []gui.SessionItem{
				{ID: "s1", Name: "main"},
			},
		},
	}

	nodes := gui.BuildTreeNodes(projects, nil)
	require.Len(t, nodes, 2) // project + session (no PM)
}

func TestBuildTreeNodes_RecursiveHierarchy(t *testing.T) {
	t.Parallel()
	projects := []gui.ProjectItem{
		{
			ID:       "proj-1",
			Name:     "lazyclaude",
			Expanded: true,
			Sessions: []gui.SessionItem{
				{ID: "root-pm", Name: "root-pm", Role: "pm"},
				{ID: "worker-a", Name: "worker-a", ParentID: "root-pm"},
				{ID: "subteam-x", Name: "subteam-x", Role: "pm", ParentID: "root-pm"},
				{ID: "worker-x1", Name: "worker-x1", ParentID: "subteam-x"},
				{ID: "orphan", Name: "orphan-worker"},
			},
		},
	}

	nodes := gui.BuildTreeNodes(projects, nil)
	// project + root-pm(d1) + worker-a(d2) + subteam-x(d2) + worker-x1(d3) + orphan(d1)
	require.Len(t, nodes, 6)

	assert.Equal(t, gui.ProjectNode, nodes[0].Kind)

	assert.Equal(t, "root-pm", nodes[1].Session.ID)
	assert.Equal(t, 1, nodes[1].Depth)

	assert.Equal(t, "worker-a", nodes[2].Session.ID)
	assert.Equal(t, 2, nodes[2].Depth)

	assert.Equal(t, "subteam-x", nodes[3].Session.ID)
	assert.Equal(t, 2, nodes[3].Depth)

	assert.Equal(t, "worker-x1", nodes[4].Session.ID)
	assert.Equal(t, 3, nodes[4].Depth)

	assert.Equal(t, "orphan", nodes[5].Session.ID)
	assert.Equal(t, 1, nodes[5].Depth)
}

func TestBuildTreeNodes_PMCollapsed(t *testing.T) {
	t.Parallel()
	projects := []gui.ProjectItem{
		{
			ID:       "proj-1",
			Name:     "lazyclaude",
			Expanded: true,
			Sessions: []gui.SessionItem{
				{ID: "pm-1", Name: "pm", Role: "pm"},
				{ID: "w1", Name: "worker", ParentID: "pm-1"},
			},
		},
	}

	collapsed := map[string]bool{"pm-1": true}
	nodes := gui.BuildTreeNodes(projects, collapsed)
	// project + pm (collapsed, no children)
	require.Len(t, nodes, 2)
	assert.Equal(t, "pm-1", nodes[1].Session.ID)
	assert.False(t, nodes[1].Session.Expanded, "collapsed PM should have Expanded=false")
}

func TestBuildTreeNodes_PMExpanded(t *testing.T) {
	t.Parallel()
	projects := []gui.ProjectItem{
		{
			ID:       "proj-1",
			Name:     "lazyclaude",
			Expanded: true,
			Sessions: []gui.SessionItem{
				{ID: "pm-1", Name: "pm", Role: "pm"},
				{ID: "w1", Name: "worker", ParentID: "pm-1"},
			},
		},
	}

	nodes := gui.BuildTreeNodes(projects, nil)
	require.Len(t, nodes, 3) // project + pm + worker
	assert.True(t, nodes[1].Session.Expanded, "expanded PM should have Expanded=true")
	assert.Equal(t, "worker", nodes[2].Session.Name)
}
