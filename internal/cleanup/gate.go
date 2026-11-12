package cleanup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/deepakraj1207/atlas/internal/analysis"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
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
	graphs       map[string]analysis.Graph
	dnfProtected map[string]bool
	removing     map[string]bool // source|package key of everything in this plan
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
	for _, src := range append([]string{it.Source}, it.AlsoFoundIn...) {
		gr, ok := g.graphs[src]
		if !ok || !gr.Has(pkgOf(it)) {
			continue
		}
		if r, ok := gr.ProtectedReason(pkgOf(it)); ok {
			return "Your system needs this: " + r + ".", ""
		}
		direct, _ := gr.Dependents(pkgOf(it))
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
func removerFor(it core.SoftwareItem) (string, string) {
	switch it.Source {
	case inventory.SourceDpkg:
		return "apt", ""
	case inventory.SourceRPM:
		return "dnf", ""
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
