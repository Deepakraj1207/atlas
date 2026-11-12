package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
)

// Cleanup lists removal suggestions from Smart Analysis and things to tidy.
// Nothing is removed from here without a preview confirmed with r.
type Cleanup struct {
	cursor   int
	selected map[string]bool
}

// NewCleanup creates the screen.
func NewCleanup() *Cleanup { return &Cleanup{selected: map[string]bool{}} }

// SelfScrolling reports that the screen keeps the selection in view.
func (c *Cleanup) SelfScrolling() bool { return true }

// Selected returns the selected entry keys (for tests).
func (c *Cleanup) Selected() map[string]bool { return c.selected }

type cleanupEntry struct {
	key    string
	tidy   bool
	item   core.SoftwareItem
	reason string
	step   core.UpdateStep
}

// suggestionKinds are the Smart Analysis findings whose items can be removed.
var suggestionKinds = map[string]string{
	"duplicate": "installed more than once",
	"versions":  "one of several versions",
	"unneeded":  "nothing else needs it",
	"unused":    "",
}

func (c *Cleanup) entries(ctx Context) (suggestions, tidies []cleanupEntry) {
	d := ctx.Data
	if d.Analysis != nil {
		index := map[string]core.SoftwareItem{}
		for _, it := range d.Items {
			index[it.Source+"|"+it.Name+"|"+it.Version] = it
		}
		seen := map[string]bool{}
		for _, f := range d.Analysis.Findings {
			reason, ok := suggestionKinds[f.Kind]
			if !ok {
				continue
			}
			for _, ref := range f.Items {
				k := ref.Source + "|" + ref.Name + "|" + ref.Version
				it, ok := index[k]
				if !ok || seen[k] {
					continue // removed since, or listed twice
				}
				seen[k] = true
				r := reason
				if r == "" {
					r = ref.Note
				}
				suggestions = append(suggestions, cleanupEntry{key: "item:" + k, item: it, reason: r})
			}
		}
	}
	if d.Tidy != nil {
		for i, st := range d.Tidy.Steps {
			tidies = append(tidies, cleanupEntry{key: fmt.Sprintf("tidy:%d:%s:%s", i, st.Manager, st.Label), tidy: true, step: st})
		}
	}
	return suggestions, tidies
}

func (c *Cleanup) all(ctx Context) []cleanupEntry {
	s, t := c.entries(ctx)
	return append(s, t...)
}

func (c *Cleanup) request(entries []cleanupEntry) tea.Cmd {
	var msg RequestCleanupMsg
	for _, e := range entries {
		if e.tidy {
			msg.Tidy = append(msg.Tidy, e.step)
		} else {
			msg.Items = append(msg.Items, e.item)
		}
	}
	return send(msg)
}

// Update handles movement, selection and actions.
func (c *Cleanup) Update(msg tea.Msg, ctx Context) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return c, nil
	}
	all := c.all(ctx)
	live := map[string]bool{}
	for _, e := range all {
		live[e.key] = true
	}
	for k := range c.selected {
		if !live[k] {
			delete(c.selected, k)
		}
	}
	switch key.String() {
	case "c":
		return c, requestTidyFind(ctx)
	case "a":
		return c, requestAnalysis(ctx)
	case "up", "k":
		c.cursor--
	case "down", "j":
		c.cursor++
	case "home", "g":
		c.cursor = 0
	case "end", "G":
		c.cursor = len(all) - 1
	case " ", "x":
		if c.cursor < len(all) {
			k := all[c.cursor].key
			if c.selected[k] {
				delete(c.selected, k)
			} else {
				c.selected[k] = true
			}
		}
	case "enter":
		var chosen []cleanupEntry
		for _, e := range all {
			if c.selected[e.key] {
				chosen = append(chosen, e)
			}
		}
		if len(chosen) == 0 && c.cursor < len(all) {
			chosen = []cleanupEntry{all[c.cursor]}
		}
		if len(chosen) > 0 {
			return c, c.request(chosen)
		}
	}
	c.cursor = min(max(c.cursor, 0), max(len(all)-1, 0))
	return c, nil
}

// Hints lists the keys.
func (c *Cleanup) Hints(ctx Context) []components.KeyHint {
	h := []components.KeyHint{}
	if len(c.all(ctx)) > 0 {
		enter := "review this"
		if n := len(c.selected); n > 0 {
			enter = fmt.Sprintf("review selected (%d)", n)
		}
		h = append(h, components.KeyHint{Key: "space", Desc: "select"}, components.KeyHint{Key: "enter", Desc: enter})
	}
	return append(h, components.KeyHint{Key: "c", Desc: "look for things to tidy"}, components.KeyHint{Key: "a", Desc: "analyze"})
}

