package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func makeZip(t *testing.T, dir, name string, files ...string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for _, n := range files {
		fw, _ := w.Create(n)
		fw.Write([]byte("content of " + n))
	}
	w.Close()
	f.Close()
	return p
}

func readYAML(t *testing.T, path string) map[string]any {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := yaml.Unmarshal(b, &m); err != nil {
		t.Fatalf("%s isn't valid YAML: %v\n%s", path, err, b)
	}
	return m
}

func TestWingetManifests(t *testing.T) {
	tmp := t.TempDir()
	x64 := makeZip(t, tmp, "atlas_0.1.0_windows_amd64.zip", "atlas.exe", "LICENSE", "README.md")
	arm := makeZip(t, tmp, "atlas_0.1.0_windows_arm64.zip", "atlas.exe", "LICENSE")
	dir, err := writeWinget(Options{Version: "0.1.0", Date: "2026-11-13", X64Zip: x64, Arm64Zip: arm, Out: filepath.Join(tmp, "out")})
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(tmp, "out", "manifests", "d", "Deepakraj1207", "ATLAS", "0.1.0"); dir != want {
		t.Errorf("dir = %s, want the winget-pkgs layout %s", dir, want)
	}

	v := readYAML(t, filepath.Join(dir, "Deepakraj1207.ATLAS.yaml"))
	if v["PackageIdentifier"] != "Deepakraj1207.ATLAS" || v["PackageVersion"] != "0.1.0" || v["ManifestType"] != "version" || v["DefaultLocale"] != "en-US" {
		t.Errorf("version manifest = %v", v)
	}

	in := readYAML(t, filepath.Join(dir, "Deepakraj1207.ATLAS.installer.yaml"))
	if in["InstallerType"] != "zip" || in["NestedInstallerType"] != "portable" {
		t.Errorf("installer manifest = %v", in)
	}
	if raw, _ := os.ReadFile(filepath.Join(dir, "Deepakraj1207.ATLAS.installer.yaml")); !strings.Contains(string(raw), "\nReleaseDate: 2026-11-13\n") {
		t.Errorf("release date missing:\n%s", raw)
	}
	nested := in["NestedInstallerFiles"].([]any)[0].(map[string]any)
	if nested["RelativeFilePath"] != "atlas.exe" || nested["PortableCommandAlias"] != "atlas" {
		t.Errorf("nested = %v", nested)
	}
	insts := in["Installers"].([]any)
	if len(insts) != 2 {
		t.Fatalf("installers = %v", insts)
	}
	b, _ := os.ReadFile(x64)
	sum := sha256.Sum256(b)
	first := insts[0].(map[string]any)
	if first["Architecture"] != "x64" || first["InstallerSha256"] != strings.ToUpper(hex.EncodeToString(sum[:])) ||
		first["InstallerUrl"] != "https://github.com/Deepakraj1207/atlas/releases/download/v0.1.0/atlas_0.1.0_windows_amd64.zip" {
		t.Errorf("x64 installer = %v", first)
	}
	if insts[1].(map[string]any)["Architecture"] != "arm64" {
		t.Errorf("arm64 installer = %v", insts[1])
	}

	l := readYAML(t, filepath.Join(dir, "Deepakraj1207.ATLAS.locale.en-US.yaml"))
	if l["Moniker"] != "atlas" || l["License"] != "MIT" || l["PackageName"] != "ATLAS" || l["Publisher"] != "Deepakraj1207" ||
		!strings.Contains(l["Description"].(string), "Nothing\nhappens without a preview") {
		t.Errorf("locale manifest = %v", l)
	}
	if sd := l["ShortDescription"].(string); len(sd) > 256 {
		t.Errorf("ShortDescription too long for winget (%d)", len(sd))
	}
}

func TestWingetRefusesBadInput(t *testing.T) {
	tmp := t.TempDir()
	good := makeZip(t, tmp, "good.zip", "atlas.exe")
	nested := makeZip(t, tmp, "nested.zip", "atlas/atlas.exe")
	for name, o := range map[string]Options{
		"exe not at the top": {Version: "0.1.0", Date: "2026-11-13", X64Zip: nested},
		"bad version":        {Version: "v0.1", Date: "2026-11-13", X64Zip: good},
		"bad date":           {Version: "0.1.0", Date: "13/11/2026", X64Zip: good},
		"no x64 zip":         {Version: "0.1.0", Date: "2026-11-13"},
		"missing file":       {Version: "0.1.0", Date: "2026-11-13", X64Zip: filepath.Join(tmp, "nope.zip")},
	} {
		o.Out = filepath.Join(tmp, "out")
		if _, err := writeWinget(o); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}
