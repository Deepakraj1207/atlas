package inventory

// The fixtures in this file reproduce the *shape* of real command output so
// the parsers can be tested. They are test data only and never shown to users.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deepakraj1207/atlas/internal/core"
)

const wingetFixture = "\r   - \r   \\ \r                                                                                                                        \r" +
	"Name                                   Id                                Version          Available Source\n" +
	"-----------------------------------------------------------------------------------------------------------\n" +
	"Git                                    Git.Git                           2.44.0                     winget\n" +
	"Microsoft Visual C++ 2015-2022 Redist… Microsoft.VCRedist.2015+.x64      14.38.33135.0    14.40.0   winget\n" +
	"Some Local Tool                        ARP\\Machine\\X64\\SomeLocalTool     1.2.3\n" +
	"Microsoft.UI.Xaml.2.8                  MSIX\\Microsoft.UI.Xaml.2.8_8.23… 8.2310.30001.0\n" +
	"日本語ツール                             Vendor.JpTool                     3.0              4.0       winget\n" +
	"Truncated Id App                       Vendor.SomeVeryLongPackageIde…    5.1                        winget\n"

func TestParseWingetList(t *testing.T) {
	items, err := parseWingetList(wingetFixture)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(items) != 6 {
		t.Fatalf("want 6 items, got %d: %+v", len(items), items)
	}
	byName := map[string]core.SoftwareItem{}
	for _, it := range items {
		byName[it.Name] = it
	}

	git := byName["Git"]
	if git.PackageID != "Git.Git" || git.Version != "2.44.0" || git.Kind != core.KindApp || git.Source != SourceWinget {
		t.Errorf("git = %+v", git)
	}
	vc := byName["Microsoft Visual C++ 2015-2022 Redist…"]
	if vc.Version != "14.38.33135.0" || vc.Kind != core.KindComponent {
		t.Errorf("vc redist = %+v (available version must not leak into Version)", vc)
	}
	if local := byName["Some Local Tool"]; local.PackageID != "" || local.Version != "1.2.3" {
		t.Errorf("ARP-only entry should have no package ID: %+v", local)
	}
	if xaml := byName["Microsoft.UI.Xaml.2.8"]; xaml.Kind != core.KindComponent || xaml.PackageID != "" {
		t.Errorf("MSIX framework should be a component without ID: %+v", xaml)
	}
	if jp := byName["日本語ツール"]; jp.PackageID != "Vendor.JpTool" || jp.Version != "3.0" {
		t.Errorf("wide characters misaligned columns: %+v", jp)
	}
	if tr := byName["Truncated Id App"]; tr.PackageID != "" {
		t.Errorf("truncated IDs must be dropped, got %q", tr.PackageID)
	}
}

func TestParseWingetLocalizedHeaderAndNoAvailableColumn(t *testing.T) {
	out := "Nom          ID             Version  Source\n" +
		"--------------------------------------------\n" +
		"Firefox      Mozilla.Firefox 125.0   winget\n"
	items, err := parseWingetList(out)
	if err != nil || len(items) != 1 {
		t.Fatalf("got %v, %v", items, err)
	}
	if items[0].PackageID != "Mozilla.Firefox" || items[0].Version != "125.0" {
		t.Errorf("localized header parse: %+v", items[0])
	}
}

func TestParseWingetGarbage(t *testing.T) {
	if _, err := parseWingetList("Failed when searching source; results will not be included: msstore\n"); err == nil {
		t.Error("output without a table should be an error")
	}
	if items, err := parseWingetList("   \r\n"); err != nil || len(items) != 0 {
		t.Error("empty output should be no items, no error")
	}
}

func TestWingetScannerUsesTableEvenOnNonZeroExit(t *testing.T) {
	run := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if name != "winget" || strings.Join(args, " ") != "list --disable-interactivity" {
			t.Errorf("unexpected command %s %v", name, args)
		}
		return []byte(wingetFixture), &CommandError{Command: "winget", ExitCode: 1}
	}
	items, err := NewWingetScanner(run).Scan(context.Background())
	if err != nil || len(items) == 0 {
		t.Errorf("want items despite exit code, got %d, %v", len(items), err)
	}
}

