//go:build windows

package updates

import (
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// DefaultManagers returns the Windows package managers.
func DefaultManagers(elevated bool) []core.PackageManager {
	return []core.PackageManager{
		NewWinget(inventory.ExecRunner, elevated),
		NewChocolatey(inventory.ExecRunner, elevated),
		NewScoop(inventory.ScoopRoots(), elevated),
	}
}

// DefaultTimeouts gives winget and Chocolatey time to refresh their catalogues.
func DefaultTimeouts() map[string]time.Duration {
	return map[string]time.Duration{ManagerWinget: 120 * time.Second, ManagerChocolatey: 120 * time.Second}
}
