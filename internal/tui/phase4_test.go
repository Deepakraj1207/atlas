package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/analysis"
	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/database"
	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/screens"
)

// analysisSource is a test double; its data is test data only.
type analysisSource struct {
	id    string
	in    analysis.Input
	err   error
	block chan struct{}
}

func (s analysisSource) ID() string     { return s.id }
func (s analysisSource) Label() string  { return "Data " + s.id }
func (s analysisSource) Detail() string { return "test" }
func (s analysisSource) Read(ctx context.Context) (analysis.Input, error) {
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return analysis.Input{}, ctx.Err()
		}
	}
	return s.in, s.err
}

func runtimeItems() []core.SoftwareItem {
	reg := inventory.SourceRegistry
	return []core.SoftwareItem{
		{Name: "Python 3.8.10 (64-bit)", Version: "3.8.10", Source: reg, Kind: core.KindApp},
		{Name: "Git", Version: "2.44.0", Source: reg, Kind: core.KindApp},
		{Name: "git", Version: "2.45.1", Source: "scoop", Kind: core.KindApp},
		{Name: "Editor", Version: "1.0", Source: reg, Kind: core.KindApp},
	}
}

func bootAnalysis(t *testing.T, items []core.SoftwareItem, sources ...analysis.Source) Model {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "atlas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	inv := inventory.NewService([]core.Scanner{testScanner{id: "one", items: items}}, db, nil, nil)
	ana := analysis.NewService(sources, db, nil, false)
	m := New(Options{ConfigResult: config.Load(filepath.Join(t.TempDir(), "c.yaml")), Inventory: inv, Analysis: ana, DBPath: "test.db"})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	return drain(t, next.(Model), m.Init())
}

func analyzeFromHome(t *testing.T, m Model) Model {
	t.Helper()
	m = press(t, m, "1")
	m = press(t, m, "right")
	m = press(t, m, "right") // Analyze My Software
	m = press(t, m, "enter")
	if !m.PreviewOpen() || !strings.Contains(m.View(), "Analysis only reads information") {
		t.Fatalf("analysis must preview first:\n%s", m.View())
	}
	return press(t, m, "enter")
}

func TestAnalysisNeedsAScanFirst(t *testing.T) {
	m := bootAnalysis(t, runtimeItems())
	m = press(t, m, "4")
	if !strings.Contains(m.View(), "Scan your system first") {
		t.Error("Smart Analysis should ask for a scan first")
	}
	m = press(t, m, "5")
	if !strings.Contains(m.View(), "Scan your system first") {
		t.Error("Dependencies should ask for a scan first")
	}
	m = press(t, m, "1")
	m = press(t, m, "right")
	m = press(t, m, "right")
	m = press(t, m, "enter")
	if !strings.Contains(m.View(), "Not available: there is no software list to analyze yet") {
		t.Errorf("analyze preview should explain it needs a scan:\n%s", m.View())
	}
	m = press(t, m, "enter")
	if m.Analyzing() {
		t.Error("an unavailable analysis must not start")
	}
}

func TestAnalysisFlowAcrossScreens(t *testing.T) {
	m := scanFromHome(t, bootAnalysis(t, runtimeItems()))
	if !strings.Contains(m.View(), "not run yet") || !strings.Contains(m.View(), "Analyze your software to spot") {
		t.Error("Home should invite an analysis after a scan")
	}
	m = analyzeFromHome(t, m)
	if m.Analyzing() {
		t.Fatal("analysis should have finished")
	}
	if st := m.Status(); st.Kind != components.StatusWarning || !strings.Contains(st.Text, "1 worth your attention") {
		t.Errorf("status = %+v", st)
	}
	home := m.View()
	if !strings.Contains(home, "1 worth your attention") || !strings.Contains(home, "Smart Analysis found 1 things") {
		t.Errorf("Home should summarise the analysis:\n%s", home)
	}

	m = press(t, m, "4")
	v := m.View()
	for _, want := range []string{"Python 3.8 no longer gets security fixes", "Support for Python 3.8 ended on 7 Oct 2024", "Why it matters:", "What you can do:", "Source: Python release status", "Git is installed more than once"} {
		if !strings.Contains(v, want) {
			t.Errorf("Smart Analysis missing %q:\n%s", want, v)
		}
	}
	m = press(t, m, "end")
	v = m.View()
	for _, want := range []string{"What ATLAS checked", "Apps not opened in over a year: not checked", "Linux doesn't keep a record", "Same app installed by two different tools: 1 found"} {
		if !strings.Contains(v, want) {
			t.Errorf("checks section missing %q:\n%s", want, v)
		}
	}

	m = press(t, m, "5")
	v = m.View()
	for _, want := range []string{"Python 3.8", "support ended 7 Oct 2024", "not recorded", "Windows doesn't record which programs use this"} {
		if !strings.Contains(v, want) {
			t.Errorf("Dependencies missing %q:\n%s", want, v)
		}
	}

	m = press(t, m, "8")
	if !strings.Contains(m.View(), "Analyzed your software") {
		t.Error("analysis should be in Activity History")
	}
}

