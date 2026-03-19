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

// hasPopup returns true if any visible (non-suspended) popup exists.
func (a *App) hasPopup() bool {
	return a.visiblePopupCount() > 0
}

// showToolPopup pushes a notification onto the popup stack.
func (a *App) showToolPopup(n *notify.ToolNotification) {
	a.pushPopup(n)
}

// dismissPopup sends the choice to all popups and clears the stack.
func (a *App) dismissPopup(choice Choice) {
	if len(a.popupStack) == 0 {
		return
	}

	// Collect all windows to respond to
	entries := make([]popupEntry, len(a.popupStack))
	copy(entries, a.popupStack)
	a.popupStack = nil
	a.popupFocusIdx = 0

	if a.sessions != nil {
		go func() {
			for _, e := range entries {
				if err := a.sessions.SendChoice(e.notification.Window, choice); err != nil {
					a.gui.Update(func(g *gocui.Gui) error {
						a.setStatus(g, fmt.Sprintf("send choice: %v", err))
						return nil
					})
				}
			}
		}()
	}
}

// layoutToolPopup renders the active popup overlay centered on screen.
func (a *App) layoutToolPopup(g *gocui.Gui, maxX, maxY int) error {
	entry := a.activeEntry()
	if entry == nil {
		g.DeleteView(popupViewName)
		g.DeleteView(popupActionsViewName)
		return nil
	}
	n := entry.notification

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
	y1 := y0 + popH - 2

	v, err := g.SetView(popupViewName, x0, y0, x1, y1, 0)
	if err != nil && !isUnknownView(err) {
		return err
	}
	v.Clear()

	if n.IsDiff() {
		a.renderDiffPopup(v, entry)
	} else {
		a.renderToolPopup(v, n)
	}

	// Actions bar below popup
	v2, err := g.SetView(popupActionsViewName, x0, y1+1, x1, y1+3, 0)
	if err != nil && !isUnknownView(err) {
		return err
	}
	v2.Frame = false
	v2.Clear()

	// Show stack indicator if multiple popups
	stackInfo := ""
	visible := a.visiblePopupCount()
	if visible > 1 {
		stackInfo = fmt.Sprintf(" [%d/%d]", a.popupFocusIdx+1, visible)
	}

	if n.IsDiff() {
		fmt.Fprintf(v2, " y: yes  a: allow  n: no  j/k: scroll  Esc: suspend%s", stackInfo)
	} else {
		fmt.Fprintf(v2, " y: yes  a: allow  n: no  Esc: suspend%s", stackInfo)
	}

	if _, err := g.SetCurrentView(popupViewName); err != nil && !isUnknownView(err) {
		return err
	}

	return nil
}

func (a *App) renderToolPopup(v *gocui.View, n *notify.ToolNotification) {
	v.Title = fmt.Sprintf(" %s ", n.ToolName)
	td := presentation.ParseToolInput(n.ToolName, n.Input, n.CWD)
	for _, line := range presentation.FormatToolLines(td) {
		fmt.Fprintln(v, line)
	}
}

func (a *App) renderDiffPopup(v *gocui.View, entry *popupEntry) {
	n := entry.notification
	v.Title = fmt.Sprintf(" Diff: %s ", filepath.Base(n.OldFilePath))

	diffLines, diffKinds := getDiffLinesForEntry(entry)
	_, viewH := v.Size()
	visibleLines := viewH - 1

	start := entry.scrollY
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

// getDiffLinesForEntry generates and caches diff output for a popup entry.
func getDiffLinesForEntry(entry *popupEntry) ([]string, []presentation.DiffLineKind) {
	if entry.diffCache != nil {
		return entry.diffCache, entry.diffKinds
	}

	n := entry.notification
	diffOutput := generateDiffFromContents(n.OldFilePath, n.NewContents)
	parsed := presentation.ParseUnifiedDiff(diffOutput)

	lines := make([]string, len(parsed))
	kinds := make([]presentation.DiffLineKind, len(parsed))
	for i, dl := range parsed {
		lines[i] = presentation.FormatDiffLine(dl, 4)
		kinds[i] = dl.Kind
	}

	entry.diffCache = lines
	entry.diffKinds = kinds
	return lines, kinds
}

// generateDiffFromContents creates a unified diff between the old file and new contents.
func generateDiffFromContents(oldFilePath, newContents string) string {
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
		return string(out)
	}
	if err != nil {
		return fmt.Sprintf("(no differences or error: %v)", err)
	}
	return string(out)
}
