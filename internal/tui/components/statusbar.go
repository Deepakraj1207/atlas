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
	// A message too long for one line takes the hint line as well for as
	// long as it shows: reading it in full matters more than the hints, and
	// "Nothing was sent" must never be the part that gets cut off.
	if showHints && lipgloss.Width(dot)+lipgloss.Width(st.Text) > width {
		lines := wrapTo(st.Text, max(width-2, 1), 2)
		return dot + style.Render(lines[0]) + "\n  " + style.Render(lines[1])
	}
	rightW := lipgloss.Width(right)
	// The message matters more than the facts: they give way whenever the
	// message wouldn't fit in full beside them.
	if rightW > 0 && lipgloss.Width(dot)+lipgloss.Width(st.Text)+2+rightW > width {
		right, rightW = "", 0
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

// wrapTo wraps s to width and returns exactly n lines: missing lines are
// empty, and text beyond the last line is cut with an ellipsis.
func wrapTo(s string, width, n int) []string {
	var lines []string
	for _, l := range strings.Split(Wrap(s, width), "\n") {
		lines = append(lines, strings.TrimRight(l, " "))
	}
	for len(lines) < n {
		lines = append(lines, "")
	}
	if len(lines) > n {
		lines[n-1] = truncate(strings.Join(lines[n-1:], " "), width)
		lines = lines[:n]
	}
	return lines
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
