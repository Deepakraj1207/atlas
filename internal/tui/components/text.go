// Package components holds reusable view fragments shared by every screen.
// Each function renders a string for a given width; none keeps state.
package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// Wrap word-wraps text to width using Lip Gloss.
func Wrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	return lipgloss.NewStyle().Width(width).Render(text)
}

// Bullets renders a list with a leading marker, wrapping each item.
func Bullets(t theme.Theme, width int, items []string) string {
	if len(items) == 0 {
		return ""
	}
	marker := t.Accent.Render("• ")
	inner := max(width-2, 1)
	lines := make([]string, 0, len(items))
	for _, it := range items {
		body := Wrap(it, inner)
		// Indent continuation lines so the text column lines up.
		body = strings.ReplaceAll(body, "\n", "\n  ")
		lines = append(lines, marker+t.Body.Render(body))
	}
	return strings.Join(lines, "\n")
}

// LabelValue renders aligned "Label   value" rows.
func LabelValue(t theme.Theme, rows [][2]string) string {
	w := 0
	for _, r := range rows {
		w = max(w, lipgloss.Width(r[0]))
	}
	lines := make([]string, 0, len(rows))
	for _, r := range rows {
		lines = append(lines, t.Muted.Width(w+2).Render(r[0])+t.Body.Render(r[1]))
	}
	return strings.Join(lines, "\n")
}

// Divider draws a horizontal rule.
func Divider(t theme.Theme, width int) string {
	if width <= 0 {
		return ""
	}
	return t.Divider.Render(strings.Repeat("─", width))
}

// Fit constrains a block to exactly width x height, clipping overflow and
// padding with blank lines. Used to keep regions from pushing each other.
func Fit(s string, width, height int) string {
	return lipgloss.NewStyle().Width(width).Height(height).MaxWidth(width).MaxHeight(height).Render(s)
}

// Truncate shortens s to width cells, adding an ellipsis when cut.
func Truncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}

// PadRight truncates or pads s to exactly width cells.
func PadRight(s string, width int) string {
	s = Truncate(s, width)
	if gap := width - lipgloss.Width(s); gap > 0 {
		s += strings.Repeat(" ", gap)
	}
	return s
}
