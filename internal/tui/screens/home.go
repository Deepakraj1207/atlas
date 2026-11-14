package screens

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/platform"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// staleAfter is when Home suggests scanning again.
const staleAfter = 7 * 24 * time.Hour

// Home is the Dashboard: a welcome, a "What would you like to do?" list,
// the computer at a glance, and what needs attention. It only shows real,
// saved data; anything not known yet says so.
type Home struct {
	focus int
}

// NewHome creates the Dashboard screen.
func NewHome() *Home { return &Home{} }

// Focus returns the index of the highlighted action (for tests).
func (h *Home) Focus() int { return h.focus }

// Update moves through the action list and runs the chosen action.
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
	case "home", "g":
		h.focus = 0
	case "end", "G":
		h.focus = len(actions) - 1
	case "enter", " ":
		a := actions[h.focus]
		switch {
		case a.Search:
			return h, send(SearchMsg{})
		case a.Navigate:
			return h, send(NavigateMsg{Section: a.Section})
		}
		return h, send(ShowActionPreviewMsg{Plan: a.Plan, Confirm: a.Confirm})
	}
	return h, nil
}

// Hints lists the Dashboard's keys.
func (h *Home) Hints(ctx Context) []components.KeyHint {
	return []components.KeyHint{
		{Key: "↑↓", Desc: "choose"},
		{Key: "enter", Desc: "select"},
	}
}

// View renders the Dashboard. On wide terminals the actions and the
// at-a-glance figures sit side by side so both fit a default 120×30 window;
// otherwise everything is one column.
func (h *Home) View(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	d := ctx.Data
	var b strings.Builder

	b.WriteString(h.intro(ctx, w))
	b.WriteString("\n\n")

	wide := ctx.Layout.TwoColumn && w >= 84
	if wide {
		rightW := min(max(w*2/5, 34), 48)
		leftW := w - rightW - 4
		left := lipgloss.NewStyle().Width(leftW).Render(h.actions(ctx, leftW))
		right := lipgloss.NewStyle().Width(rightW).Render(h.glance(ctx, rightW))
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, "    ", right))
		b.WriteString("\n")
	} else {
		b.WriteString(h.actions(ctx, w))
		b.WriteString("\n")
	}

	for _, p := range []string{progress(ctx, d.Scan, scanProgressView), progress(ctx, d.Check, checkProgressView), progress(ctx, d.Analyze, analyzeProgressView)} {
		if p != "" {
			b.WriteString("\n")
			b.WriteString(p)
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(components.Divider(t, w))
	b.WriteString("\n\n")
	if wide {
		leftW := (w - 4) / 2
		rightW := w - leftW - 4
		left := lipgloss.NewStyle().Width(leftW).Render(h.recommended(ctx, leftW))
		right := lipgloss.NewStyle().Width(rightW).Render(h.recentActivity(ctx, rightW))
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, left, "    ", right))
	} else {
		b.WriteString(h.glance(ctx, w))
		b.WriteString("\n\n")
		b.WriteString(h.recommended(ctx, w))
		b.WriteString("\n\n")
		b.WriteString(h.recentActivity(ctx, w))
	}

	if ctx.Advanced() {
		b.WriteString("\n\n")
		b.WriteString(h.diagnostics(ctx))
	}
	return b.String()
}

