package core

import "context"

// SoftwareItem is the minimal description of one installed application or
// tool. Phase 2 extends it; Phase 1 defines it only so the ports below have a
// concrete shape.
type SoftwareItem struct {
	Name    string
	Version string
	// Source names where the item was found, e.g. "winget", "registry", "apt".
	Source string
}

// Scanner discovers installed software without modifying the system.
// Implementations live in internal/inventory (Phase 2).
type Scanner interface {
	// Name is a short identifier such as "windows-registry" or "dpkg".
	Name() string
	// Scan returns everything the scanner can see. It must be read-only.
	Scan(ctx context.Context) ([]SoftwareItem, error)
}

// PackageManager is the port for a package manager adapter (winget, scoop,
// chocolatey, apt, snap, flatpak, ...). Phase 1 defines the port only; no
// adapter exists yet and nothing calls it.
type PackageManager interface {
	// ID is a stable identifier such as "winget".
	ID() string
	// DisplayName is shown to users, e.g. "Windows Package Manager".
	DisplayName() string
	// Available reports whether the manager is installed and usable here.
	Available(ctx context.Context) (bool, error)
	// PlanUpdateAll returns a plan describing what "update everything" would
	// do, without doing it.
	PlanUpdateAll(ctx context.Context) (ActionPlan, error)
}
