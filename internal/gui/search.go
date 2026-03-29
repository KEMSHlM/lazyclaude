package gui

import (
	"fmt"
	"strings"

	"github.com/KEMSHlM/lazyclaude/internal/gui/presentation"
	"github.com/jesseduffield/gocui"
)

const searchInputView = "search-input"

// layoutSearchInput creates or updates the inline search input at the bottom
// of the active panel. Called from layoutMain when DialogSearch is active.
func (a *App) layoutSearchInput(g *gocui.Gui, panelRect Rect) error {
	// Search input sits at the bottom of the panel, 1 row high.
	x0 := panelRect.X0 + 1
	x1 := panelRect.X1 - 1
	y0 := panelRect.Y1 - 2
	y1 := panelRect.Y1
	if y0 <= panelRect.Y0+1 {
		// Panel too small for search input.
		return nil
	}

	v, err := g.SetView(searchInputView, x0, y0, x1, y1, 0)
	if err != nil && !isUnknownView(err) {
		return err
	}
	v.Frame = false
	v.Editable = true
	v.Editor = &searchInputEditor{app: a}

	// Render the search bar content: "/" prefix + query.
	v.Clear()
	query := a.dialog.SearchQuery
	fmt.Fprintf(v, "%s/%s %s%s",
		presentation.FgCyan, presentation.Reset,
		query,
		presentation.Dim+"_"+presentation.Reset)

	g.SetViewOnTop(searchInputView)
	return nil
}

// closeSearch removes the search input view and restores state.
// If cancel is true, restores the pre-search cursor position.
func (a *App) closeSearch(g *gocui.Gui, cancel bool) {
	if cancel {
		// Restore cursor positions from before search.
		switch a.dialog.SearchPanel {
		case "sessions":
			a.cursor = a.dialog.SearchPreCursor
		case "plugins":
			a.pluginState.SetCursor(a.dialog.SearchPreCursor)
		case "logs":
			a.logs.cursorY = a.dialog.SearchPreCursor
		}
	}

	a.dialog.Kind = DialogNone
	a.dialog.SearchQuery = ""
	a.dialog.SearchPanel = ""
	a.dialog.SearchPreCursor = 0
	g.DeleteView(searchInputView)
	g.Cursor = false

	// Restore focus to the panel.
	panelName := a.panelManager.ActivePanel().Name()
	if _, err := g.SetCurrentView(panelName); err != nil && !isUnknownView(err) {
		_ = err
	}
}

// searchInputEditor handles text input in the search filter field.
// On each keystroke it re-filters the active panel's content.
type searchInputEditor struct {
	app *App
}

func (e *searchInputEditor) Edit(v *gocui.View, key gocui.Key, ch rune, mod gocui.Modifier) bool {
	switch {
	case key == gocui.KeyBackspace || key == gocui.KeyBackspace2:
		if e.app.dialog.SearchQuery == "" {
			return true
		}
		q := e.app.dialog.SearchQuery
		// Remove last rune.
		runes := []rune(q)
		e.app.dialog.SearchQuery = string(runes[:len(runes)-1])
	case key == gocui.KeySpace:
		e.app.dialog.SearchQuery += " "
	case ch != 0 && mod == gocui.ModNone:
		e.app.dialog.SearchQuery += string(ch)
	default:
		return false
	}

	e.app.applySearchFilter()
	return true
}

// applySearchFilter applies the current search query to the active panel.
func (a *App) applySearchFilter() {
	switch a.dialog.SearchPanel {
	case "sessions":
		// Filtering is applied during renderTree in layoutMain.
		// Reset cursor to 0 when query changes.
		filtered := a.filteredTreeNodes()
		if len(filtered) > 0 {
			a.cursor = 0
		}
	case "plugins":
		a.pluginState.SetCursor(0)
	case "logs":
		a.logs.cursorY = 0
	}
}

