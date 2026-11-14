package cleanup

// Fixtures mirror real command output and folder layouts. They are test data
// only and never shown to users.

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deepakraj1207/atlas/internal/analysis"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/database"
	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/platform"
)

const statusFixture = `Package: python3.12
Status: install ok installed

Package: python3
Status: install ok installed
Depends: python3.12

Package: meld
Status: install ok installed
Depends: python3

Package: lonely-tool
Status: install ok installed

Package: dash
Status: install ok installed
Essential: yes

Package: libc-utils
Status: install ok installed
Priority: required
`

func writeFile(t *testing.T, path, body string) string {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func newSvc(t *testing.T, cfg Config) (*Service, *database.DB) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return NewService(cfg, db, nil), db
}

func linuxCfg(t *testing.T) Config {
	status := writeFile(t, filepath.Join(t.TempDir(), "status"), statusFixture)
	return Config{
		Removers: []Remover{NewAptRemover(false), NewSnapRemover(false), NewFlatpakRemover(), NewDnfRemover(false)},
		Deps:     []analysis.Source{analysis.NewDpkgDeps(status)},
	}
}

func deb(name string, kind core.Kind) core.SoftwareItem {
	return core.SoftwareItem{Name: name, PackageID: name, Version: "1.0", Source: inventory.SourceDpkg, Kind: kind}
}

func plan(t *testing.T, s *Service, items ...core.SoftwareItem) (allowed, blocked []core.UpdateStep) {
	t.Helper()
	steps, err := s.PlanRemoval(context.Background(), items)
	if err != nil {
		t.Fatal(err)
	}
	for _, st := range steps {
		if st.Action != core.ActionRemove {
			t.Errorf("removal steps must be marked as removals: %+v", st)
		}
		if st.Blocked != "" {
			blocked = append(blocked, st)
		} else {
			allowed = append(allowed, st)
		}
	}
	return allowed, blocked
}

func TestGateBlocksWhatOtherSoftwareNeeds(t *testing.T) {
	s, _ := newSvc(t, linuxCfg(t))
	allowed, blocked := plan(t, s, deb("python3", core.KindApp), deb("lonely-tool", core.KindApp))
	if len(blocked) != 1 || !strings.Contains(blocked[0].Blocked, "Needed by meld") {
		t.Fatalf("python3 is needed by meld and must be blocked: %+v", blocked)
	}
	if len(allowed) != 1 || allowed[0].Commands[0].String() != "sudo apt-get remove lonely-tool" {
		t.Errorf("lonely-tool should be removable with plain apt-get remove: %+v", allowed)
	}
}

func TestGateAllowsWhenDependentsAreAlsoBeingRemoved(t *testing.T) {
	s, _ := newSvc(t, linuxCfg(t))
	allowed, blocked := plan(t, s, deb("python3", core.KindApp), deb("meld", core.KindApp))
	if len(blocked) != 0 || len(allowed) != 1 || len(allowed[0].Updates) != 2 {
		t.Errorf("removing meld too frees python3: allowed=%+v blocked=%+v", allowed, blocked)
	}
}

func TestGateBlocksEssentialProtectedAndUnknownRuntimes(t *testing.T) {
	s, _ := newSvc(t, linuxCfg(t))
	_, blocked := plan(t, s,
		deb("dash", core.KindComponent),
		deb("libc-utils", core.KindComponent),
		deb("sudo", core.KindApp),
		deb("linux-image-6.8.0-31-generic", core.KindComponent),
		core.SoftwareItem{Name: "Microsoft .NET Runtime - 8.0.4 (x64)", Version: "8.0.4", Source: inventory.SourceRegistry, Kind: core.KindComponent},
		core.SoftwareItem{Name: "Some Library", Version: "1", Source: inventory.SourceRegistry, Kind: core.KindComponent},
		core.SoftwareItem{Name: "App Installer", Version: "1", Source: inventory.SourceRegistry, Kind: core.KindApp},
	)
	want := []string{"essential", "required", "relies on it", "relies on it", "can't tell what software needs this runtime", "system component", "relies on it"}
	if len(blocked) != len(want) {
		t.Fatalf("want %d blocked, got %+v", len(want), blocked)
	}
	for i, w := range want {
		if !strings.Contains(blocked[i].Blocked, w) {
			t.Errorf("item %q: reason %q should mention %q", blocked[i].Updates[0].Name, blocked[i].Blocked, w)
		}
	}
}

