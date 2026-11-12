package inventory

import (
	"regexp"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
)

// regValues reads values from one Uninstall registry subkey. The Windows
// implementation wraps registry.Key; tests use a map.
type regValues interface {
	String(name string) (string, bool)
	Int(name string) (uint64, bool)
}

var kbUpdate = regexp.MustCompile(`(?i)^(security update|update|hotfix) for .*\(KB\d+\)|^KB\d{6,}`)

// registryItem maps one Uninstall subkey to a SoftwareItem, applying the
// same rules Windows "Installed apps" uses to hide entries: no display name,
// system components, child updates and patches.
func registryItem(v regValues, scope string) (core.SoftwareItem, bool) {
	name, _ := v.String("DisplayName")
	name = strings.TrimSpace(name)
	if name == "" {
		return core.SoftwareItem{}, false
	}
	if sc, ok := v.Int("SystemComponent"); ok && sc == 1 {
		return core.SoftwareItem{}, false
	}
	if parent, _ := v.String("ParentKeyName"); parent != "" {
		return core.SoftwareItem{}, false
	}
	if rt, _ := v.String("ReleaseType"); rt != "" {
		switch strings.ToLower(rt) {
		case "update", "hotfix", "security update", "service pack", "update rollup":
			return core.SoftwareItem{}, false
		}
	}
	if kbUpdate.MatchString(name) {
		return core.SoftwareItem{}, false
	}

	it := core.SoftwareItem{
		Name:   name,
		Source: SourceRegistry,
		Kind:   classifyWindowsName(name),
		Scope:  scope,
	}
	it.Version, _ = v.String("DisplayVersion")
	it.Publisher, _ = v.String("Publisher")
	it.InstallLocation, _ = v.String("InstallLocation")
	it.Version = strings.TrimSpace(it.Version)
	it.Publisher = strings.TrimSpace(it.Publisher)
	it.InstallLocation = strings.Trim(strings.TrimSpace(it.InstallLocation), `"`)
	if d, _ := v.String("InstallDate"); len(d) == 8 && isDigits(d) {
		it.InstallDate = d[:4] + "-" + d[4:6] + "-" + d[6:]
	}
	return it, true
}

func isDigits(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return s != ""
}

// registryScanner reads the three Uninstall locations Windows uses.
type registryScanner struct{}

// NewRegistryScanner returns the Windows installed-programs scanner. On
// other platforms its Scan reports the source as unavailable.
func NewRegistryScanner() core.Scanner { return registryScanner{} }

func (registryScanner) ID() string    { return SourceRegistry }
func (registryScanner) Label() string { return "Installed programs (Windows)" }
func (registryScanner) Detail() string {
	return `Reads HKLM and HKCU \Software\Microsoft\Windows\CurrentVersion\Uninstall (64- and 32-bit views)`
}
