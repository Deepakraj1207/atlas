package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// KeyHint pairs a key label with a short description.
type KeyHint struct {
	Key  string
	Desc string
}

// HintRow renders hints on one line ("↑↓ navigate  enter select"),
// dropping trailing hints that do not fit.
func HintRow(t theme.Theme, width int, hints []KeyHint) string {
	var parts []string
	used := 0
	for _, h := range hints {
		s := t.Key.Render(h.Key) + " " + t.KeyDesc.Render(h.Desc)
		w := lipgloss.Width(s)
		sep := 0
		if len(parts) > 0 {
			sep = 3
		}
		if used+sep+w > width {
			break
		}
		parts = append(parts, s)
		used += sep + w
	}
	return strings.Join(parts, "   ")
}

// HelpGroup is a titled set of hints for the help overlay.
type HelpGroup struct {
	Title string
	Hints []KeyHint
}

// Help renders the context-sensitive help overlay content. The result may
// be taller than the screen; the caller scrolls it.
func Help(t theme.Theme, width int, context string, groups []HelpGroup) string {
	var b strings.Builder
	b.WriteString(t.Logo.Render("?") + " " + t.Title.Render("Keyboard help"))
	if context != "" {
		b.WriteString("  ")
		b.WriteString(t.Muted.Render(context))
	}
	b.WriteString("\n\n")

	keyW := 0
	for _, g := range groups {
		for _, h := range g.Hints {
			keyW = max(keyW, lipgloss.Width(h.Key))
		}
	}
	for i, g := range groups {
		if len(g.Hints) == 0 {
			continue
		}
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(t.Heading.Render(g.Title))
		b.WriteString("\n")
		for _, h := range g.Hints {
			b.WriteString("  ")
			b.WriteString(t.Accent.Width(keyW + 2).Render(h.Key))
			desc := Wrap(h.Desc, max(width-keyW-4, 10))
			desc = strings.ReplaceAll(desc, "\n", "\n"+strings.Repeat(" ", keyW+4))
			b.WriteString(t.Muted.Render(desc))
			b.WriteString("\n")
		}
	}
	b.WriteString("\n")
	b.WriteString(t.Subtle.Render("Press ? or Esc to close this help."))
	return b.String()
}
