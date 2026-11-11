package layout

import "testing"

func TestBreakpoints(t *testing.T) {
	cases := []struct {
		w, h int
		want Breakpoint
	}{
		{0, 0, TooSmall},
		{39, 30, TooSmall},
		{120, 9, TooSmall},
		{40, 10, Narrow},
		{79, 24, Narrow},
		{80, 24, Medium},
		{109, 24, Medium},
		{110, 24, Wide},
		{200, 50, Wide},
	}
	for _, c := range cases {
		got := Compute(c.w, c.h).Breakpoint
		if got != c.want {
			t.Errorf("Compute(%d,%d).Breakpoint = %v, want %v", c.w, c.h, got, c.want)
		}
	}
}

func TestWideShowsSidebarAndReducesContentWidth(t *testing.T) {
	l := Compute(120, 30)
	if !l.ShowSidebar || l.ShowTabBar {
		t.Error("wide layout should use the sidebar, not the tab bar")
	}
	if l.SidebarWidth != SidebarWidth || l.ContentWidth != 120-SidebarWidth {
		t.Errorf("content width = %d, sidebar = %d", l.ContentWidth, l.SidebarWidth)
	}
	if !l.TwoColumn {
		t.Error("wide and tall should allow two columns")
	}
	if l.ContentHeight != 30-HeaderHeight-FooterHeight {
		t.Errorf("content height = %d", l.ContentHeight)
	}
}

func TestMediumAndNarrowUseTabBar(t *testing.T) {
	m := Compute(90, 24)
	if m.ShowSidebar || !m.ShowTabBar || m.UseShortTitles {
		t.Errorf("medium: %+v", m)
	}
	if m.ContentWidth != 90 {
		t.Errorf("medium content width = %d, want full width", m.ContentWidth)
	}
	n := Compute(60, 24)
	if n.ShowSidebar || !n.ShowTabBar || !n.UseShortTitles {
		t.Errorf("narrow: %+v", n)
	}
	if n.TwoColumn {
		t.Error("narrow must be single column")
	}
	if n.ContentHeight != 24-HeaderHeight-FooterHeight-TabBarHeight {
		t.Errorf("narrow content height = %d", n.ContentHeight)
	}
}

func TestShortTerminalsDropHeaderThenHints(t *testing.T) {
	l := Compute(100, 15)
	if l.ShowHeader {
		t.Error("height 15 should hide the header")
	}
	if !l.ShowHints {
		t.Error("height 15 should still show hints")
	}
	l = Compute(100, 12)
	if l.ShowHeader || l.ShowHints {
		t.Error("height 12 should hide header and hints")
	}
	if l.ContentHeight != 12-1-TabBarHeight {
		t.Errorf("content height = %d", l.ContentHeight)
	}
}

func TestTooSmallNeverGoesNegative(t *testing.T) {
	l := Compute(-5, -5)
	if l.ContentWidth < 0 || l.ContentHeight < 0 {
		t.Errorf("negative dimensions: %+v", l)
	}
	if l.ShowSidebar || l.ShowTabBar || l.ShowHeader || l.ShowHints {
		t.Error("too-small layout must hide all chrome")
	}
}

func TestContentHeightAlwaysAtLeastOne(t *testing.T) {
	for h := MinHeight; h < 40; h++ {
		for _, w := range []int{MinWidth, 80, 110, 200} {
			if l := Compute(w, h); l.ContentHeight < 1 || l.ContentWidth < 1 {
				t.Errorf("Compute(%d,%d) content %dx%d", w, h, l.ContentWidth, l.ContentHeight)
			}
		}
	}
}
