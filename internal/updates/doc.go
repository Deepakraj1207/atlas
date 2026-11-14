// Package updates finds and applies software updates through each package
// manager's own command-line tool.
//
// Checking is read-only. Applying happens only through a TerminalJob that
// the TUI runs after the user confirms a preview listing every command. The
// job hands the terminal to the package manager, so its own output and
// prompts (sudo passwords, Windows permission dialogs, confirmations,
// licence agreements) are shown directly. ATLAS never answers a prompt,
// accepts an agreement or supplies a password on the user's behalf.
package updates

import (
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// Manager IDs match the inventory source IDs where both exist, so updates
// can be matched to the saved software list.
const (
	ManagerWinget     = inventory.SourceWinget
	ManagerChocolatey = inventory.SourceChocolatey
	ManagerScoop      = inventory.SourceScoop
	ManagerApt        = "apt"
	ManagerDnf        = "dnf"
	ManagerZypper     = "zypper"
	ManagerSnap       = inventory.SourceSnap
	ManagerFlatpak    = inventory.SourceFlatpak
)

// inventorySource maps a manager to the inventory source that lists the
// same packages, for matching names, versions and kinds.
func inventorySource(manager string) string {
	switch manager {
	case ManagerApt:
		return inventory.SourceDpkg
	case ManagerDnf, ManagerZypper:
		return inventory.SourceRPM
	}
	return manager
}

// privileged wraps a command in sudo unless ATLAS already runs as root.
func privileged(elevated bool, c core.Command) (core.Command, core.Elevation) {
	if elevated {
		return c, core.ElevationAdmin
	}
	return core.Command{
		Program:      "sudo",
		Args:         append([]string{c.Program}, c.Args...),
		OKCodes:      c.OKCodes,
		RestartCodes: c.RestartCodes,
	}, core.ElevationSudo
}

func ids(updates []core.Update) []string {
	out := make([]string, 0, len(updates))
	for _, u := range updates {
		out = append(out, u.PackageID)
	}
	return out
}
