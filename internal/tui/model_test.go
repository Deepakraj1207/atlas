package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/layout"
	"github.com/deepakraj1207/atlas/internal/tui/nav"
)

func newTestModel(t *testing.T) (Model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	m := New(Options{ConfigResult: config.Load(path), Version: "test"})
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return m2.(Model), path
}

func key(s string) tea.KeyMsg {
	switch s {
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func init() { statusClearAfter = 0 }

// press sends a key and runs returned commands synchronously, feeding each
// message back in until none remain, so persisted side effects are
// observable. Status-clear ticks are dropped so tests can read the status.
func press(t *testing.T, m Model, k string) Model {
	t.Helper()
	next, cmd := m.Update(key(k))
	m = next.(Model)
	for cmd != nil {
		msg := cmd()
		if msg == nil {
			break
		}
		switch msg.(type) {
		case tea.QuitMsg, statusClearMsg:
			return m
		}
		next, cmd = m.Update(msg)
		m = next.(Model)
	}
	return m
}

func TestStartsOnHomeWithDefaults(t *testing.T) {
	m, _ := newTestModel(t)
	if m.Section() != nav.Home {
		t.Errorf("start section = %v, want Home", m.Section())
	}
	if m.Config().UI.Mode != config.ModeSimple {
		t.Error("should start in Simple mode")
	}
	if !strings.Contains(m.Status().Text, "default settings") {
		t.Errorf("first-run status should mention defaults, got %q", m.Status().Text)
	}
}

func TestTabAndDigitsNavigate(t *testing.T) {
	m, _ := newTestModel(t)
	m = press(t, m, "tab")
	if m.Section() != nav.MySoftware {
		t.Errorf("tab = %v, want My Software", m.Section())
	}
	m = press(t, m, "shift+tab")
	m = press(t, m, "shift+tab")
	if m.Section() != nav.Settings {
		t.Errorf("shift+tab twice from My Software = %v, want Settings", m.Section())
	}
	m = press(t, m, "4")
	if m.Section() != nav.SmartAnalysis {
		t.Errorf("digit 4 = %v, want Smart Analysis", m.Section())
	}
	if !strings.Contains(m.View(), "Not available in this version") {
		t.Error("coming-soon section should render its empty state")
	}
}

func TestModeTogglePersistsToConfigFile(t *testing.T) {
	m, path := newTestModel(t)
	m = press(t, m, "m")
	if m.Config().UI.Mode != config.ModeAdvanced {
		t.Fatal("m should switch to Advanced mode")
	}
	if m.Status().Kind != components.StatusSuccess || !strings.Contains(m.Status().Text, "saved") {
		t.Errorf("status should confirm the save, got %+v", m.Status())
	}
	res := config.Load(path)
	if res.Err != nil || res.Config.UI.Mode != config.ModeAdvanced {
		t.Errorf("mode not persisted: %+v err=%v", res.Config, res.Err)
	}
	if !strings.Contains(m.View(), "Diagnostics") {
		t.Error("Advanced mode should reveal diagnostics on Home")
	}
	m = press(t, m, "m")
	if m.Config().UI.Mode != config.ModeSimple {
		t.Error("second m should return to Simple")
	}
	if strings.Contains(m.View(), "Diagnostics") {
		t.Error("Simple mode should hide diagnostics")
	}
}

func TestThemeTogglePersists(t *testing.T) {
	m, path := newTestModel(t)
	m = press(t, m, "t")
	if m.ThemeName() != config.ThemeLight || m.Config().UI.Theme != config.ThemeLight {
		t.Error("t should switch to the light theme")
	}
	if config.Load(path).Config.UI.Theme != config.ThemeLight {
		t.Error("theme not persisted")
	}
}

func TestSaveFailureIsReportedNotSilent(t *testing.T) {
	dir := t.TempDir()
	// A directory where the config file should be makes the rename fail.
	path := filepath.Join(dir, "config.yaml")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
	m := New(Options{ConfigResult: config.Load(path)})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = press(t, next.(Model), "m")
	if m.Config().UI.Mode != config.ModeAdvanced {
		t.Error("mode should still change in memory")
	}
	if m.Status().Kind != components.StatusError || !strings.Contains(m.Status().Text, "could not be saved") {
		t.Errorf("save failure must be shown, got %+v", m.Status())
	}
}

func TestMalformedConfigIsBackedUpOnFirstSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("ui: [oops\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := New(Options{ConfigResult: config.Load(path)})
	if m.Status().Kind != components.StatusWarning {
		t.Errorf("malformed config should produce a warning status, got %+v", m.Status())
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = press(t, next.(Model), "m")
	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Error("malformed file should be backed up before being replaced")
	}
	if config.Load(path).Err != nil {
		t.Error("new file should load cleanly")
	}
}

func TestHelpOverlayOpensAndCloses(t *testing.T) {
	m, _ := newTestModel(t)
	m = press(t, m, "?")
	if !m.HelpOpen() {
		t.Fatal("? should open help")
	}
	view := m.View()
	if !strings.Contains(view, "Keyboard help") || !strings.Contains(view, "Home:") {
		t.Error("help should be context sensitive and visible")
	}
	// Navigation keys are swallowed while help is open.
	m = press(t, m, "tab")
	if m.Section() != nav.Home {
		t.Error("tab should not navigate while help is open")
	}
	m = press(t, m, "esc")
	if m.HelpOpen() {
		t.Error("esc should close help")
	}
}

func TestQuickActionOpensPreviewAndChangesNothing(t *testing.T) {
	m, path := newTestModel(t)
	m = press(t, m, "right")
	m = press(t, m, "enter")
	if !m.PreviewOpen() {
		t.Fatal("enter on a quick action should open the preview")
	}
	view := m.View()
	for _, want := range []string{"Check for Updates", "Not available yet", "What will happen", "Is it safe?", "Can it be undone?"} {
		if !strings.Contains(view, want) {
			t.Errorf("preview missing %q", want)
		}
	}
	m = press(t, m, "esc")
	if m.PreviewOpen() {
		t.Error("esc should close the preview")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("previewing must not write anything")
	}
}

func TestSettingsScreenTogglesAndPersists(t *testing.T) {
	m, path := newTestModel(t)
	m = press(t, m, "9")
	if m.Section() != nav.Settings {
		t.Fatal("9 should open Settings")
	}
	m = press(t, m, "down") // Theme row
	m = press(t, m, "enter")
	if m.Config().UI.Theme != config.ThemeLight {
		t.Error("enter on Theme should toggle it")
	}
	m = press(t, m, "down") // Keyboard hints row
	m = press(t, m, "enter")
	if m.Config().UI.ShowHints {
		t.Error("enter on Keyboard hints should turn them off")
	}
	res := config.Load(path)
	if res.Config.UI.Theme != config.ThemeLight || res.Config.UI.ShowHints {
		t.Errorf("settings not persisted: %+v", res.Config)
	}
}

func TestResizeChangesLayoutAndTooSmallMessage(t *testing.T) {
	m, _ := newTestModel(t)
	if !m.Layout().ShowSidebar {
		t.Error("120 columns should show the sidebar")
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 20})
	m = next.(Model)
	if m.Layout().Breakpoint != layout.Narrow || m.Layout().ShowSidebar {
		t.Errorf("60 columns should be narrow without sidebar: %+v", m.Layout())
	}
	if !strings.Contains(m.View(), "1 Home") {
		t.Error("narrow layout should render the tab bar")
	}
	next, _ = m.Update(tea.WindowSizeMsg{Width: 30, Height: 8})
	m = next.(Model)
	if !strings.Contains(m.View(), "needs a terminal of at least") {
		t.Error("too-small terminals should get a plain explanation")
	}
}

func TestContentScrollsOnShortTerminals(t *testing.T) {
	m, _ := newTestModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 14})
	m = next.(Model)
	if !strings.Contains(m.View(), "more below") {
		t.Fatal("clipped Home content should show a 'more below' indicator")
	}
	m = press(t, m, "pgdown")
	if m.Scroll() == 0 {
		t.Error("pgdown should scroll")
	}
	if !strings.Contains(m.View(), "more above") {
		t.Error("scrolled view should show 'more above'")
	}
	m = press(t, m, "tab")
	if m.Scroll() != 0 {
		t.Error("changing section should reset scroll")
	}
}

func TestQuitKeys(t *testing.T) {
	m, _ := newTestModel(t)
	for _, k := range []string{"q", "ctrl+c"} {
		_, cmd := m.Update(key(k))
		if cmd == nil {
			t.Fatalf("%s should produce a command", k)
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("%s should quit", k)
		}
	}
}

func TestViewNeverExceedsTerminalHeight(t *testing.T) {
	m, _ := newTestModel(t)
	for _, size := range [][2]int{{40, 10}, {60, 14}, {80, 24}, {100, 16}, {120, 30}, {200, 50}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		mm := next.(Model)
		for _, section := range nav.All() {
			mm.nav.Go(section)
			lines := strings.Count(mm.View(), "\n") + 1
			if lines > size[1] {
				t.Errorf("%dx%d %s: view has %d lines", size[0], size[1], section.Title(), lines)
			}
		}
	}
}
