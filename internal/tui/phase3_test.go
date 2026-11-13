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
	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/nav"
	"github.com/deepakraj1207/atlas/internal/tui/screens"
	"github.com/deepakraj1207/atlas/internal/updates"
)

// testManager is a test double; its updates are test data only.
type testManager struct {
	id      string
	ups     []core.Update
	err     error
	blocked string
	elev    core.Elevation
}

func (m testManager) ID() string     { return m.id }
func (m testManager) Label() string  { return "Manager " + m.id }
func (m testManager) Detail() string { return "test" }
func (m testManager) CheckUpdates(context.Context) ([]core.Update, error) {
	return m.ups, m.err
}
func (m testManager) PlanUpdates(ups []core.Update) []core.UpdateStep {
	args := []string{"upgrade"}
	for _, u := range ups {
		args = append(args, u.PackageID)
	}
	return []core.UpdateStep{{
		Manager: m.id, Label: m.Label(), Updates: ups, Blocked: m.blocked, Elevation: m.elev,
		Commands: []core.Command{{Program: "pm-" + m.id, Args: args}},
		Rollback: "reinstall the old version with pm-" + m.id,
	}}
}

type env struct {
	db    *database.DB
	upd   *updates.Service
	calls *[]string
	exits map[string]int
}

func newEnv(t *testing.T, managers ...core.PackageManager) env {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "atlas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	e := env{db: db, calls: &[]string{}, exits: map[string]int{}}
	e.upd = updates.NewService(managers, db, nil, nil).WithProcessStarter(
		func(c core.Command, in io.Reader, out, errw io.Writer) (int, error) {
			*e.calls = append(*e.calls, c.String())
			return e.exits[c.Program], nil
		})
	return e
}

// bootEnv builds a model whose terminal handover runs the job in-process
// with a scripted Enter keypress.
func bootEnv(t *testing.T, e env, out *bytes.Buffer) Model {
	t.Helper()
	inv := inventory.NewService([]core.Scanner{testScanner{id: "one", items: testItems()}}, e.db, nil, nil)
	m := New(Options{ConfigResult: config.Load(filepath.Join(t.TempDir(), "c.yaml")), Inventory: inv, Updates: e.upd, DBPath: "test.db"})
	m.exec = func(j core.TerminalJob, cb tea.ExecCallback) tea.Cmd {
		return func() tea.Msg {
			j.SetStdin(strings.NewReader("\n"))
			j.SetStdout(out)
			j.SetStderr(out)
			return cb(j.Run())
		}
	}
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	return drain(t, next.(Model), m.Init())
}

func threeUpdates() []core.Update {
	return []core.Update{
		{Manager: "a", PackageID: "editor", Name: "Editor", CurrentVersion: "1.0", AvailableVersion: "1.1"},
		{Manager: "a", PackageID: "shell", Name: "Shell", CurrentVersion: "2.0", AvailableVersion: "2.5"},
		{Manager: "a", PackageID: "pinned", Name: "Pinned Tool", CurrentVersion: "3", AvailableVersion: "4", Held: true, Note: "pinned by you"},
	}
}

func checkFromHome(t *testing.T, m Model) Model {
	t.Helper()
	m = press(t, m, "1")
	m = press(t, m, "right") // Check for Updates
	m = press(t, m, "enter")
	if !m.PreviewOpen() || !strings.Contains(m.View(), "Checking only reads version information") {
		t.Fatalf("check must preview first:\n%s", m.View())
	}
	return press(t, m, "enter")
}

func TestCheckForUpdatesFlow(t *testing.T) {
	var out bytes.Buffer
	e := newEnv(t, testManager{id: "a", ups: threeUpdates()}, testManager{id: "b", err: core.ErrSourceUnavailable})
	m := bootEnv(t, e, &out)
	if !strings.Contains(m.View(), "not checked yet") {
		t.Error("Home should say updates were not checked yet")
	}
	m = checkFromHome(t, m)
	if m.Checking() {
		t.Fatal("check should have finished")
	}
	if st := m.Status(); st.Kind != components.StatusSuccess || !strings.Contains(st.Text, "2 updates available") {
		t.Errorf("status = %+v (held updates must not be counted)", st)
	}
	if !strings.Contains(m.View(), "2 updates are waiting") {
		t.Error("Home should recommend reviewing updates")
	}
	m = press(t, m, "3")
	v := m.View()
	for _, want := range []string{"Editor", "1.0", "1.1", "Pinned Tool (held)", "1 held back", "2 app updates · checked just now", "Press u to update all 2 apps.", "Manager a"} {
		if !strings.Contains(v, want) {
			t.Errorf("Updates screen missing %q:\n%s", want, v)
		}
	}
	m = press(t, m, "8")
	if !strings.Contains(m.View(), "Checked for updates") {
		t.Error("the check should be in Activity History")
	}
}

