package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
)

// Discover lists suggestions and the curated catalogue by category.
type Discover struct {
	cursor   int
	category int // 0 = all
	selected map[string]bool
}

// NewDiscover creates the screen.
func NewDiscover() *Discover { return &Discover{selected: map[string]bool{}} }

// SelfScrolling reports that the screen keeps the selection in view.
func (s *Discover) SelfScrolling() bool { return true }

// Selected returns selected entry IDs (for tests).
func (s *Discover) Selected() map[string]bool { return s.selected }

type discoverRow struct {
	item      core.DiscoverItem
	suggested bool
}

// rows returns suggestions first (in the "All" view), then entries by
// category. Suggested entries appear once.
func (s *Discover) rows(ctx Context) (rows []discoverRow, cats []string) {
	d := ctx.Data.Discovery
	if d == nil {
		return nil, nil
	}
	cats = append([]string{"All"}, d.Categories...)
	s.category = min(max(s.category, 0), len(cats)-1)
	want := cats[s.category]
	seen := map[string]bool{}
	if want == "All" {
		for _, it := range d.Items {
			if len(it.Because) > 0 && !ctx.Data.InstalledNow[it.Entry.ID] {
				rows = append(rows, discoverRow{item: it, suggested: true})
				seen[it.Entry.ID] = true
			}
		}
	}
	for _, c := range d.Categories {
		if want != "All" && c != want {
			continue
		}
		for _, it := range d.Items {
			if it.Entry.Category == c && !seen[it.Entry.ID] {
				rows = append(rows, discoverRow{item: it})
			}
		}
	}
	return rows, cats
}

func (s *Discover) installable(ctx Context, it core.DiscoverItem) bool {
	return !it.Installed && it.Via != "" && !ctx.Data.InstalledNow[it.Entry.ID]
}

// Update handles movement, selection, category and install.
func (s *Discover) Update(msg tea.Msg, ctx Context) (Screen, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return s, nil
	}
	rows, cats := s.rows(ctx)
	switch key.String() {
	case "up", "k":
		s.cursor--
	case "down", "j":
		s.cursor++
	case "pgup", "ctrl+u":
		s.cursor -= 5
	case "pgdown", "ctrl+d":
		s.cursor += 5
	case "home", "g":
		s.cursor = 0
	case "end", "G":
		s.cursor = len(rows) - 1
	case "f", "right", "l":
		if len(cats) > 0 {
			s.category = (s.category + 1) % len(cats)
			s.cursor = 0
		}
	case "left", "h":
		if len(cats) > 0 {
			s.category = (s.category + len(cats) - 1) % len(cats)
			s.cursor = 0
		}
	case "s":
		if ctx.Data.Summary == nil {
			return s, requestScan(ctx)
		}
	case " ", "x":
		if s.cursor < len(rows) && s.installable(ctx, rows[s.cursor].item) {
			id := rows[s.cursor].item.Entry.ID
			if s.selected[id] {
				delete(s.selected, id)
			} else {
				s.selected[id] = true
			}
		}
	case "enter":
		var ids []string
		for _, r := range rows {
			if s.selected[r.item.Entry.ID] {
				ids = append(ids, r.item.Entry.ID)
			}
		}
		if len(ids) == 0 && s.cursor < len(rows) {
			it := rows[s.cursor].item
			if !s.installable(ctx, it) {
				why := it.Unavailable
				if it.Installed || ctx.Data.InstalledNow[it.Entry.ID] {
					why = "It's already installed."
				}
				return s, send(StatusMsg{Status: components.Status{Text: it.Entry.Name + ": " + why, Kind: components.StatusWarning}})
			}
			ids = []string{it.Entry.ID}
		}
		if len(ids) > 0 {
			s.selected = map[string]bool{}
			return s, send(RequestInstallMsg{IDs: ids})
		}
	}
	s.cursor = min(max(s.cursor, 0), max(len(rows)-1, 0))
	return s, nil
}

// Hints lists the keys.
func (s *Discover) Hints(ctx Context) []components.KeyHint {
	enter := "install"
	if n := len(s.selected); n > 0 {
		enter = fmt.Sprintf("install selected (%d)", n)
	}
	return []components.KeyHint{{Key: "↑/↓", Desc: "choose"}, {Key: "enter", Desc: enter}, {Key: "space", Desc: "select"}, {Key: "←/→", Desc: "category"}}
}

