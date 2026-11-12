package analysis

// Names and fixtures below mirror real-world formats. They are test data
// only and never shown to users.

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/database"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

var now = time.Date(2026, 11, 12, 12, 0, 0, 0, time.UTC)

func TestLookupSupport(t *testing.T) {
	cases := []struct {
		family, branch string
		want           string
	}{
		{"Python", "3.8", "ended"},
		{"Python", "3.12", "supported"},
		{"Python", "3.11", "supported"},
		{"Node.js", "20", "ended"},
		{"Node.js", "22", "ending"}, // 30 Apr 2027 is under six months away
		{"Node.js", "24", "supported"},
		{".NET", "8.0", "ended"}, // 10 Nov 2026, two days before "now"
		{".NET", "10.0", "supported"},
		{"Visual C++", "2010 x64", "ended"},
		{"Visual C++", "2015-2022 x64", "supported"},
		{"Java", "17", "unknown"},
		{"Python", "4.0", "unknown"},
	}
	for _, c := range cases {
		if got := LookupSupport(c.family, c.branch, false, now).State; got != c.want {
			t.Errorf("%s %s = %s, want %s", c.family, c.branch, got, c.want)
		}
	}
	// Month-precision dates end on the last day of the month.
	if s := LookupSupport("Python", "3.10", false, time.Date(2026, 10, 31, 23, 0, 0, 0, time.UTC)); s.State != "ending" {
		t.Errorf("3.10 on 31 Oct 2026 should still be supported (ending), got %s", s.State)
	}
	if s := LookupSupport("Node.js", "22", false, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)); s.State != "ending" {
		t.Errorf("Node 22 four months before EOL should be ending, got %s", s.State)
	}
	if FormatEnd("2024-10-07") != "7 Oct 2024" || FormatEnd("2026-10") != "Oct 2026" {
		t.Error("FormatEnd")
	}
}

func TestDetectRuntime(t *testing.T) {
	reg := inventory.SourceRegistry
	cases := []struct {
		item           core.SoftwareItem
		family, branch string
	}{
		{core.SoftwareItem{Name: "Python 3.12.3 (64-bit)", Source: reg}, "Python", "3.12"},
		{core.SoftwareItem{Name: "python", Version: "3.11.9", Source: inventory.SourceScoop}, "Python", "3.11"},
		{core.SoftwareItem{Name: "python3.12", Source: inventory.SourceDpkg}, "Python", "3.12"},
		{core.SoftwareItem{Name: "python3", Version: "3.12.4-1.fc40", Source: inventory.SourceRPM}, "Python", "3.12"},
		{core.SoftwareItem{Name: "Node.js", Version: "20.11.1", Source: reg}, "Node.js", "20"},
		{core.SoftwareItem{Name: "nodejs", Version: "18.19.1+dfsg-6ubuntu5", Source: inventory.SourceDpkg}, "Node.js", "18"},
		{core.SoftwareItem{Name: "Microsoft .NET Runtime - 8.0.4 (x64)", Source: reg}, ".NET", "8.0"},
		{core.SoftwareItem{Name: "Microsoft ASP.NET Core 6.0.25 - Shared Framework (x64)", Source: reg}, ".NET", "6.0"},
		{core.SoftwareItem{Name: "Microsoft Windows Desktop Runtime - 7.0.14 (x64)", Source: reg}, ".NET", "7.0"},
		{core.SoftwareItem{Name: "Microsoft .NET Core SDK 3.1.426 (x64)", Source: reg}, ".NET", "3.1"},
		{core.SoftwareItem{Name: "dotnet-runtime-8.0", Source: inventory.SourceDpkg}, ".NET", "8.0"},
		{core.SoftwareItem{Name: "Java 8 Update 391 (64-bit)", Source: reg}, "Java", "8"},
		{core.SoftwareItem{Name: "Eclipse Temurin JDK with Hotspot 17.0.9+9 (x64)", Source: reg}, "Java", "17"},
		{core.SoftwareItem{Name: "temurin21-jdk", Source: inventory.SourceScoop}, "Java", "21"},
		{core.SoftwareItem{Name: "openjdk-17-jre-headless", Source: inventory.SourceDpkg}, "Java", "17"},
		{core.SoftwareItem{Name: "java-1.8.0-openjdk", Source: inventory.SourceRPM}, "Java", "8"},
		{core.SoftwareItem{Name: "Microsoft Visual C++ 2015-2022 Redistributable (x64) - 14.38.33135", Source: reg}, "Visual C++", "2015-2022 x64"},
		{core.SoftwareItem{Name: "Microsoft Visual C++ 2010  x86 Redistributable - 10.0.40219", Source: reg}, "Visual C++", "2010 x86"},
	}
	for _, c := range cases {
		fam, br, ok := DetectRuntime(c.item)
		if !ok || fam != c.family || br != c.branch {
			t.Errorf("%q: got %q %q %v, want %q %q", c.item.Name, fam, br, ok, c.family, c.branch)
		}
	}
	for _, n := range []core.SoftwareItem{
		{Name: "Python Launcher", Source: reg},
		{Name: "python3.12-venv", Source: inventory.SourceDpkg},
		{Name: "python3", Version: "3.12.3", Source: inventory.SourceDpkg}, // Debian meta-package
		{Name: "Microsoft .NET Framework 4.8.1", Source: reg},
		{Name: "Node.js Tools", Source: reg},
		{Name: "JavaScript Debugger 1.2", Source: reg},
		{Name: "Java Auto Updater", Source: reg},
		{Name: "Git", Version: "2.44.0", Source: reg},
	} {
		if fam, _, ok := DetectRuntime(n); ok {
			t.Errorf("%q should not be a runtime, got %s", n.Name, fam)
		}
	}
}

