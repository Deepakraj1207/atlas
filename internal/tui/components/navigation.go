package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/tui/nav"
	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// Sidebar renders the vertical navigation for wide terminals: a quiet list
// with the current section marked "›". When focused (the user is moving
// through sections) the current row is highlighted; otherwise it's only
// coloured, so it's clear where the arrow keys go. footer, if any, is drawn
// at the bottom.
func Sidebar(t theme.Theme, current nav.Section, width, height int, focused bool, footer string) string {
	inner := max(width-1, 1) // one column for the divider
	var lines []string
	for _, s := range nav.All() {
		lines = append(lines, navRow(t, s, current, inner, focused))
	}
	body := strings.Join(lines, "\n")
	if footer != "" {
		if pad := height - len(lines) - lipgloss.Height(footer); pad > 0 {
			body += strings.Repeat("\n", pad)
		}
		body += "\n" + footer
	}
	body = Fit(body, inner, height)
	divider := strings.TrimRight(strings.Repeat(t.Divider.Render("│")+"\n", height), "\n")
	return lipgloss.JoinHorizontal(lipgloss.Top, body, divider)
}

// navRow is one sidebar line: " › Title      1 ".
func navRow(t theme.Theme, s, current nav.Section, width int, focused bool) string {
	info := s.Info()
	key := s.Shortcut()
	title := Truncate(info.Title, max(width-7, 1))
	gap := max(width-4-lipgloss.Width(title)-lipgloss.Width(key)-1, 1)
	if s == current {
		row := " › " + title + strings.Repeat(" ", gap) + key + " "
		if focused {
			return t.Selected.Render(PadRight(row, width))
		}
		return t.NavCur.Render(" › "+title) + strings.Repeat(" ", gap) + t.Subtle.Render(key) + " "
	}
	style := t.NavItem
	if info.Availability == nav.ComingSoon {
		style = t.NavSoon
	}
	return "   " + style.Render(title) + strings.Repeat(" ", gap) + t.Subtle.Render(key) + " "
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

// TabBar renders the section strip used instead of the sidebar on narrow
// and medium terminals: the tabs, then a thin rule.
func TabBar(t theme.Theme, current nav.Section, width int, preferShort bool) string {
	return NavTabs(t, current, width, preferShort, false)
}

// NavTabs is TabBar with focus: the current tab is highlighted while the
// user is moving between sections.
func NavTabs(t theme.Theme, current nav.Section, width int, preferShort, focused bool) string {
	mode := ChooseTabMode(width, current, preferShort)
	var parts []string
	for _, s := range nav.All() {
		label := " " + tabLabel(s, current, mode) + " "
		switch {
		case s == current && focused:
			parts = append(parts, t.Selected.Foreground(t.Accent.GetForeground()).Render(label))
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

// HeaderInfo is what the top line shows.
type HeaderInfo struct {
	Version string
	// Status is a few words such as "System ready" with its tone.
	Status string
	Tone   theme.Tone
	// System is a short description of the computer, e.g.
	// "Windows 11 Home · 16 GB RAM". Empty when unknown.
	System string
	// Mode is "Simple" or "Advanced".
	Mode string
}

// tagline is ATLAS's full name, shown beside the logo when there is room.
const tagline = "Application & Tool Lifecycle Analysis System"

// Header renders the top line and a thin rule under it. Less important
// parts are dropped first as the terminal narrows: the full name, then the
// system description, then the version.
func Header(t theme.Theme, width int, h HeaderInfo) string {
	logo := t.Logo.Render("◆ ATLAS")
	version := ""
	if h.Version != "" {
		version = t.Subtle.Render(h.Version)
	}
	status := t.Dot(h.Tone) + " " + t.ToneStyle(h.Tone).Render(h.Status)
	mode := ""
	if h.Mode != "" {
		mode = t.Badge.Render(h.Mode)
	}
	sys := ""
	if h.System != "" {
		sys = t.Subtle.Render(h.System)
	}
	join := func(sep string, parts ...string) string {
		var keep []string
		for _, p := range parts {
			if p != "" {
				keep = append(keep, p)
			}
		}
		return strings.Join(keep, sep)
	}
	type variant struct{ left, right string }
	variants := []variant{
		{join("  ", logo, version, t.Muted.Render(tagline)), join("   ", status, sys, mode)},
		{join("  ", logo, version), join("   ", status, sys, mode)},
		{join("  ", logo, version), join("   ", status, mode)},
		{logo, join("   ", status, mode)},
		{logo, status},
		{logo, ""},
	}
	line := ""
	for _, v := range variants {
		gap := width - lipgloss.Width(v.left) - lipgloss.Width(v.right)
		if gap >= 2 || (v.right == "" && gap >= 0) {
			line = v.left + strings.Repeat(" ", max(gap, 0)) + v.right
			break
		}
	}
	if line == "" {
		line = Truncate(logo, width)
	}
	return line + "\n" + Divider(t, width)
}