// intro is the ATLAS logo with the welcome beside it. The six-row logo is
// for large windows; it falls back to the three-row one, which goes beside
// the welcome where it fits, above it on tall terminals, and is left out on
// small ones.
func (h *Home) intro(ctx Context, w int) string {
	t := ctx.Theme
	greeting := "Welcome to ATLAS"
	if ctx.Data.Summary != nil {
		greeting = "Welcome back"
	}
	const about = "Keep your software tidy, up to date and understood, in plain language. Nothing happens without your say-so."
	text := func(width int) string {
		return t.Title.Render(greeting) + "\n" + t.Muted.Render(components.Wrap(about, width))
	}
	beside := func(logo string, logoW int) string {
		return lipgloss.JoinHorizontal(lipgloss.Center, logo, "    ", text(w-logoW-4))
	}
	termH := ctx.Layout.Height
	largeW, smallW := components.LargeLogoWidth, components.BigLogoWidth
	switch {
	case w >= largeW+4+36 && termH >= 40:
		return beside(components.LargeLogo(t), largeW)
	case w >= largeW && termH >= 46:
		return components.LargeLogo(t) + "\n\n" + text(w)
	case w >= smallW+4+40:
		return lipgloss.JoinHorizontal(lipgloss.Top, components.BigLogo(t), "    ", text(w-smallW-4))
	case w >= smallW && termH >= 30:
		return components.BigLogo(t) + "\n\n" + text(w)
	default:
		return text(w)
	}
}

// actions is the "What would you like to do?" block.
func (h *Home) actions(ctx Context, width int) string {
	t := ctx.Theme
	actions := quickActions(ctx)
	items := make([]components.ActionItem, len(actions))
	for i, a := range actions {
		items[i] = components.ActionItem{Label: a.Label, Desc: a.Desc}
	}
	h.focus = min(max(h.focus, 0), len(items)-1)
	return t.Heading.Render("What would you like to do?") + "\n\n" +
		components.ActionList(t, width, items, h.focus, !ctx.NavFocused) + "\n\n" +
		t.Subtle.Render(components.Wrap("Anything that changes your computer is previewed first.", width))
}

// glance is the computer and the software at a glance.
func (h *Home) glance(ctx Context, width int) string {
	t := ctx.Theme
	return h.system(ctx, width) + "\n\n" + components.Stats(t, width, h.stats(ctx)) + "\n\n" + h.overview(ctx)
}

func progress(ctx Context, p *ScanProgress, view func(Context, ScanProgress) string) string {
	if p == nil {
		return ""
	}
	return view(ctx, *p)
}

// system describes the computer with only what is known, one fact per
// line so it reads well in a narrow column.
func (h *Home) system(ctx Context, width int) string {
	t := ctx.Theme
	label := components.SectionLabel(t, "System")
	s := ctx.System
	if s == nil {
		return label + "\n" + t.Subtle.Render("Reading system information…")
	}
	var lines []string
	if s.OS != "" {
		lines = append(lines, t.Body.Render(components.Truncate(s.OS, width)))
	}
	if s.MemTotal > 0 {
		mem := t.Body.Render(platform.GB(s.MemTotal) + " memory")
		if used := s.MemUsed(); used > 0 {
			mem += t.Subtle.Render(" · " + platform.GB(used) + " in use")
		}
		lines = append(lines, mem)
	}
	if s.DiskTotal > 0 {
		disk := platform.GB(s.DiskFree) + " free of " + platform.GB(s.DiskTotal)
		if s.DiskPath != "" && s.DiskPath != "/" {
			disk += " on " + s.DiskPath
		}
		lines = append(lines, t.Body.Render(disk))
	}
	if len(lines) == 0 {
		return label + "\n" + t.Subtle.Render("ATLAS couldn't read this computer's details.")
	}
	return label + "\n" + strings.Join(lines, "\n")
}

