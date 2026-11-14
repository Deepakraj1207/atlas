package cleanup

import (
	"fmt"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// Remover uninstalls software through one package manager.
type Remover interface {
	ID() string
	Label() string
	PlanRemove(items []core.SoftwareItem) []core.UpdateStep
}

// target turns an item into a step target. Origin carries the inventory
// source so the saved list can be updated afterwards.
func target(manager string, it core.SoftwareItem) core.Update {
	return core.Update{Manager: manager, PackageID: it.PackageID, Name: it.Name, CurrentVersion: it.Version, Origin: it.Source, Scope: it.Scope}
}

func targets(manager string, items []core.SoftwareItem) []core.Update {
	out := make([]core.Update, len(items))
	for i, it := range items {
		out[i] = target(manager, it)
	}
	return out
}

func pkgIDs(items []core.SoftwareItem) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.PackageID
		if out[i] == "" {
			out[i] = it.Name
		}
	}
	return out
}

func privileged(elevated bool, c core.Command) (core.Command, core.Elevation) {
	if elevated {
		return c, core.ElevationAdmin
	}
	return core.Command{Program: "sudo", Args: append([]string{c.Program}, c.Args...), OKCodes: c.OKCodes, RestartCodes: c.RestartCodes}, core.ElevationSudo
}

// ---- winget (also used for programs only listed by Windows) ----

type wingetRemover struct{ elevated bool }

// NewWingetRemover removes by exact winget ID, or by exact display name for
// programs only Windows lists. It runs each program's own uninstaller.
func NewWingetRemover(elevated bool) Remover { return wingetRemover{elevated: elevated} }

func (wingetRemover) ID() string    { return inventory.SourceWinget }
func (wingetRemover) Label() string { return "Windows Package Manager" }

func (w wingetRemover) PlanRemove(items []core.SoftwareItem) []core.UpdateStep {
	elev := core.ElevationInstaller
	if w.elevated {
		elev = core.ElevationAdmin
	}
	var steps []core.UpdateStep
	for _, it := range items {
		hasID := it.PackageID != "" && (it.Source == inventory.SourceWinget || contains(it.AlsoFoundIn, inventory.SourceWinget))
		args := []string{"uninstall", "--name", it.Name, "--exact"}
		rollback := "To get it back, reinstall it from its publisher, or with winget install if it's available there."
		if hasID {
			args = []string{"uninstall", "--id", it.PackageID, "--exact"}
			rollback = "To get it back: winget install --id " + it.PackageID
		}
		steps = append(steps, core.UpdateStep{
			Manager: w.ID(), Label: w.Label(), Action: core.ActionRemove, Elevation: elev,
			Updates:  []core.Update{target(w.ID(), it)},
			Commands: []core.Command{{Program: "winget", Args: args}},
			Rollback: rollback,
		})
	}
	return steps
}

// ---- Chocolatey ----

type chocoRemover struct{ elevated bool }

// NewChocolateyRemover removes with "choco uninstall", which asks before
// running each package's uninstall script. It needs an administrator terminal.
func NewChocolateyRemover(elevated bool) Remover { return chocoRemover{elevated: elevated} }

func (chocoRemover) ID() string    { return inventory.SourceChocolatey }
func (chocoRemover) Label() string { return "Chocolatey" }

func (c chocoRemover) PlanRemove(items []core.SoftwareItem) []core.UpdateStep {
	st := core.UpdateStep{
		Manager: c.ID(), Label: c.Label(), Action: core.ActionRemove, Elevation: core.ElevationAdmin,
		Updates:  targets(c.ID(), items),
		Commands: []core.Command{{Program: "choco", Args: append([]string{"uninstall"}, pkgIDs(items)...), RestartCodes: []int{1641, 3010}}},
		Rollback: "To get it back: choco install " + strings.Join(pkgIDs(items), " "),
	}
	if !c.elevated {
		st.Blocked = "Chocolatey only removes software from an administrator terminal. Start ATLAS with “Run as administrator” to remove it, or run the command yourself in one."
	}
	return []core.UpdateStep{st}
}

// ---- Scoop ----

type scoopRemover struct{ elevated bool }

// NewScoopRemover removes with "scoop uninstall" (without --purge, so the
// app's saved settings in scoop\persist are kept).
func NewScoopRemover(elevated bool) Remover { return scoopRemover{elevated: elevated} }

func (scoopRemover) ID() string    { return inventory.SourceScoop }
func (scoopRemover) Label() string { return "Scoop" }