func TestDnfProtectedList(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "dnf.conf"), "# protected\ndnf\nkernel-core\n")
	writeFile(t, filepath.Join(dir, "extra.conf"), "mytool\n")
	s, _ := newSvc(t, Config{Removers: []Remover{NewDnfRemover(true)}, DnfProtectedDir: dir, RPM: platform.RPMRoute{Manager: "dnf"}})
	_, blocked := plan(t, s, core.SoftwareItem{Name: "mytool", PackageID: "mytool", Source: inventory.SourceRPM, Kind: core.KindApp})
	if len(blocked) != 1 || !strings.Contains(blocked[0].Blocked, "protected") {
		t.Errorf("DNF-protected packages must be blocked: %+v", blocked)
	}
}

func TestRemoverChoiceOnWindows(t *testing.T) {
	s, _ := newSvc(t, Config{Removers: []Remover{NewWingetRemover(false), NewChocolateyRemover(false), NewScoopRemover(false)}})
	reg := inventory.SourceRegistry
	allowed, blocked := plan(t, s,
		core.SoftwareItem{Name: "Git", PackageID: "Git.Git", Source: reg, AlsoFoundIn: []string{inventory.SourceWinget}, Kind: core.KindApp},
		core.SoftwareItem{Name: "Old Editor", Source: reg, Kind: core.KindApp},
		core.SoftwareItem{Name: "ripgrep", PackageID: "main/ripgrep", Source: inventory.SourceScoop, Kind: core.KindApp},
		core.SoftwareItem{Name: "7-Zip", PackageID: "7zip", Source: reg, AlsoFoundIn: []string{inventory.SourceChocolatey}, Kind: core.KindApp},
		core.SoftwareItem{Name: "Both Tools App", PackageID: "x", Source: inventory.SourceWinget, AlsoFoundIn: []string{inventory.SourceChocolatey}, Kind: core.KindApp},
	)
	cmds := map[string]string{}
	for _, st := range append(allowed, blocked...) {
		c := ""
		if len(st.Commands) > 0 {
			c = st.Commands[0].String()
		}
		cmds[st.Updates[0].Name] = c + " | " + st.Blocked
	}
	if got := cmds["Git"]; !strings.HasPrefix(got, "winget uninstall --id Git.Git --exact") {
		t.Errorf("Git: %s", got)
	}
	if got := cmds["Old Editor"]; !strings.HasPrefix(got, `winget uninstall --name "Old Editor" --exact`) {
		t.Errorf("registry-only app should be removed by exact name: %s", got)
	}
	if got := cmds["ripgrep"]; !strings.HasPrefix(got, "scoop uninstall ripgrep") {
		t.Errorf("ripgrep: %s", got)
	}
	if got := cmds["7-Zip"]; !strings.HasPrefix(got, "choco uninstall 7zip") || !strings.Contains(got, "administrator") {
		t.Errorf("choco removal needs admin and must be blocked otherwise: %s", got)
	}
	if got := cmds["Both Tools App"]; !strings.Contains(got, "more than one tool") {
		t.Errorf("ambiguous ownership must be blocked: %s", got)
	}
}

func TestNoAssumeYesPurgeOrDataFlags(t *testing.T) {
	items := []core.SoftwareItem{{Name: "a", PackageID: "a"}}
	for _, r := range []Remover{NewAptRemover(false), NewDnfRemover(false), NewZypperRemover(false), NewSnapRemover(false), NewFlatpakRemover(),
		NewWingetRemover(false), NewChocolateyRemover(true), NewScoopRemover(false)} {
		for _, st := range r.PlanRemove(items) {
			if st.Rollback == "" {
				t.Errorf("%s: every removal must say how to get it back", r.ID())
			}
			for _, c := range st.Commands {
				for _, a := range c.Args {
					switch a {
					case "-y", "--yes", "--assumeyes", "--purge", "purge", "--delete-data", "--force", "--noninteractive", "--non-interactive", "-n", "--clean-deps", "-u", "--silent", "--accept-source-agreements":
						t.Errorf("%s: forbidden flag %q in %s", r.ID(), a, c.String())
					}
				}
			}
		}
	}
}

