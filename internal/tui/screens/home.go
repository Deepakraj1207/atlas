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
			return h, send(NavigateMsg{Section: a.Section, Finding: a.Finding})
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

// View renders the Dashboard: a large logo, then a few framed panels with
// plenty of space between them. Wide terminals put the actions and the
// computer side by side; narrow ones stack the panels.
func (h *Home) View(ctx Context) string {
	w := ctx.Width()
	d := ctx.Data
	var b strings.Builder

	b.WriteString(h.intro(ctx, w))
	b.WriteString("\n\n")

	wide := ctx.Layout.TwoColumn && w >= 84
	if wide {
		leftW := (w - 2) / 2
		rightW := w - leftW - 2
		acts := h.actions(ctx, leftW-4)
		comp := h.glance(ctx, rightW-4)
		rows := max(strings.Count(acts, "\n"), strings.Count(comp, "\n")) + 3
		t := ctx.Theme
		b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top,
			components.Box(t, "What would you like to do?", acts, leftW, rows),
			"  ",
			components.Box(t, "Your computer", comp, rightW, rows)))
	} else {
		b.WriteString(components.Box(ctx.Theme, "What would you like to do?", h.actions(ctx, w-4), w, 0))
		b.WriteString("\n\n")
		b.WriteString(components.Box(ctx.Theme, "Your computer", h.glance(ctx, w-4), w, 0))
	}

	for _, p := range []string{progress(ctx, d.Scan, scanProgressView), progress(ctx, d.Check, checkProgressView), progress(ctx, d.Analyze, analyzeProgressView)} {
		if p != "" {
			b.WriteString("\n\n")
			b.WriteString(components.Box(ctx.Theme, "Working", p, w, 0))
		}
	}

	b.WriteString("\n\n")
	b.WriteString(h.recommended(ctx, w, lipgloss.Height(b.String())))

	if ctx.Advanced() {
		b.WriteString("\n\n")
		b.WriteString(components.Box(ctx.Theme, "Diagnostics", h.diagnostics(ctx), w, 0))
	}
	return b.String()
}

// intro is the logo and a one-line greeting, centred. The large logo needs
// a roomy terminal; smaller ones get the three-row logo, and short ones
// just the greeting so the actions stay in view.
func (h *Home) intro(ctx Context, w int) string {
	t := ctx.Theme
	greeting := "Welcome to ATLAS"
	if ctx.Data.Summary != nil {
		greeting = "Welcome back"
	}
	center := func(s string) string { return lipgloss.PlaceHorizontal(w, lipgloss.Center, s) }
	hello := center(t.Muted.Render(greeting))
	height := ctx.Layout.VisibleHeight
	switch {
	case height >= 24 && w >= components.HeroLogoWidth+2:
		return center(components.HeroLogo(t)) + "\n" + hello
	case height >= 20 && w >= components.BigLogoWidth+2:
		return center(components.BigLogo(t)) + "\n\n" + hello
	default:
		return t.Title.Render(greeting)
	}
}

// actions is the list inside the "What would you like to do?" panel; the
// highlighted action is described under it.
func (h *Home) actions(ctx Context, width int) string {
	t := ctx.Theme
	actions := quickActions(ctx)
	items := make([]components.ActionItem, len(actions))
	for i, a := range actions {
		items[i] = components.ActionItem{Label: a.Label, Desc: a.Desc}
	}
	h.focus = min(max(h.focus, 0), len(items)-1)
	return components.ActionList(t, width, items, h.focus, !ctx.NavFocused)
}

// glance is the inside of the "Your computer" panel: the headline figures
// one per line, a thin rule, then the computer itself.
func (h *Home) glance(ctx Context, width int) string {
	t := ctx.Theme
	stats := h.stats(ctx)
	valW := 0
	for _, st := range stats {
		valW = max(valW, lipgloss.Width(st.Value))
	}
	var lines []string
	for _, st := range stats {
		val := t.Strong.Render(st.Value)
		switch {
		case st.Dim:
			val = t.Subtle.Render(st.Value)
		case st.Tone != theme.ToneNeutral:
			val = t.ToneStyle(st.Tone).Bold(true).Render(st.Value)
		}
		pad := strings.Repeat(" ", valW-lipgloss.Width(st.Value))
		lines = append(lines, pad+val+" "+t.Muted.Render(st.Label))
	}
	figures := strings.Join(lines, "\n")
	figW := lipgloss.Width(figures)
	if width >= figW+6+36 {
		// Room for two columns: figures on the left, the computer beside.
		sep := t.Divider.Render(strings.Repeat("│\n", len(lines)-1) + "│")
		about := h.system(ctx, width-figW-6) + "\n" + h.lastScan(ctx)
		return lipgloss.JoinHorizontal(lipgloss.Top, figures, "  ", sep, "  ", about)
	}
	return figures + "\n" + components.Rule(t, width) + "\n" + h.system(ctx, width) + "\n" + h.lastScan(ctx)
}

// lastScan says when the list was last read, or why it can't be.
func (h *Home) lastScan(ctx Context) string {
	t := ctx.Theme
	d := ctx.Data
	switch {
	case !d.Available:
		return t.Warning.Render("Saved data unavailable")
	case !d.Loaded:
		return t.Subtle.Render("Loading…")
	case d.LoadErr != "":
		return t.Danger.Render("Couldn't load the saved list")
	case d.Summary == nil:
		return t.Subtle.Render("Not scanned yet")
	}
	line := t.Subtle.Render("Scanned " + components.HumanTime(d.Summary.FinishedAt, ctx.Now))
	if d.Summary.Status == core.ScanPartial {
		line += t.Warning.Render(" · some sources missing")
	}
	return line
}

