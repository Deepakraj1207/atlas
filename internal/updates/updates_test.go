package updates

// Fixtures reproduce the shape of real command output. They are test data
// only and never shown to users.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/database"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

const wingetUpgradeFixture = "\r   - \r" +
	"Name                Id                     Version   Available Source\n" +
	"---------------------------------------------------------------------\n" +
	"Git                 Git.Git                2.44.0    2.45.1    winget\n" +
	"Some Long Tool Name Vendor.VeryLongIdent…  1.0       1.1       winget\n" +
	"3 upgrades available.\n" +
	"\n" +
	"The following packages have an upgrade available, but require explicit targeting for upgrade:\n" +
	"Name                Id                     Version   Available Source\n" +
	"---------------------------------------------------------------------\n" +
	"Pinned App          Vendor.Pinned          3.0       3.1       winget\n"

func TestParseWingetUpgrade(t *testing.T) {
	ups, err := parseWingetUpgrade(wingetUpgradeFixture)
	if err != nil || len(ups) != 3 {
		t.Fatalf("got %+v, %v", ups, err)
	}
	if g := ups[0]; g.PackageID != "Git.Git" || g.CurrentVersion != "2.44.0" || g.AvailableVersion != "2.45.1" || g.Origin != "winget" || g.Held {
		t.Errorf("git = %+v", g)
	}
	if ups[1].PackageID != "" || !strings.Contains(ups[1].Note, "shortened") {
		t.Errorf("truncated ID must be cleared: %+v", ups[1])
	}
	if !ups[2].Held {
		t.Error("explicit-targeting table should be held")
	}
	if ups, err := parseWingetUpgrade("No installed package found matching input criteria.\n"); err != nil || len(ups) != 0 {
		t.Errorf("no-updates message: %v %v", ups, err)
	}
	if _, err := parseWingetUpgrade("Something unexpected\n"); err == nil {
		t.Error("unknown output should be an error")
	}
}

func TestWingetPlan(t *testing.T) {
	ups, _ := parseWingetUpgrade(wingetUpgradeFixture)
	steps := NewWinget(nil, false).PlanUpdates(ups[:2])
	if len(steps) != 2 {
		t.Fatalf("one step per package, got %d", len(steps))
	}
	if got := steps[0].Commands[0].String(); got != "winget upgrade --id Git.Git --exact --source winget" {
		t.Errorf("command = %s", got)
	}
	if strings.Contains(steps[0].Commands[0].String(), "accept") {
		t.Error("ATLAS must never accept agreements on the user's behalf")
	}
	if steps[0].Elevation != core.ElevationInstaller || !strings.Contains(steps[0].Rollback, "--version 2.44.0") {
		t.Errorf("step = %+v", steps[0])
	}
	if steps[1].Blocked == "" || len(steps[1].Commands) != 0 {
		t.Error("a truncated ID must block the step")
	}
}

func TestChocolatey(t *testing.T) {
	run := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("7zip.install|23.1.0|24.8.0|false\ngit|2.44.0|2.45.1|true\nnot a line\n"),
			&inventory.CommandError{Command: "choco", ExitCode: 2}
	}
	m := NewChocolatey(run, false)
	ups, err := m.CheckUpdates(context.Background())
	if err != nil || len(ups) != 2 {
		t.Fatalf("exit code 2 means outdated packages exist: %+v %v", ups, err)
	}
	if !ups[1].Held || ups[0].Held {
		t.Error("pinned flag not honoured")
	}
	st := m.PlanUpdates(ups[:1])
	if len(st) != 1 || st[0].Blocked == "" || !strings.Contains(st[0].Blocked, "administrator") {
		t.Errorf("non-admin chocolatey must be blocked with a reason: %+v", st)
	}
	st = NewChocolatey(run, true).PlanUpdates(ups[:1])
	if st[0].Blocked != "" || st[0].Commands[0].String() != "choco upgrade 7zip.install" {
		t.Errorf("admin plan = %+v", st[0])
	}
	if ok, restart := st[0].Commands[0].Interpret(3010); !ok || !restart {
		t.Error("3010 should mean done, restart needed")
	}
	fail := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return nil, &inventory.CommandError{Command: "choco", ExitCode: 1}
	}
	if _, err := NewChocolatey(fail, false).CheckUpdates(context.Background()); err == nil {
		t.Error("other exit codes are failures")
	}
}

