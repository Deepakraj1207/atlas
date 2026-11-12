package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
)

// SmartAnalysis lists findings, most important first, with a final entry
// that explains what was checked.
type SmartAnalysis struct{ cursor int }

// NewSmartAnalysis creates the screen.
func NewSmartAnalysis() *SmartAnalysis { return &SmartAnalysis{} }

// SelfScrolling reports that the screen keeps the selection in view.
func (a *SmartAnalysis) SelfScrolling() bool { return true }

// Cursor returns the selected entry (for tests).
func (a *SmartAnalysis) Cursor() int { return a.cursor }

func (a *SmartAnalysis) entries(ctx Context) int {
	if ctx.Data.Analysis == nil {
		return 0
	}
	return len(ctx.Data.Analysis.Findings) + 1 // + "What ATLAS checked"
}

// Update moves the selection and starts actions.
func (a *SmartAnalysis) Update(msg tea.Msg, ctx Context) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return a, nil
	}
	n := a.entries(ctx)
	switch key.String() {
	case "a":
		return a, requestAnalysis(ctx)
	case "s":
		if ctx.Data.Summary == nil {
			return a, requestScan(ctx)
		}
	case "up", "k":
		a.cursor--
	case "down", "j":
		a.cursor++
	case "pgup", "ctrl+u":
		a.cursor -= 5
	case "pgdown", "ctrl+d":
		a.cursor += 5
	case "home", "g":
		a.cursor = 0
	case "end", "G":
		a.cursor = n - 1
	}
	a.cursor = min(max(a.cursor, 0), max(n-1, 0))
	return a, nil
}

// Hints lists the keys.
func (a *SmartAnalysis) Hints(ctx Context) []components.KeyHint {
	switch {
	case ctx.Data.Summary == nil:
		return []components.KeyHint{{Key: "s", Desc: "scan first"}}
	case ctx.Data.Analysis == nil:
		return []components.KeyHint{{Key: "a", Desc: "analyze"}}
	}
	return []components.KeyHint{{Key: "↑/↓", Desc: "choose"}, {Key: "a", Desc: "analyze again"}}
}

