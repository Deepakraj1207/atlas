package screens

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
)

// staleAfter is when Home suggests scanning again.
const staleAfter = 7 * 24 * time.Hour

// Home is the landing screen.
type Home struct {
	focus int
}

// NewHome creates the Home screen.
func NewHome() *Home { return &Home{} }

// Focus returns the index of the focused quick action (for tests).
func (h *Home) Focus() int { return h.focus }

// Update handles keys for the quick action row.
func (h *Home) Update(msg tea.Msg, ctx Context) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return h, nil
	}
	actions := quickActions(ctx)
	switch key.String() {
	case "left", "h", "up", "k":
		h.focus = (h.focus + len(actions) - 1) % len(actions)
	case "right", "l", "down", "j":
		h.focus = (h.focus + 1) % len(actions)
	case "enter", " ":
		a := actions[h.focus]
		return h, send(ShowActionPreviewMsg{Plan: a.Plan, Confirm: a.Confirm})
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
	b.WriteString(t.Subtle.Render(components.Wrap("Press Enter on an action to see exactly what it will do before anything happens.", w)))
	b.WriteString("\n\n")

	if p := ctx.Data.Scan; p != nil {
		b.WriteString(scanProgressView(ctx, *p))
		b.WriteString("\n\n")
	}
	if p := ctx.Data.Check; p != nil {
		b.WriteString(checkProgressView(ctx, *p))
		b.WriteString("\n\n")
	}

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
	return b.String()
}

