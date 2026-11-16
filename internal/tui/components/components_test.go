package components

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/tui/layout"
	"github.com/deepakraj1207/atlas/internal/tui/nav"
	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

func TestChooseTabModeDegradesWithWidth(t *testing.T) {
	if got := ChooseTabMode(200, nav.Home, false); got != TabFull {
		t.Errorf("200 cols: %v, want full", got)
	}
	if got := ChooseTabMode(200, nav.Home, true); got != TabShort {
		t.Errorf("preferShort should pick short when it fits, got %v", got)
	}
	if got := ChooseTabMode(80, nav.Home, false); got == TabFull {
		t.Error("80 cols cannot fit full names for nine sections")
	}
	if got := ChooseTabMode(40, nav.Home, true); got != TabCompact {
		t.Errorf("40 cols: %v, want compact", got)
	}
}

func TestTabBarAlwaysShowsEveryShortcut(t *testing.T) {
	th := theme.ByName("dark")
	for _, w := range []int{40, 60, 80, 100} {
		bar := TabBar(th, nav.Settings, w, w < 80)
		for _, s := range nav.All() {
			if !strings.Contains(bar, s.Shortcut()) {
				t.Errorf("width %d: shortcut %s missing from tab bar", w, s.Shortcut())
			}
		}
		if !strings.Contains(bar, "Settings") {
			t.Errorf("width %d: current section name missing", w)
		}
	}
}

func TestScrollClampsAndMarksOverflow(t *testing.T) {
	th := theme.ByName("dark")
	var lines []string
	for i := 1; i <= 30; i++ {
		lines = append(lines, "line")
	}
	content := strings.Join(lines, "\n")

	out, off := Scroll(th, content, 40, 10, 0)
	if off != 0 || strings.Count(out, "\n") != 9 {
		t.Errorf("window should be 10 lines at offset 0, got offset %d, %d lines", off, strings.Count(out, "\n")+1)
	}
	if !strings.Contains(out, "more below") || strings.Contains(out, "more above") {
		t.Error("top of long content should mark only 'more below'")
	}

	out, off = Scroll(th, content, 40, 10, 999)
	if off != 20 {
		t.Errorf("offset should clamp to 20, got %d", off)
	}
	if !strings.Contains(out, "more above") || strings.Contains(out, "more below") {
		t.Error("bottom of long content should mark only 'more above'")
	}

	out, off = Scroll(th, "short", 40, 10, 5)
	if off != 0 || out != "short" {
		t.Errorf("short content should be returned unchanged at offset 0, got %q/%d", out, off)
	}
}

func TestStatesRenderPlainLanguage(t *testing.T) {
	th := theme.ByName("light")
	e := EmptyState{Title: "Nothing here", Message: "Come back later.", Plan: []string{"Do a thing"}, Hint: "Arrives in Phase 2"}.Render(th, 40)
	for _, want := range []string{"Nothing here", "Come back later.", "What this will do", "Do a thing", "Arrives in Phase 2"} {
		if !strings.Contains(e, want) {
			t.Errorf("empty state missing %q", want)
		}
	}
	simple := ErrorState{Title: "Oops", Summary: "It broke.", Details: "stack trace"}.Render(th, 40)
	if strings.Contains(simple, "stack trace") || !strings.Contains(simple, "Advanced mode") {
		t.Error("details must be hidden in Simple mode with a pointer to Advanced")
	}
	adv := ErrorState{Title: "Oops", Summary: "It broke.", Details: "stack trace", ShowDetails: true}.Render(th, 40)
	if !strings.Contains(adv, "stack trace") {
		t.Error("details must be shown when requested")
	}
}

func TestHumanTime(t *testing.T) {
	now := time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		ago  time.Duration
		want string
	}{
		{10 * time.Second, "just now"},
		{time.Minute, "1 minute ago"},
		{45 * time.Minute, "45 minutes ago"},
		{3 * time.Hour, "3 hours ago"},
		{30 * time.Hour, "yesterday"},
		{4 * 24 * time.Hour, "4 days ago"},
	}
	for _, c := range cases {
		if got := HumanTime(now.Add(-c.ago), now); got != c.want {
			t.Errorf("%v ago = %q, want %q", c.ago, got, c.want)
		}
	}
	if HumanTime(time.Time{}, now) != "never" {
		t.Error("zero time should read never")
	}
	if got := HumanTime(now.Add(-30*24*time.Hour), now); !strings.Contains(got, "2026") {
		t.Errorf("old dates should be absolute, got %q", got)
	}
}

func TestTruncateAndPad(t *testing.T) {
	if got := Truncate("Hello world", 6); got != "Hello…" {
		t.Errorf("Truncate = %q", got)
	}
	if got := PadRight("ab", 5); got != "ab   " {
		t.Errorf("PadRight = %q", got)
	}
	if got := PadRight("日本語テキスト", 6); lipgloss.Width(got) != 6 {
		t.Errorf("PadRight wide = %q (width %d)", got, lipgloss.Width(got))
	}
}

