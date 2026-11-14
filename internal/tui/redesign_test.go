package tui

import (
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/platform"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/nav"
	"github.com/deepakraj1207/atlas/internal/tui/screens"
)

// bootWithSystem is boot with known facts about the computer.
func bootWithSystem(t *testing.T, sys platform.SystemInfo) Model {
	t.Helper()
	inv, _ := newTestInventory(t)
	m := New(Options{ConfigResult: config.Load(filepath.Join(t.TempDir(), "c.yaml")), Version: "0.1.0", Inventory: inv, DBPath: "test.db",
		SystemInfo: func() platform.SystemInfo { return sys }})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return drain(t, next.(Model), m.Init())
}

var testSystem = platform.SystemInfo{OS: "Windows 11 Home 25H2", MemTotal: 16 << 30, MemAvailable: 8 << 30, DiskTotal: 500 << 30, DiskFree: 200 << 30, DiskPath: "C:"}

func TestEscMovesFocusToTheSectionsAndBack(t *testing.T) {
	m, _ := newTestModel(t)
	if m.NavFocused() {
		t.Fatal("the screen has focus at start, so the arrow keys move through its actions")
	}
	m = press(t, m, "down")
	if f := m.Screen(nav.Home).(*screens.Home).Focus(); f != 1 {
		t.Fatalf("down should move the action list, focus = %d", f)
	}

	m = press(t, m, "esc")
	if !m.NavFocused() || !strings.Contains(m.View(), "↑↓ navigate") {
		t.Fatalf("esc should move focus to the sections:\n%s", m.View())
	}
	m = press(t, m, "down")
	m = press(t, m, "down")
	if m.Section() != nav.Updates {
		t.Errorf("down twice in the sections = %v, want Updates", m.Section())
	}
	if f := m.Screen(nav.Home).(*screens.Home).Focus(); f != 1 {
		t.Errorf("keys must not reach the screen while the sections have focus, focus = %d", f)
	}
	m = press(t, m, "up")
	m = press(t, m, "enter")
	if m.NavFocused() || m.Section() != nav.MySoftware {
		t.Errorf("enter should open the section: focused=%v section=%v", m.NavFocused(), m.Section())
	}
	m = press(t, m, "esc")
	m = press(t, m, "5")
	if m.NavFocused() || m.Section() != nav.Dependencies {
		t.Errorf("a digit jumps and gives the screen focus: focused=%v section=%v", m.NavFocused(), m.Section())
	}
	m = press(t, m, "esc")
	m = press(t, m, "esc")
	if m.NavFocused() {
		t.Error("esc again returns to the screen")
	}
}

func TestNarrowTerminalsMoveThroughTabs(t *testing.T) {
	m, _ := newTestModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 70, Height: 24})
	m = press(t, next.(Model), "esc")
	m = press(t, m, "right")
	if m.Section() != nav.MySoftware {
		t.Errorf("right in the tab bar = %v, want My Software", m.Section())
	}
	m = press(t, m, "left")
	if m.Section() != nav.Home {
		t.Errorf("left in the tab bar = %v, want Dashboard", m.Section())
	}
}

// Esc first closes what a screen has open; only then does it leave.
func TestEscClosesTheScreensOwnThingsFirst(t *testing.T) {
	m := scanFromHome(t, bootWithSystem(t, testSystem))
	m = press(t, m, "/")
	if m.Section() != nav.MySoftware {
		t.Fatalf("/ should open My Software, got %v", m.Section())
	}
	sw := m.Screen(nav.MySoftware).(*screens.MySoftware)
	if !sw.CapturesInput() {
		t.Fatal("/ should start typing a search")
	}
	for _, r := range "beta" {
		m = press(t, m, string(r))
	}
	m = press(t, m, "enter")
	if v := m.View(); !strings.Contains(v, "Beta") || strings.Contains(v, "Alpha Editor") {
		t.Fatalf("search should filter the list:\n%s", v)
	}
	m = press(t, m, "esc")
	if m.NavFocused() || !strings.Contains(m.View(), "Alpha Editor") {
		t.Errorf("the first esc clears the search: focused=%v\n%s", m.NavFocused(), m.View())
	}
	m = press(t, m, "enter")
	m = press(t, m, "esc")
	if m.NavFocused() || m.Screen(nav.MySoftware).(*screens.MySoftware).DetailOpen() {
		t.Error("esc closes the details view before anything else")
	}
	m = press(t, m, "esc")
	if !m.NavFocused() {
		t.Error("with nothing open, esc moves to the sections")
	}
}

