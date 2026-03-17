package gui

import (
	"fmt"

	"github.com/KEMSHlM/lazyclaude/internal/gui/context"
	"github.com/jesseduffield/gocui"
)

// AppMode determines which set of views to display.
type AppMode int

const (
	ModeMain AppMode = iota // lazyclaude      → session list + preview
	ModeDiff                // lazyclaude diff  → diff popup viewer
	ModeTool                // lazyclaude tool  → tool popup viewer
)

// App is the root TUI application (lazygit Gui equivalent).
type App struct {
	gui          *gocui.Gui
	mode         AppMode
	contextMgr   *context.Manager
	activeTabIdx int // active side panel tab (0=Sessions, 1=Server)
}

// NewApp creates a new App. Call Run() to start the event loop.
func NewApp(mode AppMode) (*App, error) {
	g, err := gocui.NewGui(gocui.NewGuiOpts{
		OutputMode:      gocui.OutputTrue,
		SupportOverlaps: false,
	})
	if err != nil {
		return nil, fmt.Errorf("init gocui: %w", err)
	}

	app := &App{
		gui:        g,
		mode:       mode,
		contextMgr: context.NewManager(),
	}

	g.SetManagerFunc(app.layout)
	g.Mouse = true

	if err := app.setupGlobalKeybindings(); err != nil {
		g.Close()
		return nil, err
	}

	return app, nil
}

// NewAppHeadless creates an App in headless mode for testing.
func NewAppHeadless(mode AppMode, width, height int) (*App, error) {
	g, err := gocui.NewGui(gocui.NewGuiOpts{
		OutputMode: gocui.OutputTrue,
		Headless:   true,
		Width:      width,
		Height:     height,
	})
	if err != nil {
		return nil, fmt.Errorf("init gocui headless: %w", err)
	}

	app := &App{
		gui:        g,
		mode:       mode,
		contextMgr: context.NewManager(),
	}

	g.SetManagerFunc(app.layout)

	if err := app.setupGlobalKeybindings(); err != nil {
		g.Close()
		return nil, err
	}

	return app, nil
}

// TestLayout exposes layout for testing. Not for production use.
func (a *App) TestLayout(g *gocui.Gui) error {
	return a.layout(g)
}

// Run starts the main event loop. Blocks until quit.
func (a *App) Run() error {
	defer a.gui.Close()
	if err := a.gui.MainLoop(); err != nil && err != gocui.ErrQuit {
		return err
	}
	return nil
}

// Mode returns the current app mode.
func (a *App) Mode() AppMode {
	return a.mode
}

// ContextMgr returns the context manager.
func (a *App) ContextMgr() *context.Manager {
	return a.contextMgr
}

// Gui returns the underlying gocui.Gui (for testing).
func (a *App) Gui() *gocui.Gui {
	return a.gui
}

func (a *App) layout(g *gocui.Gui) error {
	maxX, maxY := g.Size()
	switch a.mode {
	case ModeMain:
		return a.layoutMain(g, maxX, maxY)
	case ModeDiff, ModeTool:
		return a.layoutPopup(g, maxX, maxY)
	}
	return nil
}

// ActiveTabIdx returns the active side panel tab index.
func (a *App) ActiveTabIdx() int {
	return a.activeTabIdx
}

// SetActiveTab switches the side panel tab.
func (a *App) SetActiveTab(idx int) {
	tabs := SideTabs()
	if idx >= 0 && idx < len(tabs) {
		a.activeTabIdx = idx
	}
}

func (a *App) layoutMain(g *gocui.Gui, maxX, maxY int) error {
	splitX := maxX / 3
	if splitX < 20 {
		splitX = 20
	}
	if splitX >= maxX-10 {
		splitX = maxX / 2
	}

	tabs := SideTabs()
	tabTitle := " " + TabBar(tabs, a.activeTabIdx) + " "

	// Left side panel: split into upper (sessions) and lower (server)
	leftMidY := (maxY - 2) * 2 / 3

	// Sessions view (upper left) — always present but visibility depends on tab
	if v, err := g.SetView("sessions", 0, 0, splitX-1, leftMidY, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = tabTitle
		v.Highlight = true
		v.SelBgColor = gocui.ColorBlue
	} else {
		v.Title = tabTitle
	}

	// Server view (lower left)
	if v, err := g.SetView("server", 0, leftMidY+1, splitX-1, maxY-2, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Server "
		v.Wrap = true
	}

	// Main panel (right side — preview / details)
	if v, err := g.SetView("main", splitX, 0, maxX-1, maxY-2, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Title = " Main "
		v.Wrap = true
	}

	// Options bar (bottom, frameless)
	if v, err := g.SetView("options", 0, maxY-2, maxX-1, maxY, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
	}

	// Set focus to active tab's view
	activeView := tabs[a.activeTabIdx].Name
	if _, err := g.SetCurrentView(activeView); err != nil && err != gocui.ErrUnknownView {
		return err
	}
	return nil
}

func (a *App) layoutPopup(g *gocui.Gui, maxX, maxY int) error {
	// Content area (top)
	if v, err := g.SetView("content", 0, 0, maxX-1, maxY-3, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Wrap = false
	}

	// Actions bar (bottom)
	if v, err := g.SetView("actions", 0, maxY-2, maxX-1, maxY, 0); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
	}

	if _, err := g.SetCurrentView("content"); err != nil && err != gocui.ErrUnknownView {
		return err
	}
	return nil
}

func (a *App) setupGlobalKeybindings() error {
	// Ctrl+C to quit (always)
	if err := a.gui.SetKeybinding("", gocui.KeyCtrlC, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		return gocui.ErrQuit
	}); err != nil {
		return err
	}

	// q to quit in main mode
	if err := a.gui.SetKeybinding("", 'q', gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if a.mode == ModeMain {
			return gocui.ErrQuit
		}
		return nil
	}); err != nil {
		return err
	}

	// Esc: quit in popup mode, pop context in main mode
	if err := a.gui.SetKeybinding("", gocui.KeyEsc, gocui.ModNone, func(g *gocui.Gui, v *gocui.View) error {
		if a.mode == ModeDiff || a.mode == ModeTool {
			return gocui.ErrQuit
		}
		if a.contextMgr.Depth() > 1 {
			a.contextMgr.Pop()
		}
		return nil
	}); err != nil {
		return err
	}

	return nil
}
