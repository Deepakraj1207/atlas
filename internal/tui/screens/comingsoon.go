package screens

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/nav"
)

// ComingSoon is the honest empty state for a section that is not built yet.
// It never shows placeholder data.
type ComingSoon struct {
	section nav.Section
}

// NewComingSoon creates the screen for a section.
func NewComingSoon(s nav.Section) *ComingSoon {
	return &ComingSoon{section: s}
}

// Update ignores all input.
func (c *ComingSoon) Update(msg tea.Msg, ctx Context) (Screen, tea.Cmd) {
	return c, nil
}

// Hints has nothing screen-specific.
func (c *ComingSoon) Hints(ctx Context) []components.KeyHint { return nil }

// View renders the empty state.
func (c *ComingSoon) View(ctx Context) string {
	info := c.section.Info()
	t := ctx.Theme
	hint := fmt.Sprintf("Arrives in Phase %d. Nothing here reads or changes your system yet.", info.Phase)
	body := t.Title.Render(info.Title) + "\n\n" + components.EmptyState{
		Title:   "Not available in this version",
		Message: info.Summary,
		Plan:    info.Plan,
		Hint:    hint,
	}.Render(t, ctx.Width())
	return components.Fit(body, ctx.Width(), ctx.Height())
}

var _ Screen = (*ComingSoon)(nil)
