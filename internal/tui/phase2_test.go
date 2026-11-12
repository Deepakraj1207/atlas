package tui

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/nav"
	"github.com/deepakraj1207/atlas/internal/tui/screens"
)

// scanFromHome previews and confirms "Scan My System" (first Home button).
func scanFromHome(t *testing.T, m Model) Model {
	t.Helper()
	m = press(t, m, "1")
	m = press(t, m, "enter")
	if !m.PreviewOpen() {
		t.Fatal("Scan My System should open a preview first")
	}
	v := m.View()
	for _, want := range []string{"Scan My System", "only reads information", "Can it be undone?", "Enter to continue"} {
		if !strings.Contains(v, want) {
			t.Errorf("scan preview missing %q", want)
		}
	}
	return press(t, m, "enter")
}

func TestScanFlowSavesAndShowsEverywhere(t *testing.T) {
	m, _ := newTestModel(t)
	m = scanFromHome(t, m)

	if m.Scanning() {
		t.Fatal("scan should have finished")
	}
	if st := m.Status(); st.Kind != components.StatusSuccess || !strings.Contains(st.Text, "found 2 apps") {
		t.Errorf("status = %+v", st)
	}
	d := m.Data()
	if d.Summary == nil || d.Summary.AppCount != 2 || d.Summary.ComponentCount != 1 || len(d.Items) != 3 {
		t.Fatalf("data after scan = %+v", d)
	}
	home := m.View()
	if !strings.Contains(home, "Installed apps") || strings.Contains(home, "not yet scanned") || !strings.Contains(home, "Scanned your system · just now") {
		t.Error("Home should show the real count and the activity entry")
	}

	m = press(t, m, "2")
	v := m.View()
	if !strings.Contains(v, "Alpha Editor") || !strings.Contains(v, "Beta Shell") {
		t.Error("My Software should list scanned apps")
	}
	if strings.Contains(v, "libgamma") || !strings.Contains(v, "1 system components hidden") {
		t.Error("components should be hidden by default and the count explained")
	}
	if !strings.Contains(v, "Test one") {
		t.Error("Simple mode should show the plain-language source label")
	}

	m = press(t, m, "8")
	v = m.View()
	if !strings.Contains(v, "Scanned your system") || !strings.Contains(v, "Done") {
		t.Error("Activity History should record the scan")
	}
}

func TestMySoftwareSearchFilterAndDetails(t *testing.T) {
	m, _ := newTestModel(t)
	m = scanFromHome(t, m)
	m = press(t, m, "2")
	sw := m.Screen(nav.MySoftware).(*screens.MySoftware)

	m = press(t, m, "/")
	if !sw.CapturesInput() {
		t.Fatal("/ should focus search")
	}
	// q, m and digits must type into the box, not quit or switch mode.
	m = typeText(t, m, "beta q2m")
	if m.Section() != nav.MySoftware || m.Config().UI.Mode != config.ModeSimple {
		t.Fatal("global keys must not fire while typing a search")
	}
	if sw.Query() != "beta q2m" {
		t.Errorf("query = %q", sw.Query())
	}
	if !strings.Contains(m.View(), "No apps match") {
		t.Error("an unmatched search should explain itself")
	}
	for range "q2m" {
		m = press(t, m, "backspace")
	}
	m = press(t, m, "backspace")
	m = press(t, m, "enter")
	v := m.View()
	if !strings.Contains(v, "Beta Shell") || strings.Contains(v, "Alpha Editor") || !strings.Contains(v, "1 match") {
		t.Errorf("search should narrow to Beta Shell:\n%s", v)
	}

	m = press(t, m, "esc")
	if sw.Query() != "" {
		t.Error("esc should clear the search")
	}
	m = press(t, m, "f")
	if !sw.ShowAll() || !strings.Contains(m.View(), "libgamma (system)") {
		t.Error("f should include system components, labelled")
	}

	m = press(t, m, "enter")
	if !sw.DetailOpen() || !strings.Contains(m.View(), "Alpha Co") || !strings.Contains(m.View(), "Found by") {
		t.Error("enter should open details for the first item")
	}
	if strings.Contains(m.View(), "Alpha.Editor") {
		t.Error("package ID is Advanced-only")
	}
	m = press(t, m, "m")
	if !strings.Contains(m.View(), "Alpha.Editor") {
		t.Error("Advanced mode should reveal the package ID")
	}
	m = press(t, m, "esc")
	if sw.DetailOpen() {
		t.Error("esc should close details")
	}
}

