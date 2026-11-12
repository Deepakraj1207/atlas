package tui

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/analysis"
	"github.com/deepakraj1207/atlas/internal/cleanup"
	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/database"
	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/nav"
	"github.com/deepakraj1207/atlas/internal/tui/screens"
)

const phase5Status = `Package: python3
Status: install ok installed

Package: meld
Status: install ok installed
Depends: python3

Package: lonely-tool
Status: install ok installed
`

// tidyStub is a test double; its targets are test data only.
type tidyStub struct{ steps []core.UpdateStep }

func (tidyStub) ID() string                                        { return "stub" }
func (tidyStub) Label() string                                     { return "Stub caches" }
func (tidyStub) Detail() string                                    { return "test" }
func (t tidyStub) Find(context.Context) ([]core.UpdateStep, error) { return t.steps, nil }

type cleanEnv struct {
	m     Model
	calls *[]string
	out   *bytes.Buffer
}

func phase5Items() []core.SoftwareItem {
	d := inventory.SourceDpkg
	return []core.SoftwareItem{
		{Name: "python3", PackageID: "python3", Version: "3.12", Source: d, Kind: core.KindApp},
		{Name: "meld", PackageID: "meld", Version: "3.22", Source: d, Kind: core.KindApp},
		{Name: "lonely-tool", PackageID: "lonely-tool", Version: "1.0", Source: d, Kind: core.KindApp},
		{Name: "Git", Version: "2.44.0", Source: inventory.SourceRegistry, Kind: core.KindApp},
		{Name: "git", Version: "2.45.1", Source: inventory.SourceScoop, Kind: core.KindApp},
	}
}

