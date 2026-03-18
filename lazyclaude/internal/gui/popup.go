package gui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/KEMSHlM/lazyclaude/internal/gui/presentation"
	"github.com/KEMSHlM/lazyclaude/internal/notify"
	"github.com/jesseduffield/gocui"
)

const popupViewName = "tool-popup"
const popupActionsViewName = "tool-popup-actions"

// hasPopup returns true if a tool popup is currently showing.
func (a *App) hasPopup() bool {
	return a.pendingTool != nil
}

// showToolPopup activates the tool popup overlay.
func (a *App) showToolPopup(n *notify.ToolNotification) {
	a.pendingTool = n
	a.popupScrollY = 0
}

// dismissPopup closes the popup and sends the choice.
func (a *App) dismissPopup(choice Choice) {
	if a.pendingTool == nil {
		return
	}
	window := a.pendingTool.Window
	a.pendingTool = nil
	a.popupDiffCache = nil
	a.popupDiffKinds = nil
	a.popupScrollY = 0

	if a.sessions != nil {
		go func() {
			if err := a.sessions.SendChoice(window, choice); err != nil {
				a.gui.Update(func(g *gocui.Gui) error {
					a.setStatus(g, fmt.Sprintf("send choice: %v", err))
					return nil
				})
			}
		}()
	}
}


// layoutToolPopup renders the tool/diff popup overlay centered on screen.
func (a *App) layoutToolPopup(g *gocui.Gui, maxX, maxY int) error {
	if a.pendingTool == nil {
		g.DeleteView(popupViewName)
		g.DeleteView(popupActionsViewName)
		return nil
	}

	// Popup dimensions: 70% width, 60% height, centered
	popW := maxX * 7 / 10
	popH := maxY * 6 / 10
	if popW < 40 {
		popW = maxX - 4
	}
	if popH < 10 {
		popH = maxY - 4
	}
	x0 := (maxX - popW) / 2
	y0 := (maxY - popH) / 2
	x1 := x0 + popW
	y1 := y0 + popH - 2 // leave room for actions bar

	v, err := g.SetView(popupViewName, x0, y0, x1, y1, 0)
	if err != nil && !isUnknownView(err) {
		return err
	}
	v.Clear()

	if a.pendingTool.IsDiff() {
		a.renderDiffPopup(v)
	} else {
		a.renderToolPopup(v)
	}

	// Actions bar below popup
	v2, err := g.SetView(popupActionsViewName, x0, y1+1, x1, y1+3, 0)
	if err != nil && !isUnknownView(err) {
		return err
	}
	v2.Frame = false
	v2.Clear()
	if a.pendingTool.IsDiff() {
		fmt.Fprint(v2, " y: yes  a: allow always  n: no  j/k: scroll  Esc: cancel")
	} else {
		fmt.Fprint(v2, " y: yes  a: allow always  n: no  Esc: cancel")
	}

	if _, err := g.SetCurrentView(popupViewName); err != nil && !isUnknownView(err) {
		return err
	}

	return nil
}

func (a *App) renderToolPopup(v *gocui.View) {
	n := a.pendingTool
	v.Title = fmt.Sprintf(" %s ", n.ToolName)
	td := presentation.ParseToolInput(n.ToolName, n.Input, n.CWD)
	for _, line := range presentation.FormatToolLines(td) {
		fmt.Fprintln(v, line)
	}
}

func (a *App) renderDiffPopup(v *gocui.View) {
	n := a.pendingTool
	v.Title = fmt.Sprintf(" Diff: %s ", filepath.Base(n.OldFilePath))

	diffLines, diffKinds := a.getDiffLines()
	_, viewH := v.Size()
	visibleLines := viewH - 1

	start := a.popupScrollY
	end := start + visibleLines
	if end > len(diffLines) {
		end = len(diffLines)
	}
	if start < 0 {
		start = 0
	}

	for i := start; i < end; i++ {
		line := diffLines[i]
		kind := diffKinds[i]
		switch kind {
		case presentation.DiffAdd:
			fmt.Fprintf(v, "\x1b[32m%s\x1b[0m\n", line)
		case presentation.DiffDel:
			fmt.Fprintf(v, "\x1b[31m%s\x1b[0m\n", line)
		case presentation.DiffHunk:
			fmt.Fprintf(v, "\x1b[36m%s\x1b[0m\n", line)
		case presentation.DiffHeader:
			fmt.Fprintf(v, "\x1b[1m%s\x1b[0m\n", line)
		default:
			fmt.Fprintln(v, line)
		}
	}
}

// getDiffLines generates and caches diff output for the current popup.
func (a *App) getDiffLines() ([]string, []presentation.DiffLineKind) {
	if a.popupDiffCache != nil {
		return a.popupDiffCache, a.popupDiffKinds
	}

	n := a.pendingTool
	diffOutput := generateDiffFromContents(n.OldFilePath, n.NewContents)
	parsed := presentation.ParseUnifiedDiff(diffOutput)

	lines := make([]string, len(parsed))
	kinds := make([]presentation.DiffLineKind, len(parsed))
	for i, dl := range parsed {
		lines[i] = presentation.FormatDiffLine(dl, 4)
		kinds[i] = dl.Kind
	}

	a.popupDiffCache = lines
	a.popupDiffKinds = kinds
	return lines, kinds
}

// generateDiffFromContents creates a unified diff between the old file and new contents.
func generateDiffFromContents(oldFilePath, newContents string) string {
	// Write new contents to temp file
	tmpDir := os.TempDir()
	newFile, err := os.CreateTemp(tmpDir, "lazyclaude-diff-new-*")
	if err != nil {
		return fmt.Sprintf("(error creating temp file: %v)", err)
	}
	defer os.Remove(newFile.Name())
	if _, err := newFile.WriteString(newContents); err != nil {
		newFile.Close()
		return fmt.Sprintf("(error writing temp file: %v)", err)
	}
	if err := newFile.Close(); err != nil {
		return fmt.Sprintf("(error closing temp file: %v)", err)
	}

	// If old file doesn't exist, show all as additions
	if _, err := os.Stat(oldFilePath); os.IsNotExist(err) {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("--- /dev/null\n+++ %s\n@@ -0,0 +1 @@\n", filepath.Base(oldFilePath)))
		for _, line := range strings.Split(newContents, "\n") {
			if line != "" {
				sb.WriteString("+" + line + "\n")
			}
		}
		return sb.String()
	}

	cmd := exec.Command("git", "diff", "--no-index", "--unified=3", "--", oldFilePath, newFile.Name())
	out, err := cmd.Output()
	if err != nil && len(out) > 0 {
		return string(out) // git diff returns exit 1 when files differ
	}
	if err != nil {
		return fmt.Sprintf("(no differences or error: %v)", err)
	}
	return string(out)
}

// Popup keybindings are set in setupGlobalKeybindings on both the global view ("")
// and the popup view (popupViewName) to ensure keys reach the popup when focused.