func TestScanCanBeCancelled(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	inv, _ := newTestInventory(t, testScanner{id: "slow", items: testItems(), block: block})
	m := boot(t, inv, filepath.Join(t.TempDir(), "config.yaml"))

	// Start the scan without draining, since the scanner blocks.
	next, _ := m.Update(screens.StartScanMsg{})
	m = next.(Model)
	if !m.Scanning() || !strings.Contains(m.View(), "Scanning your system") {
		t.Fatal("scan should be running and visible on Home")
	}
	if !strings.Contains(m.View(), "press x to stop") && !strings.Contains(m.View(), "Press x to stop") {
		t.Error("the way to stop should be shown")
	}
	next, _ = m.Update(key("x"))
	m = next.(Model)
	m = drain(t, m, listen(m.scan.events))
	if m.Scanning() {
		t.Fatal("scan should have stopped")
	}
	if !strings.Contains(m.Status().Text, "Scan stopped") {
		t.Errorf("status = %+v", m.Status())
	}
	if m.Data().Summary != nil {
		t.Error("a cancelled scan must not save a list")
	}
	if acts := m.Data().Activity; len(acts) != 1 || acts[0].Outcome != core.OutcomeCancelled {
		t.Errorf("cancel should be recorded: %+v", acts)
	}
}

func TestPartialScanIsExplained(t *testing.T) {
	inv, _ := newTestInventory(t,
		testScanner{id: "one", items: testItems()},
		testScanner{id: "broken", err: errors.New("exit status 5")},
	)
	m := boot(t, inv, filepath.Join(t.TempDir(), "config.yaml"))
	m = scanFromHome(t, m)
	if st := m.Status(); st.Kind != components.StatusWarning || !strings.Contains(st.Text, "some sources couldn't be read") {
		t.Errorf("status = %+v", st)
	}
	if !strings.Contains(m.View(), "some sources missing") {
		t.Error("Home should flag the partial scan")
	}
	m = press(t, m, "8")
	if !strings.Contains(m.View(), "couldn't read: Test broken") {
		t.Error("history should name the failed source")
	}
	if strings.Contains(m.View(), "exit status 5") {
		t.Error("technical error text is Advanced-only")
	}
	m = press(t, m, "m")
	if !strings.Contains(m.View(), "exit status 5") {
		t.Error("Advanced mode should show the technical error")
	}
}

func TestClearSavedListRequiresConfirmation(t *testing.T) {
	m, _ := newTestModel(t)
	m = scanFromHome(t, m)
	m = press(t, m, "9")
	for i := 0; i < 4; i++ {
		m = press(t, m, "down")
	}
	if !strings.Contains(m.View(), "Clear…") {
		t.Fatal("Settings should offer to clear the saved list")
	}
	m = press(t, m, "enter")
	if !m.PreviewOpen() || !strings.Contains(m.View(), "What will change") || !strings.Contains(m.View(), "2 apps and 1 system components") {
		t.Fatalf("clearing must preview what changes:\n%s", m.View())
	}
	m = press(t, m, "esc")
	if m.Data().Summary == nil {
		t.Fatal("esc must cancel without clearing")
	}
	m = press(t, m, "enter")
	m = press(t, m, "enter")
	if m.Data().Summary != nil || len(m.Data().Items) != 0 {
		t.Error("confirmed clear should empty the list")
	}
	if m.Status().Kind != components.StatusSuccess || !strings.Contains(m.Status().Text, "No software on your computer was touched") {
		t.Errorf("status = %+v", m.Status())
	}
	if acts := m.Data().Activity; len(acts) != 2 || acts[0].Kind != "inventory.clear" {
		t.Errorf("clear should be recorded: %+v", acts)
	}
}

