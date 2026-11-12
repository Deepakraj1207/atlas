package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/database"
	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/layout"
	"github.com/deepakraj1207/atlas/internal/tui/nav"
)

// testScanner is a test double. Its items are test data and never shipped.
type testScanner struct {
	id    string
	items []core.SoftwareItem
	err   error
	block chan struct{}
}

func (s testScanner) ID() string     { return s.id }
func (s testScanner) Label() string  { return "Test " + s.id }
func (s testScanner) Detail() string { return "reads test data" }
func (s testScanner) Scan(ctx context.Context) ([]core.SoftwareItem, error) {
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return s.items, s.err
}

func testItems() []core.SoftwareItem {
	return []core.SoftwareItem{
		{Name: "Alpha Editor", Version: "1.0", Publisher: "Alpha Co", PackageID: "Alpha.Editor", Source: "one", Kind: core.KindApp},
		{Name: "Beta Shell", Version: "2.1", Source: "one", Kind: core.KindApp},
		{Name: "libgamma", Version: "0.9", Source: "one", Kind: core.KindComponent},
	}
}

func newTestInventory(t *testing.T, scanners ...core.Scanner) (core.InventoryService, string) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "atlas.db")
	db, err := database.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if len(scanners) == 0 {
		scanners = []core.Scanner{
			testScanner{id: "one", items: testItems()},
			testScanner{id: "absent", err: core.ErrSourceUnavailable},
		}
	}
	return inventory.NewService(scanners, db, nil, nil), dbPath
}

// boot builds a model with a real temp database, sizes it and runs Init.
func boot(t *testing.T, inv core.InventoryService, cfgPath string) Model {
	t.Helper()
	m := New(Options{ConfigResult: config.Load(cfgPath), Version: "test", Inventory: inv, DBPath: "test.db"})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return drain(t, next.(Model), m.Init())
}

func newTestModel(t *testing.T) (Model, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	inv, _ := newTestInventory(t)
	return boot(t, inv, path), path
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

// drain runs commands synchronously, feeding each message back in until
// none remain, so persisted side effects are observable. Timer messages
// (status clearing, spinner frames) are dropped so tests stay fast and can
// read the status line.
func drain(t *testing.T, m Model, first tea.Cmd) Model {
	t.Helper()
	queue := []tea.Cmd{first}
	for len(queue) > 0 {
		cmd := queue[0]
		queue = queue[1:]
		if cmd == nil {
			continue
		}
		msg := cmd()
		switch msg := msg.(type) {
		case nil, tea.QuitMsg, statusClearMsg, spinner.TickMsg:
			continue
		case tea.BatchMsg:
			queue = append(queue, msg...)
			continue
		}
		next, c := m.Update(msg)
		m = next.(Model)
		queue = append(queue, c)
	}
	return m
}

// press sends a key and drains the resulting commands.
func press(t *testing.T, m Model, k string) Model {
	t.Helper()
	next, cmd := m.Update(key(k))
	return drain(t, next.(Model), cmd)
}

func typeText(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		m = press(t, m, string(r))
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
	d := m.Data()
	if !d.Available || !d.Loaded || d.Summary != nil || len(d.Items) != 0 {
		t.Errorf("fresh database should load as empty: %+v", d)
	}
	if !strings.Contains(m.View(), "not yet scanned") {
		t.Error("Home should say not yet scanned before the first scan")
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
	m = press(t, m, "6")
	if m.Section() != nav.Cleanup {
		t.Errorf("digit 6 = %v, want Cleanup", m.Section())
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
	inv, _ := newTestInventory(t)
	m := boot(t, inv, path)
	m = press(t, m, "m")
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
	inv, _ := newTestInventory(t)
	m := boot(t, inv, path)
	if m.Status().Kind != components.StatusWarning {
		t.Errorf("malformed config should produce a warning status, got %+v", m.Status())
	}
	m = press(t, m, "m")
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
	m = press(t, m, "right")
	m = press(t, m, "right")
	m = press(t, m, "enter")
	if !m.PreviewOpen() {
		t.Fatal("enter on a quick action should open the preview")
	}
	view := m.View()
	for _, want := range []string{"Find New Tools", "Not available: discovery arrives in Phase 6", "What will happen", "Is it safe?", "Can it be undone?"} {
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
	m = press(t, m, "9") // Settings is root-scrolled and long
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
	m = press(t, m, "1")
	if m.Scroll() != 0 {
		t.Error("changing section should reset scroll")
	}
}

func TestLongPreviewScrollsInsteadOfClipping(t *testing.T) {
	m, _ := newTestModel(t)
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 16})
	m = next.(Model)
	m = press(t, m, "enter") // Scan My System preview
	if !m.PreviewOpen() || !strings.Contains(m.View(), "more below") {
		t.Fatalf("a preview taller than the screen must say there is more:\n%s", m.View())
	}
	for i := 0; i < 6; i++ {
		m = press(t, m, "pgdown")
	}
	if v := m.View(); !strings.Contains(v, "Enter to continue") || !strings.Contains(v, "more above") {
		t.Errorf("scrolling should reach the end of the preview:\n%s", v)
	}
	if !m.PreviewOpen() {
		t.Error("scrolling must not close the preview")
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
