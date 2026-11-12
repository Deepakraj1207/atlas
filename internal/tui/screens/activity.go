package screens

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/tui/components"
)

// ActivityHistory lists everything ATLAS has done, newest first.
type ActivityHistory struct {
	cursor int
}

// NewActivityHistory creates the screen.
func NewActivityHistory() *ActivityHistory { return &ActivityHistory{} }

// SelfScrolling reports that the screen keeps the selection in view itself.
func (a *ActivityHistory) SelfScrolling() bool { return true }

// Cursor returns the selected entry (for tests).
func (a *ActivityHistory) Cursor() int { return a.cursor }

// Update moves the selection.
func (a *ActivityHistory) Update(msg tea.Msg, ctx Context) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return a, nil
	}
	n := len(ctx.Data.Activity)
	switch key.String() {
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

// Hints lists Activity History keys.
func (a *ActivityHistory) Hints(ctx Context) []components.KeyHint {
	if len(ctx.Data.Activity) == 0 {
		return nil
	}
	return []components.KeyHint{{Key: "↑/↓", Desc: "choose entry"}}
}

// View renders the history.
func (a *ActivityHistory) View(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	d := ctx.Data
	head := t.Title.Render("Activity History") + "\n" +
		t.Muted.Render(components.Wrap("Everything ATLAS has done on this computer, newest first. Nothing happens silently.", w)) + "\n\n"

	switch {
	case !d.Available:
		return head + components.ErrorState{
			Title:       "History can't be loaded",
			Summary:     "ATLAS couldn't open its saved data, so it can't show or record activity right now.",
			Details:     d.StoreErr,
			ShowDetails: ctx.Advanced(),
		}.Render(t, w)
	case !d.Loaded:
		return head + t.Muted.Render("Loading…")
	case d.LoadErr != "":
		return head + components.ErrorState{
			Title: "History couldn't be read", Details: d.LoadErr, ShowDetails: ctx.Advanced(),
		}.Render(t, w)
	case len(d.Activity) == 0:
		return head + components.EmptyState{
			Title:   "Nothing has happened yet",
			Message: "When ATLAS scans, or changes anything in later phases, each step is recorded here with its outcome.",
		}.Render(t, w)
	}

	a.cursor = min(max(a.cursor, 0), len(d.Activity)-1)
	whenW := 0
	for _, e := range d.Activity {
		whenW = max(whenW, len(components.HumanTime(e.At, ctx.Now)))
	}

	var lines []string
	selStart, selEnd := 0, 0
	for i, e := range d.Activity {
		when := components.PadRight(components.HumanTime(e.At, ctx.Now), whenW)
		marker := "  "
		title := t.Body.Render(e.Title)
		if i == a.cursor {
			marker = t.Accent.Render("› ")
			title = t.Accent.Bold(true).Render(e.Title)
			selStart = len(lines)
		}
		lines = append(lines, components.Truncate(marker+t.Subtle.Render(when)+"  "+components.PadRight(outcomeLabel(t, e.Outcome), 11)+" "+title, w))
		if i == a.cursor {
			indent := strings.Repeat(" ", 4)
			body := components.Wrap(e.Summary, max(w-4, 10))
			for _, l := range strings.Split(body, "\n") {
				lines = append(lines, indent+t.Muted.Render(l))
			}
			if ctx.Advanced() {
				lines = append(lines, indent+t.Subtle.Render(components.Timestamp(e.At)+" · "+e.Kind))
				for _, l := range strings.Split(e.Details, "\n") {
					if l != "" {
						lines = append(lines, indent+t.Subtle.Render(components.Truncate(l, max(w-4, 10))))
					}
				}
			}
			selEnd = len(lines)
		}
	}

	avail := max(ctx.Height()-strings.Count(head, "\n"), 1)
	offset := 0
	if selEnd > avail {
		offset = min(selStart, selEnd-avail)
	}
	end := min(offset+avail, len(lines))
	return head + strings.Join(lines[offset:end], "\n")
}

var (
	_ Screen        = (*ActivityHistory)(nil)
	_ SelfScrolling = (*ActivityHistory)(nil)
)
