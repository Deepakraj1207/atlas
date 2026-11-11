package components

import (
	"strings"

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

// Render draws the empty state within width.
func (e EmptyState) Render(t theme.Theme, width int) string {
	var b strings.Builder
	if e.Title != "" {
		b.WriteString(t.Heading.Render(e.Title))
		b.WriteString("\n")
	}
	if e.Message != "" {
		b.WriteString(t.Muted.Render(Wrap(e.Message, width)))
		b.WriteString("\n")
	}
	if len(e.Plan) > 0 {
		b.WriteString("\n")
		b.WriteString(t.Heading.Render("What this will do"))
		b.WriteString("\n")
		b.WriteString(Bullets(t, width, e.Plan))
		b.WriteString("\n")
	}
	if e.Hint != "" {
		b.WriteString("\n")
		b.WriteString(t.Accent.Render(Wrap(e.Hint, width)))
	}
	return strings.TrimRight(b.String(), "\n")
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

// Render draws the error state within width.
func (e ErrorState) Render(t theme.Theme, width int) string {
	var b strings.Builder
	title := e.Title
	if title == "" {
		title = "Something needs attention"
	}
	b.WriteString(t.Danger.Bold(true).Render("! " + title))
	b.WriteString("\n")
	if e.Summary != "" {
		b.WriteString(t.Body.Render(Wrap(e.Summary, width)))
		b.WriteString("\n")
	}
	if e.ShowDetails && e.Details != "" {
		b.WriteString(t.Subtle.Render("Details"))
		b.WriteString("\n")
		b.WriteString(t.Muted.Render(Wrap(e.Details, width)))
		b.WriteString("\n")
	} else if e.Details != "" {
		b.WriteString(t.Subtle.Render("Switch to Advanced mode (m) to see technical details."))
		b.WriteString("\n")
	}
	if e.Hint != "" {
		b.WriteString(t.Accent.Render(Wrap(e.Hint, width)))
	}
	return strings.TrimRight(b.String(), "\n")
}
