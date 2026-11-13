package discover

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/database"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

func TestBuiltInCatalogueIsValid(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Entries) < 20 || c.AsOf == "" {
		t.Fatalf("catalogue too small or undated: %d entries, as of %q", len(c.Entries), c.AsOf)
	}
	for _, e := range c.Entries {
		hasWindows, hasWinget := false, false
		for _, p := range e.Packages {
			hasWindows = hasWindows || windowsManagers[p.Manager]
			hasWinget = hasWinget || p.Manager == "winget"
		}
		if hasWindows && !hasWinget {
			t.Errorf("%s: Windows tools need a winget package, the default on Windows 11", e.ID)
		}
	}
}

func TestParseRejectsBadEntries(t *testing.T) {
	base := `as_of: "2026-11"
entries:
  - id: x
    name: X
    category: Utilities
    summary: s
    why: w
    homepage: https://example.org
    provides: [x]
    packages:
      - {manager: winget, id: Vendor.X}
`
	if _, err := Parse([]byte(base)); err != nil {
		t.Fatalf("valid entry rejected: %v", err)
	}
	bad := map[string]string{
		"bad winget id":     strings.Replace(base, "Vendor.X", "NoDot", 1),
		"unknown category":  strings.Replace(base, "Utilities", "Games", 1),
		"http homepage":     strings.Replace(base, "https://", "http://", 1),
		"bad product key":   strings.Replace(base, "[x]", "[X-Y]", 1),
		"classic not snap":  strings.Replace(base, "id: Vendor.X}", "id: Vendor.X, classic: true}", 1),
		"duplicate manager": base + "      - {manager: winget, id: Vendor.Y}\n",
		"no date":           strings.Replace(base, `as_of: "2026-11"`, "", 1),
	}
	for name, doc := range bad {
		if _, err := Parse([]byte(doc)); err == nil {
			t.Errorf("%s: should be rejected", name)
		}
	}
}