func TestDependenciesShowRealGraph(t *testing.T) {
	g := analysis.Input{GraphFor: inventory.SourceDpkg}
	dep, _ := analysis.NewDpkgDeps(writeStatus(t, `Package: python3.12
Status: install ok installed

Package: python3
Status: install ok installed
Depends: python3.12

Package: meld
Status: install ok installed
Depends: python3
`)).Read(context.Background())
	g.Graph = dep.Graph
	items := []core.SoftwareItem{
		{Name: "python3.12", PackageID: "python3.12", Version: "3.12.3-1", Source: inventory.SourceDpkg, Kind: core.KindComponent},
		{Name: "python3", PackageID: "python3", Source: inventory.SourceDpkg, Kind: core.KindApp},
		{Name: "meld", PackageID: "meld", Source: inventory.SourceDpkg, Kind: core.KindApp},
	}
	m := scanFromHome(t, bootAnalysis(t, items, analysisSource{id: "deps", in: g}))
	m = analyzeFromHome(t, m)
	m = press(t, m, "5")
	v := m.View()
	for _, want := range []string{"Python 3.12", "needed by 2", "Needed directly by: python3.", "1 more packages need it through those", "Removing it would break these"} {
		if !strings.Contains(v, want) {
			t.Errorf("Dependencies missing %q:\n%s", want, v)
		}
	}
	m = press(t, m, "m")
	if !strings.Contains(m.View(), "Dependency data: Data deps") {
		t.Error("Advanced mode should name the dependency data source")
	}
}

func writeStatus(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "status")
	if err := writeFile(p, body); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestStaleAnalysisAndClearing(t *testing.T) {
	m := scanFromHome(t, bootAnalysis(t, runtimeItems()))
	m = analyzeFromHome(t, m)
	m = press(t, m, "left")
	m = press(t, m, "left") // back to Scan My System
	m = scanFromHome(t, m)  // the list changes after the analysis
	m = press(t, m, "4")
	if !strings.Contains(m.View(), "Your software list changed after this analysis") {
		t.Error("a stale analysis should say so")
	}
	if !m.Data().AnalysisStale() {
		t.Error("AnalysisStale should be true")
	}
	m = press(t, m, "9")
	for i := 0; i < 4; i++ {
		m = press(t, m, "down")
	}
	m = press(t, m, "enter")
	if !strings.Contains(m.View(), "Smart Analysis results based on the list are removed too") {
		t.Errorf("clear preview should mention the analysis:\n%s", m.View())
	}
	m = press(t, m, "enter")
	if m.Data().Analysis != nil {
		t.Error("clearing the list must clear the analysis")
	}
}

func TestAnalysisCanBeStopped(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	m := scanFromHome(t, bootAnalysis(t, runtimeItems(), analysisSource{id: "slow", block: block}))
	next, _ := m.Update(screens.StartAnalysisMsg{})
	m = next.(Model)
	if !m.Analyzing() || !strings.Contains(m.View(), "Analyzing your software") {
		t.Fatal("analysis should be running and visible")
	}
	next, _ = m.Update(screens.StartScanMsg{})
	m = next.(Model)
	if m.Scanning() {
		t.Error("no scan during an analysis")
	}
	next, _ = m.Update(key("x"))
	m = next.(Model)
	m = drain(t, m, listen(m.analysis.events))
	if m.Analyzing() || !strings.Contains(m.Status().Text, "Analysis stopped") || m.Data().Analysis != nil {
		t.Errorf("stopped analysis: status %+v, saved %v", m.Status(), m.Data().Analysis != nil)
	}
}

func writeFile(p, body string) error { return os.WriteFile(p, []byte(body), 0o644) }

func TestDependenciesKeepGroupHeadingOnSmallTerminals(t *testing.T) {
	items := []core.SoftwareItem{
		{Name: "openjdk-21-jdk", Version: "21.0.4", Source: inventory.SourceDpkg, Kind: core.KindApp},
		{Name: "openjdk-21-jre", Version: "21.0.4", Source: inventory.SourceDpkg, Kind: core.KindApp},
		{Name: "openjdk-21-jre-headless", Version: "21.0.4", Source: inventory.SourceDpkg, Kind: core.KindApp},
	}
	m := analyzeFromHome(t, scanFromHome(t, bootAnalysis(t, items)))
	next, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 18})
	m = next.(Model)
	m = press(t, m, "5")
	m = press(t, m, "end")
	if v := m.View(); !strings.Contains(v, "Java 21") || !strings.Contains(v, "openjdk-21-jre-headless") {
		t.Errorf("the selected runtime's group heading should stay visible:\n%s", v)
	}
}