func TestPreviewOffSkipsOnlyReadOnlyActions(t *testing.T) {
	m, _ := newTestModel(t)
	m = press(t, m, "9")
	m = press(t, m, "down")
	m = press(t, m, "down")
	m = press(t, m, "down") // Preview read-only actions
	m = press(t, m, "enter")
	if m.Config().UI.ConfirmActions {
		t.Fatal("previews should now be off")
	}
	m = scanDirect(t, m)
	if m.PreviewOpen() || m.Data().Summary == nil {
		t.Fatal("a read-only scan should run straight away with previews off")
	}
	m = press(t, m, "9")
	m = press(t, m, "down") // Saved software list (cursor kept at row 3)
	m = press(t, m, "enter")
	if !m.PreviewOpen() {
		t.Error("clearing changes data, so it must always preview")
	}
}

func scanDirect(t *testing.T, m Model) Model {
	t.Helper()
	m = press(t, m, "1")
	return press(t, m, "enter")
}

func TestStoreUnavailableIsExplained(t *testing.T) {
	m := New(Options{ConfigResult: config.Load(filepath.Join(t.TempDir(), "c.yaml")), StoreErr: "unable to open database file: permission denied"})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = drain(t, next.(Model), m.Init())
	if m.Status().Kind != components.StatusWarning || !strings.Contains(m.Status().Text, "scanning is off") {
		t.Errorf("status = %+v", m.Status())
	}
	m = press(t, m, "enter")
	if !strings.Contains(m.View(), "Not available: ATLAS can't open its saved data") {
		t.Error("scan preview should explain why it is unavailable")
	}
	m = press(t, m, "enter")
	if m.Scanning() {
		t.Fatal("an unavailable plan must not start")
	}
	m = press(t, m, "2")
	if !strings.Contains(m.View(), "can't be loaded") || strings.Contains(m.View(), "permission denied") {
		t.Error("My Software should show a plain error, details hidden in Simple mode")
	}
	m = press(t, m, "m")
	if !strings.Contains(m.View(), "permission denied") {
		t.Error("Advanced mode should show the technical reason")
	}
}

func TestEmptyMySoftwareInvitesScan(t *testing.T) {
	m, _ := newTestModel(t)
	m = press(t, m, "2")
	if !strings.Contains(m.View(), "You haven't scanned yet") {
		t.Fatal("empty state expected")
	}
	m = press(t, m, "s")
	if !m.PreviewOpen() {
		t.Error("s should open the scan preview")
	}
}

func TestDataSurvivesRestart(t *testing.T) {
	inv, _ := newTestInventory(t)
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	m := scanFromHome(t, boot(t, inv, cfg))
	if m.Data().Summary == nil {
		t.Fatal("scan should save")
	}
	m2 := boot(t, inv, cfg)
	if d := m2.Data(); d.Summary == nil || len(d.Items) != 3 || len(d.Activity) != 1 {
		t.Errorf("restarted model should load saved data: %+v", d)
	}
}

func TestMySoftwareScrollsLongLists(t *testing.T) {
	var many []core.SoftwareItem
	for i := 0; i < 80; i++ {
		many = append(many, core.SoftwareItem{Name: "App " + string(rune('A'+i/26)) + string(rune('a'+i%26)), Source: "one", Kind: core.KindApp})
	}
	inv, _ := newTestInventory(t, testScanner{id: "one", items: many})
	m := scanFromHome(t, boot(t, inv, filepath.Join(t.TempDir(), "c.yaml")))
	m = press(t, m, "2")
	m = press(t, m, "end")
	v := m.View()
	if !strings.Contains(v, "App Db") || strings.Contains(v, "App Aa") {
		t.Error("end should scroll to the last item")
	}
	if lines := strings.Count(v, "\n") + 1; lines > 30 {
		t.Errorf("view has %d lines for a 30-line terminal", lines)
	}
	_ = context.Background
}