func TestProductKey(t *testing.T) {
	for in, want := range map[string]string{
		"Notepad++ 8.6.4 (64-bit x64)":        "notepadplusplus",
		"Notepad":                             "notepad",
		"7-Zip 23.01 (x64)":                   "7zip",
		"Microsoft Visual Studio Code (User)": "microsoftvisualstudiocode",
		"Git":                                 "git",
	} {
		if got := ProductKey(in); got != want {
			t.Errorf("ProductKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func catalogue(t *testing.T) Catalog {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func find(d core.Discovery, id string) *core.DiscoverItem {
	for i := range d.Items {
		if d.Items[i].Entry.ID == id {
			return &d.Items[i]
		}
	}
	return nil
}

func TestAssessOnWindows(t *testing.T) {
	here := Here{
		Windows: true, Scanned: true,
		Managers: map[string]bool{"winget": true, "chocolatey": true},
		Blocked:  map[string]string{"chocolatey": "needs admin"},
		Items: []core.SoftwareItem{
			{Name: "Git", Version: "2.44.0", Source: inventory.SourceRegistry},
			{Name: "Notepad", Source: inventory.SourceWinget},
			{Name: "Some App", PackageID: "VideoLAN.VLC", Source: inventory.SourceRegistry, AlsoFoundIn: []string{inventory.SourceWinget}},
			{Name: "Python 3.12.3 (64-bit)", Version: "3.12.3", Source: inventory.SourceRegistry},
		},
	}
	d := Assess(catalogue(t), here)
	if !d.Scanned || len(d.ManagersHere) != 2 {
		t.Errorf("discovery = %+v", d)
	}
	if g := find(d, "git"); !g.Installed || g.InstalledAs != "Git" {
		t.Errorf("git should be recognised as installed: %+v", g)
	}
	if v := find(d, "vlc"); !v.Installed {
		t.Error("VLC should be recognised by its winget package ID")
	}
	if n := find(d, "notepadplusplus"); n.Installed {
		t.Error("Windows Notepad is not Notepad++")
	}
	if find(d, "btop") != nil || find(d, "baobab") != nil {
		t.Error("Linux-only tools must not be offered on Windows")
	}
	gh := find(d, "gh")
	if gh.Via != "winget" || len(gh.Because) == 0 || gh.Because[0] != "because you have Git" {
		t.Errorf("gh should be suggested via winget because of Git: %+v", gh)
	}
	uv := find(d, "uv")
	if len(uv.Because) == 0 || !strings.Contains(uv.Because[0], "Python 3.12.3") {
		t.Errorf("uv should be suggested because of Python: %+v", uv)
	}
	if k := find(d, "keepassxc"); k.Installed || k.Because[len(k.Because)-1] != "useful for almost everyone" {
		t.Errorf("keepassxc = %+v", k)
	}
	if g := find(d, "git"); len(g.Because) != 0 {
		t.Error("installed tools are never suggested")
	}
}

func TestAssessOnLinuxAndUnscanned(t *testing.T) {
	d := Assess(catalogue(t), Here{Windows: false, Scanned: true, Managers: map[string]bool{"apt": true, "flatpak": true}, Blocked: map[string]string{}})
	if find(d, "powertoys") != nil {
		t.Error("Windows-only tools must not be offered on Linux")
	}
	if v := find(d, "vlc"); v.Via != "apt" {
		t.Errorf("native packages are preferred on Linux: %+v", v)
	}
	if o := find(d, "obsidian"); o.Via != "flatpak" {
		t.Errorf("flatpak is the fallback: %+v", o)
	}
	if b := find(d, "bitwarden"); b.Via != "" || !strings.Contains(b.Unavailable, "offered through: Flatpak") && !strings.Contains(b.Unavailable, "Flatpak (Flathub)") {
		// bitwarden has flatpak, so it should be installable
		if b.Via != "flatpak" {
			t.Errorf("bitwarden = %+v", b)
		}
	}
	if l := find(d, "gh"); l.Via != "apt" {
		t.Errorf("gh = %+v", l)
	}

	d = Assess(catalogue(t), Here{Windows: true})
	if v := find(d, "vlc"); v.Via != "" || !strings.Contains(v.Unavailable, "Scan your system first") {
		t.Errorf("without a scan nothing is installable: %+v", v)
	}
}

func newService(t *testing.T, cfg Config, sources []core.SourceResult, items []core.SoftwareItem) (*Service, *database.DB) {
	t.Helper()
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	if sources != nil {
		db.SaveScan(context.Background(), core.ScanReport{Status: core.ScanComplete, Sources: sources, Items: items})
	}
	s, err := NewService(cfg, db, nil)
	if err != nil {
		t.Fatal(err)
	}
	return s, db
}

func ok(src string) core.SourceResult { return core.SourceResult{Source: src, Status: core.SourceOK} }

func cmdOf(st core.UpdateStep) string {
	var parts []string
	for _, c := range st.Commands {
		parts = append(parts, c.String())
	}
	return strings.Join(parts, " && ")
}

func TestPlanInstallWindows(t *testing.T) {
	yes := true
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "buckets", "main"), 0o755)
	s, _ := newService(t, Config{Windows: &yes, ScoopRoots: []inventory.ScoopRoot{{Dir: root, Scope: "user"}}},
		[]core.SourceResult{ok(inventory.SourceWinget), {Source: inventory.SourceScoop, Status: core.SourceOK}, {Source: inventory.SourceChocolatey, Status: core.SourceUnavailable}},
		[]core.SoftwareItem{{Name: "Git", Source: inventory.SourceRegistry}})
	steps, err := s.PlanInstall(context.Background(), []string{"vlc", "git", "not-a-tool"})
	if err != nil || len(steps) != 2 {
		t.Fatalf("steps = %+v %v", steps, err)
	}
	if got := cmdOf(steps[0]); got != "winget install --id VideoLAN.VLC --exact --source winget" || steps[0].Action != core.ActionInstall || steps[0].Elevation != core.ElevationInstaller {
		t.Errorf("vlc: %s %+v", got, steps[0])
	}
	if strings.Contains(cmdOf(steps[0]), "accept") {
		t.Error("ATLAS must never accept agreements on the user's behalf")
	}
	if !strings.Contains(steps[1].Blocked, "already installed") {
		t.Errorf("git is installed: %+v", steps[1])
	}
	if !strings.Contains(steps[0].Rollback, "My Software") {
		t.Error("every install should say how to remove it")
	}
}

func TestPlanInstallScoopBucketAndLinuxManagers(t *testing.T) {
	yes, no := true, false
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "buckets", "main"), 0o755)
	s, _ := newService(t, Config{Windows: &yes, ScoopRoots: []inventory.ScoopRoot{{Dir: root, Scope: "user"}}},
		[]core.SourceResult{ok(inventory.SourceScoop)}, nil)
	steps, _ := s.PlanInstall(context.Background(), []string{"vlc", "ripgrep"})
	if got := cmdOf(steps[0]); got != "scoop bucket add extras && scoop install extras/vlc" {
		t.Errorf("a missing bucket must be added first, visibly: %s", got)
	}
	if got := cmdOf(steps[1]); got != "scoop install main/ripgrep" {
		t.Errorf("ripgrep: %s", got)
	}

	lists := t.TempDir()
	os.WriteFile(filepath.Join(lists, "x_Packages"), nil, 0o644)
	flathub := func(ctx context.Context, name string, args ...string) ([]byte, error) { return []byte("fedora\n"), nil }
	s, _ = newService(t, Config{Windows: &no, AptListsDir: lists, Run: flathub},
		[]core.SourceResult{ok(inventory.SourceDpkg), ok(inventory.SourceSnap), ok(inventory.SourceFlatpak)}, nil)
	steps, _ = s.PlanInstall(context.Background(), []string{"vlc", "vscode", "obsidian", "bat"})
	if got := cmdOf(steps[0]); got != "sudo apt-get install vlc" || strings.Contains(got, "-y") {
		t.Errorf("vlc: %s", got)
	}
	if got := cmdOf(steps[1]); got != "sudo snap install code --classic" || !strings.Contains(steps[1].Rollback, "classic confinement") {
		t.Errorf("classic snaps must say so: %s / %s", got, steps[1].Rollback)
	}
	if !strings.Contains(steps[2].Blocked, "Flathub isn't set up") {
		t.Errorf("flatpak without flathub must be blocked: %+v", steps[2])
	}
	if !strings.Contains(steps[3].Rollback, "batcat") {
		t.Error("package notes should be shown")
	}

	os.Remove(filepath.Join(lists, "x_Packages"))
	d, _ := s.Discover(context.Background())
	if v := find(d, "vlc"); v.Via == "apt" || (v.Via == "" && !strings.Contains(v.Unavailable, "apt update")) {
		t.Errorf("apt without package lists must not be used: %+v", v)
	}
}