type mapValues map[string]any

func (m mapValues) String(n string) (string, bool) { s, ok := m[n].(string); return s, ok }
func (m mapValues) Int(n string) (uint64, bool)    { i, ok := m[n].(int); return uint64(i), ok }

func TestRegistryItemRules(t *testing.T) {
	cases := []struct {
		name string
		v    mapValues
		ok   bool
	}{
		{"normal", mapValues{"DisplayName": "Example Editor", "DisplayVersion": "1.2", "Publisher": "Example Co", "InstallDate": "20240301", "InstallLocation": `"C:\Program Files\Example"`}, true},
		{"no name", mapValues{"DisplayVersion": "1"}, false},
		{"system component", mapValues{"DisplayName": "Hidden", "SystemComponent": 1}, false},
		{"child update", mapValues{"DisplayName": "Patch", "ParentKeyName": "Office"}, false},
		{"release type update", mapValues{"DisplayName": "Something", "ReleaseType": "Security Update"}, false},
		{"kb name", mapValues{"DisplayName": "Update for Windows (KB5034441)"}, false},
	}
	for _, c := range cases {
		it, ok := registryItem(c.v, "machine")
		if ok != c.ok {
			t.Errorf("%s: ok = %v, want %v", c.name, ok, c.ok)
		}
		if c.name == "normal" {
			if it.InstallDate != "2024-03-01" || it.InstallLocation != `C:\Program Files\Example` || it.Scope != "machine" || it.Source != SourceRegistry {
				t.Errorf("normal entry mapped wrong: %+v", it)
			}
		}
	}
	it, _ := registryItem(mapValues{"DisplayName": "Microsoft .NET Runtime - 8.0.4 (x64)"}, "machine")
	if it.Kind != core.KindComponent {
		t.Error(".NET runtime should be a component")
	}
}

