package cleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deepakraj1207/atlas/internal/analysis"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/platform"
)

// protectedNames are never removed: the operating system or ATLAS relies on
// them. Compared case-insensitively against name and package ID.
var protectedNames = map[string]bool{
	// package managers and privilege tools ATLAS uses
	"sudo": true, "apt": true, "apt-utils": true, "dpkg": true, "rpm": true, "dnf": true, "dnf5": true, "yum": true,
	"snapd": true, "flatpak": true, "policykit-1": true, "polkitd": true, "pkexec": true,
	"winget": true, "microsoft.appinstaller": true, "app installer": true, "chocolatey": true, "scoop": true,
	// core system
	"systemd": true, "systemd-sysv": true, "init": true, "bash": true, "coreutils": true, "util-linux": true,
	"login": true, "passwd": true, "libc6": true, "libc-bin": true, "glibc": true, "base-files": true,
	"base-passwd": true, "ubuntu-minimal": true, "ubuntu-standard": true, "openssh-server": true,
	"network-manager": true, "networkmanager": true,
	"microsoft edge": true, "microsoft edge webview2 runtime": true, "microsoft.edge": true, "microsoft.edgewebview2runtime": true,
}

var protectedPrefixes = []string{"linux-image", "linux-generic", "linux-modules", "kernel", "grub", "shim-signed", "systemd-boot"}

func isProtected(it core.SoftwareItem) bool {
	for _, v := range []string{it.Name, it.PackageID} {
		l := strings.ToLower(strings.TrimSpace(v))
		if l == "" {
			continue
		}
		if protectedNames[l] {
			return true
		}
		if inventory.SourceDpkg == it.Source || inventory.SourceRPM == it.Source {
			for _, p := range protectedPrefixes {
				if strings.HasPrefix(l, p) {
					return true
				}
			}
		}
	}
	return false
}

// readDnfProtected reads DNF's own list of packages it refuses to remove.
func readDnfProtected(dir string) map[string]bool {
	out := map[string]bool{}
	files, _ := filepath.Glob(filepath.Join(dir, "*.conf"))
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, l := range strings.Split(string(data), "\n") {
			l = strings.TrimSpace(l)
			if l != "" && !strings.HasPrefix(l, "#") {
				out[strings.ToLower(l)] = true
			}
		}
	}
	return out
}

// gate decides whether an item may be removed.
type gate struct {
	// deps holds, per inventory source with a dependency source, what
	// reading it produced: the graph, or why it couldn't be read.
	deps         map[string]depRecords
	dnfProtected map[string]bool
	removing     map[string]bool // source|package key of everything in this plan
}

// depRecords is one dependency source's outcome for a removal plan.
type depRecords struct {
	result core.SourceResult
	graph  analysis.Graph
	read   bool // the graph arrived
}

// label names the records in a sentence: "package dependencies (dpkg)".
func (r depRecords) label() string {
	l := r.result.Label
	if l == "" {
		l = "Dependency records for " + r.result.Source
	}
	return strings.ToLower(l[:1]) + l[1:]
}

// unverified explains why records that were expected can't be relied on,
// or returns "" when they can.
func (r depRecords) unverified() string {
	label := r.label()
	switch {
	case r.result.Status == core.SourceTimeout:
		return "Reading the " + label + " took too long and was stopped, so ATLAS can't check what needs this. Nothing was removed; try again."
	case r.result.Status == core.SourceUnavailable:
		return "The " + label + " isn't available on this computer, although this was installed with that tool, so ATLAS can't check what needs it."
	case r.result.Status == core.SourceCancelled:
		return "The dependency check was stopped before the " + label + " were read, so ATLAS can't check what needs this."
	case r.result.Status != core.SourceOK || !r.read:
		why := r.result.Message
		if why == "" {
			why = "no result"
		}
		return "ATLAS couldn't read the " + label + " just now (" + why + "), so it can't check what needs this. Nothing was removed; try again."
	case len(r.graph.Nodes) == 0:
		return "The " + label + " came back empty, which can't be right while this is installed, so ATLAS can't check what needs it."
	}
	return ""
}

