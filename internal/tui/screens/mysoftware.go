package screens

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
)

// MySoftware lists the saved inventory with search and a kind filter.
type MySoftware struct {
	search    textinput.Model
	searching bool
	showAll   bool
	cursor    int
	offset    int
	detail    bool
}

// NewMySoftware creates the screen.
func NewMySoftware() *MySoftware {
	in := textinput.New()
	in.Prompt = ""
	in.Placeholder = "name, publisher or package ID"
	in.CharLimit = 80
	in.Cursor.SetMode(cursor.CursorStatic)
	return &MySoftware{search: in}
}

// SelfScrolling reports that the list handles its own scrolling.
func (s *MySoftware) SelfScrolling() bool { return true }

// CapturesInput is true while the search box has focus.
func (s *MySoftware) CapturesInput() bool { return s.searching }

// Query returns the current search text (for tests).
func (s *MySoftware) Query() string { return s.search.Value() }

// ShowAll reports whether system components are included (for tests).
func (s *MySoftware) ShowAll() bool { return s.showAll }

// DetailOpen reports whether the details pane is showing (for tests).
func (s *MySoftware) DetailOpen() bool { return s.detail }

// Visible returns the items that pass the current filters.
func (s *MySoftware) Visible(items []core.SoftwareItem) []core.SoftwareItem {
	terms := strings.Fields(strings.ToLower(s.search.Value()))
	var out []core.SoftwareItem
	for _, it := range items {
		if !s.showAll && it.Kind == core.KindComponent {
			continue
		}
		if matches(it, terms) {
			out = append(out, it)
		}
	}
	return out
}

func matches(it core.SoftwareItem, terms []string) bool {
	if len(terms) == 0 {
		return true
	}
	hay := strings.ToLower(it.Name + " " + it.Publisher + " " + it.PackageID + " " + it.Description)
	for _, term := range terms {
		if !strings.Contains(hay, term) {
			return false
		}
	}
	return true
}

func (s *MySoftware) listHeight(ctx Context) int {
	// title + summary + blank + (search) + header
	used := 4
	if s.searching || s.search.Value() != "" {
		used++
	}
	return max(ctx.Height()-used, 1)
}

func (s *MySoftware) clamp(n, height int) {
	s.cursor = min(max(s.cursor, 0), max(n-1, 0))
	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	if s.cursor >= s.offset+height {
		s.offset = s.cursor - height + 1
	}
	s.offset = min(max(s.offset, 0), max(n-height, 0))
}

// Update handles search, movement and details.
func (s *MySoftware) Update(msg tea.Msg, ctx Context) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	if s.searching {
		switch key.String() {
		case "esc":
			s.searching = false
			s.search.Blur()
			s.search.SetValue("")
		case "enter", "tab", "down":
			s.searching = false
			s.search.Blur()
		default:
			var cmd tea.Cmd
			s.search, cmd = s.search.Update(msg)
			s.cursor, s.offset = 0, 0
			return s, cmd
		}
		return s, nil
	}

	visible := s.Visible(ctx.Data.Items)
	if s.detail {
		switch key.String() {
		case "esc", "enter", "backspace", "q", "left", "h":
			s.detail = false
		case "r":
			if s.cursor < len(visible) {
				return s, send(RequestCleanupMsg{Items: []core.SoftwareItem{visible[s.cursor]}})
			}
		}
		return s, nil
	}

	h := s.listHeight(ctx)
	switch key.String() {
	case "up", "k":
		s.cursor--
	case "down", "j":
		s.cursor++
	case "pgup", "ctrl+u":
		s.cursor -= h
	case "pgdown", "ctrl+d":
		s.cursor += h
	case "home", "g":
		s.cursor = 0
	case "end", "G":
		s.cursor = len(visible) - 1
	case "enter", "right", "l":
		if len(visible) > 0 {
			s.detail = true
		}
	case "/":
		if ctx.Data.Summary != nil {
			s.searching = true
			return s, s.search.Focus()
		}
	case "f":
		s.showAll = !s.showAll
		s.cursor, s.offset = 0, 0
	case "esc":
		s.search.SetValue("")
		s.cursor, s.offset = 0, 0
	case "s":
		return s, requestScan(ctx)
	case "r":
		if s.cursor < len(visible) {
			return s, send(RequestCleanupMsg{Items: []core.SoftwareItem{visible[s.cursor]}})
		}
	}
	s.clamp(len(s.Visible(ctx.Data.Items)), h)
	return s, nil
}