func (s scoopRemover) PlanRemove(items []core.SoftwareItem) []core.UpdateStep {
	var user, global []core.SoftwareItem
	for _, it := range items {
		if it.Scope == "machine" {
			global = append(global, it)
		} else {
			user = append(user, it)
		}
	}
	names := func(items []core.SoftwareItem) []string {
		out := make([]string, len(items))
		for i, it := range items {
			out[i] = it.Name
		}
		return out
	}
	var steps []core.UpdateStep
	if len(user) > 0 {
		steps = append(steps, core.UpdateStep{
			Manager: s.ID(), Label: s.Label(), Action: core.ActionRemove, Elevation: core.ElevationNone,
			Updates:  targets(s.ID(), user),
			Commands: []core.Command{{Program: "scoop", Args: append([]string{"uninstall"}, names(user)...)}},
			Rollback: "To get it back: scoop install " + strings.Join(pkgIDs(user), " ") + ". Your settings in scoop\\persist are kept.",
		})
	}
	if len(global) > 0 {
		st := core.UpdateStep{
			Manager: s.ID(), Label: s.Label() + " (all users)", Action: core.ActionRemove, Elevation: core.ElevationAdmin,
			Updates:  targets(s.ID(), global),
			Commands: []core.Command{{Program: "scoop", Args: append([]string{"uninstall", "--global"}, names(global)...)}},
			Rollback: "To get it back: scoop install --global " + strings.Join(pkgIDs(global), " "),
		}
		if !s.elevated {
			st.Blocked = "Scoop apps installed for all users can only be removed from an administrator terminal."
		}
		steps = append(steps, st)
	}
	return steps
}

// ---- Linux ----

type linuxRemover struct {
	id, label, program string
	args               []string
	privileged         bool
	elevated           bool
	elevation          core.Elevation
	rollback           func(ids []string) string
}

func (l linuxRemover) ID() string    { return l.id }
func (l linuxRemover) Label() string { return l.label }

func (l linuxRemover) PlanRemove(items []core.SoftwareItem) []core.UpdateStep {
	cmd := core.Command{Program: l.program, Args: append(append([]string{}, l.args...), pkgIDs(items)...)}
	elev := l.elevation
	if l.privileged {
		cmd, elev = privileged(l.elevated, cmd)
	}
	return []core.UpdateStep{{
		Manager: l.id, Label: l.label, Action: core.ActionRemove, Elevation: elev,
		Updates: targets(l.id, items), Commands: []core.Command{cmd}, Rollback: l.rollback(pkgIDs(items)),
	}}
}

// NewAptRemover removes with "apt-get remove" (not purge, so settings files
// are kept). APT lists what it will remove and asks before doing it.
func NewAptRemover(elevated bool) Remover {
	return linuxRemover{id: "apt", label: "Debian/Ubuntu packages (APT)", program: "apt-get", args: []string{"remove"}, privileged: true, elevated: elevated,
		rollback: func(ids []string) string {
			return "To get it back: sudo apt-get install " + strings.Join(ids, " ") + ". Settings files are kept because ATLAS doesn't purge."
		}}
}

// NewDnfRemover removes with "dnf remove", which lists and asks first.
func NewDnfRemover(elevated bool) Remover {
	return linuxRemover{id: "dnf", label: "Fedora/RHEL packages (DNF)", program: "dnf", args: []string{"remove"}, privileged: true, elevated: elevated,
		rollback: func(ids []string) string { return "To get it back: sudo dnf install " + strings.Join(ids, " ") }}
}

// NewZypperRemover removes with "zypper remove" (openSUSE, SLES), which
// lists everything it will remove and asks first. No --clean-deps: it only
// removes what was chosen and what zypper says must go with it.
func NewZypperRemover(elevated bool) Remover {
	return linuxRemover{id: "zypper", label: "openSUSE/SUSE packages (Zypper)", program: "zypper", args: []string{"remove"}, privileged: true, elevated: elevated,
		rollback: func(ids []string) string { return "To get it back: sudo zypper install " + strings.Join(ids, " ") }}
}

// NewSnapRemover removes with "snap remove", which keeps a snapshot of the
// app's data for 31 days.
func NewSnapRemover(elevated bool) Remover {
	return linuxRemover{id: "snap", label: "Snap", program: "snap", args: []string{"remove"}, privileged: true, elevated: elevated,
		rollback: func(ids []string) string {
			return fmt.Sprintf("To get it back: sudo snap install %s. Snap keeps a snapshot of its data for 31 days; restore it with sudo snap restore (see sudo snap saved).", strings.Join(ids, " "))
		}}
}

// NewFlatpakRemover removes with "flatpak uninstall", which asks first and
// keeps app data in ~/.var/app.
func NewFlatpakRemover() Remover {
	return linuxRemover{id: "flatpak", label: "Flatpak", program: "flatpak", args: []string{"uninstall"}, elevation: core.ElevationPolkit,
		rollback: func(ids []string) string {
			return "To get it back: flatpak install " + strings.Join(ids, " ") + ". App data in ~/.var/app is kept."
		}}
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
