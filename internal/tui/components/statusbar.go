package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// StatusKind colours a status message.
type StatusKind int

const (
	StatusInfo StatusKind = iota
	StatusSuccess
	StatusWarning
	StatusError
)

// Status is a transient message for the status bar.
type Status struct {
	Text string
	Kind StatusKind
}

// StatusBar renders the footer: a message row and, optionally, a hint row.
// right is drawn at the end of the message row (e.g. the mode badge).
func StatusBar(t theme.Theme, width int, st Status, right string, hints []KeyHint, showHints bool) string {
	var style lipgloss.Style
	switch st.Kind {
	case StatusSuccess:
		style = t.Success
	case StatusWarning:
		style = t.Warning
	case StatusError:
		style = t.Danger
	default:
		style = t.Status
	}
	rightW := lipgloss.Width(right)
	msgW := max(width-rightW-1, 1)
	msg := style.MaxWidth(msgW).Render(truncate(st.Text, msgW))
	gap := max(width-lipgloss.Width(msg)-rightW, 0)
	row := msg + strings.Repeat(" ", gap) + right

	if !showHints {
		return row
	}
	return row + "\n" + HintRow(t, width, hints)
}

func truncate(s string, w int) string {
	if w <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= w {
		return s
	}
	r := []rune(s)
	if w == 1 {
		return string(r[:1])
	}
	return string(r[:w-1]) + "…"
}