// View renders the screen.
func (s *Discover) View(ctx Context) string {
	t := ctx.Theme
	w := ctx.Width()
	d := ctx.Data
	asOf := ""
	if d.Discovery != nil {
		asOf = " Catalogue as of " + d.Discovery.CatalogAsOf + "."
	}
	head := t.Title.Render("Discover Apps") + "\n" +
		t.Muted.Render(components.Wrap("Free, well-known tools picked for being useful and safe. Suggestions come from what you already have; ATLAS doesn't contact the internet to make them. Installing always shows a preview first."+asOf, w)) + "\n\n"
	switch {
	case !d.DiscoverAvailable:
		return head + components.ErrorState{Title: "Discover Apps is unavailable", Summary: "ATLAS couldn't open its saved data or its catalogue.", Details: d.StoreErr, ShowDetails: ctx.Advanced()}.Render(t, w)
	case d.Discovery == nil:
		return head + t.Muted.Render("Loading…")
	}
	rows, cats := s.rows(ctx)
	s.cursor = min(max(s.cursor, 0), max(len(rows)-1, 0))

	var tabs []string
	for i, c := range cats {
		if i == s.category {
			tabs = append(tabs, t.Accent.Bold(true).Render("["+c+"]"))
		} else {
			tabs = append(tabs, t.Muted.Render(c))
		}
	}
	head += components.Truncate(strings.Join(tabs, " "), w) + "\n"
	if !d.Discovery.Scanned {
		head += t.Warning.Render(components.Wrap("Scan your system first (press s) so ATLAS can tell what you already have and which package managers it can use.", w)) + "\n"
	}
	head += "\n"

	var lines []string
	selStart, selEnd := 0, 0
	indent := "      "
	body := func(text string, style func(...string) string) {
		for _, l := range strings.Split(components.Wrap(text, max(w-6, 10)), "\n") {
			lines = append(lines, indent+style(l))
		}
	}
	section := ""
	for i, r := range rows {
		heading := r.item.Entry.Category
		if r.suggested {
			heading = "Suggested for you"
		}
		if heading != section {
			if section != "" {
				lines = append(lines, "")
			}
			section = heading
			lines = append(lines, t.Heading.Render(heading))
		}
		it := r.item
		mark := "[ ] "
		status := ""
		switch {
		case it.Installed || d.InstalledNow[it.Entry.ID]:
			mark, status = " ✓  ", t.Success.Render(" · installed")
		case it.Via == "":
			mark, status = " –  ", t.Subtle.Render(" · not available here")
		case s.selected[it.Entry.ID]:
			mark = "[x] "
		}
		text := it.Entry.Name + " · " + it.Entry.Summary
		prefix, style := "  ", t.Body.Render
		if i == s.cursor {
			prefix, style = t.Accent.Render("› "), t.Accent.Bold(true).Render
			selStart = len(lines)
		}
		lines = append(lines, components.Truncate(prefix+style(mark+text)+status, w))
		if i != s.cursor {
			continue
		}
		if len(it.Because) > 0 && !it.Installed {
			body("Why it's suggested: "+strings.Join(it.Because, ", ")+".", t.Accent.Render)
		}
		body(it.Entry.Why, t.Muted.Render)
		switch {
		case it.Installed:
			body("Already installed as "+it.InstalledAs+".", t.Success.Render)
		case d.InstalledNow[it.Entry.ID]:
			body("Installed this session. Scan again to add it to My Software.", t.Success.Render)
		case it.Via == "":
			body(it.Unavailable, t.Subtle.Render)
		default:
			body("Installs with "+it.ViaLabel+". Press Enter to see exactly what will happen first.", t.Subtle.Render)
		}
		if ctx.Advanced() {
			if it.Via != "" {
				pkg := it.Package.Manager + ": " + it.Package.ID
				if it.Package.Classic {
					pkg += " (classic snap)"
				}
				body("Package: "+pkg, t.Subtle.Render)
			}
			body("Homepage: "+it.Entry.Homepage, t.Subtle.Render)
		}
		selEnd = len(lines)
	}
	if len(rows) == 0 {
		lines = append(lines, t.Muted.Render("Nothing in this category for this computer."))
	}
	lines = append(lines, "", t.Subtle.Render(components.Wrap("Suggestions never include things you already have.", w)))
	avail := ctx.Height() - strings.Count(head, "\n")
	return head + strings.Join(windowLines(lines, selStart, max(selEnd, selStart+1), avail), "\n")
}

var (
	_ Screen        = (*Discover)(nil)
	_ SelfScrolling = (*Discover)(nil)
)