// View renders the findings or the appropriate state.
func (a *SmartAnalysis) View(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	d := ctx.Data
	head := t.Title.Render("Smart Analysis") + "\n" +
		t.Muted.Render(components.Wrap("Plain-language observations about your software. Looking at them changes nothing.", w)) + "\n\n"

	switch {
	case !d.AnalysisAvailable:
		return head + components.ErrorState{
			Title: "Analysis is unavailable", Summary: "ATLAS couldn't open its saved data.",
			Details: d.StoreErr, ShowDetails: ctx.Advanced(),
		}.Render(t, w)
	case !d.Loaded:
		return head + t.Muted.Render("Loading…")
	case d.Analysis == nil && d.Analyze != nil:
		return head + analyzeProgressView(ctx, *d.Analyze)
	case d.Analysis == nil && d.Summary == nil:
		return head + components.EmptyState{
			Title:   "Scan your system first",
			Message: "Analysis works from the list of software a scan finds.",
			Hint:    "Press s to scan. It only reads information.",
		}.Render(t, w)
	case d.Analysis == nil:
		return head + components.EmptyState{
			Title:   "You haven't analyzed your software yet",
			Message: "ATLAS looks for things worth knowing, explains why they matter, and suggests what you can do. It never changes anything by itself.",
			Plan: []string{
				"Runtimes such as Python, Node.js or .NET whose support has ended or ends soon",
				"Apps installed twice by different tools, or in several versions",
				"Runtimes nothing else needs any more, and updates still waiting",
			},
			Hint: "Press a to analyze.",
		}.Render(t, w)
	}

	r := d.Analysis
	att, info := r.Counts()
	var parts []string
	switch {
	case att+info == 0:
		parts = append(parts, "Nothing needs your attention")
	case att == 0:
		parts = append(parts, "Nothing urgent", fmt.Sprintf("%d worth knowing", info))
	default:
		parts = append(parts, fmt.Sprintf("%d worth your attention", att), fmt.Sprintf("%d worth knowing", info))
	}
	parts = append(parts, "analyzed "+components.HumanTime(r.FinishedAt, ctx.Now))
	line := t.Muted.Render(strings.Join(parts, " · "))
	if d.Analyze != nil {
		line += t.Accent.Render(" · analyzing " + d.Analyze.Spinner)
	}
	head += components.Truncate(line, w) + "\n"
	head += staleNotes(ctx, r) + "\n"

	a.cursor = min(max(a.cursor, 0), a.entries(ctx)-1)
	var lines []string
	selStart, selEnd := 0, 0
	indent := "    "
	body := func(text string, style func(...string) string) {
		for _, l := range strings.Split(components.Wrap(text, max(w-4, 10)), "\n") {
			lines = append(lines, indent+style(l))
		}
	}
	for i, f := range r.Findings {
		mark := t.Accent.Render("i")
		if f.Severity == core.SeverityAttention {
			mark = t.Warning.Render("!")
		}
		title := t.Body.Render(f.Title)
		prefix := "  "
		if i == a.cursor {
			prefix = t.Accent.Render("› ")
			title = t.Accent.Bold(true).Render(f.Title)
			selStart = len(lines)
		}
		lines = append(lines, components.Truncate(prefix+mark+" "+title, w))
		if i != a.cursor {
			continue
		}
		body(f.Summary, t.Body.Render)
		if f.Why != "" {
			body("Why it matters: "+f.Why, t.Muted.Render)
		}
		if f.Suggestion != "" {
			body("What you can do: "+f.Suggestion, t.Muted.Render)
		}
		const show = 8
		for j, it := range f.Items {
			if j == show {
				lines = append(lines, indent+t.Subtle.Render(fmt.Sprintf("…and %d more", len(f.Items)-show)))
				break
			}
			text := "• " + it.Name
			if it.Version != "" {
				text += " " + it.Version
			}
			if it.Note != "" {
				text += " (" + it.Note + ")"
			}
			if ctx.Advanced() && it.Source != "" {
				text += " · " + it.Source
			}
			lines = append(lines, indent+components.Truncate(t.Body.Render(text), max(w-4, 10)))
		}
		if ctx.Advanced() {
			for _, dl := range f.Details {
				lines = append(lines, indent+components.Truncate(t.Subtle.Render(dl), max(w-4, 10)))
			}
		}
		if f.Reference != "" {
			body("Source: "+f.Reference, t.Subtle.Render)
		}
		selEnd = len(lines)
	}

	// Final entry: what was checked, including what couldn't be.
	ci := len(r.Findings)
	prefix, title := "  ", t.Heading.Render("What ATLAS checked")
	if a.cursor == ci {
		prefix, title = t.Accent.Render("› "), t.Accent.Bold(true).Render("What ATLAS checked")
		selStart = len(lines)
	}
	if len(r.Findings) > 0 {
		lines = append(lines, "")
	}
	lines = append(lines, prefix+title)
	if a.cursor == ci {
		for _, c := range r.Checks {
			res := t.Body.Render(fmt.Sprintf("%d found", c.Found))
			switch {
			case !c.Available:
				res = t.Subtle.Render("not checked")
			case c.Found == 0:
				res = t.Success.Render("none found")
			}
			lines = append(lines, indent+components.Truncate(c.Name+": "+res, max(w-4, 10)))
			if c.Note != "" {
				body(c.Note, t.Subtle.Render)
			}
		}
		if ctx.Advanced() {
			for _, s := range r.Sources {
				_, desc := sourceResultMark(t, s, true)
				lines = append(lines, indent+components.Truncate(t.Subtle.Render(s.Label+": ")+desc, max(w-4, 10)))
			}
			lines = append(lines, indent+t.Subtle.Render("Support data as of "+r.SupportDataAsOf))
		}
		selEnd = len(lines)
	}
	if len(r.Findings) == 0 {
		head += components.EmptyState{Title: "Nothing needs your attention", Message: "Here is everything ATLAS looked at:"}.Render(t, w) + "\n\n"
	}
	avail := ctx.Height() - strings.Count(head, "\n")
	return head + strings.Join(windowLines(lines, selStart, selEnd, avail), "\n")
}

var (
	_ Screen        = (*SmartAnalysis)(nil)
	_ SelfScrolling = (*SmartAnalysis)(nil)
)