// Hints lists My Software keys for the current state.
func (s *MySoftware) Hints(ctx Context) []components.KeyHint {
	switch {
	case s.searching:
		return []components.KeyHint{{Key: "enter", Desc: "done"}, {Key: "esc", Desc: "clear search"}}
	case s.detail:
		return []components.KeyHint{{Key: "esc", Desc: "back to list"}, {Key: "r", Desc: "remove…"}}
	case ctx.Data.Summary == nil:
		return []components.KeyHint{{Key: "s", Desc: "scan"}}
	}
	filter := "show system components"
	if s.showAll {
		filter = "apps only"
	}
	return []components.KeyHint{
		{Key: "↑/↓", Desc: "move"},
		{Key: "enter", Desc: "details"},
		{Key: "/", Desc: "search"},
		{Key: "f", Desc: filter},
		{Key: "r", Desc: "remove…"},
		{Key: "s", Desc: "scan again"},
	}
}

// View renders the list, details, or the appropriate empty/error state.
func (s *MySoftware) View(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	d := ctx.Data
	title := t.Title.Render("My Software")

	switch {
	case !d.Available:
		return title + "\n\n" + components.ErrorState{
			Title:       "Your software list can't be loaded",
			Summary:     "ATLAS couldn't open its saved data, so it can't show or save a software list right now. Everything else still works.",
			Details:     d.StoreErr,
			ShowDetails: ctx.Advanced(),
			Hint:        "Settings (9) shows where the data file lives.",
		}.Render(t, w)
	case !d.Loaded:
		return title + "\n\n" + t.Muted.Render("Loading your saved software list…")
	case d.LoadErr != "":
		return title + "\n\n" + components.ErrorState{
			Title:       "Your saved list couldn't be read",
			Summary:     "Running a new scan replaces it.",
			Details:     d.LoadErr,
			ShowDetails: ctx.Advanced(),
			Hint:        "Press s to scan again.",
		}.Render(t, w)
	case d.Summary == nil && d.Scan != nil:
		return title + "\n\n" + scanProgressView(ctx, *d.Scan)
	case d.Summary == nil:
		return title + "\n\n" + components.EmptyState{
			Title:   "You haven't scanned yet",
			Message: "ATLAS doesn't know what is installed until it scans. A scan only reads information and usually takes a few seconds.",
			Plan: []string{
				"Find installed applications and developer tools automatically",
				"Show where each one came from and which version you have",
				"Search and filter without needing to know package manager names",
			},
			Hint: "Press s to scan your system.",
		}.Render(t, w)
	}

	visible := s.Visible(d.Items)
	h := s.listHeight(ctx)
	s.clamp(len(visible), h)
	if s.detail && len(visible) > 0 {
		return components.Fit(s.detailView(ctx, visible[s.cursor]), w, ctx.Height())
	}

	var b strings.Builder
	b.WriteString(title + "\n")
	b.WriteString(components.Truncate(s.summaryLine(ctx, len(visible)), w) + "\n")
	if s.searching || s.search.Value() != "" {
		s.search.Width = max(w-10, 10)
		label := t.Muted.Render("Search: ")
		if s.searching {
			label = t.Accent.Render("Search: ")
		}
		b.WriteString(label + s.search.View() + "\n")
	}
	b.WriteString("\n")

	if len(visible) == 0 {
		b.WriteString(s.noMatches(ctx))
		return components.Fit(b.String(), w, ctx.Height())
	}

	end := min(s.offset+h, len(visible))
	rows := [][]string{s.headerCells(ctx)}
	for i := s.offset; i < end; i++ {
		rows = append(rows, s.cells(ctx, visible[i]))
	}
	cols := s.columns(ctx, rows)
	b.WriteString(s.row(ctx, cols, rows[0], false, true) + "\n")
	for i := s.offset; i < end; i++ {
		b.WriteString(s.row(ctx, cols, rows[i-s.offset+1], i == s.cursor, false))
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	return components.Fit(b.String(), w, ctx.Height())
}

func (s *MySoftware) summaryLine(ctx Context, shown int) string {
	t := ctx.Theme
	sum := ctx.Data.Summary
	apps, comps := core.CountKinds(ctx.Data.Items)
	var parts []string
	switch {
	case s.search.Value() != "":
		parts = append(parts, fmt.Sprintf("%d match", shown)+map[bool]string{true: "", false: "es"}[shown == 1])
	case s.showAll:
		parts = append(parts, fmt.Sprintf("%d apps and %d system components", apps, comps))
	default:
		parts = append(parts, fmt.Sprintf("%d apps", apps))
		if comps > 0 {
			parts = append(parts, fmt.Sprintf("%d system components hidden (f to show)", comps))
		}
	}
	parts = append(parts, "scanned "+components.HumanTime(sum.FinishedAt, ctx.Now))
	line := t.Muted.Render(strings.Join(parts, " · "))
	if sum.Status == core.ScanPartial {
		line += t.Warning.Render(" · some sources couldn't be read")
	}
	if ctx.Data.Scan != nil {
		line += t.Accent.Render(" · scanning " + ctx.Data.Scan.Spinner)
	}
	return line
}

func (s *MySoftware) noMatches(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	q := s.search.Value()
	_, comps := core.CountKinds(ctx.Data.Items)
	switch {
	case q != "" && !s.showAll && comps > 0:
		return components.EmptyState{
			Title:   "No apps match “" + q + "”",
			Message: "Press f to include system components in the search, or Esc to clear it.",
		}.Render(t, w)
	case q != "":
		return components.EmptyState{Title: "Nothing matches “" + q + "”", Message: "Press Esc to clear the search."}.Render(t, w)
	case len(ctx.Data.Items) == 0:
		lines := []string{}
		for _, src := range ctx.Data.Summary.Sources {
			_, desc := sourceResultMark(t, src, ctx.Advanced())
			lines = append(lines, src.Label+": "+desc)
		}
		return components.EmptyState{
			Title:   "The last scan didn't find any software",
			Message: "None of the sources ATLAS checked listed anything. Here is what each one reported:",
			Plan:    lines,
			Hint:    "Press s to scan again.",
		}.Render(t, w)
	default:
		return components.EmptyState{
			Title:   "No apps found",
			Message: "The last scan only found system components. Press f to show them.",
		}.Render(t, w)
	}
}

type column struct {
	width int
}

// widest returns the widest cell in a column, clamped to [lo, hi].
func widest(rows [][]string, col, lo, hi int) int {
	w := lo
	for _, r := range rows {
		if col < len(r) {
			w = max(w, lipgloss.Width(r[col]))
		}
	}
	return min(w, hi)
}

// columns sizes columns from the rows on screen so short versions don't
// waste space and long Debian versions aren't cut needlessly. The name
// column always keeps at least a third of the width; other columns drop
// on narrow terminals.
func (s *MySoftware) columns(ctx Context, rows [][]string) []column {
	w := ctx.Width() - 2 // selection marker
	verW := widest(rows, 1, 7, 24)
	if ctx.Advanced() {
		idW, srcW := widest(rows, 2, 10, 34), widest(rows, 3, 6, 16)
		if rest := w - verW - idW - srcW - 3; rest >= w/3 {
			return []column{{rest}, {verW}, {idW}, {srcW}}
		}
		if rest := w - verW - idW - 2; rest >= w/3 {
			return []column{{rest}, {verW}, {idW}}
		}
	} else {
		fromW := widest(rows, 2, 8, 28)
		if rest := w - verW - fromW - 2; rest >= w/3 {
			return []column{{rest}, {verW}, {fromW}}
		}
	}
	if rest := w - verW - 1; rest >= w/2 {
		return []column{{rest}, {verW}}
	}
	return []column{{w}}
}

func (s *MySoftware) headerCells(ctx Context) []string {
	if ctx.Advanced() {
		return []string{"Name", "Version", "Package ID", "Source"}
	}
	return []string{"Name", "Version", "Found by"}
}

func (s *MySoftware) cells(ctx Context, it core.SoftwareItem) []string {
	v := it.Version
	if v == "" {
		v = "–"
	}
	name := it.Name
	if it.Kind == core.KindComponent && s.showAll && !ctx.Advanced() {
		name += " (system)"
	}
	if ctx.Advanced() {
		id := it.PackageID
		if id == "" {
			id = "–"
		}
		return []string{name, v, id, it.Source}
	}
	from := ctx.SourceLabel(it.Source)
	if n := len(it.AlsoFoundIn); n > 0 {
		from += fmt.Sprintf(" +%d", n)
	}
	return []string{name, v, from}
}

func (s *MySoftware) row(ctx Context, cols []column, cells []string, selected, header bool) string {
	t := ctx.Theme
	parts := make([]string, 0, len(cols))
	for i, c := range cols {
		val := ""
		if i < len(cells) {
			val = cells[i]
		}
		parts = append(parts, components.PadRight(val, c.width))
	}
	line := strings.Join(parts, " ")
	switch {
	case header:
		return "  " + t.Subtle.Render(line)
	case selected:
		return t.Accent.Render("› ") + t.Accent.Bold(true).Render(line)
	default:
		return "  " + t.Body.Render(line)
	}
}

func (s *MySoftware) detailView(ctx Context, it core.SoftwareItem) string {
	t := ctx.Theme
	w := ctx.Width()
	var b strings.Builder
	b.WriteString(t.Title.Render(it.Name) + "\n")
	if it.Description != "" {
		b.WriteString(t.Muted.Render(components.Wrap(it.Description, w)) + "\n")
	}
	b.WriteString("\n")

	found := []string{ctx.SourceLabel(it.Source)}
	for _, src := range it.AlsoFoundIn {
		found = append(found, ctx.SourceLabel(src))
	}
	opt := func(v string) string {
		if v == "" {
			return t.Subtle.Render("unknown")
		}
		return v
	}
	rows := [][2]string{
		{"Version", opt(it.Version)},
		{"Publisher", opt(it.Publisher)},
		{"Type", kindLabel(it.Kind)},
		{"Found by", strings.Join(found, ", ")},
	}
	if it.InstallDate != "" {
		rows = append(rows, [2]string{"Installed", it.InstallDate})
	}
	if sc := scopeLabel(it.Scope); sc != "" {
		rows = append(rows, [2]string{"Installed for", sc})
	}
	if ctx.Advanced() {
		srcIDs := append([]string{it.Source}, it.AlsoFoundIn...)
		rows = append(rows,
			[2]string{"Package ID", opt(it.PackageID)},
			[2]string{"Location", opt(it.InstallLocation)},
			[2]string{"Source IDs", strings.Join(srcIDs, ", ")})
	}
	b.WriteString(components.LabelValue(t, rows))
	b.WriteString("\n\n")
	if it.Kind == core.KindComponent {
		b.WriteString(t.Muted.Render(components.Wrap("System components support other software. Removing them can break things, so ATLAS will warn you before any change in later phases.", w)) + "\n")
	}
	b.WriteString(t.Subtle.Render(components.Wrap("Press r to remove it (ATLAS checks what depends on it first and shows a preview), or Esc to go back. Updates are in Updates (3).", w)))
	return lipgloss.NewStyle().MaxWidth(w).Render(b.String())
}

var (
	_ Screen        = (*MySoftware)(nil)
	_ SelfScrolling = (*MySoftware)(nil)
	_ InputCapturer = (*MySoftware)(nil)
)
