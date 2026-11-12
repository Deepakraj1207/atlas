package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// scanProgressView renders the live per-source state of a running scan.
func scanProgressView(ctx Context, p ScanProgress) string {
	t := ctx.Theme
	var b strings.Builder
	title := "Scanning your system " + p.Spinner
	if p.Cancelling {
		title = "Stopping the scan…"
	}
	b.WriteString(t.Heading.Render(title))
	b.WriteString("\n")
	b.WriteString(t.Muted.Render(components.Wrap("Only reading. Nothing on your computer is being changed. Press x to stop.", ctx.Width())))
	b.WriteString("\n")

	labelW := 0
	for _, s := range p.Sources {
		labelW = max(labelW, lipgloss.Width(s.Info.Label))
	}
	for _, s := range p.Sources {
		var mark, state string
		switch {
		case s.Result == nil && s.Running:
			mark, state = t.Accent.Render(p.Spinner), t.Muted.Render("reading…")
		case s.Result == nil:
			mark, state = t.Subtle.Render("·"), t.Subtle.Render("waiting")
		default:
			mark, state = sourceResultMark(t, *s.Result, ctx.Advanced())
		}
		b.WriteString("  " + mark + " " + t.Body.Width(labelW+2).Render(s.Info.Label) + state + "\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

// sourceResultMark returns a short coloured marker and description.
func sourceResultMark(t theme.Theme, r core.SourceResult, advanced bool) (string, string) {
	switch r.Status {
	case core.SourceOK:
		return t.Success.Render("•"), t.Body.Render(fmt.Sprintf("%d found", r.Count))
	case core.SourceUnavailable:
		return t.Subtle.Render("–"), t.Subtle.Render("not on this computer")
	case core.SourceCancelled:
		return t.Subtle.Render("–"), t.Subtle.Render("stopped")
	default:
		msg := r.Message
		if advanced && r.Details != "" {
			msg += ": " + r.Details
		}
		return t.Danger.Render("!"), t.Danger.Render(msg)
	}
}

// outcomeLabel is the coloured word shown for an activity outcome.
func outcomeLabel(t theme.Theme, o core.Outcome) string {
	switch o {
	case core.OutcomeSuccess:
		return t.Success.Render("Done")
	case core.OutcomePartial:
		return t.Warning.Render("Partly done")
	case core.OutcomeFailed:
		return t.Danger.Render("Failed")
	case core.OutcomeCancelled:
		return t.Muted.Render("Cancelled")
	default:
		return t.Accent.Render("Note")
	}
}

// outcomeMark is a one-cell coloured marker for compact lists.
func outcomeMark(t theme.Theme, o core.Outcome) string {
	switch o {
	case core.OutcomeSuccess:
		return t.Success.Render("•")
	case core.OutcomePartial:
		return t.Warning.Render("!")
	case core.OutcomeFailed:
		return t.Danger.Render("!")
	case core.OutcomeCancelled:
		return t.Subtle.Render("–")
	default:
		return t.Accent.Render("·")
	}
}

// kindLabel is the plain-language type name.
func kindLabel(k core.Kind) string {
	if k == core.KindComponent {
		return "System component"
	}
	return "App"
}

// scopeLabel is the plain-language install scope.
func scopeLabel(s string) string {
	switch s {
	case "machine":
		return "Everyone on this computer"
	case "user":
		return "Just you"
	default:
		return ""
	}
}
