package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/tui/components"
)

// Home is the landing screen.
type Home struct {
	actions []QuickAction
	focus   int
}

// NewHome creates the Home screen.
func NewHome() *Home {
	return &Home{actions: quickActions()}
}

// Focus returns the index of the focused quick action (for tests).
func (h *Home) Focus() int { return h.focus }

// Update handles keys for the quick action row.
func (h *Home) Update(msg tea.Msg, ctx Context) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return h, nil
	}
	switch key.String() {
	case "left", "h", "up", "k":
		h.focus = (h.focus + len(h.actions) - 1) % len(h.actions)
	case "right", "l", "down", "j":
		h.focus = (h.focus + 1) % len(h.actions)
	case "enter", " ":
		return h, send(ShowActionPreviewMsg{Plan: h.actions[h.focus].Plan})
	}
	return h, nil
}

// Hints lists Home's keys.
func (h *Home) Hints(ctx Context) []components.KeyHint {
	return []components.KeyHint{
		{Key: "←/→", Desc: "choose action"},
		{Key: "enter", Desc: "preview action"},
	}
}

// View renders Home.
func (h *Home) View(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	var b strings.Builder

	b.WriteString(t.Title.Render("Welcome to ATLAS"))
	b.WriteString("\n")
	b.WriteString(t.Muted.Render(components.Wrap("Keep your software tidy, up to date and understood, in plain language. Nothing happens without your say-so.", w)))
	b.WriteString("\n\n")

	b.WriteString(t.Heading.Render("Quick actions"))
	b.WriteString("\n")
	b.WriteString(h.buttons(ctx))
	b.WriteString("\n")
	b.WriteString(t.Subtle.Render(components.Wrap("Press Enter on an action to see exactly what it would do. In this version, actions are previews only.", w)))
	b.WriteString("\n\n")

	if ctx.Layout.TwoColumn {
		leftW := (w - 3) / 2
		rightW := w - leftW - 3
		left := lipgloss.NewStyle().Width(leftW).Render(h.overview(ctx))
		right := lipgloss.NewStyle().Width(rightW).Render(h.recentActivity(ctx, rightW))
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, "   ", right))
	} else {
		b.WriteString(h.overview(ctx))
		b.WriteString("\n\n")
		b.WriteString(h.recentActivity(ctx, w))
	}
	b.WriteString("\n\n")

	b.WriteString(h.recommended(ctx))

	if ctx.Advanced() {
		b.WriteString("\n\n")
		b.WriteString(h.diagnostics(ctx))
	}
	return components.Fit(b.String(), w, ctx.Height())
}

// buttons lays the quick action buttons out in as many rows as needed.
func (h *Home) buttons(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	var rows []string
	var row []string
	used := 0
	for i, a := range h.actions {
		style := t.Button
		if i == h.focus {
			style = t.ButtonOn
		}
		btn := style.Render(a.Label)
		bw := lipgloss.Width(btn)
		if len(row) > 0 && used+1+bw > w {
			rows = append(rows, strings.Join(row, " "))
			row, used = nil, 0
		}
		row = append(row, btn)
		used += bw
		if len(row) > 1 {
			used++
		}
	}
	if len(row) > 0 {
		rows = append(rows, strings.Join(row, " "))
	}
	return strings.Join(rows, "\n")
}

func (h *Home) overview(ctx Context) string {
	t := ctx.Theme
	notScanned := t.Muted.Render("not yet scanned")
	rows := [][2]string{
		{"Installed apps", notScanned},
		{"Updates available", notScanned},
		{"Last scan", t.Muted.Render("never")},
	}
	return t.Heading.Render("Software overview") + "\n" + components.LabelValue(t, rows)
}

func (h *Home) recentActivity(ctx Context, width int) string {
	return components.EmptyState{
		Title:   "Recent activity",
		Message: "Nothing yet. ATLAS has not performed any actions on this computer. Everything it does will be listed here and in Activity History.",
	}.Render(ctx.Theme, width)
}

func (h *Home) recommended(ctx Context) string {
	t := ctx.Theme
	items := []string{
		"Scan your system to see what is installed. Scanning arrives in Phase 2; the button above shows what it will do.",
		"Press ? at any time to see the keys for the screen you are on.",
	}
	if ctx.Advanced() {
		items = append(items, "You are in Advanced mode: package IDs, paths and command previews are shown where available. Press m to switch back.")
	} else {
		items = append(items, "Developer? Press m for Advanced mode to see package IDs, paths and command previews.")
	}
	return t.Heading.Render("Recommended actions") + "\n" + components.Bullets(t, ctx.Width(), items)
}

func (h *Home) diagnostics(ctx Context) string {
	t := ctx.Theme
	l := ctx.Layout
	rows := [][2]string{
		{"Version", ctx.Version},
		{"Platform", ctx.Platform},
		{"Terminal", fmt.Sprintf("%d×%d (%s layout)", l.Width, l.Height, l.Breakpoint)},
		{"Settings file", ctx.ConfigPath},
		{"Log file", ctx.LogPath},
	}
	return t.Heading.Render("Diagnostics") + "\n" + components.LabelValue(t, rows)
}

var _ Screen = (*Home)(nil)