const dpkgFixture = `Package: python3.12
Status: install ok installed
Version: 3.12.3-1

Package: python3
Status: install ok installed
Depends: python3.12 (>= 3.12.3-0~), libpython3-stdlib (= 3.12.3-0ubuntu2)
Provides: python3-supported-min (= 3.12)

Package: python3-apt
Status: install ok installed
Depends: python3 (<< 3.13), python3:any, libc6 (>= 2.34)

Package: meld
Status: install ok installed
Depends: python3-gi | python3-apt, python3-supported-min

Package: python3.11
Status: install ok installed
Version: 3.11.9-1

Package: bash
Status: install ok installed
Essential: yes

Package: removed
Status: deinstall ok config-files
Depends: python3.12
`

func TestDpkgDependencies(t *testing.T) {
	in, err := parseDpkgDeps(context.Background(), strings.NewReader(dpkgFixture))
	if err != nil {
		t.Fatal(err)
	}
	g := in.Graph
	if g.Has("removed") || !g.Has("python3.11") {
		t.Error("only installed packages are nodes")
	}
	direct, indirect := g.Dependents("python3.12")
	if strings.Join(direct, ",") != "python3" || indirect != 2 {
		t.Errorf("python3.12 dependents = %v + %d indirect (python3-apt, meld via python3)", direct, indirect)
	}
	direct, _ = g.Dependents("python3")
	if strings.Join(direct, ",") != "meld,python3-apt" {
		t.Errorf("python3 dependents = %v (meld needs it via Provides)", direct)
	}
	if d, i := g.Dependents("python3.11"); len(d) != 0 || i != 0 {
		t.Error("python3.11 has no dependents")
	}
	if r, ok := g.ProtectedReason("bash"); !ok || !strings.Contains(r, "essential") {
		t.Errorf("Essential packages must be marked protected, got %q %v", r, ok)
	}
	if _, ok := g.ProtectedReason("python3.11"); ok {
		t.Error("ordinary packages aren't protected")
	}
}

func TestRPMDependencies(t *testing.T) {
	req := "dnf\tpython(abi) = 3.12\ndnf\trpmlib(CompressedFileNames)\ndnf\t/usr/bin/python3\npython3-libs\tpython3\n"
	prov := "python3\tpython(abi) = 3.12\npython3\tpython3\ndnf\tdnf\npython3-libs\tpython3-libs\n"
	g := parseRPMDeps(req, prov)
	direct, _ := g.Dependents("python3")
	if strings.Join(direct, ",") != "dnf,python3-libs" {
		t.Errorf("rpm dependents = %v", direct)
	}
}