// View renders both sections.
func (c *Cleanup) View(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	d := ctx.Data
	head := t.Title.Render("Cleanup") + "\n" +
		t.Muted.Render(components.Wrap("Remove software you don't need and clear out caches. Nothing is removed until you confirm, and ATLAS checks what depends on what first.", w)) + "\n\n"

	switch {
	case !d.CleanupAvailable:
		return head + components.ErrorState{Title: "Cleanup is unavailable", Summary: "ATLAS couldn't open its saved data.", Details: d.StoreErr, ShowDetails: ctx.Advanced()}.Render(t, w)
	case !d.Loaded:
		return head + t.Muted.Render("Loading…")
	}

	suggestions, tidies := c.entries(ctx)
	all := append(append([]cleanupEntry{}, suggestions...), tidies...)
	c.cursor = min(max(c.cursor, 0), max(len(all)-1, 0))

	var lines []string
	selStart, selEnd := 0, 0
	indent := "        "
	body := func(text string, style func(...string) string) {
		for _, l := range strings.Split(components.Wrap(text, max(w-8, 10)), "\n") {
			lines = append(lines, indent+style(l))
		}
	}
	note := func(text string, style func(...string) string) {
		for _, l := range strings.Split(components.Wrap(text, max(w-2, 10)), "\n") {
			lines = append(lines, "  "+style(l))
		}
	}
	row := func(i int, e cleanupEntry, text string) {
		mark := "[ ] "
		if c.selected[e.key] {
			mark = "[x] "
		}
		prefix, style := "  ", t.Body.Render
		if i == c.cursor {
			prefix, style = t.Accent.Render("› "), t.Accent.Bold(true).Render
			selStart = len(lines)
		}
		lines = append(lines, components.Truncate(prefix+style(mark+text), w))
	}

	lines = append(lines, t.Heading.Render("Suggested by Smart Analysis"))
	switch {
	case d.Analysis == nil && d.Summary == nil:
		note("Scan your system and analyze it to get suggestions.", t.Muted.Render)
	case d.Analysis == nil:
		note("Analyze your software (press a) to get suggestions: duplicates, old versions, runtimes nothing needs and apps you don't open.", t.Muted.Render)
	case len(suggestions) == 0:
		note("The last analysis didn't suggest removing anything.", t.Muted.Render)
	}
	if d.AnalysisStale() && len(suggestions) > 0 {
		note("Your software list changed after the last analysis; press a to refresh these.", t.Warning.Render)
	}
	for i, e := range suggestions {
		text := e.item.Name
		if e.item.Version != "" {
			text += " " + e.item.Version
		}
		text += " · " + ctx.SourceLabel(e.item.Source)
		if e.reason != "" {
			text += " · " + e.reason
		}
		row(i, e, text)
		if i == c.cursor {
			body("Removing it is checked against what depends on it first, and the preview shows exactly what will happen.", t.Muted.Render)
			if e.item.Kind == core.KindComponent {
				body("This is a system component; ATLAS only removes it if nothing else needs it.", t.Subtle.Render)
			}
			selEnd = len(lines)
		}
	}

	lines = append(lines, "", t.Heading.Render("Things to tidy"))
	switch {
	case d.Tidying != nil:
		for _, l := range strings.Split(tidyProgressView(ctx, *d.Tidying), "\n") {
			lines = append(lines, "  "+l)
		}
	case d.Tidy == nil:
		note("Press c to look for package caches, old versions kept after updates, and packages nothing needs. Looking changes nothing.", t.Muted.Render)
	case len(tidies) == 0:
		note("Nothing to tidy (checked "+components.HumanTime(d.Tidy.FinishedAt, ctx.Now)+").", t.Muted.Render)
	default:
		note(fmt.Sprintf("Found %s · about %s · checked %s", countWord(len(tidies), "thing"), humanBytes(d.Tidy.Bytes()), components.HumanTime(d.Tidy.FinishedAt, ctx.Now)), t.Subtle.Render)
	}
	for j, e := range tidies {
		i := len(suggestions) + j
		text := e.step.Label
		if len(e.step.Updates) == 1 {
			text += " · " + e.step.Updates[0].Name
		} else {
			text += " · " + countWord(len(e.step.Updates), "item")
		}
		if e.step.Bytes > 0 {
			text += " · " + humanBytes(e.step.Bytes)
		}
		row(i, e, text)
		if i == c.cursor {
			const show = 6
			for k, u := range e.step.Updates {
				if k == show {
					body(fmt.Sprintf("…and %d more", len(e.step.Updates)-show), t.Subtle.Render)
					break
				}
				body("• "+strings.TrimSpace(u.Name+" "+u.CurrentVersion), t.Body.Render)
			}
			if e.step.Rollback != "" {
				body(e.step.Rollback+".", t.Muted.Render)
			}
			if e.step.Blocked != "" {
				body(e.step.Blocked, t.Warning.Render)
			}
			selEnd = len(lines)
		}
	}
	if len(all) == 0 {
		selStart, selEnd = 0, len(lines)
	}
	avail := ctx.Height() - strings.Count(head, "\n")
	return head + strings.Join(windowLines(lines, selStart, max(selEnd, selStart+1), avail), "\n")
}

var (
	_ Screen        = (*Cleanup)(nil)
	_ SelfScrolling = (*Cleanup)(nil)
)