// stats are the headline figures. A dash means "not known yet" (no scan,
// check or analysis), never zero.
func (h *Home) stats(ctx Context) []components.Stat {
	d := ctx.Data
	unknown := func(label string) components.Stat { return components.Stat{Value: "—", Label: label, Dim: true} }
	var out []components.Stat
	if d.Summary != nil {
		out = append(out, components.Stat{Value: fmt.Sprint(d.Summary.AppCount), Label: plural(d.Summary.AppCount, "app", "apps")})
	} else {
		out = append(out, unknown("apps"))
	}
	if d.UpdateSummary != nil {
		apps, comps := d.PendingUpdates()
		n := apps + comps
		st := components.Stat{Value: fmt.Sprint(n), Label: plural(n, "update", "updates")}
		if n > 0 {
			st.Tone = theme.ToneBusy
		}
		out = append(out, st)
	} else {
		out = append(out, unknown("updates"))
	}
	if a := d.Analysis; a != nil {
		out = append(out, components.Stat{Value: fmt.Sprint(len(a.Runtimes)), Label: plural(len(a.Runtimes), "runtime", "runtimes")})
		n := 0
		for _, f := range a.Findings {
			if f.Kind == "unused" || f.Kind == "unneeded" {
				n += len(f.Items)
			}
		}
		st := components.Stat{Value: fmt.Sprint(n), Label: "to clean up"}
		if n > 0 {
			st.Tone = theme.ToneWarn
		}
		out = append(out, st)
	} else {
		out = append(out, unknown("runtimes"), unknown("to clean up"))
	}
	return out
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
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
	if d.AnalysisAvailable && d.Loaded && d.Summary != nil {
		switch a := d.Analysis; {
		case a == nil:
			rows = append(rows, [2]string{"Smart Analysis", muted("not run yet")})
		default:
			att, info := a.Counts()
			val := t.Success.Render("nothing to look at")
			if att > 0 {
				val = t.Warning.Render(fmt.Sprintf("%d worth your attention", att))
			} else if info > 0 {
				val = t.Body.Render(fmt.Sprintf("%d worth knowing", info))
			}
			rows = append(rows, [2]string{"Smart Analysis", val})
		}
	}
	return components.SectionLabel(t, "Software overview") + "\n" + components.LabelValue(t, rows)
}

func (h *Home) recentActivity(ctx Context, width int) string {
	t := ctx.Theme
	acts := ctx.Data.Activity
	if len(acts) == 0 {
		return components.SectionLabel(t, "Recent activity") + "\n" +
			t.Muted.Render(components.Wrap("Nothing yet. Everything ATLAS does will be listed here and in Activity History.", width))
	}
	var b strings.Builder
	b.WriteString(components.SectionLabel(t, "Recent activity"))
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

func (h *Home) recommended(ctx Context, width int) string {
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
	case d.Analyze != nil:
		items = append(items, "Analysis is running. Results appear in Smart Analysis (4) and Dependencies (5).")
	case d.Loaded && d.Summary == nil:
		items = append(items, "Scan your system to see what is installed. It only reads information and takes a few seconds.")
	case d.Summary != nil:
		if ctx.Now.Sub(d.Summary.FinishedAt) > staleAfter {
			items = append(items, "Your last scan was "+components.HumanTime(d.Summary.FinishedAt, ctx.Now)+". Scan again to refresh the list.")
		}
		if d.Summary.Status == core.ScanPartial {
			items = append(items, "Some sources couldn't be read in the last scan. Activity History (8) shows which, and why.")
		}
		if d.UpdatesSinceScan {
			items = append(items, "You installed updates after your last scan. Scan again so My Software shows the new versions.")
		}
		items = append(items, "Browse and search everything that was found in My Software (2).")
	}
	if d.Loaded && d.AnalysisAvailable && d.Summary != nil && !d.Busy() {
		switch a := d.Analysis; {
		case a == nil:
			items = append(items, "Analyze your software to spot runtimes that no longer get security fixes, duplicates, and what depends on what.")
		case d.AnalysisStale():
			items = append(items, "Your software list changed since the last analysis. Analyze again to bring Smart Analysis up to date.")
		default:
			if att, _ := a.Counts(); att > 0 {
				items = append(items, fmt.Sprintf("Smart Analysis found %d things worth your attention. See Smart Analysis (4).", att))
			}
		}
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
	return components.SectionLabel(t, "Next steps") + "\n" + components.Bullets(t, width, items)
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
	return components.SectionLabel(t, "Diagnostics") + "\n" + components.LabelValue(t, rows)
}

var _ Screen = (*Home)(nil)