// filteredTreeNodes returns tree nodes filtered by the search query.
// If no search is active, returns all nodes.
func (a *App) filteredTreeNodes() []TreeNode {
	nodes := a.cachedNodes
	if a.dialog.Kind != DialogSearch || a.dialog.SearchPanel != "sessions" || a.dialog.SearchQuery == "" {
		return nodes
	}
	return filterTreeNodes(nodes, a.dialog.SearchQuery)
}

// filterTreeNodes filters tree nodes by case-insensitive substring match
// on project name or session name. Project nodes are included if any of
// their children match, or if the project name itself matches.
func filterTreeNodes(nodes []TreeNode, query string) []TreeNode {
	q := strings.ToLower(query)
	var result []TreeNode
	for _, node := range nodes {
		switch node.Kind {
		case ProjectNode:
			if node.Project != nil && strings.Contains(strings.ToLower(node.Project.Name), q) {
				result = append(result, node)
			}
		case SessionNode:
			if node.Session != nil && strings.Contains(strings.ToLower(node.Session.Name), q) {
				result = append(result, node)
			}
		}
	}
	return result
}

// filterLogLines filters log lines by case-insensitive substring match.
func filterLogLines(lines []string, query string) []string {
	if query == "" {
		return lines
	}
	q := strings.ToLower(query)
	var result []string
	for _, line := range lines {
		if strings.Contains(strings.ToLower(line), q) {
			result = append(result, line)
		}
	}
	return result
}

// filteredLogLines returns log lines filtered by search query.
func (a *App) filteredLogLines() []string {
	lines := a.readLogLines()
	if a.dialog.Kind != DialogSearch || a.dialog.SearchPanel != "logs" || a.dialog.SearchQuery == "" {
		return lines
	}
	return filterLogLines(lines, a.dialog.SearchQuery)
}

// filteredInstalledPlugins returns installed plugins filtered by search query.
func (a *App) filteredInstalledPlugins() []PluginItem {
	if a.plugins == nil {
		return nil
	}
	installed := a.plugins.Installed()
	if a.dialog.Kind != DialogSearch || a.dialog.SearchPanel != "plugins" || a.dialog.SearchQuery == "" {
		return installed
	}
	return filterPluginItems(installed, a.dialog.SearchQuery)
}

// filterPluginItems filters installed plugins by case-insensitive substring match on ID.
func filterPluginItems(items []PluginItem, query string) []PluginItem {
	q := strings.ToLower(query)
	var result []PluginItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.ID), q) {
			result = append(result, item)
		}
	}
	return result
}

// filteredAvailablePlugins returns marketplace plugins filtered by search query.
func (a *App) filteredAvailablePlugins() []AvailablePluginItem {
	if a.plugins == nil {
		return nil
	}
	available := a.plugins.Available()
	if a.dialog.Kind != DialogSearch || a.dialog.SearchPanel != "plugins" || a.dialog.SearchQuery == "" {
		return available
	}
	return filterAvailablePlugins(available, a.dialog.SearchQuery)
}

// filterAvailablePlugins filters marketplace plugins by case-insensitive
// substring match on Name or Description.
func filterAvailablePlugins(items []AvailablePluginItem, query string) []AvailablePluginItem {
	q := strings.ToLower(query)
	var result []AvailablePluginItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Name), q) ||
			strings.Contains(strings.ToLower(item.Description), q) {
			result = append(result, item)
		}
	}
	return result
}

// filteredMCPServers returns MCP servers filtered by search query.
func (a *App) filteredMCPServers() []MCPItem {
	if a.mcpServers == nil {
		return nil
	}
	servers := a.mcpServers.Servers()
	if a.dialog.Kind != DialogSearch || a.dialog.SearchPanel != "plugins" || a.dialog.SearchQuery == "" {
		return servers
	}
	return filterMCPItems(servers, a.dialog.SearchQuery)
}

// filterMCPItems filters MCP items by case-insensitive substring match on Name.
func filterMCPItems(items []MCPItem, query string) []MCPItem {
	q := strings.ToLower(query)
	var result []MCPItem
	for _, item := range items {
		if strings.Contains(strings.ToLower(item.Name), q) {
			result = append(result, item)
		}
	}
	return result
}