func bootCleanup(t *testing.T) cleanEnv {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(context.Background(), filepath.Join(dir, "atlas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	status := filepath.Join(dir, "status")
	if err := writeFile(status, phase5Status); err != nil {
		t.Fatal(err)
	}
	deps := analysis.NewDpkgDeps(status)
	calls := &[]string{}
	cln := cleanup.NewService(cleanup.Config{
		Removers: []cleanup.Remover{cleanup.NewAptRemover(false), cleanup.NewScoopRemover(false), cleanup.NewWingetRemover(false)},
		Deps:     []analysis.Source{deps},
		Tidiers: []cleanup.Tidier{tidyStub{steps: []core.UpdateStep{{
			Manager: "apt", Label: "Downloaded package files (APT)", Action: core.ActionClean, Bytes: 812_000_000,
			Updates:  []core.Update{{Name: "40 downloaded package files"}},
			Commands: []core.Command{{Program: "apt-get", Args: []string{"clean"}}},
			Rollback: "APT downloads them again when needed",
		}}}},
	}, db, nil).WithProcessStarter(func(c core.Command, in io.Reader, out, errw io.Writer) (int, error) {
		*calls = append(*calls, c.String())
		return 0, nil
	})
	inv := inventory.NewService([]core.Scanner{testScanner{id: "one", items: phase5Items()}}, db, nil, nil)
	ana := analysis.NewService([]analysis.Source{deps}, db, nil, false)
	out := &bytes.Buffer{}
	m := New(Options{ConfigResult: config.Load(filepath.Join(dir, "c.yaml")), Inventory: inv, Analysis: ana, Cleanup: cln, DBPath: "test.db"})
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
	return cleanEnv{m: scanFromHome(t, m), calls: calls, out: out}
}

// selectSoftware opens My Software, searches for name and selects it.
func selectSoftware(t *testing.T, m Model, name string) Model {
	t.Helper()
	m = press(t, m, "2")
	m = press(t, m, "/")
	m = typeText(t, m, name)
	return press(t, m, "enter")
}

func TestRemoveFromMySoftwareNeedsDeliberateConfirmation(t *testing.T) {
	e := bootCleanup(t)
	m := selectSoftware(t, e.m, "lonely")
	m = press(t, m, "r")
	if !m.PreviewOpen() {
		t.Fatalf("r should open a removal preview:\n%s", m.View())
	}
	v := m.View()
	for _, want := range []string{"Remove lonely-tool", "Removed: lonely-tool 1.0", "checked dependencies just now", "never purges", "apt-get install lonely-tool", "Press r to go ahead"} {
		if !strings.Contains(v, want) {
			t.Errorf("removal preview missing %q:\n%s", want, v)
		}
	}
	m = press(t, m, "enter")
	if !m.PreviewOpen() || len(*e.calls) != 0 || !strings.Contains(m.Status().Text, "To go ahead, press r") {
		t.Fatal("Enter must never confirm a removal")
	}
	m = press(t, m, "esc")
	if m.PreviewOpen() || len(*e.calls) != 0 {
		t.Fatal("Esc must cancel without running anything")
	}

	m = press(t, m, "r")
	m = press(t, m, "r")
	if got := strings.Join(*e.calls, ";"); got != "sudo apt-get remove lonely-tool" {
		t.Errorf("commands run = %q", got)
	}
	if !strings.Contains(e.out.String(), "removing lonely-tool") || !strings.Contains(e.out.String(), "1 removed") {
		t.Errorf("terminal output = %s", e.out.String())
	}
	if st := m.Status(); st.Kind != components.StatusSuccess || !strings.Contains(st.Text, "Cleanup finished") {
		t.Errorf("status = %+v", st)
	}
	for _, it := range m.Data().Items {
		if it.Name == "lonely-tool" {
			t.Error("removed software must leave My Software")
		}
	}
	if acts := m.Data().Activity; acts[0].Kind != "cleanup.run" || acts[0].Outcome != core.OutcomeSuccess {
		t.Errorf("removal must be recorded: %+v", acts[0])
	}
}

func TestBlockedRemovalExplainsAndCannotBeForced(t *testing.T) {
	e := bootCleanup(t)
	m := selectSoftware(t, e.m, "python3")
	m = press(t, m, "r")
	v := m.View()
	if !strings.Contains(v, "Not available") || !strings.Contains(v, "Not removed: python3. Needed by meld") {
		t.Fatalf("blocked preview should explain:\n%s", v)
	}
	if strings.Contains(v, "Press r to go ahead") {
		t.Error("an unavailable plan must not offer to go ahead")
	}
	m = press(t, m, "r")
	m = press(t, m, "enter")
	if len(*e.calls) != 0 {
		t.Errorf("a blocked removal must never run, got %v", *e.calls)
	}
}

func TestCleanupScreenSuggestionsAndTidyUps(t *testing.T) {
	e := bootCleanup(t)
	m := press(t, e.m, "6")
	if !strings.Contains(m.View(), "Analyze your software (press a)") || !strings.Contains(m.View(), "Press c to look for package caches") {
		t.Errorf("empty Cleanup should explain both sections:\n%s", m.View())
	}

	m = press(t, m, "a")
	m = press(t, m, "enter") // confirm the (read-only) analysis preview
	m = press(t, m, "6")
	v := m.View()
	if !strings.Contains(v, "Git 2.44.0 · Test windows-registry · installed more than once") && !strings.Contains(v, "installed more than once") {
		t.Errorf("duplicate suggestion expected:\n%s", v)
	}

	m = press(t, m, "c")
	if !m.PreviewOpen() || !strings.Contains(m.View(), "Looking only reads information") {
		t.Fatalf("looking for things to tidy previews first:\n%s", m.View())
	}
	m = press(t, m, "enter")
	if !strings.Contains(m.View(), "Downloaded package files (APT) · 40 downloaded package files · 812.0 MB") {
		t.Fatalf("tidy entry expected:\n%s", m.View())
	}
	if len(*e.calls) != 0 {
		t.Fatal("looking must not run anything")
	}

	m = press(t, m, "end")
	m = press(t, m, " ")
	if len(m.Screen(nav.Cleanup).(*screens.Cleanup).Selected()) != 1 {
		t.Fatal("space should select the tidy entry")
	}
	m = press(t, m, "enter")
	v = m.View()
	for _, want := range []string{"Tidy up", "Deleted: 40 downloaded package files", "about 812.0 MB", "Press r to go ahead"} {
		if !strings.Contains(v, want) {
			t.Errorf("tidy preview missing %q:\n%s", want, v)
		}
	}
	m = press(t, m, "r")
	if got := strings.Join(*e.calls, ";"); got != "apt-get clean" {
		t.Errorf("commands = %q", got)
	}
	if !strings.Contains(m.Status().Text, "freeing about 812.0 MB") {
		t.Errorf("status = %+v", m.Status())
	}
	if strings.Contains(m.View(), "40 downloaded package files ·") {
		t.Error("a finished clean-up should leave the list")
	}
}

func TestEnterStillConfirmsNonDestructivePreviews(t *testing.T) {
	e := bootCleanup(t)
	m := press(t, e.m, "6")
	m = press(t, m, "c")
	m = press(t, m, "r") // r is not a confirm key for read-only previews
	if !m.PreviewOpen() {
		t.Fatal("r must not confirm a non-destructive preview")
	}
	m = press(t, m, "enter")
	if m.PreviewOpen() || m.Data().Tidy == nil {
		t.Error("Enter confirms read-only previews")
	}
}
