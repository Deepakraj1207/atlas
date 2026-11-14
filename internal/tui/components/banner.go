package components

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// bigLogo is ATLAS in ATLAS's own three-row block letters. It uses only
// the full and half block characters (█ ▀ ▄), which every Windows console
// font draws at single width.
var bigLogo = []string{
	"▄▀▀▄ ▀█▀ █    ▄▀▀▄ ▄▀▀▀",
	"█▀▀█  █  █    █▀▀█  ▀▀▄",
	"▀  ▀  ▀  ▀▀▀▀ ▀  ▀ ▀▀▀ ",
}

// BigLogoWidth is how many columns BigLogo needs.
var BigLogoWidth = lipgloss.Width(bigLogo[0])

// BigLogoHeight is how many rows BigLogo needs.
var BigLogoHeight = len(bigLogo)

// BigLogo renders the block-letter logo with a left-to-right gradient
// from the theme's accent to its info colour.
func BigLogo(t theme.Theme) string {
	from, okF := hexRGB(string(t.Palette.Accent))
	to, okT := hexRGB(string(t.Palette.Info))
	lines := make([]string, len(bigLogo))
	for y, row := range bigLogo {
		runes := []rune(row)
		var b strings.Builder
		for x, r := range runes {
			if r == ' ' || !okF || !okT {
				b.WriteString(t.Logo.Render(string(r)))
				continue
			}
			c := mix(from, to, float64(x)/float64(max(len(runes)-1, 1)))
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(c)).Render(string(r)))
		}
		lines[y] = b.String()
	}
	return strings.Join(lines, "\n")
}

func hexRGB(s string) ([3]float64, bool) {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return [3]float64{}, false
	}
	var out [3]float64
	for i := range 3 {
		v, err := strconv.ParseUint(s[i*2:i*2+2], 16, 8)
		if err != nil {
			return [3]float64{}, false
		}
		out[i] = float64(v)
	}
	return out, true
}

func mix(a, b [3]float64, f float64) string {
	ch := func(i int) int { return int(a[i] + (b[i]-a[i])*f + 0.5) }
	return fmt.Sprintf("#%02X%02X%02X", ch(0), ch(1), ch(2))
}