func TestScoop(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	write("apps/ripgrep/current/manifest.json", `{"version":"14.0.0"}`)
	write("apps/ripgrep/current/install.json", `{"bucket":"main"}`)
	write("buckets/main/bucket/ripgrep.json", `{"version":"14.1.0"}`)
	write("apps/fd/current/manifest.json", `{"version":"9.0.0"}`)
	write("apps/fd/current/install.json", `{"bucket":"main","hold":true}`)
	write("buckets/main/bucket/fd.json", `{"version":"10.0.0"}`)
	write("apps/same/current/manifest.json", `{"version":"1.0"}`)
	write("apps/same/current/install.json", `{"bucket":"extras"}`)
	write("buckets/extras/same.json", `{"version":"1.0"}`)
	// Newer Scoop names its files scoop-manifest.json / scoop-install.json.
	write("apps/zoxide/current/scoop-manifest.json", `{"version":"0.9.4"}`)
	write("apps/zoxide/current/scoop-install.json", `{"bucket":"main"}`)
	write("buckets/main/bucket/zoxide.json", `{"version":"0.9.6"}`)

	m := NewScoop([]inventory.ScoopRoot{{Dir: root, Scope: "user"}}, false)
	ups, err := m.CheckUpdates(context.Background())
	if err != nil || len(ups) != 3 {
		t.Fatalf("got %+v, %v", ups, err)
	}
	if z := ups[2]; z.PackageID != "zoxide" || z.CurrentVersion != "0.9.4" || z.AvailableVersion != "0.9.6" {
		t.Errorf("zoxide (new file names) = %+v", z)
	}
	if !ups[0].Held || ups[1].PackageID != "ripgrep" || ups[1].AvailableVersion != "14.1.0" || ups[1].Origin != "main" {
		t.Errorf("updates = %+v", ups)
	}
	st := m.PlanUpdates([]core.Update{ups[1], {Manager: "scoop", PackageID: "g", Scope: "machine"}})
	if len(st) != 2 || st[0].Commands[1].String() != "scoop update ripgrep" || st[1].Blocked == "" {
		t.Errorf("plan = %+v", st)
	}
	if _, err := NewScoop([]inventory.ScoopRoot{{Dir: filepath.Join(root, "x")}}, false).CheckUpdates(context.Background()); err != core.ErrSourceUnavailable {
		t.Errorf("missing scoop = %v", err)
	}
}

func TestApt(t *testing.T) {
	run := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name == "apt-mark" {
			return []byte("curl\n"), nil
		}
		return []byte("Listing...\n" +
			"git/noble-updates,noble-security 1:2.43.0-1ubuntu7.2 amd64 [upgradable from: 1:2.43.0-1ubuntu7.1]\n" +
			"curl/noble-updates 8.5.0-2ubuntu10.5 amd64 [upgradable from: 8.5.0-2ubuntu10.4]\n"), nil
	}
	lists := t.TempDir()
	if _, err := NewApt(run, false, lists).CheckUpdates(context.Background()); err == nil || !strings.Contains(err.Error(), "No package information") {
		t.Errorf("missing lists must not read as up to date: %v", err)
	}
	os.WriteFile(filepath.Join(lists, "archive.ubuntu.com_ubuntu_dists_noble_main_binary-amd64_Packages"), nil, 0o644)
	ups, err := NewApt(run, false, lists).CheckUpdates(context.Background())
	if err != nil || len(ups) != 2 {
		t.Fatalf("got %+v %v", ups, err)
	}
	if g := ups[0]; g.Name != "git" || g.CurrentVersion != "1:2.43.0-1ubuntu7.1" || g.AvailableVersion != "1:2.43.0-1ubuntu7.2" || g.Origin != "noble-updates" {
		t.Errorf("git = %+v", g)
	}
	if !ups[1].Held {
		t.Error("apt-mark hold should mark curl held")
	}
	st := NewApt(nil, false, "").PlanUpdates(ups[:1])
	if st[0].Commands[0].String() != "sudo apt-get install --only-upgrade git" || st[0].Elevation != core.ElevationSudo {
		t.Errorf("non-root plan = %+v", st[0])
	}
	st = NewApt(nil, true, "").PlanUpdates(ups[:1])
	if st[0].Commands[0].Program != "apt-get" || st[0].Elevation != core.ElevationAdmin {
		t.Errorf("root plan should not use sudo: %+v", st[0])
	}
	if strings.Contains(st[0].Commands[0].String(), "-y") {
		t.Error("apt must ask the user to confirm itself")
	}
}

