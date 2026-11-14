//go:build !windows

package updates

import (
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/platform"
)

// DefaultManagers returns the Linux package managers. RPM packages are
// checked and updated with the distribution's own tool (DNF or Zypper),
// detected from /etc/os-release.
func DefaultManagers(elevated bool) []core.PackageManager {
	return Managers(elevated, platform.LinuxRPMRoute())
}

// Managers is DefaultManagers for a given RPM route (tests).
func Managers(elevated bool, rpm platform.RPMRoute) []core.PackageManager {
	ms := []core.PackageManager{NewApt(inventory.ExecRunner, elevated, AptListsDir)}
	switch {
	case rpm.Manager == "dnf":
		ms = append(ms, NewDnf(inventory.ExecRunner, elevated))
	case rpm.Manager == "zypper":
		ms = append(ms, NewZypper(inventory.ExecRunner, elevated))
	case rpm.RPMBased:
		ms = append(ms, NewRPMRefused(rpm.Reason))
	}
	return append(ms, NewSnap(inventory.ExecRunner, elevated), NewFlatpak(inventory.ExecRunner))
}

// DefaultTimeouts gives DNF and Flatpak time to refresh their metadata.
func DefaultTimeouts() map[string]time.Duration {
	return map[string]time.Duration{ManagerDnf: 120 * time.Second, ManagerZypper: 120 * time.Second, ManagerFlatpak: 90 * time.Second}
}
