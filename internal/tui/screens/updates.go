package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
)

// Updates lists available updates and lets the user install them after a
// preview. Held updates are shown but never selected.
type Updates struct {
	cursor         int
	offset         int
	selected       map[int64]bool
	showComponents bool
}

// NewUpdates creates the screen.
func NewUpdates() *Updates { return &Updates{selected: map[int64]bool{}} }

// SelfScrolling reports that the list handles its own scrolling.
func (u *Updates) SelfScrolling() bool { return true }

// Selected returns the selected update IDs (for tests).
func (u *Updates) Selected() map[int64]bool { return u.selected }

// visible returns the updates on screen: apps, plus components on request.
func (u *Updates) visible(all []core.Update) []core.Update {
	var out []core.Update
	for _, up := range all {
		if up.Component && !u.showComponents {
			continue
		}
		out = append(out, up)
	}
	return out
}

func (u *Updates) pruneSelection(all []core.Update) {
	live := map[int64]bool{}
	for _, up := range all {
		live[up.ID] = true
	}
	for id := range u.selected {
		if !live[id] {
			delete(u.selected, id)
		}
	}
}

func (u *Updates) chosen(all []core.Update) []core.Update {
	var out []core.Update
	for _, up := range all {
		if u.selected[up.ID] && !up.Held {
			out = append(out, up)
		}
	}
	return out
}

func allPending(all []core.Update) []core.Update {
	var out []core.Update
	for _, up := range all {
		if !up.Held {
			out = append(out, up)
		}
	}
	return out
}

func (u *Updates) listHeight(ctx Context) int { return max(ctx.Height()-5, 1) }

func (u *Updates) clamp(n, h int) {
	u.cursor = min(max(u.cursor, 0), max(n-1, 0))
	if u.cursor < u.offset {
		u.offset = u.cursor
	}
	if u.cursor >= u.offset+h {
		u.offset = u.cursor - h + 1
	}
	u.offset = min(max(u.offset, 0), max(n-h, 0))
}

// Update handles movement, selection and actions.
func (u *Updates) Update(msg tea.Msg, ctx Context) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return u, nil
	}
	all := ctx.Data.Updates
	u.pruneSelection(all)
	vis := u.visible(all)
	h := u.listHeight(ctx)
	switch key.String() {
	case "c":
		return u, requestCheck(ctx)
	case "up", "k":
		u.cursor--
	case "down", "j":
		u.cursor++
	case "pgup", "ctrl+u":
		u.cursor -= h
	case "pgdown", "ctrl+d":
		u.cursor += h
	case "home", "g":
		u.cursor = 0
	case "end", "G":
		u.cursor = len(vis) - 1
	case " ", "x":
		if u.cursor < len(vis) && !vis[u.cursor].Held {
			id := vis[u.cursor].ID
			u.selected[id] = !u.selected[id]
			if !u.selected[id] {
				delete(u.selected, id)
			}
		}
	case "a":
		if len(u.selected) > 0 {
			u.selected = map[int64]bool{}
		} else {
			for _, up := range vis {
				if !up.Held {
					u.selected[up.ID] = true
				}
			}
		}
	case "f":
		u.showComponents = !u.showComponents
		u.cursor, u.offset = 0, 0
	case "u":
		if p := allPending(all); len(p) > 0 {
			return u, send(RequestApplyMsg{Updates: p})
		}
	case "enter":
		if c := u.chosen(all); len(c) > 0 {
			return u, send(RequestApplyMsg{Updates: c})
		}
		if u.cursor < len(vis) && !vis[u.cursor].Held {
			return u, send(RequestApplyMsg{Updates: []core.Update{vis[u.cursor]}})
		}
		if u.cursor < len(vis) {
			return u, send(StatusMsg{Status: components.Status{Text: vis[u.cursor].Name + " is held back: " + vis[u.cursor].Note + ".", Kind: components.StatusWarning}})
		}
	}
	u.clamp(len(u.visible(all)), h)
	return u, nil
}

// Hints lists the Updates keys for the current state.
func (u *Updates) Hints(ctx Context) []components.KeyHint {
	if ctx.Data.UpdateSummary == nil || len(ctx.Data.Updates) == 0 {
		return []components.KeyHint{{Key: "c", Desc: "check for updates"}}
	}
	enter := "update this"
	if len(u.selected) > 0 {
		enter = fmt.Sprintf("update selected (%d)", len(u.selected))
	}
	return []components.KeyHint{
		{Key: "u", Desc: "update all"},
		{Key: "space", Desc: "select"},
		{Key: "enter", Desc: enter},
		{Key: "a", Desc: "select all/none"},
		{Key: "f", Desc: "system components"},
		{Key: "c", Desc: "check again"},
	}
}

