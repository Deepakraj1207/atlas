package components

import (
	"strings"
	"testing"

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
