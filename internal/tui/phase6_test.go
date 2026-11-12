package tui

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/database"
	"github.com/deepakraj1207/atlas/internal/discover"
	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/nav"
	"github.com/deepakraj1207/atlas/internal/tui/screens"
)

func bootDiscover(t *testing.T, scan bool) (Model, *[]string) {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(context.Background(), filepath.Join(dir, "atlas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	yes := true
	calls := &[]string{}
	dsc, err := discover.NewService(discover.Config{Windows: &yes}, db, nil)
	if err != nil {
		t.Fatal(err)
	}
	dsc.WithProcessStarter(func(c core.Command, in io.Reader, out, errw io.Writer) (int, error) {
		*calls = append(*calls, c.String())
		return 0, nil
	})
	// Test data only: a Windows-like computer with winget, Git and Python.
	inv := inventory.NewService([]core.Scanner{testScanner{id: inventory.SourceWinget, items: []core.SoftwareItem{
		{Name: "Git", Version: "2.44.0", PackageID: "Git.Git", Source: inventory.SourceWinget, Kind: core.KindApp},
		{Name: "Python 3.12.3 (64-bit)", Version: "3.12.3", Source: inventory.SourceWinget, Kind: core.KindApp},
	}}}, db, nil, nil)
	out := &bytes.Buffer{}
	m := New(Options{ConfigResult: config.Load(filepath.Join(dir, "c.yaml")), Inventory: inv, Discover: dsc, DBPath: "test.db"})
	m.exec = func(j core.TerminalJob, cb tea.ExecCallback) tea.Cmd {
		return func() tea.Msg {
			j.SetStdin(strings.NewReader("\n"))
			j.SetStdout(out)
			j.SetStderr(out)
			return cb(j.Run())
		}
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = drain(t, next.(Model), m.Init())
	if scan {
		m = scanFromHome(t, m)
	}
	return press(t, m, "7"), calls
}

func TestDiscoverSuggestsAndInstalls(t *testing.T) {
	m, calls := bootDiscover(t, true)
	v := m.View()
	for _, want := range []string{"Suggested for you", "GitHub CLI", "Why it's suggested: useful for almost everyone", "doesn't contact the internet", "Catalogue as of 2026-11"} {
		if !strings.Contains(v, want) {
			t.Errorf("Discover missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "btop") {
		t.Error("Linux-only tools must not appear on Windows")
	}
	// The first suggestion is highlighted; find GitHub CLI and install it.
	for i := 0; i < 20 && !strings.Contains(highlighted(m), "GitHub CLI"); i++ {
		m = press(t, m, "down")
	}
	if !strings.Contains(m.View(), "Why it's suggested: because you have Git") {
		t.Errorf("the reason should be shown:\n%s", m.View())
	}
	m = press(t, m, "enter")
	if !m.PreviewOpen() {
		t.Fatalf("install must preview first:\n%s", m.View())
	}
	v = m.View()
	for _, want := range []string{"Install GitHub CLI", "Installed: GitHub CLI · Windows Package Manager", "ATLAS never answers for you", "remove it later from My Software", "Enter to continue"} {
		if !strings.Contains(v, want) {
			t.Errorf("install preview missing %q:\n%s", want, v)
		}
	}
	if len(*calls) != 0 {
		t.Fatal("nothing may run before confirmation")
	}
	m = press(t, m, "enter")
	if got := strings.Join(*calls, ";"); got != "winget install --id GitHub.cli --exact --source winget" {
		t.Errorf("commands = %q", got)
	}
	if st := m.Status(); st.Kind != components.StatusSuccess || !strings.Contains(st.Text, "Installed 1") {
		t.Errorf("status = %+v", st)
	}
	if !m.Data().InstalledNow["gh"] || strings.Contains(suggestedSection(m.View()), "GitHub CLI") {
		t.Error("an installed tool leaves the suggestions")
	}
	if acts := m.Data().Activity; len(acts) == 0 || acts[0].Kind != "discover.install" {
		t.Errorf("install must be recorded: %+v", acts)
	}
}

// highlighted returns the line with the selection marker.
func highlighted(m Model) string {
	for _, l := range strings.Split(m.View(), "\n") {
		if strings.Contains(l, "› [") || strings.Contains(l, "›  ") {
			return l
		}
	}
	return ""
}

func suggestedSection(v string) string {
	i := strings.Index(v, "Suggested for you")
	if i < 0 {
		return ""
	}
	rest := v[i:]
	for _, c := range []string{"Internet", "Office", "Media", "Security", "Files", "Utilities", "Development"} {
		if j := strings.Index(rest, "\n"+c); j > 0 {
			return rest[:j]
		}
	}
	return rest
}

func TestDiscoverAlreadyInstalledAndMultiSelect(t *testing.T) {
	m, calls := bootDiscover(t, true)
	scr := m.Screen(nav.DiscoverApps).(*screens.Discover)
	// Switch to Development and find Git, which is installed.
	for i := 0; i < 8 && !strings.Contains(m.View(), "[Development]"); i++ {
		m = press(t, m, "right")
	}
	for i := 0; i < 20 && !strings.Contains(highlighted(m), "Git · "); i++ {
		m = press(t, m, "down")
	}
	if !strings.Contains(m.View(), "Already installed as Git") {
		t.Fatalf("installed tools should say so:\n%s", m.View())
	}
	m = press(t, m, " ")
	if len(scr.Selected()) != 0 {
		t.Error("installed tools can't be selected")
	}
	m = press(t, m, "enter")
	if m.PreviewOpen() || !strings.Contains(m.Status().Text, "already installed") {
		t.Errorf("enter on an installed tool explains instead of installing: %+v", m.Status())
	}

	m = press(t, m, "home")
	m = press(t, m, " ")
	m = press(t, m, "down")
	for strings.Contains(highlighted(m), "✓") || strings.Contains(highlighted(m), "Git · ") {
		m = press(t, m, "down")
	}
	m = press(t, m, " ")
	if len(scr.Selected()) != 2 {
		t.Fatalf("two tools should be selected, got %v", scr.Selected())
	}
	m = press(t, m, "enter")
	if !strings.Contains(m.View(), "Install 2 apps") {
		t.Errorf("multi-select preview:\n%s", m.View())
	}
	m = press(t, m, "esc")
	if len(*calls) != 0 {
		t.Error("esc must not install anything")
	}
}

func TestDiscoverBeforeScanning(t *testing.T) {
	m, calls := bootDiscover(t, false)
	if !strings.Contains(m.View(), "Scan your system first (press s)") {
		t.Errorf("unscanned Discover should ask for a scan:\n%s", m.View())
	}
	m = press(t, m, "enter")
	if m.PreviewOpen() || !strings.Contains(m.Status().Text, "Scan your system first") || len(*calls) != 0 {
		t.Errorf("nothing is installable before a scan: %+v", m.Status())
	}
	m = press(t, m, "s")
	if !m.PreviewOpen() || !strings.Contains(m.View(), "Scan My System") {
		t.Error("s should offer the scan")
	}
}

func TestDiscoverFitsEveryTerminalSize(t *testing.T) {
	m, _ := bootDiscover(t, true)
	for _, size := range [][2]int{{40, 10}, {60, 14}, {60, 18}, {80, 24}, {120, 30}, {200, 50}} {
		next, _ := m.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		mm := next.(Model)
		for _, k := range []string{"", "down", "end"} {
			if k != "" {
				mm = press(t, mm, k)
			}
			if lines := strings.Count(mm.View(), "\n") + 1; lines > size[1] {
				t.Errorf("%dx%d after %q: %d lines", size[0], size[1], k, lines)
			}
		}
	}
}
