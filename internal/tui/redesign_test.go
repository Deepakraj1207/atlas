package tui

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/core"
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

// Seen as a first-time user at 120×30: "33 updates are waiting" was the
// third next step, below the fold, under "Browse and search…". What needs
// attention comes first; general tips come last.
func TestNextStepsPutWhatMattersFirst(t *testing.T) {
	var out bytes.Buffer
	e := newEnv(t, testManager{id: "a", ups: threeUpdates()})
	m := checkFromHome(t, scanFromHome(t, bootEnv(t, e, &out)))
	steps := screens.NextSteps(m.ctx())
	if !strings.HasPrefix(steps[0], "2 updates are waiting") {
		t.Errorf("waiting updates should be the first next step: %q", steps)
	}
	last := steps[len(steps)-1]
	browse := -1
	for i, s := range steps {
		if strings.HasPrefix(s, "Browse and search") {
			browse = i
		}
	}
	if browse <= 0 || !strings.Contains(last, "Advanced mode") {
		t.Errorf("browsing and tips come after what needs doing: %q", steps)
	}

	fresh, _ := newTestModel(t)
	if steps := screens.NextSteps(fresh.ctx()); !strings.HasPrefix(steps[0], "Scan your system") {
		t.Errorf("before a scan, scanning is the first step: %q", steps)
	}
}

// As a first-time user: Enter on a finding did nothing. It now opens the
// section where you act on it, and the hint says which.
func TestEnterOnAFindingOpensWhereYouActOnIt(t *testing.T) {
	m := analyzeFromHome(t, scanFromHome(t, bootAnalysis(t, runtimeItems())))
	m = press(t, m, "4")
	if v := m.View(); !strings.Contains(v, "no longer gets security fixes") || !strings.Contains(v, "enter open Dependencies") {
		t.Fatalf("the first finding is the unsupported runtime, with a hint:\n%s", v)
	}
	m = press(t, m, "enter")
	if m.Section() != nav.Dependencies {
		t.Errorf("Enter on a support finding should open Dependencies, got %v", m.Section())
	}

	m.data.Analysis = &core.AnalysisResult{Findings: []core.Finding{{Kind: "updates", Title: "2 updates are waiting"}, {Kind: "unneeded", Title: "Python 3.8 may no longer be needed"}}}
	m = press(t, m, "4")
	m = press(t, m, "enter")
	if m.Section() != nav.Updates {
		t.Errorf("Enter on waiting updates should open Updates, got %v", m.Section())
	}
	m = press(t, m, "4")
	m = press(t, m, "down")
	m = press(t, m, "enter")
	if m.Section() != nav.Cleanup {
		t.Errorf("Enter on something to remove should open Cleanup, got %v", m.Section())
	}
	m = press(t, m, "4")
	m = press(t, m, "down") // "What ATLAS checked": nothing to open
	if strings.Contains(m.View(), "enter open") {
		t.Errorf("no Enter hint on the list of checks:\n%s", m.View())
	}
	m = press(t, m, "enter")
	if m.Section() != nav.SmartAnalysis {
		t.Error("Enter on the list of checks stays put")
	}
}