func TestSearchActionOpensMySoftwareSearch(t *testing.T) {
	m := scanFromHome(t, bootWithSystem(t, testSystem))
	for i := 0; i < 6; i++ {
		m = press(t, m, "down")
	}
	m = press(t, m, "enter")
	if m.Section() != nav.MySoftware || !m.Screen(nav.MySoftware).(*screens.MySoftware).CapturesInput() {
		t.Errorf("Search software should open the search box: section %v", m.Section())
	}
}

func TestHeaderAndStatusBarShowRealFacts(t *testing.T) {
	m := bootWithSystem(t, testSystem)
	v := m.View()
	for _, want := range []string{"◆ ATLAS", "v0.1.0", "Not scanned yet", "Windows 11 Home 25H2 · 16.0 GB RAM", "Simple", "RAM 8.0 / 16.0 GB"} {
		if !strings.Contains(v, want) {
			t.Errorf("view missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "0 apps") || strings.Contains(v, "updates ·") {
		t.Errorf("no counts before a scan or a check:\n%s", v)
	}

	m = scanFromHome(t, m)
	m.status = components.Status{} // the resting state, after the message clears
	v = m.View()
	for _, want := range []string{"● System ready", "2 apps"} {
		if !strings.Contains(v, want) {
			t.Errorf("after a scan, view missing %q:\n%s", want, v)
		}
	}
}

func TestUnknownSystemFactsAreLeftOut(t *testing.T) {
	m := bootWithSystem(t, platform.SystemInfo{OS: "Linux"})
	v := m.View()
	if strings.Contains(v, "RAM") || strings.Contains(v, "free of") {
		t.Errorf("unknown memory and disk must not be shown:\n%s", v)
	}
	if text := dashboardText(m); !strings.Contains(text, "Linux") {
		t.Errorf("the OS name is still shown:\n%s", text)
	}
}

func TestDashboardFiguresAreRealOrMarkedUnknown(t *testing.T) {
	m := bootWithSystem(t, testSystem)
	text := dashboardText(m)
	for _, want := range []string{"Welcome to ATLAS", "What would you like to do?", "› Scan my system", "Search software",
		"— apps", "— updates", "— runtimes", "8.0 GB in use", "200 GB free of 500 GB on C:"} {
		if !strings.Contains(text, want) {
			t.Errorf("dashboard missing %q:\n%s", want, text)
		}
	}
	m = scanFromHome(t, m)
	text = dashboardText(m)
	if !strings.Contains(text, "Welcome back") || !strings.Contains(text, "2 apps") || strings.Contains(text, "— apps") {
		t.Errorf("after a scan the figure is the real count:\n%s", text)
	}
}

func TestViewNeverExceedsTerminalWidth(t *testing.T) {
	m := scanFromHome(t, bootWithSystem(t, testSystem))
	for _, size := range [][2]int{{40, 10}, {60, 14}, {72, 24}, {80, 24}, {100, 16}, {110, 30}, {120, 30}, {200, 50}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		mm := next.(Model)
		for _, focus := range []bool{false, true} {
			mm.navFocus = focus
			for _, section := range nav.All() {
				mm.nav.Go(section)
				for i, line := range strings.Split(mm.View(), "\n") {
					if w := lipgloss.Width(line); w > size[0] {
						t.Errorf("%dx%d %s (nav focus %v) line %d is %d wide: %q", size[0], size[1], section.Title(), focus, i, w, line)
					}
				}
			}
		}
	}
}

func TestBigLogoWhereThereIsRoom(t *testing.T) {
	m := bootWithSystem(t, testSystem)
	logoRow := "▄▀▀▄ ▀█▀ █    ▄▀▀▄ ▄▀▀▀"
	if v := m.View(); !strings.Contains(v, logoRow) || !strings.Contains(v, "Welcome to ATLAS") {
		t.Errorf("at 120×30 the big logo sits beside the welcome:\n%s", v)
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	if text := dashboardText(next.(Model)); strings.Contains(text, logoRow) || !strings.Contains(text, "Welcome to ATLAS") {
		t.Errorf("a small terminal gets the plain welcome only:\n%s", text)
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 60, Height: 40})
	if text := dashboardText(next.(Model)); !strings.Contains(text, logoRow) {
		t.Errorf("a narrow but tall terminal has room above the welcome:\n%s", text)
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 160, Height: 45})
	if v := next.(Model).View(); !strings.Contains(v, logoRow) {
		t.Errorf("large windows keep the three-row logo:\n%s", v)
	}
}

// Reported on Windows: "Smart analysis not working". The Dashboard teaches
// Enter, but an empty Smart Analysis only answered to "a", so Enter did
// nothing. Now Enter does what each empty screen suggests, with the usual
// preview first.
func TestEnterDoesWhatAnEmptyScreenSuggests(t *testing.T) {
	m := bootAnalysis(t, runtimeItems())
	for _, section := range []string{"2", "4", "5"} {
		m = press(t, m, section)
		m = press(t, m, "enter")
		if !m.PreviewOpen() || !strings.Contains(m.View(), "Scan My System") {
			t.Errorf("section %s before a scan: Enter should offer the scan:\n%s", section, m.View())
		}
		m = press(t, m, "esc")
	}

	m = scanFromHome(t, m)
	for _, section := range []string{"4", "5"} {
		m = press(t, m, section)
		if v := m.View(); !strings.Contains(v, "Press Enter to analyze") || !strings.Contains(v, "enter analyze") {
			t.Errorf("section %s should say Enter analyzes:\n%s", section, v)
		}
		m = press(t, m, "enter")
		if !m.PreviewOpen() || !strings.Contains(m.View(), "Analyze My Software") {
			t.Errorf("section %s: Enter should offer the analysis:\n%s", section, m.View())
		}
		m = press(t, m, "esc")
	}

	m = press(t, m, "3")
	m = press(t, m, "enter")
	if !m.PreviewOpen() || !strings.Contains(m.View(), "Check for Updates") {
		t.Errorf("Updates with nothing listed: Enter should offer the check:\n%s", m.View())
	}
	m = press(t, m, "esc")
	m = press(t, m, "6")
	m = press(t, m, "enter")
	if !m.PreviewOpen() || !strings.Contains(m.View(), "Look for things to tidy") {
		t.Errorf("Cleanup with nothing listed: Enter should offer to look:\n%s", m.View())
	}
}

// The path from the report: Dashboard → "Find unused applications" → Enter
// starts the analysis (after its preview), and the results appear.
func TestFindUnusedAppsThenEnterAnalyzes(t *testing.T) {
	m := scanFromHome(t, bootAnalysis(t, runtimeItems()))
	m = press(t, m, "1")
	for i := 0; i < 3; i++ {
		m = press(t, m, "down")
	}
	m = press(t, m, "enter")
	if m.Section() != nav.SmartAnalysis {
		t.Fatalf("Find unused applications should open Smart Analysis, got %v", m.Section())
	}
	m = press(t, m, "enter")
	if !m.PreviewOpen() {
		t.Fatalf("Enter should preview the analysis:\n%s", m.View())
	}
	m = press(t, m, "enter")
	if m.Analyzing() || m.Data().Analysis == nil {
		t.Fatalf("the analysis should have run")
	}
	if v := m.View(); !strings.Contains(v, "What ATLAS checked") {
		t.Errorf("Smart Analysis should show its results:\n%s", v)
	}
}