// Dependencies shows each runtime, its support status and what needs it.
type Dependencies struct{ cursor int }

// NewDependencies creates the screen.
func NewDependencies() *Dependencies { return &Dependencies{} }

// SelfScrolling reports that the screen keeps the selection in view.
func (dp *Dependencies) SelfScrolling() bool { return true }

// Update moves the selection.
func (dp *Dependencies) Update(msg tea.Msg, ctx Context) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return dp, nil
	}
	n := 0
	if ctx.Data.Analysis != nil {
		n = len(ctx.Data.Analysis.Runtimes)
	}
	switch key.String() {
	case "a":
		return dp, requestAnalysis(ctx)
	case "s":
		if ctx.Data.Summary == nil {
			return dp, requestScan(ctx)
		}
	case "up", "k":
		dp.cursor--
	case "down", "j":
		dp.cursor++
	case "pgup", "ctrl+u":
		dp.cursor -= 5
	case "pgdown", "ctrl+d":
		dp.cursor += 5
	case "home", "g":
		dp.cursor = 0
	case "end", "G":
		dp.cursor = n - 1
	}
	dp.cursor = min(max(dp.cursor, 0), max(n-1, 0))
	return dp, nil
}

// Hints lists the keys.
func (dp *Dependencies) Hints(ctx Context) []components.KeyHint {
	if ctx.Data.Analysis == nil {
		if ctx.Data.Summary == nil {
			return []components.KeyHint{{Key: "s", Desc: "scan first"}}
		}
		return []components.KeyHint{{Key: "a", Desc: "analyze"}}
	}
	return []components.KeyHint{{Key: "↑/↓", Desc: "choose runtime"}, {Key: "a", Desc: "analyze again"}}
}

