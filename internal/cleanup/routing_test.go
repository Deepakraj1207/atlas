package cleanup

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/platform"
)

func rpmItem(name string) core.SoftwareItem {
	return core.SoftwareItem{Name: name, PackageID: name, Version: "1.0", Source: inventory.SourceRPM, Kind: core.KindApp}
}

// The audit's P0: RPM packages were always removed with DNF, which doesn't
// exist on openSUSE. The distribution's own tool is used, or ATLAS refuses.
func TestRPMRemovalFollowsTheDistribution(t *testing.T) {
	removers := []Remover{NewDnfRemover(false), NewZypperRemover(false)}

	s, _ := newSvc(t, Config{Removers: removers, RPM: platform.RPMRoute{Manager: "zypper"}})
	allowed, blocked := plan(t, s, rpmItem("mytool"))
	if len(blocked) != 0 || len(allowed) != 1 {
		t.Fatalf("openSUSE: allowed=%+v blocked=%+v", allowed, blocked)
	}
	if got := allowed[0].Commands[0].String(); got != "sudo zypper remove mytool" || allowed[0].Manager != "zypper" {
		t.Errorf("openSUSE should remove with zypper, got %q (%s)", got, allowed[0].Manager)
	}

	s, _ = newSvc(t, Config{Removers: removers, RPM: platform.RPMRoute{Manager: "dnf"}})
	if allowed, _ := plan(t, s, rpmItem("mytool")); len(allowed) != 1 || allowed[0].Commands[0].String() != "sudo dnf remove mytool" {
		t.Errorf("Fedora should remove with dnf: %+v", allowed)
	}

	refusal := "Ubuntu 24.04.5 LTS doesn't manage its software with RPM, so ATLAS won't change RPM packages here."
	s, _ = newSvc(t, Config{Removers: removers, RPM: platform.RPMRoute{Reason: refusal}})
	if _, blocked := plan(t, s, rpmItem("mytool")); len(blocked) != 1 || blocked[0].Blocked != refusal || len(blocked[0].Commands) != 0 {
		t.Errorf("unsupported: must refuse with the reason and no command: %+v", blocked)
	}

	// No route given at all (a misconfiguration) also refuses.
	s, _ = newSvc(t, Config{Removers: removers})
	if _, blocked := plan(t, s, rpmItem("mytool")); len(blocked) != 1 || !strings.Contains(blocked[0].Blocked, "won't guess") {
		t.Errorf("no route: %+v", blocked)
	}
}

func TestDnfProtectedListOnlyAppliesToDnf(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "x.conf"), "mytool\n")
	removers := []Remover{NewDnfRemover(true), NewZypperRemover(true)}
	s, _ := newSvc(t, Config{Removers: removers, DnfProtectedDir: dir, RPM: platform.RPMRoute{Manager: "dnf"}})
	if _, blocked := plan(t, s, rpmItem("mytool")); len(blocked) != 1 || !strings.Contains(blocked[0].Blocked, "DNF lists this as a protected") {
		t.Errorf("dnf: %+v", blocked)
	}
	s, _ = newSvc(t, Config{Removers: removers, DnfProtectedDir: dir, RPM: platform.RPMRoute{Manager: "zypper"}})
	if allowed, _ := plan(t, s, rpmItem("mytool")); len(allowed) != 1 {
		t.Errorf("zypper: DNF's list doesn't apply: %+v", allowed)
	}
}
