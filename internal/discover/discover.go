package discover

import (
	"slices"
	"strings"

	"github.com/deepakraj1207/atlas/internal/analysis"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// sourceToManager maps inventory sources to the manager that installs
// from them.
var sourceToManager = map[string]string{
	inventory.SourceWinget: "winget", inventory.SourceScoop: "scoop", inventory.SourceChocolatey: "chocolatey",
	inventory.SourceDpkg: "apt", inventory.SourceRPM: "dnf", inventory.SourceSnap: "snap", inventory.SourceFlatpak: "flatpak",
}

// Here describes this computer for Assess.
type Here struct {
	Windows bool
	// Scanned is false when there is no saved software list.
	Scanned bool
	// Managers are package managers present (from the latest scan).
	Managers map[string]bool
	Items    []core.SoftwareItem
	// Blocked explains managers that exist but can't install from here,
	// e.g. Chocolatey without administrator rights.
	Blocked map[string]string
}

// keysOf returns the product keys an installed item answers to.
func keysOf(it core.SoftwareItem) []string {
	keys := []string{ProductKey(it.Name), exactKey(it.Name)}
	if fam, _, ok := analysis.DetectRuntime(it); ok {
		keys = append(keys, exactKey(fam))
	}
	return keys
}

// Assess rates every catalogue entry for this computer: installed or not,
// how it would be installed, and why it is suggested. Pure function.
func Assess(c Catalog, here Here) core.Discovery {
	d := core.Discovery{Scanned: here.Scanned, CatalogAsOf: c.AsOf}
	for _, m := range Managers {
		if here.Managers[m] {
			d.ManagersHere = append(d.ManagersHere, m)
		}
	}
	installed := map[string]core.SoftwareItem{} // product key -> item
	byPackage := map[string]core.SoftwareItem{} // manager|id -> item
	for _, it := range here.Items {
		for _, k := range keysOf(it) {
			if k != "" {
				if _, ok := installed[k]; !ok {
					installed[k] = it
				}
			}
		}
		if it.PackageID == "" {
			continue
		}
		for _, src := range append([]string{it.Source}, it.AlsoFoundIn...) {
			if m, ok := sourceToManager[src]; ok {
				byPackage[m+"|"+strings.ToLower(it.PackageID)] = it
			}
		}
	}

	cats := map[string]bool{}
	for _, e := range c.Entries {
		forHere := false
		for _, p := range e.Packages {
			if windowsManagers[p.Manager] == here.Windows {
				forHere = true
			}
		}
		if !forHere {
			continue // e.g. a Windows-only tool on Linux
		}
		item := core.DiscoverItem{Entry: e}
		for _, k := range e.Provides {
			if it, ok := installed[k]; ok {
				item.Installed, item.InstalledAs = true, it.Name
				break
			}
		}
		if !item.Installed {
			for _, p := range e.Packages {
				if it, ok := byPackage[p.Manager+"|"+strings.ToLower(p.ID)]; ok {
					item.Installed, item.InstalledAs = true, it.Name
					break
				}
			}
		}

		var offered []string
		for _, m := range Managers {
			idx := slices.IndexFunc(e.Packages, func(p core.CatalogPackage) bool { return p.Manager == m })
			if idx < 0 || windowsManagers[m] != here.Windows {
				continue
			}
			offered = append(offered, ManagerLabel(m))
			if item.Via == "" && here.Managers[m] && here.Blocked[m] == "" {
				item.Via, item.ViaLabel, item.Package = m, ManagerLabel(m), e.Packages[idx]
			}
		}
		switch {
		case item.Via != "":
		case !here.Scanned:
			item.Unavailable = "Scan your system first so ATLAS knows which package managers you have."
		default:
			item.Unavailable = "Not available through the package managers on this computer. It's offered through: " + strings.Join(offered, ", ") + "."
			for _, p := range e.Packages {
				if why := here.Blocked[p.Manager]; why != "" && here.Managers[p.Manager] {
					item.Unavailable = why
				}
			}
		}

		if !item.Installed && item.Via != "" {
			for _, k := range e.Related {
				if it, ok := installed[k]; ok {
					item.Because = append(item.Because, "because you have "+it.Name)
					break
				}
			}
			if e.Everyone {
				item.Because = append(item.Because, "useful for almost everyone")
			}
		}
		d.Items = append(d.Items, item)
		cats[e.Category] = true
	}
	for _, c := range Categories {
		if cats[c] {
			d.Categories = append(d.Categories, c)
		}
	}
	return d
}