// As a Linux user: "Find unused applications" opened Smart Analysis on an
// unrelated finding. It now selects the unused-apps finding, or says plainly
// that none were found, or that it couldn't check and why.
func TestFindUnusedApplicationsIsHonest(t *testing.T) {
	m := analyzeFromHome(t, scanFromHome(t, bootAnalysis(t, runtimeItems())))
	m = press(t, m, "1")
	m = press(t, m, "down") // from "Analyze installed software" to "Find unused applications"
	m = press(t, m, "enter")
	if m.Section() != nav.SmartAnalysis || m.Status().Kind != components.StatusWarning ||
		!strings.Contains(m.Status().Text, "ATLAS can't tell which apps are unused here: Linux doesn't keep a record of when programs are opened.") {
		t.Errorf("couldn't check: say so and why. section %v, status %+v", m.Section(), m.Status())
	}
	if !strings.Contains(m.View(), "› What ATLAS checked") {
		t.Errorf("the list of checks should be selected:\n%s", m.View())
	}

	open := func(r *core.AnalysisResult) Model {
		m.data.Analysis = r
		next, cmd := m.Update(screens.NavigateMsg{Section: nav.SmartAnalysis, Finding: core.FindingUnused})
		return drain(t, next.(Model), cmd)
	}
	found := open(&core.AnalysisResult{Findings: []core.Finding{{Kind: "updates", Title: "2 updates are waiting"},
		{Kind: core.FindingUnused, Title: "3 apps haven't been opened in over a year"}}})
	if v := found.View(); !strings.Contains(v, "3 apps haven't been opened in over a year") || !strings.Contains(v, "enter open Cleanup") {
		t.Errorf("the unused-apps finding should be selected:\n%s", v)
	}
	none := open(&core.AnalysisResult{Findings: []core.Finding{{Kind: "updates", Title: "2 updates are waiting"}},
		Checks: []core.CheckSummary{{Name: core.CheckUnusedApps, Available: true}}})
	if st := none.Status(); st.Kind != components.StatusSuccess || st.Text != "No apps unused for over a year were found." {
		t.Errorf("checked and found none: %+v", st)
	}
}

// Seen with real apt data: "20260816ubuntu0.24.04.1" and "…04.2" were both
// cut to the same 22 characters, so Installed and New looked identical.
func TestUpdatesShowHowVersionsDiffer(t *testing.T) {
	var out bytes.Buffer
	long := []core.Update{
		{Manager: "a", PackageID: "ca-certificates", Name: "ca-certificates", CurrentVersion: "20260816ubuntu0.24.04.1", AvailableVersion: "20260816ubuntu0.24.04.2"},
		{Manager: "a", PackageID: "docker-ce", Name: "docker-ce", CurrentVersion: "5:29.9.2-1~ubuntu.24.04~noble", AvailableVersion: "5:29.9.3-1~ubuntu.24.04~noble"},
	}
	e := newEnv(t, testManager{id: "a", ups: long})
	m := checkFromHome(t, bootEnv(t, e, &out))
	m = press(t, m, "3")
	v := m.View()
	for _, want := range []string{"20260816ubuntu0.24.04.1", "20260816ubuntu0.24.04.2", "5:29.9.2-1~ubuntu.24.04~noble", "5:29.9.3-1~ubuntu.24.04~noble"} {
		if !strings.Contains(v, want) {
			t.Errorf("at 120 columns the full version %q should be visible:\n%s", want, v)
		}
	}

	// Too narrow for the table to show versions: the line above it spells
	// out the selected update.
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m = next.(Model)
	if v := m.View(); !strings.Contains(v, "20260816ubuntu0.24.04.1 → 20260816ubuntu0.24.04.2") {
		t.Errorf("at 60 columns the selected update's versions should be spelled out:\n%s", v)
	}
	m = press(t, m, "down")
	if v := m.View(); !strings.Contains(v, "→ 5:29.9.3-1~ubuntu.24.04~noble") || strings.Contains(v, "→ 20260816") {
		t.Errorf("the detail follows the cursor (both long versions don't fit, so the new one):\n%s", v)
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	if v := next.(Model).View(); !strings.Contains(v, "→ 5:29.9.3-1~ubuntu.24.04~noble") {
		t.Errorf("at 40 columns at least the new version is shown:\n%s", v)
	}
}

// As someone turning AI on without a key: say at once what is missing.
func TestTurningAIOnSaysWhatIsMissing(t *testing.T) {
	m := bootAI(t, &fakeExplain{ready: false}, "")
	m = press(t, m, "9")
	for i := 0; i < 5; i++ {
		m = press(t, m, "down")
	}
	m = press(t, m, "enter")
	st := m.Status()
	if !m.cfg.AI.Enabled || st.Kind != components.StatusWarning ||
		!strings.HasPrefix(st.Text, "AI explanations on (saved). Not ready yet: No Anthropic API key found.") {
		t.Errorf("status = %+v", st)
	}
	// And asking without a key leads with what matters most.
	m = press(t, m, "2")
	m = press(t, m, "e")
	if st := m.Status(); !strings.HasPrefix(st.Text, "Nothing was sent. No Anthropic API key found.") || strings.Contains(st.Text, "..") {
		t.Errorf("status = %q", st.Text)
	}
}

// As a returning user: "Welcome! Using default settings" every launch,
// because no settings file exists until a setting is changed.
func TestWelcomeIsOnlyForAFirstRun(t *testing.T) {
	inv, _ := newTestInventory(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	first := boot(t, inv, cfg)
	if first.Status().Text != welcomeText {
		t.Fatalf("a first run is welcomed, got %q", first.Status().Text)
	}
	scanFromHome(t, first)
	again := boot(t, inv, cfg) // same saved data, still no settings file
	if again.Status().Text != "" {
		t.Errorf("a returning user shouldn't be welcomed again, got %q", again.Status().Text)
	}
	if v := again.View(); !strings.Contains(v, "● System ready") {
		t.Errorf("the status line shows the resting state instead:\n%s", v)
	}
}

// When ATLAS can't open its saved data, screens shouldn't offer what can't run.
func TestNoScanOfferWhenSavedDataIsUnavailable(t *testing.T) {
	m := New(Options{ConfigResult: config.Load(filepath.Join(t.TempDir(), "c.yaml")), StoreErr: "unable to open database file"})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = drain(t, next.(Model), m.Init())
	for _, section := range []string{"2", "4", "5"} {
		m = press(t, m, section)
		if v := m.View(); strings.Contains(v, "enter scan") || strings.Contains(v, "enter analyze") || strings.Contains(v, "Everything else still works") {
			t.Errorf("section %s offers something that can't run:\n%s", section, v)
		}
		m = press(t, m, "enter")
		if m.PreviewOpen() {
			t.Errorf("section %s: Enter shouldn't open a preview for something unavailable", section)
			m = press(t, m, "esc")
		}
	}
}

// On a very short terminal the actions matter more than the tagline.
func TestShortTerminalShowsActionsNotTheTagline(t *testing.T) {
	m, _ := newTestModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 12})
	v := next.(Model).View()
	if strings.Contains(v, "Keep your software tidy") || !strings.Contains(v, "Welcome to ATLAS") {
		t.Errorf("12 rows: greeting without the tagline:\n%s", v)
	}
	for _, want := range []string{"› Scan my system", "Check for updates", "Analyze installed software"} {
		if !strings.Contains(v, want) {
			t.Errorf("12 rows should show the action %q:\n%s", want, v)
		}
	}
}