func TestChocolateyAndScoopDependencies(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	write("choco/lib/someapp/someapp.nuspec", `<package><metadata><id>someapp</id><dependencies><group targetFramework="net45"><dependency id="vcredist140" version="14.0"/></group><dependency id="dotnet-8.0-runtime"/></dependencies></metadata></package>`)
	write("choco/lib/vcredist140/vcredist140.nuspec", `<package><metadata><id>vcredist140</id></metadata></package>`)
	in, err := NewChocolateyDeps(filepath.Join(root, "choco")).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := in.Graph.Dependents("vcredist140"); strings.Join(d, ",") != "someapp" {
		t.Errorf("choco dependents = %v", d)
	}
	if !in.Graph.Has("VCREDIST140") {
		t.Error("lookups must ignore case")
	}

	write("scoop/apps/app1/current/manifest.json", `{"depends":"main/python"}`)
	write("scoop/apps/app2/current/manifest.json", `{"depends":["python","7zip"]}`)
	write("scoop/apps/python/current/manifest.json", `{"version":"3.12.3"}`)
	in, err = NewScoopDeps([]inventory.ScoopRoot{{Dir: filepath.Join(root, "scoop")}}).Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if d, _ := in.Graph.Dependents("python"); strings.Join(d, ",") != "app1,app2" {
		t.Errorf("scoop dependents = %v", d)
	}
	if _, err := NewScoopDeps(nil).Read(context.Background()); err != core.ErrSourceUnavailable {
		t.Errorf("no scoop = %v", err)
	}
	if _, err := NewChocolateyDeps("").Read(context.Background()); err != core.ErrSourceUnavailable {
		t.Errorf("no choco = %v", err)
	}
}

