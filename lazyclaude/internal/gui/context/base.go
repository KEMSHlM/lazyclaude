package context

// ContextKind classifies how a context is displayed.
type ContextKind int

const (
	KindSide  ContextKind = iota // side panel (e.g., session list)
	KindMain                     // main panel (e.g., preview)
	KindPopup                    // popup overlay (e.g., diff, tool, help)
)

// Context represents a UI context with its own keybindings and rendering logic.
// Keybindings are not part of this interface to avoid circular imports with the gui package.
// Instead, controllers register keybindings directly with gocui.
type Context interface {
	// Name returns the context identifier.
	Name() string

	// Kind returns how this context is displayed.
	Kind() ContextKind

	// OnFocus is called when this context becomes active.
	OnFocus()

	// OnBlur is called when this context is deactivated.
	OnBlur()
}

// BaseContext provides common context functionality.
type BaseContext struct {
	name string
	kind ContextKind
}

// NewBaseContext creates a BaseContext.
func NewBaseContext(name string, kind ContextKind) BaseContext {
	return BaseContext{name: name, kind: kind}
}

func (c *BaseContext) Name() string     { return c.name }
func (c *BaseContext) Kind() ContextKind { return c.kind }
func (c *BaseContext) OnFocus()          {}
func (c *BaseContext) OnBlur()           {}