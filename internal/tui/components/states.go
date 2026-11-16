package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// EmptyState describes a screen or region with nothing to show yet.
type EmptyState struct {
	Title   string
	Message string
	// Plan lists what the area will do once available.
	Plan []string
	// Hint is a short closing line such as "Arrives in Phase 2".
	Hint string
}

// Render draws the empty state as a framed panel with the title in its
// border, at most a comfortable reading width.
func (e EmptyState) Render(t theme.Theme, width int) string {
	pw := panelWidth(width)
	inner := max(pw-4, 8)
	var parts []string
	if e.Message != "" {
		parts = append(parts, wrapLines(t.Muted, e.Message, inner))
	}
	if len(e.Plan) > 0 {
		parts = append(parts, Bullets(t, inner, e.Plan))
	}
	if e.Hint != "" {
		parts = append(parts, wrapLines(t.Accent, e.Hint, inner))
	}
	return Box(t, e.Title, strings.Join(parts, "\n\n"), pw, 0)
}

// wrapLines wraps text and styles each line on its own, so no line is
// padded to the width of the longest.
func wrapLines(style lipgloss.Style, text string, width int) string {
	lines := strings.Split(Wrap(text, width), "\n")
	for i, l := range lines {
		lines[i] = style.Render(l)
	}
	return strings.Join(lines, "\n")
}

// ErrorState describes a problem in plain language with optional detail.
type ErrorState struct {
	Title string
	// Summary says what went wrong and what ATLAS did about it.
	Summary string
	// Details is the raw error text; only shown when ShowDetails is true.
	Details     string
	ShowDetails bool
	// Hint tells the user what they can do next.
	Hint string
}

// Render draws the error state as a framed panel with the title, in the
// danger colour, in its border.
func (e ErrorState) Render(t theme.Theme, width int) string {
	pw := panelWidth(width)
	inner := max(pw-4, 8)
	title := e.Title
	if title == "" {
		title = "Something needs attention"
	}
	var parts []string
	if e.Summary != "" {
		parts = append(parts, wrapLines(t.Body, e.Summary, inner))
	}
	if e.ShowDetails && e.Details != "" {
		parts = append(parts, t.Subtle.Render("Details")+"\n"+wrapLines(t.Muted, e.Details, inner))
	} else if e.Details != "" {
		parts = append(parts, wrapLines(t.Subtle, "Switch to Advanced mode (m) to see technical details.", inner))
	}
	if e.Hint != "" {
		parts = append(parts, wrapLines(t.Accent, e.Hint, inner))
	}
	return frame(t, "! "+title, t.Danger.Bold(true), strings.Join(parts, "\n\n"), pw, 0)
}