func TestChocolateyOnlyAsAdministrator(t *testing.T) {
	yes := true
	s, _ := newService(t, Config{Windows: &yes}, []core.SourceResult{ok(inventory.SourceChocolatey)}, nil)
	d, _ := s.Discover(context.Background())
	if v := find(d, "vlc"); v.Via != "" || !strings.Contains(v.Unavailable, "administrator") {
		t.Errorf("non-admin chocolatey: %+v", v)
	}
	s, _ = newService(t, Config{Windows: &yes, Elevated: true}, []core.SourceResult{ok(inventory.SourceChocolatey)}, nil)
	steps, _ := s.PlanInstall(context.Background(), []string{"vlc"})
	if got := cmdOf(steps[0]); got != "choco install vlc" {
		t.Errorf("admin chocolatey: %s", got)
	}
}

func TestRecordResults(t *testing.T) {
	no := false
	s, db := newService(t, Config{Windows: &no}, nil, nil)
	ctx := context.Background()
	results := []core.StepResult{
		{Step: core.UpdateStep{Label: "APT", Action: core.ActionInstall, Updates: []core.Update{{Name: "VLC media player"}}}, Status: core.StepDone, Command: "sudo apt-get install vlc"},
		{Step: core.UpdateStep{Label: "APT", Action: core.ActionInstall, Updates: []core.Update{{Name: "btop"}}}, Status: core.StepFailed, ExitCode: 100},
	}
	if err := s.RecordResults(ctx, results); err != nil {
		t.Fatal(err)
	}
	acts, _ := db.RecentActivity(ctx, 1)
	a := acts[0]
	if a.Kind != "discover.install" || a.Outcome != core.OutcomePartial || !strings.Contains(a.Summary, "Installed: VLC media player") || !strings.Contains(a.Details, "exit code 100") {
		t.Errorf("activity = %+v", a)
	}
	// One app installed: "it", not "them" (seen on Windows 11).
	if !strings.Contains(a.Summary, "Scan again to add it to My Software.") {
		t.Errorf("summary = %q", a.Summary)
	}
}