func TestUserAssistDecoding(t *testing.T) {
	if rot13("{6Q809377-6NS0-444O-8957-N3773S02200R}\\Tvg\\tvg-onfu.rkr") != "{6D809377-6AF0-444B-8957-A3773F02200E}\\Git\\git-bash.exe" {
		t.Error("rot13")
	}
	b := make([]byte, 72)
	binary.LittleEndian.PutUint32(b[4:], 7)
	when := time.Date(2024, 3, 3, 10, 0, 0, 0, time.UTC)
	ft := uint64(when.UnixNano()/100) + 116444736000000000
	binary.LittleEndian.PutUint64(b[60:], ft)
	count, last, ok := parseUserAssistValue(b)
	if !ok || count != 7 || !last.Equal(when) {
		t.Errorf("decode = %d %v %v", count, last, ok)
	}
	if _, _, ok := parseUserAssistValue(make([]byte, 16)); ok {
		t.Error("short values must be rejected")
	}
	if _, _, ok := parseUserAssistValue(make([]byte, 72)); ok {
		t.Error("zero timestamps mean 'never' and must be rejected")
	}
	env := func(k string) string {
		return map[string]string{"ProgramW6432": `C:\Program Files`, "SystemRoot": `C:\Windows`}[k]
	}
	if got := resolveUserAssistPath(`{6D809377-6AF0-444B-8957-A3773F02200E}\Git\git-bash.exe`, env); got != `C:\Program Files\Git\git-bash.exe` {
		t.Errorf("resolve = %s", got)
	}
	usage := []UsageRecord{
		{Path: `C:\Program Files\Git\git-bash.exe`, LastRun: when},
		{Path: `C:\Program Files\GitHub Desktop\x.exe`, LastRun: now},
	}
	if last, ok := lastOpened(`C:\Program Files\Git\`, usage); !ok || !last.Equal(when) {
		t.Error("prefix match must respect folder boundaries (Git vs GitHub Desktop)")
	}
	if _, ok := lastOpened(`C:\`, usage); ok {
		t.Error("a drive root must never match")
	}
}

func baseInputs() Inputs {
	scan := core.ScanSummary{FinishedAt: now.Add(-time.Hour)}
	us := core.UpdateCheckSummary{FinishedAt: now.Add(-time.Hour)}
	return Inputs{Now: now, Scan: &scan, UpdateSummary: &us, Graphs: map[string]Graph{}, GraphLabels: map[string]string{}}
}

func findKey(fs []core.Finding, prefix string) *core.Finding {
	for i := range fs {
		if strings.HasPrefix(fs[i].Key, prefix) {
			return &fs[i]
		}
	}
	return nil
}

func TestSupportRules(t *testing.T) {
	in := baseInputs()
	reg := inventory.SourceRegistry
	in.Items = []core.SoftwareItem{
		{Name: "Python 3.8.10 (64-bit)", Version: "3.8.10", Source: reg, Kind: core.KindApp},
		{Name: "python3.10", Version: "3.10.12", Source: inventory.SourceDpkg, Kind: core.KindComponent},
		{Name: "Node.js", Version: "22.3.0", Source: reg, Kind: core.KindApp},
		{Name: "Microsoft Visual C++ 2010  x64 Redistributable - 10.0.40219", Source: reg, Kind: core.KindComponent},
		{Name: "Microsoft Visual C++ 2015-2022 Redistributable (x64) - 14.38", Source: reg, Kind: core.KindComponent},
	}
	in.Now = time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC) // Node 22 ends Apr 2027
	fs, rts, checks := Analyze(in)
	if len(rts) != 5 {
		t.Fatalf("runtimes = %+v", rts)
	}
	py := findKey(fs, "eol:python")
	if py == nil || py.Severity != core.SeverityAttention || !strings.Contains(py.Summary, "7 Oct 2024") || py.Reference == "" {
		t.Errorf("python 3.8 finding = %+v", py)
	}
	distro := findKey(fs, "eol-distro:python")
	if distro == nil || distro.Severity != core.SeverityInfo || !strings.Contains(distro.Summary, "distribution may still provide") {
		t.Errorf("distro-managed EOL must be info and say so: %+v", distro)
	}
	if f := findKey(fs, "ending:node"); f == nil || !strings.Contains(f.Title, "Apr 2027") {
		t.Errorf("node ending finding = %+v", f)
	}
	vc := findKey(fs, "eol:visual-c++")
	if vc == nil || vc.Severity != core.SeverityInfo || len(vc.Items) != 1 || !strings.Contains(vc.Suggestion, "Leave them installed") {
		t.Errorf("old VC++ finding = %+v", vc)
	}
	if fs[0].Severity != core.SeverityAttention {
		t.Error("attention findings must come first")
	}
	if checks[0].Found != 3 || !strings.Contains(checks[0].Note, "Java isn't judged") {
		t.Errorf("support check = %+v", checks[0])
	}
}

func TestDuplicateAndVersionRules(t *testing.T) {
	in := baseInputs()
	in.Items = []core.SoftwareItem{
		{Name: "Git", Version: "2.44.0", Source: inventory.SourceRegistry, Kind: core.KindApp, InstallLocation: `C:\Program Files\Git`},
		{Name: "git", Version: "2.45.1", Source: inventory.SourceScoop, Kind: core.KindApp},
		{Name: "Krita (x64) 5.2.2", Version: "5.2.2", Source: inventory.SourceRegistry, Kind: core.KindApp},
		{Name: "Krita (x64) 5.1.5", Version: "5.1.5", Source: inventory.SourceRegistry, Kind: core.KindApp},
		{Name: "firefox", Version: "1:1snap1-0ubuntu5", Source: inventory.SourceDpkg, Kind: core.KindApp},
		{Name: "firefox", Version: "125.0", Source: inventory.SourceSnap, Kind: core.KindApp},
		{Name: "clang", Version: "1:18.0", Source: inventory.SourceDpkg, Kind: core.KindApp},
		{Name: "clang-18", Version: "1:18.1.3", Source: inventory.SourceDpkg, Kind: core.KindApp},
		{Name: "python3.11-venv", Version: "3.11.9", Source: inventory.SourceDpkg, Kind: core.KindApp},
		{Name: "python3.12-venv", Version: "3.12.3", Source: inventory.SourceDpkg, Kind: core.KindApp},
		{Name: "libfoo1", Version: "1", Source: inventory.SourceDpkg, Kind: core.KindComponent},
		{Name: "libfoo1", Version: "2", Source: inventory.SourceSnap, Kind: core.KindComponent},
	}
	fs, _, _ := Analyze(in)
	if f := findKey(fs, "duplicate:git"); f == nil || len(f.Items) != 2 || !strings.Contains(strings.Join(f.Details, " "), `C:\Program Files\Git`) {
		t.Errorf("git duplicate = %+v", f)
	}
	if f := findKey(fs, "versions:krita"); f == nil || len(f.Items) != 2 {
		t.Errorf("krita versions = %+v", f)
	}
	if findKey(fs, "duplicate:firefox") != nil {
		t.Error("Ubuntu's transitional firefox deb is not a second copy")
	}
	if findKey(fs, "versions:clang") != nil || findKey(fs, "versions:python") != nil {
		t.Error("distro packages with versions in their names are different packages, not versions of one")
	}
	if findKey(fs, "duplicate:libfoo") != nil || findKey(fs, "versions:libfoo") != nil {
		t.Error("system components are not reported as duplicates")
	}
	if familyKey("Mozilla Firefox (x64 en-US) 125.0.1") != "mozillafirefox" {
		t.Errorf("familyKey = %s", familyKey("Mozilla Firefox (x64 en-US) 125.0.1"))
	}
}

func TestUnusedRuleNeedsPositiveEvidence(t *testing.T) {
	in := baseInputs()
	in.Items = []core.SoftwareItem{
		{Name: "Old Editor", Source: inventory.SourceRegistry, Kind: core.KindApp, InstallLocation: `C:\Program Files\Old Editor`},
		{Name: "Fresh Tool", Source: inventory.SourceRegistry, Kind: core.KindApp, InstallLocation: `C:\Program Files\Fresh`},
		{Name: "No Record App", Source: inventory.SourceRegistry, Kind: core.KindApp, InstallLocation: `C:\Program Files\NoRecord`},
	}
	in.UsageAvailable = true
	in.Usage = []UsageRecord{
		{Path: `C:\Program Files\Old Editor\editor.exe`, LastRun: now.AddDate(-2, 0, 0)},
		{Path: `C:\Program Files\Fresh\fresh.exe`, LastRun: now.AddDate(0, -1, 0)},
	}
	fs, _, checks := Analyze(in)
	f := findKey(fs, "unused")
	if f == nil || len(f.Items) != 1 || f.Items[0].Name != "Old Editor" || !strings.Contains(f.Items[0].Note, "last opened") {
		t.Fatalf("unused = %+v", f)
	}
	var uc core.CheckSummary
	for _, c := range checks {
		if strings.HasPrefix(c.Name, "Apps not opened") {
			uc = c
		}
	}
	if !uc.Available || uc.Found != 1 || !strings.Contains(uc.Note, "record for 2 of your apps") {
		t.Errorf("usage check = %+v", uc)
	}

	in.UsageAvailable, in.UsageNote = false, "Linux doesn't keep a record of when programs are opened."
	fs, _, checks = Analyze(in)
	if findKey(fs, "unused") != nil {
		t.Error("no usage data means no unused finding")
	}
	for _, c := range checks {
		if strings.HasPrefix(c.Name, "Apps not opened") && (c.Available || !strings.Contains(c.Note, "Linux")) {
			t.Errorf("unavailable usage check must explain why: %+v", c)
		}
	}
}

func TestDependenciesOnRuntimesAndUnneeded(t *testing.T) {
	in := baseInputs()
	dep, _ := parseDpkgDeps(context.Background(), strings.NewReader(dpkgFixture))
	in.Graphs[inventory.SourceDpkg] = dep.Graph
	in.GraphLabels[inventory.SourceDpkg] = "Package dependencies (dpkg)"
	in.Items = []core.SoftwareItem{
		{Name: "python3.12", PackageID: "python3.12", Version: "3.12.3-1", Source: inventory.SourceDpkg, Kind: core.KindComponent},
		{Name: "python3.11", PackageID: "python3.11", Version: "3.11.9-1", Source: inventory.SourceDpkg, Kind: core.KindComponent},
		{Name: "python3", PackageID: "python3", Source: inventory.SourceDpkg, Kind: core.KindApp},
		{Name: "Microsoft Visual C++ 2015-2022 Redistributable (x64)", Source: inventory.SourceRegistry, Kind: core.KindComponent},
	}
	fs, rts, _ := Analyze(in)
	byName := map[string]core.Runtime{}
	for _, r := range rts {
		byName[r.Name] = r
	}
	p12 := byName["python3.12"]
	if !p12.DepsKnown || strings.Join(p12.Dependents, ",") != "python3" || p12.Indirect != 2 || p12.DepsSource == "" {
		t.Errorf("python3.12 runtime = %+v", p12)
	}
	if vc := byName["Microsoft Visual C++ 2015-2022 Redistributable (x64)"]; vc.DepsKnown {
		t.Error("Windows records no dependency data for VC++; DepsKnown must be false")
	}
	f := findKey(fs, "unneeded:dpkg:python3.11")
	if f == nil || !strings.Contains(f.Details[0], "recommend") {
		t.Errorf("python3.11 unneeded finding = %+v", f)
	}
	if findKey(fs, "unneeded:dpkg:python3.12") != nil {
		t.Error("a needed runtime must not be reported as unneeded")
	}
}

func TestFreshnessRules(t *testing.T) {
	in := baseInputs()
	in.Updates = []core.Update{{Name: "Git", AvailableVersion: "2"}, {Name: "Held", Held: true}}
	fs, _, _ := Analyze(in)
	if f := findKey(fs, "updates:pending"); f == nil || f.Severity != core.SeverityAttention || len(f.Items) != 1 {
		t.Errorf("pending = %+v", f)
	}
	in.Updates = nil
	in.UpdateSummary = nil
	old := core.ScanSummary{FinishedAt: now.AddDate(0, -2, 0)}
	in.Scan = &old
	fs, _, _ = Analyze(in)
	if findKey(fs, "updates:never") == nil || findKey(fs, "scan:stale") == nil {
		t.Errorf("never-checked and stale-scan findings expected: %+v", fs)
	}
}

// fakeSource is a test double.
type fakeSource struct {
	id  string
	in  Input
	err error
}

func (f fakeSource) ID() string                          { return f.id }
func (f fakeSource) Label() string                       { return "Source " + f.id }
func (f fakeSource) Detail() string                      { return "" }
func (f fakeSource) Read(context.Context) (Input, error) { return f.in, f.err }

func TestServiceAnalyzeSavesAndRecords(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	svc := NewService([]Source{fakeSource{id: "x", err: core.ErrSourceUnavailable}}, db, nil, false).WithClock(func() time.Time { return now })

	if _, err := svc.Analyze(ctx, nil); !errors.Is(err, ErrNoScan) {
		t.Fatalf("analysis without a scan must refuse: %v", err)
	}
	db.SaveScan(ctx, core.ScanReport{FinishedAt: now, Status: core.ScanComplete, Items: []core.SoftwareItem{
		{Name: "Python 3.8.10 (64-bit)", Version: "3.8.10", Source: inventory.SourceRegistry, Kind: core.KindApp},
	}})
	res, err := svc.Analyze(ctx, nil)
	if err != nil || res.Status != core.ScanComplete {
		t.Fatalf("analyze: %+v %v", res.Status, err)
	}
	if att, _ := res.Counts(); att != 1 || res.SupportDataAsOf != SupportDataAsOf || !res.BasedOnScan.Equal(now) {
		t.Errorf("result = %+v", res)
	}
	saved, ok, _ := svc.Latest(ctx)
	if !ok || len(saved.Findings) != len(res.Findings) {
		t.Error("analysis should be saved")
	}
	acts, _ := db.RecentActivity(ctx, 1)
	if acts[0].Kind != "analysis" || !strings.Contains(acts[0].Summary, "1 thing worth your attention") {
		t.Errorf("activity = %+v", acts[0])
	}

	failing := NewService([]Source{fakeSource{id: "y", err: errors.New("boom")}}, db, nil, false)
	res, _ = failing.Analyze(ctx, nil)
	if res.Status != core.ScanPartial || len(res.Findings) == 0 {
		t.Errorf("a failed optional source still yields an analysis, flagged partial: %+v", res.Status)
	}
}

func TestCancelBeforeStartIsReportedAsCancelled(t *testing.T) {
	db, err := database.Open(context.Background(), filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SaveScan(context.Background(), core.ScanReport{FinishedAt: now, Status: core.ScanComplete, Items: []core.SoftwareItem{{Name: "x", Source: "s", Kind: core.KindApp}}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // the user pressed x straight away
	res, err := NewService([]Source{fakeSource{id: "x"}}, db, nil, false).Analyze(ctx, nil)
	if err != nil || res.Status != core.ScanCancelled {
		t.Errorf("want cancelled with no error, got %s, %v", res.Status, err)
	}
	if _, ok, _ := db.LatestAnalysis(context.Background()); ok {
		t.Error("a cancelled analysis must not be saved")
	}
	acts, _ := db.RecentActivity(context.Background(), 1)
	if len(acts) != 1 || acts[0].Outcome != core.OutcomeCancelled {
		t.Errorf("cancel should be recorded: %+v", acts)
	}
}