// Seen on Windows CI: the figures on the right cut off "See Activity
// History (8)" at the end of a status message.
func TestStatusMessageWinsOverFigures(t *testing.T) {
	th := theme.New(theme.Dark)
	facts := "83 apps · 12 updates · RAM 2.9 / 16.0 GB"
	long := Status{Text: "Scan finished with gaps: 83 apps found; some sources couldn't be read. See Activity History (8).", Kind: StatusWarning}
	row := StatusBar(th, 120, long, facts, nil, false)
	if !strings.Contains(row, "See Activity History (8).") || strings.Contains(row, "RAM") {
		t.Errorf("a long message must be shown in full, without the figures: %q", row)
	}
	short := StatusBar(th, 120, Status{Text: "System ready", Kind: StatusReady}, facts, nil, false)
	if !strings.Contains(short, "System ready") || !strings.Contains(short, "RAM 2.9 / 16.0 GB") {
		t.Errorf("a short message leaves room for the figures: %q", short)
	}
	if w := lipgloss.Width(row); w > 120 {
		t.Errorf("row is %d wide", w)
	}
}

func TestBigLogoShape(t *testing.T) {
	for _, c := range []struct {
		render func(theme.Theme) string
		w, h   int
	}{{BigLogo, BigLogoWidth, BigLogoHeight}} {
		for _, th := range []theme.Theme{theme.New(theme.Dark), theme.New(theme.Light)} {
			lines := strings.Split(c.render(th), "\n")
			if len(lines) != c.h {
				t.Fatalf("logo has %d rows, want %d", len(lines), c.h)
			}
			for i, l := range lines {
				if w := lipgloss.Width(l); w != c.w {
					t.Errorf("row %d is %d wide, want %d", i, w, c.w)
				}
				for _, r := range l {
					if r >= 0x80 && r != '█' && r != '▀' && r != '▄' { // ASCII is spaces and colour codes
						t.Errorf("row %d uses %q; only block characters render reliably everywhere", i, r)
					}
				}
			}
		}
	}
	if mix([3]float64{0, 0, 0}, [3]float64{255, 255, 255}, 0.5) != "#808080" {
		t.Error("gradient midpoint")
	}
}

// On an 80-column terminal the tabs used to show only numbers
// ("1 Dashboard  2  3  4 …"), so a new user couldn't tell what 2-9 were.
func TestTwoRowTabsNameEverySection(t *testing.T) {
	th := theme.New(theme.Dark)
	one, two := TabRowWidths()
	if one != layout.ShortTabsWidth || two != layout.TwoRowTabsWidth {
		t.Fatalf("layout thresholds are out of step with the section names: one row needs %d (layout says %d), two rows need %d (layout says %d)",
			one, layout.ShortTabsWidth, two, layout.TwoRowTabsWidth)
	}
	for _, w := range []int{layout.TwoRowTabsWidth, 60, 80, 104} {
		bar := NavTabs(th, nav.Updates, w, 2, w < 80, false)
		lines := strings.Split(bar, "\n")
		if len(lines) != 3 {
			t.Fatalf("width %d: want two rows of tabs and a rule, got %d lines", w, len(lines))
		}
		for _, s := range nav.All() {
			if !strings.Contains(bar, s.Shortcut()+" "+s.Info().Short) {
				t.Errorf("width %d: %q isn't named:\n%s", w, s.Info().Short, bar)
			}
		}
		for _, l := range lines {
			if lipgloss.Width(l) > w {
				t.Errorf("width %d: line is %d wide: %q", w, lipgloss.Width(l), l)
			}
		}
	}
}

// Seen at 120 columns: "…and restart ATLAS. Nothing was …" The end of a long
// message was cut off. It now continues on the hint line.
func TestLongStatusMessagesUseTheHintLine(t *testing.T) {
	th := theme.New(theme.Dark)
	hints := []KeyHint{{Key: "q", Desc: "quit"}}
	long := Status{Text: `Nothing was sent. No Anthropic API key found. Set ANTHROPIC_API_KEY (or sign in with "ant auth login") and restart ATLAS.`, Kind: StatusWarning}
	bar := StatusBar(th, 80, long, "191 apps", hints, true)
	lines := strings.Split(bar, "\n")
	if len(lines) != 2 {
		t.Fatalf("the footer is always two lines, got %d", len(lines))
	}
	flat := strings.Join(strings.Fields(bar), " ")
	if !strings.Contains(flat, "and restart ATLAS.") || strings.Contains(bar, "…") || strings.Contains(bar, "quit") {
		t.Errorf("the whole message should show, in place of the hints:\n%s", bar)
	}
	for _, l := range lines {
		if lipgloss.Width(l) > 80 {
			t.Errorf("line is %d wide: %q", lipgloss.Width(l), l)
		}
	}
	short := StatusBar(th, 80, Status{Text: "System ready", Kind: StatusReady}, "191 apps", hints, true)
	if !strings.Contains(short, "quit") || !strings.Contains(short, "191 apps") {
		t.Errorf("a short message keeps the figures and the hints:\n%s", short)
	}
	// Without a hint line there is only one line to use.
	if one := StatusBar(th, 40, long, "", hints, false); strings.Contains(one, "\n") || lipgloss.Width(one) > 40 {
		t.Errorf("one-line footer must stay one line within the width: %q", one)
	}
	if got := wrapTo("a b c d e f g h", 3, 2); len(got) != 2 || lipgloss.Width(got[1]) > 3 {
		t.Errorf("wrapTo = %q", got)
	}
}