func itemKey(source, pkg string) string { return source + "|" + strings.ToLower(pkg) }

func pkgOf(it core.SoftwareItem) string {
	if it.PackageID != "" {
		return it.PackageID
	}
	return it.Name
}

// check returns a reason the item must not be removed ("" if it may be),
// and a note to show either way.
func (g gate) check(it core.SoftwareItem) (blocked, note string) {
	if isProtected(it) {
		return "ATLAS won't remove this because your system or ATLAS itself relies on it.", ""
	}
	if it.Source == inventory.SourceRPM && g.dnfProtected[strings.ToLower(pkgOf(it))] {
		return "DNF lists this as a protected package your system needs.", ""
	}
	_, _, isRuntime := analysis.DetectRuntime(it)
	// Every source this item came from that keeps dependency records must
	// be verified, and every one must agree nothing else needs it.
	checked := 0
	for _, src := range append([]string{it.Source}, it.AlsoFoundIn...) {
		rec, expected := g.deps[src]
		if !expected {
			continue // this tool keeps no dependency records
		}
		if why := rec.unverified(); why != "" {
			return why, ""
		}
		pkg := analysis.GraphKey(src, it)
		gr := rec.graph
		if !gr.Has(pkg) {
			return "The " + rec.label() +
				" didn't list this package when ATLAS checked just now, so it can't tell what needs it. Scan again; if it's still missing, those records are incomplete.", ""
		}
		if r, ok := gr.ProtectedReason(pkg); ok {
			return "Your system needs this: " + r + ".", ""
		}
		direct, _ := gr.Dependents(pkg)
		var still []string
		for _, d := range direct {
			if !g.removing[itemKey(src, d)] {
				still = append(still, d)
			}
		}
		if len(still) > 0 {
			shown := still
			more := ""
			if len(shown) > 5 {
				shown, more = shown[:5], fmt.Sprintf(" and %d more", len(still)-5)
			}
			return fmt.Sprintf("Needed by %s%s. Removing it would break them.", strings.Join(shown, ", "), more), ""
		}
		checked++
	}
	if checked > 0 {
		return "", "Checked just now: nothing else installed needs it."
	}
	switch {
	case isRuntime:
		return "ATLAS can't tell what software needs this runtime, so it won't remove it. If you're sure nothing does, remove it from Windows Settings or your package manager yourself.", ""
	case it.Kind == core.KindComponent:
		return "This is a system component and ATLAS can't tell what needs it, so it won't remove it.", ""
	}
	return "", "No dependency records exist for where this came from. Apps rarely depend on other apps, but its own uninstaller will warn you if they do."
}

// managersOf lists the Windows package managers that know about an item.
func managersOf(it core.SoftwareItem) []string {
	var out []string
	for _, m := range []string{inventory.SourceChocolatey, inventory.SourceScoop, inventory.SourceWinget} {
		if it.Source == m || contains(it.AlsoFoundIn, m) {
			out = append(out, m)
		}
	}
	return out
}

// removerFor picks the remover for an item, or explains why there is none.
// RPM packages go to the distribution's own tool (rpm), never a guess.
func removerFor(it core.SoftwareItem, rpm platform.RPMRoute) (string, string) {
	switch it.Source {
	case inventory.SourceDpkg:
		return "apt", ""
	case inventory.SourceRPM:
		if rpm.Manager == "" {
			why := rpm.Reason
			if why == "" {
				why = "ATLAS doesn't know which tool manages RPM packages here, so it won't guess."
			}
			return "", why
		}
		return rpm.Manager, ""
	case inventory.SourceSnap:
		return "snap", ""
	case inventory.SourceFlatpak:
		return "flatpak", ""
	}
	ms := managersOf(it)
	switch {
	case len(ms) > 1:
		return "", "It was installed through more than one tool, so ATLAS won't guess which one should remove it. Remove it with the tool you used to install it."
	case len(ms) == 1:
		return ms[0], ""
	case it.Source == inventory.SourceRegistry:
		return inventory.SourceWinget, ""
	}
	return "", "ATLAS doesn't know how to remove software found by " + it.Source + "."
}