func TestScoopScanner(t *testing.T) {
	root := t.TempDir()
	write := func(rel, body string) {
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	write("apps/ripgrep/current/manifest.json", `{"version":"14.1.0","description":"Fast search"}`)
	write("apps/ripgrep/current/install.json", `{"bucket":"main"}`)
	write("apps/scoop/current/manifest.json", `{"version":"0.4"}`)
	os.MkdirAll(filepath.Join(root, "apps", "broken"), 0o755) // no current folder: not installed
	// Newer Scoop names the files scoop-manifest.json / scoop-install.json
	// (seen on a real Windows 11 PC); one written with a byte-order mark.
	write("apps/7zip/current/scoop-manifest.json", "\uFEFF"+`{"version":"26.06","description":"Archiver"}`)
	write("apps/7zip/current/scoop-install.json", `{"bucket":"main"}`)
	// Metadata unreadable, but installed: the version comes from the folder
	// "current" links to.
	os.MkdirAll(filepath.Join(root, "apps", "fastfetch", "2.71.0"), 0o755)
	linked := os.Symlink(filepath.Join(root, "apps", "fastfetch", "2.71.0"), filepath.Join(root, "apps", "fastfetch", "current")) == nil

	items, err := NewScoopScanner([]ScoopRoot{{Dir: root, Scope: "user"}, {Dir: filepath.Join(root, "missing"), Scope: "machine"}}).Scan(context.Background())
	want := 2
	if linked {
		want = 3
	}
	if err != nil || len(items) != want {
		t.Fatalf("got %+v, %v", items, err)
	}
	byName := map[string]core.SoftwareItem{}
	for _, it := range items {
		byName[it.Name] = it
	}
	if it := byName["ripgrep"]; it.Version != "14.1.0" || it.PackageID != "main/ripgrep" || it.Scope != "user" {
		t.Errorf("ripgrep = %+v", it)
	}
	if it := byName["7zip"]; it.Version != "26.06" || it.PackageID != "main/7zip" || it.Description != "Archiver" {
		t.Errorf("7zip (new file names, BOM) = %+v", it)
	}
	if it, ok := byName["fastfetch"]; linked && (!ok || it.Version != "2.71.0") {
		t.Errorf("fastfetch (no metadata) = %+v", it)
	}

	if _, err := NewScoopScanner([]ScoopRoot{{Dir: filepath.Join(root, "nope")}}).Scan(context.Background()); err != core.ErrSourceUnavailable {
		t.Errorf("missing scoop should be unavailable, got %v", err)
	}
}

func TestChocolateyScanner(t *testing.T) {
	root := t.TempDir()
	write := func(id, title string) {
		dir := filepath.Join(root, "lib", id)
		os.MkdirAll(dir, 0o755)
		body := `<?xml version="1.0"?><package xmlns="http://schemas.microsoft.com/packaging/2015/06/nuspec.xsd"><metadata>` +
			`<id>` + id + `</id><version>2.0.1</version><title>` + title + `</title><authors>Someone</authors>` +
			`<description>First line
second line</description></metadata></package>`
		os.WriteFile(filepath.Join(dir, id+".nuspec"), []byte(body), 0o644)
	}
	write("7zip.install", "7-Zip (Install)")
	write("chocolatey", "Chocolatey")
	write("chocolatey-core.extension", "")

	items, err := NewChocolateyScanner(root).Scan(context.Background())
	if err != nil || len(items) != 3 {
		t.Fatalf("got %+v, %v", items, err)
	}
	kinds := map[string]core.Kind{}
	for _, it := range items {
		kinds[it.PackageID] = it.Kind
		if it.PackageID == "7zip.install" && (it.Name != "7-Zip (Install)" || it.Description != "First line" || it.Publisher != "Someone") {
			t.Errorf("7zip = %+v", it)
		}
	}
	if kinds["7zip.install"] != core.KindApp || kinds["chocolatey"] != core.KindComponent || kinds["chocolatey-core.extension"] != core.KindComponent {
		t.Errorf("kinds = %v", kinds)
	}
	if _, err := NewChocolateyScanner(filepath.Join(root, "x")).Scan(context.Background()); err != core.ErrSourceUnavailable {
		t.Errorf("missing choco should be unavailable, got %v", err)
	}
}

const dpkgStatusFixture = `Package: htop
Status: install ok installed
Priority: optional
Section: utils
Maintainer: Example Maintainers <maint@example.invalid>
Architecture: amd64
Version: 3.3.0-4
Description: interactive processes viewer
 Long description continues here.

Package: libexample1
Status: install ok installed
Section: libs
Architecture: amd64
Version: 1.0-1
Description: example library

Package: removed-tool
Status: deinstall ok config-files
Version: 0.9
Description: gone

Package: pulled-in
Status: install ok installed
Section: utils
Architecture: amd64
Version: 2.0
Description: dependency of something
`

func TestDpkgScanner(t *testing.T) {
	dir := t.TempDir()
	status := filepath.Join(dir, "status")
	ext := filepath.Join(dir, "extended_states")
	os.WriteFile(status, []byte(dpkgStatusFixture), 0o644)
	os.WriteFile(ext, []byte("Package: pulled-in\nArchitecture: amd64\nAuto-Installed: 1\n"), 0o644)

	items, err := NewDpkgScanner(status, ext).Scan(context.Background())
	if err != nil || len(items) != 3 {
		t.Fatalf("got %+v, %v", items, err)
	}
	byName := map[string]core.SoftwareItem{}
	for _, it := range items {
		byName[it.Name] = it
	}
	if h := byName["htop"]; h.Kind != core.KindApp || h.Publisher != "Example Maintainers" || h.Description != "interactive processes viewer" || h.Version != "3.3.0-4" {
		t.Errorf("htop = %+v", h)
	}
	if byName["libexample1"].Kind != core.KindComponent {
		t.Error("library should be a component")
	}
	if byName["pulled-in"].Kind != core.KindComponent {
		t.Error("auto-installed package should be a component")
	}
	if _, ok := byName["removed-tool"]; ok {
		t.Error("removed package must be skipped")
	}
	if _, err := NewDpkgScanner(filepath.Join(dir, "none"), ext).Scan(context.Background()); err != core.ErrSourceUnavailable {
		t.Errorf("missing status file should be unavailable, got %v", err)
	}
}

func TestParseRPM(t *testing.T) {
	items := parseRPM("bash\t5.2.26-3.fc40\tFedora Project\tThe GNU Bourne Again shell\nglibc-devel\t2.39-1\t(none)\tHeaders\n\n")
	if len(items) != 2 {
		t.Fatalf("got %+v", items)
	}
	if items[0].Name != "bash" || items[0].Version != "5.2.26-3.fc40" || items[0].Kind != core.KindApp {
		t.Errorf("bash = %+v", items[0])
	}
	if items[1].Publisher != "" || items[1].Kind != core.KindComponent {
		t.Errorf("glibc-devel = %+v", items[1])
	}
}

func TestParseSnapList(t *testing.T) {
	out := `Name      Version          Rev    Tracking         Publisher   Notes
core22    20240111         1122   latest/stable    canonical✓  base
firefox   124.0.2-1        4136   latest/stable/…  mozilla✓    -
snapd     2.61.2           21184  latest/stable    canonical✓  snapd
`
	items := parseSnapList(out)
	if len(items) != 3 {
		t.Fatalf("got %+v", items)
	}
	if items[1].Name != "firefox" || items[1].Publisher != "mozilla" || items[1].Kind != core.KindApp {
		t.Errorf("firefox = %+v", items[1])
	}
	if items[0].Kind != core.KindComponent || items[2].Kind != core.KindComponent {
		t.Error("base and snapd should be components")
	}
	if len(parseSnapList("No snaps are installed yet.\n")) != 0 {
		t.Error("no-snaps message should produce no items")
	}
}

func TestFlatpakScanner(t *testing.T) {
	run := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		if args[1] == "--app" {
			return []byte("org.gimp.GIMP\tGNU Image Manipulation Program\t2.10.38\tflathub\tsystem\n"), nil
		}
		return []byte("org.gnome.Platform    GNOME Application Platform version 46    flathub    user\n"), nil
	}
	items, err := NewFlatpakScanner(run).Scan(context.Background())
	if err != nil || len(items) != 2 {
		t.Fatalf("got %+v, %v", items, err)
	}
	if g := items[0]; g.Name != "GNU Image Manipulation Program" || g.PackageID != "org.gimp.GIMP" || g.Scope != "machine" || g.Kind != core.KindApp {
		t.Errorf("gimp = %+v", g)
	}
	if items[1].Kind != core.KindComponent || items[1].PackageID != "org.gnome.Platform" {
		t.Errorf("runtime = %+v", items[1])
	}
}

func TestExecRunnerMissingProgramIsUnavailable(t *testing.T) {
	_, err := ExecRunner(context.Background(), "atlas-definitely-not-a-real-program")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("got %v", err)
	}
	if !isUnavailable(err) {
		t.Error("missing program should wrap ErrSourceUnavailable")
	}
}

func TestWingetAgreementsPendingIsExplained(t *testing.T) {
	run := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		return []byte("One or more of the source agreements were not agreed to. Operation cancelled.\n"),
			&CommandError{Command: "winget", ExitCode: 2316632134}
	}
	_, err := NewWingetScanner(run).Scan(context.Background())
	var se *core.SourceError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "accept its source agreements") || !strings.Contains(se.Details, "winget list") {
		t.Errorf("agreements error should explain what to do: %v", err)
	}
}