// View renders runtimes grouped by family and version.
func (dp *Dependencies) View(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	d := ctx.Data
	head := t.Title.Render("Dependencies") + "\n" +
		t.Muted.Render(components.Wrap("Runtimes are shared building blocks such as Python, Node.js, Java, .NET and Visual C++. Other software can stop working if one it needs is removed.", w)) + "\n\n"

	switch {
	case !d.AnalysisAvailable:
		return head + components.ErrorState{Title: "Dependencies are unavailable", Summary: "ATLAS couldn't open its saved data.", Details: d.StoreErr, ShowDetails: ctx.Advanced()}.Render(t, w)
	case !d.Loaded:
		return head + t.Muted.Render("Loading…")
	case d.Analysis == nil && d.Analyze != nil:
		return head + analyzeProgressView(ctx, *d.Analyze)
	case d.Analysis == nil && d.Summary == nil:
		return head + components.EmptyState{Title: "Scan your system first", Message: "ATLAS maps dependencies from the software a scan finds.", Hint: "Press s to scan."}.Render(t, w)
	case d.Analysis == nil:
		return head + components.EmptyState{
			Title:   "Dependencies haven't been mapped yet",
			Message: "Analyzing your software also maps which runtimes other software needs, using your package managers' own records.",
			Hint:    "Press a to analyze.",
		}.Render(t, w)
	case len(d.Analysis.Runtimes) == 0:
		return head + components.EmptyState{
			Title:   "No runtimes found",
			Message: "ATLAS looked for Python, Node.js, Java, .NET and Visual C++ in your saved software list and found none.",
		}.Render(t, w)
	}

	r := d.Analysis
	summary := fmt.Sprintf("%d runtimes found · analyzed %s", len(r.Runtimes), components.HumanTime(r.FinishedAt, ctx.Now))
	head += components.Truncate(t.Muted.Render(summary), w) + "\n"
	head += staleNotes(ctx, r) + "\n"

	dp.cursor = min(max(dp.cursor, 0), len(r.Runtimes)-1)
	var lines []string
	selStart, selEnd := 0, 0
	indent := "      "
	body := func(text string, style func(...string) string) {
		for _, l := range strings.Split(components.Wrap(text, max(w-6, 10)), "\n") {
			lines = append(lines, indent+style(l))
		}
	}
	group := ""
	groupStart := 0
	for i, rt := range r.Runtimes {
		if g := rt.Family + " " + rt.Branch; g != group {
			if group != "" {
				lines = append(lines, "")
			}
			group = g
			groupStart = len(lines)
			lines = append(lines, components.Truncate(t.Heading.Render(g)+t.Subtle.Render(" · ")+supportLabel(t, rt.Support), w))
		}
		needed := t.Muted.Render("not recorded")
		switch {
		case rt.DepsKnown && len(rt.Dependents) == 0 && rt.Indirect == 0:
			needed = t.Muted.Render("nothing else needs it")
		case rt.DepsKnown:
			needed = t.Body.Render(fmt.Sprintf("needed by %d", len(rt.Dependents)+rt.Indirect))
		}
		name := rt.Name
		if rt.Version != "" && !strings.Contains(rt.Name, rt.Version) {
			name += " " + rt.Version
		}
		prefix := "  "
		nameStyled := t.Body.Render(name)
		if i == dp.cursor {
			prefix = t.Accent.Render("› ")
			nameStyled = t.Accent.Bold(true).Render(name)
			// Keep the group heading (with its support status) in view.
			selStart = groupStart
		}
		lines = append(lines, components.Truncate(prefix+"  "+nameStyled+t.Subtle.Render(" · ")+needed, w))
		if i != dp.cursor {
			continue
		}
		switch {
		case !rt.DepsKnown && (rt.Source == "windows-registry" || rt.Source == "winget"):
			body("Windows doesn't record which programs use this, so ATLAS can't tell. Treat it as needed.", t.Muted.Render)
		case !rt.DepsKnown:
			body("Its package source doesn't record which programs use it, so ATLAS can't tell. Treat it as needed.", t.Muted.Render)
		case len(rt.Dependents) == 0 && rt.Indirect == 0:
			body("Nothing else installed lists it as a requirement. You may still use it directly.", t.Muted.Render)
		default:
			shown := rt.Dependents
			more := 0
			if len(shown) > 12 {
				shown, more = shown[:12], len(shown)-12
			}
			text := "Needed directly by: " + strings.Join(shown, ", ")
			if more > 0 {
				text += fmt.Sprintf(" and %d more", more)
			}
			body(text+".", t.Body.Render)
			if rt.Indirect > 0 {
				body(fmt.Sprintf("%d more packages need it through those.", rt.Indirect), t.Muted.Render)
			}
			body("Removing it would break these. Cleanup (6) checks this again and won't remove it while anything needs it.", t.Muted.Render)
		}
		if rt.Support.Note != "" {
			body(rt.Support.Note, t.Subtle.Render)
		}
		if rt.Support.DistroManaged && rt.Support.State == "ended" {
			body("Your Linux distribution may still provide security fixes for this package.", t.Subtle.Render)
		}
		if ctx.Advanced() {
			adv := []string{"Source: " + rt.Source}
			if rt.PackageID != "" {
				adv = append(adv, "Package ID: "+rt.PackageID)
			}
			if rt.DepsSource != "" {
				adv = append(adv, "Dependency data: "+rt.DepsSource)
			}
			if rt.Support.Reference != "" {
				adv = append(adv, "Support data: "+rt.Support.Reference)
			}
			for _, l := range adv {
				lines = append(lines, indent+components.Truncate(t.Subtle.Render(l), max(w-6, 10)))
			}
		}
		selEnd = len(lines)
	}
	avail := ctx.Height() - strings.Count(head, "\n")
	return head + strings.Join(windowLines(lines, selStart, selEnd, avail), "\n")
}

var (
	_ Screen        = (*Dependencies)(nil)
	_ SelfScrolling = (*Dependencies)(nil)
)

// staleNotes returns warning lines (each ending in a newline) about an
// analysis that is out of date or incomplete.
func staleNotes(ctx Context, r *core.AnalysisResult) string {
	t := ctx.Theme
	w := ctx.Width()
	var b strings.Builder
	if ctx.Data.AnalysisStale() {
		b.WriteString(t.Warning.Render(components.Wrap("Your software list changed after this analysis. Press a to analyze again.", w)) + "\n")
	}
	if r.Status == core.ScanPartial {
		b.WriteString(t.Warning.Render(components.Wrap("Some data couldn't be read; see What ATLAS checked at the end.", w)) + "\n")
	}
	return b.String()
}
