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
	return frame(t, title, t.Heading, body, width, height)
}

func frame(t theme.Theme, title string, titleStyle lipgloss.Style, body string, width, height int) string {
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
		b.WriteString(" " + titleStyle.Render(title) + " ")
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

// PageHead is a screen's title with a short subtitle beside it, over a thin
// rule the width of the screen:
//
//	Updates  Newer versions of your software
//	────────────────────────────────────────
func PageHead(t theme.Theme, title, subtitle string, width int) string {
	head := t.Title.Render(title)
	if subtitle != "" {
		if !strings.Contains(subtitle, "\x1b") {
			subtitle = t.Subtle.Render(subtitle)
		}
		head += "  " + Truncate(subtitle, max(width-lipgloss.Width(title)-2, 0))
	}
	return head + "\n" + Rule(t, width)
}

// panelWidth is the width of a framed message: the whole screen, like the
// rule under the page head, so panels line up.
func panelWidth(width int) int { return width }

// Section is a heading inside a screen with a thin line running to the
// edge:  Things to tidy ─────────────────────
// heading is already styled; it is shown in full and the line fills the rest.
func Section(t theme.Theme, heading string, width int) string {
	heading = Truncate(heading, width)
	rest := width - lipgloss.Width(heading) - 1
	if rest < 2 {
		return heading
	}
	return heading + " " + Rule(t, rest)
}