func TestDnfSnapFlatpak(t *testing.T) {
	dnfRun := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("\nbash.x86_64    5.2.26-4.fc40    updates\nkernel-core.x86_64 6.9.4-200.fc40 updates\nObsoleting Packages\nold.noarch 1 x\n"),
			&inventory.CommandError{Command: "dnf", ExitCode: 100}
	}
	ups, err := NewDnf(dnfRun, false).CheckUpdates(context.Background())
	if err != nil || len(ups) != 2 || ups[0].Name != "bash" || ups[1].Name != "kernel-core" {
		t.Errorf("dnf = %+v %v", ups, err)
	}
	if got := NewDnf(nil, false).PlanUpdates(ups)[0].Commands[0].String(); got != "sudo dnf upgrade bash kernel-core" {
		t.Errorf("dnf plan = %s", got)
	}

	snapUps := parseSnapRefreshList("Name     Version  Rev   Size   Publisher  Notes\nfirefox  125.0    4200  280MB  mozilla✓   -\n")
	if len(snapUps) != 1 || snapUps[0].Name != "firefox" || snapUps[0].AvailableVersion != "125.0" {
		t.Errorf("snap = %+v", snapUps)
	}
	if len(parseSnapRefreshList("All snaps up to date.\n")) != 0 {
		t.Error("up to date should be empty")
	}
	if got := NewSnap(nil, false).PlanUpdates(snapUps)[0].Commands[0].String(); got != "sudo snap refresh firefox" {
		t.Errorf("snap plan = %s", got)
	}

	fp := parseFlatpakUpdates("org.gimp.GIMP\t2.10.38\tflathub\norg.gnome.Platform\t\tflathub\n")
	if len(fp) != 2 || fp[0].AvailableVersion != "2.10.38" || fp[1].AvailableVersion != "newer build" {
		t.Errorf("flatpak = %+v", fp)
	}
	if st := NewFlatpak(nil).PlanUpdates(fp)[0]; st.Elevation != core.ElevationPolkit || st.Commands[0].String() != "flatpak update org.gimp.GIMP org.gnome.Platform" {
		t.Errorf("flatpak plan = %+v", st)
	}
}

// fakeManager is a test double whose updates are test data only.
type fakeManager struct {
	id  string
	ups []core.Update
	err error
}

func (f fakeManager) ID() string     { return f.id }
func (f fakeManager) Label() string  { return "Label " + f.id }
func (f fakeManager) Detail() string { return "detail" }
func (f fakeManager) CheckUpdates(context.Context) ([]core.Update, error) {
	return f.ups, f.err
}
func (f fakeManager) PlanUpdates(ups []core.Update) []core.UpdateStep {
	return []core.UpdateStep{{Manager: f.id, Label: f.Label(), Updates: ups,
		Commands: []core.Command{{Program: f.id, Args: append([]string{"up"}, ids(ups)...)}}}}
}

func newService(t *testing.T, managers ...core.PackageManager) (*Service, *database.DB) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "atlas.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewService(managers, db, nil, nil), db
}