// View renders the update list or the appropriate state.
func (u *Updates) View(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	d := ctx.Data
	title := t.Title.Render("Updates")

	switch {
	case !d.UpdatesAvailable:
		return title + "\n\n" + components.ErrorState{
			Title:       "Updates can't be checked right now",
			Summary:     "ATLAS couldn't open its saved data, so it can't check for or keep track of updates. Everything else still works.",
			Details:     d.StoreErr,
			ShowDetails: ctx.Advanced(),
		}.Render(t, w)
	case !d.Loaded:
		return title + "\n\n" + t.Muted.Render("Loading…")
	case d.UpdateSummary == nil && d.Check != nil:
		return title + "\n\n" + checkProgressView(ctx, *d.Check)
	case d.UpdateSummary == nil && d.FailedCheck != nil:
		return title + "\n\n" + failedCheckView(ctx, *d.FailedCheck)
	case d.UpdateSummary == nil:
		return title + "\n\n" + components.EmptyState{
			Title:   "You haven't checked for updates yet",
			Message: "ATLAS asks each package manager on this computer whether newer versions exist. Checking only reads information.",
			Plan: []string{
				"See every available update in one list, whichever tool installed the software",
				"Update everything at once, or pick what to update",
				"See exactly what will run, and how to go back, before anything changes",
			},
			Hint: "Press c to check for updates.",
		}.Render(t, w)
	}

	u.pruneSelection(d.Updates)
	vis := u.visible(d.Updates)
	h := u.listHeight(ctx)
	u.clamp(len(vis), h)
	apps, comps := d.PendingUpdates()

	var b strings.Builder
	b.WriteString(title + "\n")
	b.WriteString(components.Truncate(u.summaryLine(ctx, apps, comps), w) + "\n")
	if apps+comps > 0 {
		b.WriteString(components.Truncate(t.Accent.Render(updateAllHint(apps, comps)), w) + "\n\n")
	} else {
		b.WriteString("\n")
	}

	if len(d.Updates) == 0 {
		b.WriteString(u.upToDate(ctx))
		return components.Fit(b.String(), w, ctx.Height())
	}
	if len(vis) == 0 {
		b.WriteString(components.EmptyState{
			Title:   "No app updates",
			Message: "Only system components have updates. Press f to show them; u updates them too.",
		}.Render(t, w))
		return components.Fit(b.String(), w, ctx.Height())
	}

	rows := [][]string{u.header(ctx)}
	end := min(u.offset+h, len(vis))
	for i := u.offset; i < end; i++ {
		rows = append(rows, u.cells(ctx, vis[i]))
	}
	cols := u.columns(ctx, rows)
	b.WriteString("      " + t.Subtle.Render(joinCells(rows[0], cols)) + "\n")
	for i := u.offset; i < end; i++ {
		up := vis[i]
		mark := "[ ] "
		switch {
		case up.Held:
			mark = " –  "
		case u.selected[up.ID]:
			mark = "[x] "
		}
		line := mark + joinCells(rows[i-u.offset+1], cols)
		switch {
		case i == u.cursor:
			b.WriteString(t.Accent.Render("› ") + t.Accent.Bold(true).Render(line))
		case up.Held:
			b.WriteString("  " + t.Subtle.Render(line))
		default:
			b.WriteString("  " + t.Body.Render(line))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	return components.Fit(b.String(), w, ctx.Height())
}

func (u *Updates) summaryLine(ctx Context, apps, comps int) string {
	t := ctx.Theme
	d := ctx.Data
	var parts []string
	if apps+comps == 0 {
		parts = append(parts, "No updates waiting")
	} else {
		parts = append(parts, countWord(apps, "app update"))
	}
	parts = append(parts, "checked "+components.HumanTime(d.UpdateSummary.FinishedAt, ctx.Now))
	if comps > 0 {
		hint := " (f to show)"
		if u.showComponents {
			hint = ""
		}
		parts = append(parts, countWord(comps, "system component update")+hint)
	}
	held := 0
	for _, up := range d.Updates {
		if up.Held {
			held++
		}
	}
	if held > 0 {
		parts = append(parts, fmt.Sprintf("%d held back", held))
	}
	line := t.Muted.Render(strings.Join(parts, " · "))
	if d.UpdateSummary.Status == core.ScanPartial {
		line += t.Warning.Render(" · some package managers couldn't be checked")
	}
	if f := d.FailedCheck; f != nil && f.FinishedAt.After(d.UpdateSummary.FinishedAt) {
		line += t.Warning.Render(" · your latest check didn't work; showing earlier results")
	}
	if d.Check != nil {
		line += t.Accent.Render(" · checking " + d.Check.Spinner)
	}
	return line
}

func (u *Updates) upToDate(ctx Context) string {
	t := ctx.Theme
	var lines []string
	for _, src := range ctx.Data.UpdateSummary.Sources {
		_, desc := sourceResultMark(t, src, ctx.Advanced())
		if src.Status == core.SourceOK {
			desc = t.Body.Render("up to date")
		}
		lines = append(lines, src.Label+": "+desc)
	}
	title := "Everything checked is up to date"
	msg := "Here is what each package manager reported:"
	if s := ctx.Data.UpdateSummary.Status; s == core.ScanPartial || s == core.ScanFailed {
		title = "No updates found, but not everything could be checked"
		msg = "Some package managers couldn't answer, so there may still be updates:"
	}
	return components.EmptyState{
		Title:   title,
		Message: msg,
		Plan:    lines,
		Hint:    "Press c to check again.",
	}.Render(t, ctx.Width())
}

func (u *Updates) header(ctx Context) []string {
	if ctx.Advanced() {
		return []string{"Name", "Installed", "New", "Package ID", "Manager"}
	}
	return []string{"Name", "Installed", "New", "From"}
}

func (u *Updates) cells(ctx Context, up core.Update) []string {
	cur := up.CurrentVersion
	if cur == "" {
		cur = "–"
	}
	name := up.Name
	if up.Held {
		name += " (held)"
	} else if up.Component && !ctx.Advanced() {
		name += " (system)"
	}
	if ctx.Advanced() {
		return []string{name, cur, up.AvailableVersion, up.PackageID, up.Manager}
	}
	return []string{name, cur, up.AvailableVersion, ctx.ManagerLabel(up.Manager)}
}

// columns sizes the version and source columns from the data on screen and
// gives the rest to the name, dropping columns on narrow terminals.
func (u *Updates) columns(ctx Context, rows [][]string) []int {
	w := ctx.Width() - 2 - 4 // selection marker and checkbox
	cur, nw := widest(rows, 1, 9, 22), widest(rows, 2, 3, 22)
	if ctx.Advanced() {
		id, mgr := widest(rows, 3, 10, 30), widest(rows, 4, 7, 12)
		if rest := w - cur - nw - id - mgr - 4; rest >= w/4 {
			return []int{rest, cur, nw, id, mgr}
		}
	} else {
		from := widest(rows, 3, 4, 28)
		if rest := w - cur - nw - from - 3; rest >= w/4 {
			return []int{rest, cur, nw, from}
		}
	}
	if rest := w - cur - nw - 2; rest >= w/4 {
		return []int{rest, cur, nw}
	}
	if rest := w - nw - 1; rest >= w/3 {
		return []int{rest, 0, nw}
	}
	return []int{w}
}

func joinCells(cells []string, widths []int) string {
	var parts []string
	for i, wd := range widths {
		if wd == 0 {
			continue
		}
		val := ""
		if i < len(cells) {
			val = cells[i]
		}
		parts = append(parts, components.PadRight(val, wd))
	}
	return strings.Join(parts, " ")
}

var (
	_ Screen        = (*Updates)(nil)
	_ SelfScrolling = (*Updates)(nil)
)

// failedCheckView explains a check that produced no results.
func failedCheckView(ctx Context, c core.UpdateCheck) string {
	t := ctx.Theme
	var lines []string
	for _, src := range c.Sources {
		_, desc := sourceResultMark(t, src, ctx.Advanced())
		if src.Status == core.SourceFailed && src.Details != "" && !ctx.Advanced() {
			desc += t.Muted.Render(". To fix: " + src.Details)
		}
		lines = append(lines, src.Label+": "+desc)
	}
	title := "The update check didn't work"
	summary := "None of the package managers could answer, so ATLAS can't tell whether updates are available. Nothing was changed."
	if c.Status == core.ScanNothingToScan {
		title = "No package manager to check"
		summary = "ATLAS didn't find a package manager it supports on this computer."
	}
	return components.ErrorState{Title: title, Summary: summary}.Render(t, ctx.Width()) + "\n\n" +
		components.EmptyState{Title: "What each package manager said", Plan: lines, Hint: "Press c to try again."}.Render(t, ctx.Width())
}

func countWord(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// updateAllHint says exactly what u includes: system components are hidden
// from the list by default but are updated with everything else.
func updateAllHint(apps, comps int) string {
	switch {
	case comps == 0:
		return "Press u to update all " + components.Plural(apps, "app") + "."
	case apps == 0:
		return "Press u to update all " + components.Plural(comps, "system component") + "."
	default:
		return fmt.Sprintf("Press u to update all %d: %s and %s.", apps+comps, components.Plural(apps, "app"), components.Plural(comps, "system component"))
	}
}
