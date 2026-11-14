package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// SectionLabel renders a small, quiet section label such as "System".
func SectionLabel(t theme.Theme, text string) string {
	return t.Label.Render(text)
}

// ActionItem is one entry of an ActionList.
type ActionItem struct {
	Label string
	// Desc is a short, dim explanation shown beside the label when there
	// is room.
	Desc string
}

// ActionList renders a vertical menu with the cursor row marked "›". When
// focused, the cursor row is highlighted across the list's width.
// Descriptions line up in a second column and are left out on narrow
// terminals rather than wrapped.
func ActionList(t theme.Theme, width int, items []ActionItem, cursor int, focused bool) string {
	labelW := 0
	for _, it := range items {
		labelW = max(labelW, lipgloss.Width(it.Label))
	}
	descW := width - 4 - labelW - 3
	showDesc := descW >= 24
	rowW := width
	if showDesc {
		longest := 0
		for _, it := range items {
			longest = max(longest, lipgloss.Width(it.Desc))
		}
		rowW = min(width, 4+labelW+3+min(longest, descW)+1)
	}
	lines := make([]string, 0, len(items))
	for i, it := range items {
		label := PadRight(it.Label, labelW)
		desc := ""
		if showDesc && it.Desc != "" {
			desc = "   " + Truncate(it.Desc, descW)
		}
		if i == cursor {
			if focused {
				lines = append(lines, t.Selected.Render(PadRight(" › "+label+desc, rowW)))
			} else {
				lines = append(lines, t.NavCur.Render(" › "+label)+t.Subtle.Render(desc))
			}
			continue
		}
		lines = append(lines, "   "+t.Body.Render(label)+t.Subtle.Render(desc))
	}
	// Without room for a second column, describe just the highlighted
	// action under the list.
	if !showDesc && cursor >= 0 && cursor < len(items) && items[cursor].Desc != "" {
		desc := Wrap(items[cursor].Desc, max(width-3, 8))
		lines = append(lines, "", "   "+t.Subtle.Render(strings.ReplaceAll(desc, "\n", "\n   ")))
	}
	return strings.Join(lines, "\n")
}

// Stat is one figure in a Stats row, e.g. "87" "apps".
type Stat struct {
	Value string
	Label string
	// Tone colours the value; neutral values are plain bold text.
	Tone theme.Tone
	// Dim marks a value that isn't known yet ("—").
	Dim bool
}

// Stats renders figures side by side ("87 apps    3 updates"), wrapping onto
// more lines when the width runs out.
func Stats(t theme.Theme, width int, stats []Stat) string {
	var rows []string
	var row []string
	used := 0
	for _, s := range stats {
		val := t.Strong.Render(s.Value)
		switch {
		case s.Dim:
			val = t.Subtle.Render(s.Value)
		case s.Tone != theme.ToneNeutral:
			val = t.ToneStyle(s.Tone).Bold(true).Render(s.Value)
		}
		cell := val + " " + t.Muted.Render(s.Label)
		w := lipgloss.Width(cell)
		if len(row) > 0 && used+3+w > width {
			rows = append(rows, strings.Join(row, "   "))
			row, used = nil, 0
		}
		if len(row) > 0 {
			used += 3
		}
		row = append(row, cell)
		used += w
	}
	if len(row) > 0 {
		rows = append(rows, strings.Join(row, "   "))
	}
	return strings.Join(rows, "\n")
}