func TestServiceCheckEnrichesAndSaves(t *testing.T) {
	svc, db := newService(t,
		fakeManager{id: "apt", ups: []core.Update{
			{Manager: "apt", PackageID: "libz", Name: "libz", AvailableVersion: "2"},
			{Manager: "apt", PackageID: "editor", Name: "editor", AvailableVersion: "5"},
		}},
		fakeManager{id: "snap", err: core.ErrSourceUnavailable},
	)
	ctx := context.Background()
	db.SaveScan(ctx, core.ScanReport{Status: core.ScanComplete, Items: []core.SoftwareItem{
		{Name: "libz", Version: "1", PackageID: "libz", Source: "dpkg", Kind: core.KindComponent},
		{Name: "Editor", Version: "4", PackageID: "editor", Source: "dpkg", Kind: core.KindApp},
	}})
	check, err := svc.Check(ctx, nil)
	if err != nil || check.Status != core.ScanComplete || len(check.Updates) != 2 {
		t.Fatalf("check = %+v %v", check, err)
	}
	if e := check.Updates[0]; e.Name != "Editor" || e.CurrentVersion != "4" || e.Component {
		t.Errorf("enrich editor = %+v", e)
	}
	if l := check.Updates[1]; !l.Component || l.CurrentVersion != "1" {
		t.Errorf("enrich libz = %+v", l)
	}
	_, saved, ok, _ := svc.LatestCheck(ctx)
	if !ok || len(saved) != 2 || saved[0].ID == 0 {
		t.Errorf("saved = %+v", saved)
	}
	acts, _ := db.RecentActivity(ctx, 5)
	if len(acts) != 1 || acts[0].Kind != "updates.check" || acts[0].Summary != "Found 2 updates." {
		t.Errorf("activity = %+v", acts)
	}
}

func TestServicePlanSkipsHeldAndKeepsManagerOrder(t *testing.T) {
	svc, _ := newService(t, fakeManager{id: "a"}, fakeManager{id: "b"})
	steps := svc.Plan([]core.Update{
		{Manager: "b", PackageID: "x"},
		{Manager: "a", PackageID: "y"},
		{Manager: "a", PackageID: "held", Held: true},
	})
	if len(steps) != 2 || steps[0].Manager != "a" || len(steps[0].Updates) != 1 || steps[1].Manager != "b" {
		t.Errorf("steps = %+v", steps)
	}
}

func TestRecordResultsRemovesAppliedAndLogsActivity(t *testing.T) {
	svc, db := newService(t, fakeManager{id: "apt", ups: []core.Update{
		{Manager: "apt", PackageID: "a", Name: "a", AvailableVersion: "2"},
		{Manager: "apt", PackageID: "b", Name: "b", AvailableVersion: "3"},
	}})
	ctx := context.Background()
	svc.Check(ctx, nil)
	_, ups, _, _ := svc.LatestCheck(ctx)
	results := []core.StepResult{
		{Step: core.UpdateStep{Label: "APT", Updates: ups[:1]}, Status: core.StepDone, Command: "apt-get install a", Duration: time.Second},
		{Step: core.UpdateStep{Label: "APT", Updates: ups[1:]}, Status: core.StepFailed, ExitCode: 100},
	}
	if err := svc.RecordResults(ctx, results); err != nil {
		t.Fatal(err)
	}
	_, left, _, _ := svc.LatestCheck(ctx)
	if len(left) != 1 || left[0].Name != "b" {
		t.Errorf("only applied updates should be removed: %+v", left)
	}
	acts, _ := db.RecentActivity(ctx, 1)
	a := acts[0]
	if a.Kind != "updates.apply" || a.Outcome != core.OutcomePartial || !strings.Contains(a.Summary, "Updated: a → 2") || !strings.Contains(a.Summary, "Failed: b") {
		t.Errorf("activity = %+v", a)
	}
	if !strings.Contains(a.Details, "exit code 100") || !strings.Contains(a.Details, "apt-get install a") {
		t.Errorf("details = %q", a.Details)
	}
}

func TestApplyActivityOutcomes(t *testing.T) {
	step := core.UpdateStep{Updates: []core.Update{{Name: "x"}}}
	cases := map[core.Outcome][]core.StepStatus{
		core.OutcomeSuccess:   {core.StepDone, core.StepSkipped},
		core.OutcomeFailed:    {core.StepFailed},
		core.OutcomeCancelled: {core.StepStopped},
		core.OutcomeInfo:      {core.StepSkipped},
	}
	for want, statuses := range cases {
		var rs []core.StepResult
		for _, s := range statuses {
			rs = append(rs, core.StepResult{Step: step, Status: s})
		}
		if got := ApplyActivity(rs, time.Now()).Outcome; got != want {
			t.Errorf("%v: outcome %s, want %s", statuses, got, want)
		}
	}
	many := make([]string, 8)
	for i := range many {
		many[i] = "n"
	}
	if !strings.HasSuffix(listNames(many), "and 3 more") {
		t.Error("long lists should be shortened")
	}
}