func progress(ctx Context, p *ScanProgress, view func(Context, ScanProgress) string) string {
	if p == nil {
		return ""
	}
	return view(ctx, *p)
}

// system describes the computer with only what is known, one fact per
// line so it reads well in a narrow panel.
func (h *Home) system(ctx Context, width int) string {
	t := ctx.Theme
	s := ctx.System
	if s == nil {
		return t.Subtle.Render("Reading system information…")
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
		return t.Subtle.Render("Couldn't read this computer's details")
	}
	return strings.Join(lines, "\n")
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

// recommended is the "Next step" panel: only the most important one or
// two, so the Dashboard stays calm. used is the height above it, so a short
// terminal shows one step and a tall one up to three.
func (h *Home) recommended(ctx Context, width, used int) string {
	t := ctx.Theme
	steps := NextSteps(ctx)
	n := 1
	if room := ctx.Layout.VisibleHeight - used - 2; room >= 4 {
		n = min(room/2, 3)
	}
	steps = steps[:min(n, len(steps))]
	title := "Next step"
	if len(steps) > 1 {
		title = "Next steps"
	}
	inner := width - 4
	var lines []string
	for _, s := range steps {
		for i, l := range strings.Split(components.Wrap(s, inner-2), "\n") {
			mark := "  "
			if i == 0 {
				mark = t.Accent.Render("›") + " "
			}
			lines = append(lines, mark+t.Body.Render(l))
		}
	}
	return components.Box(t, title, strings.Join(lines, "\n"), width, 0)
}

// NextSteps lists what the user could do next, most important first: things
// that need attention, then things not done yet, then general tips. Only the
// first few fit on the Dashboard without scrolling, so the order matters.
func NextSteps(ctx Context) []string {
	d := ctx.Data
	var items []string
	add := func(s string) { items = append(items, s) }

	if !d.Available {
		add("ATLAS can't open its saved data, so scanning is turned off. Settings (9) explains what went wrong.")
	}
	switch {
	case d.Scan != nil:
		add("A scan is running. You can keep using ATLAS; results appear when it finishes.")
	case d.Check != nil:
		add("An update check is running. You can keep using ATLAS; results appear in Updates (3).")
	case d.Analyze != nil:
		add("Analysis is running. Results appear in Smart Analysis (4) and Dependencies (5).")
	}
	// A run that never reported back: say so before anything else.
	if len(d.Activity) > 0 && d.Activity[0].Outcome == core.OutcomeStarted {
		add("ATLAS was closed while a package manager was working (see Activity History (8)). Scan again to see what is really installed.")
	}
	if d.Available && d.Loaded && d.Summary == nil && d.Scan == nil {
		add("Scan your system to see what is installed. It only reads information and takes a few seconds.")
	}

	if s := d.Summary; s != nil {
		// Needs attention.
		if s.Status == core.ScanPartial {
			add("Some sources couldn't be read in the last scan. Activity History (8) shows which, and why.")
		}
		if d.UpdatesSinceScan {
			add("You installed updates after your last scan. Scan again so My Software shows the new versions.")
		}
	}
	apps, comps := d.PendingUpdates()
	checked := d.Loaded && d.UpdatesAvailable && d.UpdateSummary != nil
	if checked && apps+comps > 0 && d.Check == nil {
		add(fmt.Sprintf("%s waiting. Review them in Updates (3); nothing installs until you confirm.", waiting(apps+comps)))
	}
	analyzed := d.Loaded && d.AnalysisAvailable && d.Summary != nil && d.Analysis != nil
	if analyzed && !d.AnalysisStale() && d.Analyze == nil {
		if att, _ := d.Analysis.Counts(); att > 0 {
			add(fmt.Sprintf("Smart Analysis found %s worth your attention. See Smart Analysis (4).", components.Plural(att, "thing")))
		}
	}

	// Not done yet, or out of date.
	if d.Loaded && d.UpdatesAvailable && !d.Busy() {
		switch {
		case d.UpdateSummary == nil:
			add("Check for updates to see which of your software has newer versions. Checking only reads information.")
		case apps+comps == 0 && ctx.Now.Sub(d.UpdateSummary.FinishedAt) > staleAfter:
			add("Your last update check was " + components.HumanTime(d.UpdateSummary.FinishedAt, ctx.Now) + ". Check again for newer versions.")
		}
	}
	if d.Loaded && d.AnalysisAvailable && d.Summary != nil && !d.Busy() {
		switch {
		case d.Analysis == nil:
			add("Analyze your software to spot runtimes that no longer get security fixes, duplicates, and what depends on what.")
		case d.AnalysisStale():
			add("Your software list changed since the last analysis. Analyze again to bring Smart Analysis up to date.")
		}
	}
	if s := d.Summary; s != nil {
		if ctx.Now.Sub(s.FinishedAt) > staleAfter {
			add("Your last scan was " + components.HumanTime(s.FinishedAt, ctx.Now) + ". Scan again to refresh the list.")
		}
		add("Browse and search everything that was found in My Software (2).")
	}

	// General tips, always last.
	add("Press ? at any time to see the keys for the screen you are on.")
	if ctx.Advanced() {
		add("You are in Advanced mode: package IDs, paths and command details are shown. Press m to switch back.")
	} else {
		add("Developer? Press m for Advanced mode to see package IDs, paths and command details.")
	}
	return items
}

// waiting is "1 update is" or "33 updates are".
func waiting(n int) string {
	if n == 1 {
		return "1 update is"
	}
	return fmt.Sprintf("%d updates are", n)
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
	return components.LabelValue(t, rows)
}

var _ Screen = (*Home)(nil)
