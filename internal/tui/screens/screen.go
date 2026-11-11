// Package screens contains one screen per navigation section. Screens are
// small Bubble Tea sub-models that receive a Context from the root model and
// talk back through the messages in msgs.go.
package screens

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/layout"
	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// Context is everything a screen needs from the root model to render.
type Context struct {
	Theme  theme.Theme
	Layout layout.Layout
	Config config.Config

	// Paths and diagnostics, shown in Advanced mode.
	ConfigPath   string
	ConfigResult config.Result
	LogPath      string
	// LogError is non-empty when the log file could not be opened.
	LogError string
	Version  string
	Platform string
}

// Advanced reports whether the UI is in Advanced mode.
func (c Context) Advanced() bool { return c.Config.UI.Mode == config.ModeAdvanced }

// Width is the content width available to the screen.
func (c Context) Width() int { return c.Layout.ContentWidth }

// Height is the content height available to the screen.
func (c Context) Height() int { return c.Layout.ContentHeight }

// Screen is the interface every section screen implements.
type Screen interface {
	// Update handles a message. It returns the (possibly updated) screen
	// and any command for the root model.
	Update(msg tea.Msg, ctx Context) (Screen, tea.Cmd)
	// View renders the screen within ctx.Width() x ctx.Height().
	View(ctx Context) string
	// Hints lists the screen-specific keys for the footer and help overlay.
	Hints(ctx Context) []components.KeyHint
}