// At 80 columns every section is named in the tab bar.
func TestTabsNameEverySectionAt80Columns(t *testing.T) {
	m, _ := newTestModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	v := next.(Model).View()
	for _, s := range nav.All() {
		if !strings.Contains(v, s.Shortcut()+" "+s.Info().Short) {
			t.Errorf("80×24: %q isn't named in the tabs:\n%s", s.Info().Short, v)
		}
	}
}

// A run that never reported back is shown as unfinished, and comes first in
// the next steps.
func TestUnfinishedRunIsShownAndComesFirst(t *testing.T) {
	m := scanFromHome(t, bootWithSystem(t, testSystem))
	m.data.Activity = append([]core.Activity{{Kind: "updates.apply", Title: "Started updating ca-certificates", Outcome: core.OutcomeStarted,
		Summary: "ATLAS handed the terminal to your package manager, and was closed before the result came back."}}, m.data.Activity...)
	if steps := screens.NextSteps(m.ctx()); !strings.HasPrefix(steps[0], "ATLAS was closed while a package manager was working") {
		t.Errorf("the unfinished run should be the first next step: %q", steps)
	}
	next, _ := m.Update(key("8"))
	if v := next.(Model).View(); !strings.Contains(v, "Unfinished") || !strings.Contains(v, "Started updating ca-certificates") {
		t.Errorf("Activity History should mark it unfinished:\n%s", v)
	}
}
