package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/tui/nav"
	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// Sidebar renders the vertical navigation list for wide terminals.
func Sidebar(t theme.Theme, current nav.Section, width, height int, footer string) string {
	var b strings.Builder
	inner := max(width-1, 1) // leave one column for the divider
	for _, s := range nav.All() {
		info := s.Info()
		label := s.Shortcut() + "  " + info.Title
		var line string
		switch {
		case s == current:
			line = t.NavCur.Render("› " + label)
		case info.Availability == nav.ComingSoon:
			line = t.NavSoon.Render("  " + label)
		default:
			line = t.NavItem.Render("  " + label)
		}
		b.WriteString(lipgloss.NewStyle().MaxWidth(inner).Render(line))
		b.WriteString("\n")
	}
	body := strings.TrimRight(b.String(), "\n")
	if footer != "" {
		used := lipgloss.Height(body)
		pad := height - used - lipgloss.Height(footer)
		if pad > 0 {
			body += strings.Repeat("\n", pad)
		}
		body += "\n" + footer
	}
	body = Fit(body, inner, height)

	divider := strings.TrimRight(strings.Repeat(t.Divider.Render("│")+"\n", height), "\n")
	return lipgloss.JoinHorizontal(lipgloss.Top, body, divider)
}

// TabMode is how the tab bar labels sections, chosen to fit the width.
type TabMode int

const (
	// TabFull shows "1 My Software" for every section.
	TabFull TabMode = iota
	// TabShort shows abbreviated names for every section.
	TabShort
	// TabCompact shows only the digit for inactive sections and the short
	// name for the current one.
	TabCompact
)

// tabLabel returns the label for a section in a given mode.
func tabLabel(s nav.Section, current nav.Section, mode TabMode) string {
	info := s.Info()
	switch mode {
	case TabFull:
		return s.Shortcut() + " " + info.Title
	case TabShort:
		return s.Shortcut() + " " + info.Short
	default:
		if s == current {
			return s.Shortcut() + " " + info.Short
		}
		return s.Shortcut()
	}
}

// ChooseTabMode picks the densest-readable mode whose labels fit in width.
// Each label is padded by one column on each side.
func ChooseTabMode(width int, current nav.Section, preferShort bool) TabMode {
	fits := func(mode TabMode) bool {
		total := 0
		for _, s := range nav.All() {
			total += lipgloss.Width(tabLabel(s, current, mode)) + 2
		}
		return total <= width
	}
	if !preferShort && fits(TabFull) {
		return TabFull
	}
	if fits(TabShort) {
		return TabShort
	}
	return TabCompact
}

// TabBar renders a horizontal section strip for narrow and medium terminals.
// It is two lines: the tabs and a divider.
func TabBar(t theme.Theme, current nav.Section, width int, preferShort bool) string {
	mode := ChooseTabMode(width, current, preferShort)
	var parts []string
	for _, s := range nav.All() {
		label := tabLabel(s, current, mode)
		switch {
		case s == current:
			parts = append(parts, t.NavCur.Render(label))
		case s.Info().Availability == nav.ComingSoon:
			parts = append(parts, t.NavSoon.Render(label))
		default:
			parts = append(parts, t.NavItem.Render(label))
		}
	}
	row := lipgloss.NewStyle().MaxWidth(width).Render(strings.Join(parts, ""))
	return row + "\n" + Divider(t, width)
}

// Header renders the top application line: name, tagline and right-side badges.
func Header(t theme.Theme, width int, right string) string {
	left := t.Title.Render("ATLAS") + "  " + t.Muted.Render("Application & Tool Lifecycle Analysis System")
	if width < 70 {
		left = t.Title.Render("ATLAS")
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		return lipgloss.NewStyle().MaxWidth(width).Render(left) + "\n"
	}
	return left + strings.Repeat(" ", gap) + right + "\n"
}
