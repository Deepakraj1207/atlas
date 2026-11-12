package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// scanProgressView renders the live per-source state of a running scan.
func scanProgressView(ctx Context, p ScanProgress) string {
	return progressView(ctx, p, "Scanning your system", "Stopping the scan…")
}

// analyzeProgressView renders the live state of a running analysis.
func analyzeProgressView(ctx Context, p ScanProgress) string {
	return progressView(ctx, p, "Analyzing your software", "Stopping the analysis…")
}

// windowLines returns the slice of lines to show so the selected block
// [selStart, selEnd) is visible within avail lines.
func windowLines(lines []string, selStart, selEnd, avail int) []string {
	avail = max(avail, 1)
	offset := 0
	if selEnd > avail {
		offset = min(selStart, selEnd-avail)
	}
	end := min(offset+avail, len(lines))
	return lines[offset:end]
}

// supportLabel renders a runtime's support status in plain words.
func supportLabel(t theme.Theme, s core.Support) string {
	switch s.State {
	case "ended":
		if s.DistroManaged {
			return t.Warning.Render("upstream support ended " + endText(s.EndDate))
		}
		return t.Danger.Render("support ended " + endText(s.EndDate))
	case "ending":
		return t.Warning.Render("support ends " + endText(s.EndDate))
	case "supported":
		if s.EndDate != "" {
			return t.Success.Render("supported until " + endText(s.EndDate))
		}
		return t.Success.Render("supported")
	default:
		return t.Muted.Render("support not judged")
	}
}

func endText(d string) string {
	if t, err := time.Parse("2006-01-02", d); err == nil {
		return t.Format("2 Jan 2006")
	}
	if t, err := time.Parse("2006-01", d); err == nil {
		return t.Format("Jan 2006")
	}
	return d
}

// checkProgressView renders the live state of a running update check.
func checkProgressView(ctx Context, p ScanProgress) string {
	return progressView(ctx, p, "Checking for updates", "Stopping the update check…")
}

func progressView(ctx Context, p ScanProgress, running, stopping string) string {
	t := ctx.Theme
	var b strings.Builder
	title := running + " " + p.Spinner
	if p.Cancelling {
		title = stopping
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