func TestAptTidiers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "a_1.0_amd64.deb"), strings.Repeat("x", 1500))
	writeFile(t, filepath.Join(dir, "b_2.0_amd64.deb"), strings.Repeat("x", 500))
	writeFile(t, filepath.Join(dir, "lock"), "")
	steps, err := NewAptCache(dir, false).Find(context.Background())
	if err != nil || len(steps) != 1 || steps[0].Bytes != 2000 || steps[0].Updates[0].Name != "2 downloaded package files" || steps[0].Commands[0].String() != "sudo apt-get clean" {
		t.Errorf("apt cache: %+v %v", steps, err)
	}
	if steps, _ := NewAptCache(t.TempDir(), false).Find(context.Background()); len(steps) != 0 {
		t.Error("an empty cache means nothing to do")
	}
	if _, err := NewAptCache(filepath.Join(dir, "nope"), false).Find(context.Background()); err != core.ErrSourceUnavailable {
		t.Errorf("missing cache dir: %v", err)
	}

	run := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if strings.Join(args, " ") != "-s autoremove" {
			t.Errorf("autoremove must only be simulated during Find, got %v", args)
		}
		return []byte("NOTE: This is only a simulation!\nReading package lists...\nRemv libfoo1 [1.2-3]\nRemv python3.11:amd64 [3.11.9-1]\n"), nil
	}
	steps, err = NewAptAutoremove(run, true).Find(context.Background())
	if err != nil || len(steps) != 1 || len(steps[0].Updates) != 2 || steps[0].Updates[1].Name != "python3.11" || steps[0].Action != core.ActionRemove {
		t.Fatalf("autoremove: %+v %v", steps, err)
	}
	if steps[0].Commands[0].String() != "apt-get autoremove" || steps[0].Updates[0].Origin != inventory.SourceDpkg {
		t.Errorf("as root: plain apt-get autoremove with dpkg origin; got %+v", steps[0])
	}
}

func TestDnfTidiers(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "fedora-abc", "packages", "bash-5.2.rpm"), "12345")
	writeFile(t, filepath.Join(dir, "fedora-abc", "repodata", "primary.rpm"), "ignored")
	steps, err := NewDnfCache([]string{dir, filepath.Join(dir, "missing")}, false).Find(context.Background())
	if err != nil || len(steps) != 1 || steps[0].Bytes != 5 || steps[0].Commands[0].String() != "sudo dnf clean packages" {
		t.Errorf("dnf cache: %+v %v", steps, err)
	}
	run := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("Last metadata expiration check: 0:10:00 ago\nlibold\nlibold\nanother\n"), nil
	}
	steps, _ = NewDnfAutoremove(run, false).Find(context.Background())
	if len(steps) != 1 || len(steps[0].Updates) != 2 {
		t.Errorf("dnf autoremove should skip noise and duplicates: %+v", steps)
	}
}

func TestSnapOldRevisions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "firefox_4000.snap"), strings.Repeat("x", 100))
	run := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("Name     Version  Rev   Tracking       Publisher  Notes\nfirefox  124.0    4000  latest/stable  mozilla✓   disabled\nfirefox  125.0    4136  latest/stable  mozilla✓   -\ncore22   2024     1122  latest/stable  canonical✓ base\n"), nil
	}
	steps, err := NewSnapOldRevisions(run, dir, false).Find(context.Background())
	if err != nil || len(steps) != 1 || len(steps[0].Commands) != 1 {
		t.Fatalf("snap: %+v %v", steps, err)
	}
	if got := steps[0].Commands[0].String(); got != "sudo snap remove firefox --revision=4000" || steps[0].Bytes != 100 {
		t.Errorf("only the disabled revision may be removed: %s (%d bytes)", got, steps[0].Bytes)
	}
}

