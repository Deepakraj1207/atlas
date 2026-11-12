// Package discover suggests well-known tools from a curated catalogue that
// ships with ATLAS, and installs them through the package managers already
// on the computer. Suggestions are computed locally from the saved software
// list; nothing is sent anywhere.
package discover

import (
	_ "embed"
	"fmt"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/deepakraj1207/atlas/internal/core"
)

//go:embed catalog.yaml
var catalogYAML []byte

// Catalog is the curated list of tools.
type Catalog struct {
	AsOf    string              `yaml:"as_of"`
	Entries []core.CatalogEntry `yaml:"entries"`
}

// Categories in display order.
var Categories = []string{"Internet", "Office", "Media", "Security", "Files", "Utilities", "Development"}

// Managers ATLAS can install with, in order of preference.
var Managers = []string{"winget", "scoop", "chocolatey", "apt", "dnf", "flatpak", "snap"}

var windowsManagers = map[string]bool{"winget": true, "scoop": true, "chocolatey": true}

// ManagerLabel is the plain name of a package manager.
func ManagerLabel(m string) string {
	return map[string]string{
		"winget": "Windows Package Manager", "scoop": "Scoop", "chocolatey": "Chocolatey",
		"apt": "Debian/Ubuntu packages (APT)", "dnf": "Fedora/RHEL packages (DNF)", "flatpak": "Flatpak (Flathub)", "snap": "Snap Store",
	}[m]
}

// Load parses and validates the built-in catalogue.
func Load() (Catalog, error) { return Parse(catalogYAML) }

var (
	reKey       = regexp.MustCompile(`^[a-z0-9]+$`)
	reWingetID  = regexp.MustCompile(`^[A-Za-z0-9+_-]+(\.[A-Za-z0-9+_-]+)+$`)
	reScoopID   = regexp.MustCompile(`^[a-z0-9-]+/[a-z0-9._-]+$`)
	reEntryID   = regexp.MustCompile(`^[a-z0-9-]+$`)
	reSimplePkg = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+_-]*$`)
	reFlatpakID = regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+){2,}$`)
	reHTTPS     = regexp.MustCompile(`^https://`)
)

// Parse reads and validates catalogue YAML. A malformed catalogue is a
// build-time bug, so validation is strict.
func Parse(data []byte) (Catalog, error) {
	var c Catalog
	if err := yaml.Unmarshal(data, &c); err != nil {
		return c, fmt.Errorf("catalogue: %w", err)
	}
	if c.AsOf == "" {
		return c, fmt.Errorf("catalogue: missing as_of date")
	}
	seen := map[string]bool{}
	for i, e := range c.Entries {
		where := fmt.Sprintf("catalogue entry %d (%s)", i, e.ID)
		switch {
		case !reEntryID.MatchString(e.ID) || seen[e.ID]:
			return c, fmt.Errorf("%s: id must be unique lowercase", where)
		case e.Name == "" || e.Summary == "" || e.Why == "":
			return c, fmt.Errorf("%s: name, summary and why are required", where)
		case !slices.Contains(Categories, e.Category):
			return c, fmt.Errorf("%s: unknown category %q", where, e.Category)
		case !reHTTPS.MatchString(e.Homepage):
			return c, fmt.Errorf("%s: homepage must be https", where)
		case len(e.Packages) == 0 || len(e.Provides) == 0:
			return c, fmt.Errorf("%s: needs packages and provides", where)
		}
		seen[e.ID] = true
		for _, k := range append(append([]string{}, e.Provides...), e.Related...) {
			if !reKey.MatchString(k) {
				return c, fmt.Errorf("%s: product key %q must be lowercase letters and digits", where, k)
			}
		}
		mseen := map[string]bool{}
		for _, p := range e.Packages {
			ok := false
			switch p.Manager {
			case "winget":
				ok = reWingetID.MatchString(p.ID)
			case "scoop":
				ok = reScoopID.MatchString(p.ID)
			case "flatpak":
				ok = reFlatpakID.MatchString(p.ID)
			case "chocolatey", "apt", "dnf", "snap":
				ok = reSimplePkg.MatchString(p.ID)
			}
			if !ok || mseen[p.Manager] {
				return c, fmt.Errorf("%s: bad or duplicate package %s %q", where, p.Manager, p.ID)
			}
			if p.Classic && p.Manager != "snap" {
				return c, fmt.Errorf("%s: classic only applies to snaps", where)
			}
			mseen[p.Manager] = true
		}
	}
	return c, nil
}

var (
	reParens = regexp.MustCompile(`\([^)]*\)`)
	// Dotted versions anywhere, and standalone numbers; "7" in "7-Zip" stays.
	reVersions = regexp.MustCompile(`\b[vV]?\d+(?:[._]\d+)+\b|(?:^|\s)[vV]?\d+(?:\s|$)`)
	reArchWord = regexp.MustCompile(`(?i)\b(x64|x86|arm64|amd64|64-bit|32-bit)\b`)
	reNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)
)

// ProductKey reduces a name to the key used in "provides" and "related":
// lowercase letters and digits, "+" spelled "plus", versions and
// architecture removed. "Notepad++ 8.6 (64-bit)" becomes "notepadplusplus".
func ProductKey(name string) string {
	s := strings.ReplaceAll(strings.ToLower(name), "+", "plus")
	s = reParens.ReplaceAllString(s, " ")
	s = reArchWord.ReplaceAllString(s, " ")
	s = reVersions.ReplaceAllString(s, " ")
	return reNonAlnum.ReplaceAllString(s, "")
}

func exactKey(s string) string {
	return reNonAlnum.ReplaceAllString(strings.ReplaceAll(strings.ToLower(s), "+", "plus"), "")
}
