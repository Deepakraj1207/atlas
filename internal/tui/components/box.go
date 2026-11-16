package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// Box frames body in a rounded border with the title set into the top
// edge:
//
//	╭─ Title ──────────╮
//	│ body             │
//	╰──────────────────╯
//
// width is the whole box, borders included. Lines wider than the inside
// are cut. height, when larger than the content, pads the box with empty
// rows so boxes side by side line up.
func Box(t theme.Theme, title, body string, width, height int) string {
	width = max(width, 6)
	inner := width - 4
	edge := t.Divider
	var b strings.Builder

	fill := width - 3 // ╭─ and ╮
	if title != "" {
		title = Truncate(title, max(inner-2, 1))
		fill -= lipgloss.Width(title) + 2
	}
	fill = max(fill, 0)
	b.WriteString(edge.Render("╭─"))
	if title != "" {
		b.WriteString(" " + t.Heading.Render(title) + " ")
	}
	b.WriteString(edge.Render(strings.Repeat("─", fill) + "╮"))

	lines := strings.Split(body, "\n")
	for len(lines) < height-2 {
		lines = append(lines, "")
	}
	for _, l := range lines {
		if lipgloss.Width(l) > inner {
			l = Truncate(l, inner)
		}
		b.WriteString("\n" + edge.Render("│") + " " + PadRight(l, inner) + " " + edge.Render("│"))
	}
	b.WriteString("\n" + edge.Render("╰"+strings.Repeat("─", width-2)+"╯"))
	return b.String()
}

// Rule is a thin horizontal line for use inside a Box.
func Rule(t theme.Theme, width int) string {
	return t.Divider.Render(strings.Repeat("─", max(width, 0)))
}