func TestUpdateAllPreviewsThenRunsAndRecords(t *testing.T) {
	var out bytes.Buffer
	e := newEnv(t, testManager{id: "a", ups: threeUpdates(), elev: core.ElevationSudo})
	m := checkFromHome(t, scanFromHome(t, bootEnv(t, e, &out)))
	m = press(t, m, "3")
	m = press(t, m, "u")
	if !m.PreviewOpen() {
		t.Fatal("u must open a preview, never run directly")
	}
	v := m.View()
	for _, want := range []string{"Install 2 updates", "Editor: 1.0 → 1.1", "Shell: 2.0 → 2.5", "sudo", "never sees or stores it", "Can it be undone?", "reinstall the old version"} {
		if !strings.Contains(v, want) {
			t.Errorf("apply preview missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "Pinned Tool") {
		t.Error("held updates must not be in update all")
	}
	if len(*e.calls) != 0 {
		t.Fatal("nothing may run before confirmation")
	}

	m = press(t, m, "esc")
	if len(*e.calls) != 0 || m.PreviewOpen() {
		t.Fatal("esc must cancel without running anything")
	}

	m = press(t, m, "u")
	m = press(t, m, "enter")
	if got := strings.Join(*e.calls, ";"); got != "pm-a upgrade editor shell" {
		t.Errorf("commands run = %q", got)
	}
	if !strings.Contains(out.String(), "ATLAS never answers for you") || !strings.Contains(out.String(), "Press Enter to return to ATLAS") {
		t.Errorf("terminal output = %s", out.String())
	}
	if st := m.Status(); st.Kind != components.StatusSuccess || !strings.Contains(st.Text, "Installed 2 updates") {
		t.Errorf("status = %+v", st)
	}
	apps, _ := m.Data().PendingUpdates()
	if apps != 0 || len(m.Data().Updates) != 1 {
		t.Errorf("applied updates should be removed, held kept: %+v", m.Data().Updates)
	}
	if acts := m.Data().Activity; len(acts) == 0 || acts[0].Kind != "updates.apply" || acts[0].Outcome != core.OutcomeSuccess {
		t.Errorf("apply must be recorded: %+v", acts)
	}
	m = press(t, m, "1")
	if !strings.Contains(m.View(), "You installed updates after your last scan") {
		t.Error("Home should suggest a rescan after updates")
	}
}

func TestUpdateSelectedOnlyAndFailureIsReported(t *testing.T) {
	var out bytes.Buffer
	e := newEnv(t, testManager{id: "a", ups: threeUpdates()})
	e.exits["pm-a"] = 1
	m := checkFromHome(t, bootEnv(t, e, &out))
	m = press(t, m, "3")
	m = press(t, m, "down") // Pinned Tool (sorted by name: Editor, Pinned Tool, Shell)
	m = press(t, m, " ")
	if len(m.Screen(nav.Updates).(*screens.Updates).Selected()) != 0 {
		t.Error("held updates can't be selected")
	}
	m = press(t, m, "down")
	m = press(t, m, " ")
	m = press(t, m, "enter")
	if !strings.Contains(m.View(), "Update Shell") || strings.Contains(m.View(), "Editor: 1.0") {
		t.Fatalf("only the selected update should be planned:\n%s", m.View())
	}
	m = press(t, m, "enter")
	if got := strings.Join(*e.calls, ";"); got != "pm-a upgrade shell" {
		t.Errorf("commands = %q", got)
	}
	if st := m.Status(); st.Kind != components.StatusError || !strings.Contains(st.Text, "didn't work") {
		t.Errorf("failure must be reported: %+v", st)
	}
	if apps, _ := m.Data().PendingUpdates(); apps != 2 {
		t.Error("a failed update must stay in the list")
	}
	m = press(t, m, "m")
	m = press(t, m, "8")
	if !strings.Contains(m.View(), "exit code 1") {
		t.Error("Advanced history should show the exit code")
	}
}

func TestBlockedStepsAreExplainedAndNotRun(t *testing.T) {
	var out bytes.Buffer
	e := newEnv(t, testManager{id: "a", ups: threeUpdates()[:1], blocked: "needs an administrator terminal"})
	m := checkFromHome(t, bootEnv(t, e, &out))
	m = press(t, m, "3")
	m = press(t, m, "u")
	v := m.View()
	if !strings.Contains(v, "Not available") || !strings.Contains(v, "Not included: Editor. needs an administrator terminal") {
		t.Errorf("blocked plan must explain itself:\n%s", v)
	}
	m = press(t, m, "enter")
	if len(*e.calls) != 0 {
		t.Error("an unavailable plan must never run")
	}
}

func TestCheckCanBeStoppedAndExcludesScan(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	var out bytes.Buffer
	e := newEnv(t, blockingManager{block: block})
	m := bootEnv(t, e, &out)
	next, _ := m.Update(screens.StartCheckMsg{})
	m = next.(Model)
	if !m.Checking() || !strings.Contains(m.View(), "Checking for updates") {
		t.Fatal("check should be running and visible")
	}
	next, _ = m.Update(screens.StartScanMsg{})
	m = next.(Model)
	if m.Scanning() || !strings.Contains(m.Status().Text, "Wait for the running task") {
		t.Error("a scan must not start during a check")
	}
	next, _ = m.Update(key("x"))
	m = next.(Model)
	m = drain(t, m, listen(m.check.events))
	if m.Checking() || !strings.Contains(m.Status().Text, "Update check stopped") {
		t.Errorf("status = %+v", m.Status())
	}
	if m.Data().UpdateSummary != nil {
		t.Error("a stopped check must not save results")
	}
}

type blockingManager struct{ block chan struct{} }

func (blockingManager) ID() string     { return "slow" }
func (blockingManager) Label() string  { return "Slow" }
func (blockingManager) Detail() string { return "" }
func (b blockingManager) CheckUpdates(ctx context.Context) ([]core.Update, error) {
	select {
	case <-b.block:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return nil, nil
}
func (blockingManager) PlanUpdates([]core.Update) []core.UpdateStep { return nil }

func TestUpToDateAndNeverCheckedStates(t *testing.T) {
	var out bytes.Buffer
	e := newEnv(t, testManager{id: "a"})
	m := bootEnv(t, e, &out)
	m = press(t, m, "3")
	if !strings.Contains(m.View(), "You haven't checked for updates yet") {
		t.Fatal("never-checked state expected")
	}
	m = press(t, m, "c")
	if !m.PreviewOpen() {
		t.Fatal("c should open the check preview")
	}
	m = press(t, m, "enter")
	if !strings.Contains(m.View(), "Everything checked is up to date") || !strings.Contains(m.View(), "Manager a: up to date") {
		t.Errorf("up-to-date state:\n%s", m.View())
	}
	m = press(t, m, "u")
	if m.PreviewOpen() {
		t.Error("u with nothing to update should do nothing")
	}
}

func TestComponentsAreListedSeparatelyButIncludedInUpdateAll(t *testing.T) {
	var out bytes.Buffer
	ups := []core.Update{
		{Manager: "a", PackageID: "app", Name: "App", AvailableVersion: "2"},
		{Manager: "a", PackageID: "libx", Name: "libx", AvailableVersion: "9", Component: true},
	}
	e := newEnv(t, testManager{id: "a", ups: ups})
	m := checkFromHome(t, bootEnv(t, e, &out))
	m = press(t, m, "3")
	v := m.View()
	if strings.Contains(v, "libx") || !strings.Contains(v, "1 system component update (f to show)") {
		t.Errorf("components should be hidden but counted:\n%s", v)
	}
	m = press(t, m, "f")
	if !strings.Contains(m.View(), "libx (system)") {
		t.Error("f should show component updates")
	}
	m = press(t, m, "u")
	if !strings.Contains(m.View(), "Install 2 updates") {
		t.Error("update all must include system components")
	}
}

func TestFailedCheckExplainsWhyOnUpdatesScreen(t *testing.T) {
	var out bytes.Buffer
	e := newEnv(t, testManager{id: "a", err: &core.SourceError{Message: "No package information downloaded yet", Details: "run 'sudo apt update'"}})
	m := checkFromHome(t, bootEnv(t, e, &out))
	if st := m.Status(); st.Kind != components.StatusError || !strings.Contains(st.Text, "Manager a: No package information downloaded yet") {
		t.Errorf("status = %+v", st)
	}
	m = press(t, m, "3")
	v := m.View()
	for _, want := range []string{"The update check didn't work", "Manager a: No package information downloaded yet", "To fix: run 'sudo apt update'"} {
		if !strings.Contains(v, want) {
			t.Errorf("Updates screen missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "You haven't checked") {
		t.Error("a failed check must not look like no check at all")
	}
}

func TestInstallCancelledStatus(t *testing.T) {
	st := installResultStatus([]core.StepResult{{Status: core.StepStopped, Note: "You cancelled it"}}, nil)
	if !strings.Contains(st.Text, "Install cancelled") {
		t.Errorf("declining the permission prompt should read as cancelled, got %q", st.Text)
	}
	st = installResultStatus([]core.StepResult{{Status: core.StepFailed}}, nil)
	if !strings.Contains(st.Text, "didn't finish") {
		t.Errorf("a real failure still reads as a failure, got %q", st.Text)
	}
}
