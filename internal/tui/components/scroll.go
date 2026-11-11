package components

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// Scroll shows a window of height lines from content starting at offset.
// The offset is clamped and returned. When content continues beyond the
// window, the first or last line is replaced by a plain "more" indicator so
// nothing is cut off silently.
func Scroll(t theme.Theme, content string, width, height, offset int) (string, int) {
	content = strings.TrimRight(content, "\n")
	lines := strings.Split(content, "\n")
	if height <= 0 {
		return "", 0
	}
	if len(lines) <= height {
		return content, 0
	}
	maxOffset := len(lines) - height
	offset = min(max(offset, 0), maxOffset)

	window := make([]string, height)
	copy(window, lines[offset:offset+height])
	indicator := func(text string) string {
		return lipgloss.NewStyle().MaxWidth(width).Render(t.Subtle.Render(text))
	}
	if offset > 0 {
		window[0] = indicator("↑ more above (PgUp)")
	}
	if offset < maxOffset {
		window[height-1] = indicator("↓ more below (PgDn)")
	}
	return strings.Join(window, "\n"), offset
}
