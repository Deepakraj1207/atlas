package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/discover"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

func testCatalog() discover.Catalog {
	return discover.Catalog{AsOf: "2026-11", Entries: []core.CatalogEntry{
		{Name: "7-Zip", Packages: []core.CatalogPackage{{Manager: "winget", ID: "7zip.7zip"}, {Manager: "scoop", ID: "main/7zip"}, {Manager: "chocolatey", ID: "7zip"}}},
		{Name: "Gone", Packages: []core.CatalogPackage{{Manager: "winget", ID: "Old.Gone"}, {Manager: "scoop", ID: "extras/gone"}, {Manager: "flatpak", ID: "org.example.Gone"}}},
		{Name: "jq", Packages: []core.CatalogPackage{{Manager: "apt", ID: "jq"}, {Manager: "flatpak", ID: "org.example.Jq"}}},
	}}
}

func TestCheckFindsAndMisses(t *testing.T) {
	scoop := t.TempDir()
	os.MkdirAll(filepath.Join(scoop, "buckets", "main", "bucket"), 0o755)
	os.WriteFile(filepath.Join(scoop, "buckets", "main", "bucket", "7zip.json"), []byte("{}"), 0o644)
	run := func(ctx context.Context, name string, args ...string) ([]byte, error) {
		switch {
		case name == "winget" && args[2] == "7zip.7zip":
			return []byte("Found 7-Zip [7zip.7zip]\nVersion: 26.05\n"), nil
		case name == "winget":
			return []byte("No package found matching input criteria.\n"), &inventory.CommandError{Command: "winget", ExitCode: 1}
		case name == "choco":
			return []byte("7zip|26.5.0\n"), nil
		case name == "apt-cache":
			return nil, errors.New("apt-cache not found: " + core.ErrSourceUnavailable.Error())
		}
		t.Fatalf("unexpected command %s %v", name, args)
		return nil, nil
	}
	c := Checker{Run: run, ScoopRoot: scoop, Timeout: time.Second,
		HTTPGet: func(ctx context.Context, url string) (int, error) {
			if strings.HasSuffix(url, "/org.example.Jq") {
				return 200, nil
			}
			return 404, nil
		}}
	res := c.Check(context.Background(), testCatalog(), []string{"winget", "scoop", "chocolatey", "flatpak", "apt"})
	got := map[string]string{}
	for _, r := range res {
		got[r.Manager+" "+r.ID] = r.Status
	}
	want := map[string]string{
		"winget 7zip.7zip": "found", "winget Old.Gone": "missing",
		"scoop main/7zip": "found", "scoop extras/gone": "not checked", // bucket not added here
		"chocolatey 7zip":        "found",
		"flatpak org.example.Jq": "found", "flatpak org.example.Gone": "missing",
		"apt jq": "not checked", // apt-cache couldn't run
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: %q, want %q", k, got[k], v)
		}
	}
	if len(res) != len(want) {
		t.Errorf("checked %d packages, want %d", len(res), len(want))
	}

	var buf bytes.Buffer
	if n := Report(&buf, "2026-11", res); n != 2 {
		t.Errorf("missing = %d, want 2", n)
	}
	if !strings.Contains(buf.String(), "| MISSING | winget | Gone | `Old.Gone` |") || !strings.Contains(buf.String(), "4 found, 2 missing, 2 not checked.") {
		t.Errorf("report:\n%s", buf.String())
	}
}

func TestRealCatalogueBuckets(t *testing.T) {
	cat, err := discover.Load()
	if err != nil {
		t.Fatal(err)
	}
	b := scoopBuckets(cat)
	if len(b) == 0 || b[0] > b[len(b)-1] {
		t.Errorf("buckets = %v", b)
	}
}
