//go:build !windows

package updates

import (
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// DefaultManagers returns the Linux package managers.
func DefaultManagers(elevated bool) []core.PackageManager {
	return []core.PackageManager{
		NewApt(inventory.ExecRunner, elevated, AptListsDir),
		NewDnf(inventory.ExecRunner, elevated),
		NewSnap(inventory.ExecRunner, elevated),
		NewFlatpak(inventory.ExecRunner),
	}
}

// DefaultTimeouts gives DNF and Flatpak time to refresh their metadata.
func DefaultTimeouts() map[string]time.Duration {
	return map[string]time.Duration{ManagerDnf: 120 * time.Second, ManagerFlatpak: 90 * time.Second}
}