func TestScoopTidiers(t *testing.T) {
	root := t.TempDir()
	// Newer Scoop names the file scoop-manifest.json.
	writeFile(t, filepath.Join(root, "apps", "git", "current", "scoop-manifest.json"), `{"version":"2.45.1"}`)
	writeFile(t, filepath.Join(root, "apps", "git", "2.45.1", "git.exe"), "new")
	writeFile(t, filepath.Join(root, "apps", "git", "2.44.0", "git.exe"), "old-old")
	writeFile(t, filepath.Join(root, "apps", "fd", "current", "manifest.json"), `{"version":"9"}`)
	writeFile(t, filepath.Join(root, "apps", "fd", "9", "fd.exe"), "x")
	writeFile(t, filepath.Join(root, "cache", "git#2.44.0#x.7z"), "1234567890")
	roots := []inventory.ScoopRoot{{Dir: root, Scope: "user"}}
	steps, err := NewScoopOldVersions(roots, false).Find(context.Background())
	if err != nil || len(steps) != 1 || steps[0].Commands[0].String() != "scoop cleanup git" || steps[0].Bytes != 7 || steps[0].Updates[0].CurrentVersion != "2.44.0" {
		t.Errorf("scoop old versions: %+v %v", steps, err)
	}
	steps, err = NewScoopCache(roots).Find(context.Background())
	if err != nil || len(steps) != 1 || steps[0].Bytes != 10 || steps[0].Commands[0].String() != "scoop cache rm *" || steps[0].Updates[0].Name != "1 downloaded installer" {
		t.Errorf("scoop cache: %+v %v", steps, err)
	}
}

type fakeTidier struct {
	id    string
	steps []core.UpdateStep
	err   error
}

func (f fakeTidier) ID() string                                      { return f.id }
func (f fakeTidier) Label() string                                   { return "Tidy " + f.id }
func (f fakeTidier) Detail() string                                  { return "" }
func (f fakeTidier) Find(context.Context) ([]core.UpdateStep, error) { return f.steps, f.err }

func TestFindTidyUpsAndRecordResults(t *testing.T) {
	cfg := linuxCfg(t)
	cfg.Tidiers = []Tidier{
		fakeTidier{id: "a", steps: []core.UpdateStep{{Label: "Cache", Action: core.ActionClean, Bytes: 2_500_000, Updates: []core.Update{{Name: "files"}}}}},
		fakeTidier{id: "b", err: core.ErrSourceUnavailable},
		fakeTidier{id: "c", err: errors.New("boom")},
	}
	s, db := newSvc(t, cfg)
	ctx := context.Background()
	r, err := s.FindTidyUps(ctx, nil)
	if err != nil || r.Status != core.ScanPartial || len(r.Steps) != 1 || r.Bytes() != 2_500_000 {
		t.Fatalf("find: %+v %v", r, err)
	}
	acts, _ := db.RecentActivity(ctx, 1)
	if !strings.Contains(acts[0].Summary, "freeing about 2.5 MB") || !strings.Contains(acts[0].Summary, "Nothing was changed") {
		t.Errorf("find activity = %+v", acts[0])
	}

	db.SaveScan(ctx, core.ScanReport{Status: core.ScanComplete, Items: []core.SoftwareItem{deb("lonely-tool", core.KindApp), deb("meld", core.KindApp)}})
	allowed, _ := plan(t, s, deb("lonely-tool", core.KindApp))
	var calls []string
	s.WithProcessStarter(func(c core.Command, in io.Reader, out, errw io.Writer) (int, error) {
		calls = append(calls, c.String())
		return 0, nil
	})
	job := s.NewJob(append(allowed, r.Steps...))
	job.SetStdin(strings.NewReader("\n"))
	job.SetStdout(io.Discard)
	job.Run()
	if err := s.RecordResults(ctx, job.Results()); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0] != "sudo apt-get remove lonely-tool" {
		t.Errorf("calls = %v", calls)
	}
	items, _ := db.Software(ctx)
	if len(items) != 1 || items[0].Name != "meld" {
		t.Errorf("removed software must leave the saved list: %+v", items)
	}
	acts, _ = db.RecentActivity(ctx, 1)
	if acts[0].Kind != "cleanup.run" || acts[0].Outcome != core.OutcomeSuccess || !strings.Contains(acts[0].Summary, "Freed about 2.5 MB") {
		t.Errorf("run activity = %+v", acts[0])
	}
}

func TestHumanBytes(t *testing.T) {
	for n, want := range map[int64]string{999: "999 B", 1500: "1.5 kB", 2_500_000: "2.5 MB", 1_200_000_000: "1.2 GB"} {
		if got := HumanBytes(n); got != want {
			t.Errorf("HumanBytes(%d) = %s, want %s", n, got, want)
		}
	}
}
