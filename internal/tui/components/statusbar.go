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
	// StatusBusy is a task in progress.
	StatusBusy
	// StatusReady is the resting state: nothing to report.
	StatusReady
)

// Status is a transient message for the status bar.
type Status struct {
	Text string
	Kind StatusKind
}

// Tone maps a status kind to an indicator tone.
func (s Status) Tone() theme.Tone {
	switch s.Kind {
	case StatusSuccess, StatusReady:
		return theme.ToneOK
	case StatusWarning:
		return theme.ToneWarn
	case StatusError:
		return theme.ToneBad
	case StatusBusy:
		return theme.ToneBusy
	default:
		return theme.ToneNeutral
	}
}

// StatusBar renders the footer without a box: a "● message" line with
// facts on the right (e.g. "87 apps · 3 updates"), and, optionally, a quiet
// line of key hints.
func StatusBar(t theme.Theme, width int, st Status, right string, hints []KeyHint, showHints bool) string {
	style := t.Status
	switch st.Kind {
	case StatusSuccess:
		style = t.Success
	case StatusWarning:
		style = t.Warning
	case StatusError:
		style = t.Danger
	case StatusBusy:
		style = t.Info
	}
	dot := t.Dot(st.Tone()) + " "
	rightW := lipgloss.Width(right)
	if rightW > width/2 {
		right, rightW = "", 0 // a long message matters more than the facts
	}
	msgW := max(width-rightW-lipgloss.Width(dot)-1, 1)
	msg := dot + style.Render(truncate(st.Text, msgW))
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