// buttons lays the quick action buttons out in as many rows as needed.
func (h *Home) buttons(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	var rows []string
	var row []string
	used := 0
	for i, a := range quickActions(ctx) {
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
	d := ctx.Data
	muted := t.Muted.Render
	var rows [][2]string
	switch {
	case !d.Available:
		rows = [][2]string{{"Installed apps", muted("unavailable (see Settings)")}}
	case !d.Loaded:
		rows = [][2]string{{"Installed apps", muted("loading…")}}
	case d.LoadErr != "":
		rows = [][2]string{{"Installed apps", t.Danger.Render("couldn't load saved list")}}
	case d.Summary == nil:
		rows = [][2]string{
			{"Installed apps", muted("not yet scanned")},
			{"Last scan", muted("never")},
		}
	default:
		s := d.Summary
		rows = [][2]string{
			{"Installed apps", t.Body.Render(fmt.Sprint(s.AppCount))},
			{"Last scan", t.Body.Render(components.HumanTime(s.FinishedAt, ctx.Now))},
		}
		if s.Status == core.ScanPartial {
			rows = append(rows, [2]string{"", t.Warning.Render("some sources missing")})
		}
		if ctx.Advanced() {
			read, absent := 0, 0
			for _, src := range s.Sources {
				switch src.Status {
				case core.SourceOK:
					read++
				case core.SourceUnavailable:
					absent++
				}
			}
			rows = append(rows,
				[2]string{"System components", t.Body.Render(fmt.Sprint(s.ComponentCount))},
				[2]string{"Sources read", t.Body.Render(fmt.Sprintf("%d of %d (%d not on this computer)", read, len(s.Sources), absent))})
		}
	}
	switch {
	case !d.UpdatesAvailable || !d.Loaded || d.LoadErr != "":
	case d.UpdateSummary == nil:
		rows = append(rows, [2]string{"Updates available", muted("not checked yet")})
	default:
		apps, comps := d.PendingUpdates()
		n := apps + comps
		val := t.Body.Render(fmt.Sprint(n))
		if n > 0 {
			val = t.Accent.Render(fmt.Sprint(n))
		}
		if comps > 0 && ctx.Advanced() {
			val += t.Muted.Render(fmt.Sprintf(" (%d system)", comps))
		}
		rows = append(rows,
			[2]string{"Updates available", val},
			[2]string{"Last update check", t.Body.Render(components.HumanTime(d.UpdateSummary.FinishedAt, ctx.Now))})
		if d.UpdateSummary.Status == core.ScanPartial {
			rows = append(rows, [2]string{"", t.Warning.Render("some managers not checked")})
		}
	}
	return t.Heading.Render("Software overview") + "\n" + components.LabelValue(t, rows)
}

func (h *Home) recentActivity(ctx Context, width int) string {
	t := ctx.Theme
	acts := ctx.Data.Activity
	if len(acts) == 0 {
		return components.EmptyState{
			Title:   "Recent activity",
			Message: "Nothing yet. Everything ATLAS does will be listed here and in Activity History.",
		}.Render(t, width)
	}
	var b strings.Builder
	b.WriteString(t.Heading.Render("Recent activity"))
	for i, a := range acts {
		if i == 3 {
			break
		}
		when := t.Subtle.Render(" · " + components.HumanTime(a.At, ctx.Now))
		title := components.Truncate(a.Title, max(width-lipgloss.Width(when)-2, 8))
		b.WriteString("\n" + outcomeMark(t, a.Outcome) + " " + t.Body.Render(title) + when)
	}
	if len(acts) > 3 {
		b.WriteString("\n" + t.Subtle.Render("See all in Activity History (8)."))
	}
	return b.String()
}

func (h *Home) recommended(ctx Context) string {
	t := ctx.Theme
	d := ctx.Data
	var items []string
	switch {
	case !d.Available:
		items = append(items, "ATLAS can't open its saved data, so scanning is turned off. Settings (9) explains what went wrong.")
	case d.Scan != nil:
		items = append(items, "A scan is running. You can keep using ATLAS; results appear when it finishes.")
	case d.Check != nil:
		items = append(items, "An update check is running. You can keep using ATLAS; results appear in Updates (3).")
	case d.Loaded && d.Summary == nil:
		items = append(items, "Scan your system to see what is installed. It only reads information and takes a few seconds.")
	case d.Summary != nil:
		if ctx.Now.Sub(d.Summary.FinishedAt) > staleAfter {
			items = append(items, "Your last scan was "+components.HumanTime(d.Summary.FinishedAt, ctx.Now)+". Scan again to refresh the list.")
		}
		if d.Summary.Status == core.ScanPartial {
			items = append(items, "Some sources couldn't be read in the last scan. Activity History (8) shows which, and why.")
		}
		if !d.LastApplied.IsZero() && d.LastApplied.After(d.Summary.FinishedAt) {
			items = append(items, "You installed updates after your last scan. Scan again so My Software shows the new versions.")
		}
		items = append(items, "Browse and search everything that was found in My Software (2).")
	}
	if d.Loaded && d.UpdatesAvailable && !d.Busy() {
		apps, comps := d.PendingUpdates()
		switch {
		case d.UpdateSummary == nil:
			items = append(items, "Check for updates to see which of your software has newer versions. Checking only reads information.")
		case apps+comps > 0:
			items = append(items, fmt.Sprintf("%d updates are waiting. Review them in Updates (3); nothing installs until you confirm.", apps+comps))
		case ctx.Now.Sub(d.UpdateSummary.FinishedAt) > staleAfter:
			items = append(items, "Your last update check was "+components.HumanTime(d.UpdateSummary.FinishedAt, ctx.Now)+". Check again for newer versions.")
		}
	}
	items = append(items, "Press ? at any time to see the keys for the screen you are on.")
	if ctx.Advanced() {
		items = append(items, "You are in Advanced mode: package IDs, paths and command details are shown. Press m to switch back.")
	} else {
		items = append(items, "Developer? Press m for Advanced mode to see package IDs, paths and command details.")
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
		{"Administrator", map[bool]string{true: "yes", false: "no"}[ctx.Elevated]},
		{"Settings file", ctx.ConfigPath},
		{"Database", ctx.DBPath},
		{"Log file", ctx.LogPath},
	}
	return t.Heading.Render("Diagnostics") + "\n" + components.LabelValue(t, rows)
}

var _ Screen = (*Home)(nil)
