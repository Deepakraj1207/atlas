package updates

import (
	"context"
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// ---- apt ----

type apt struct {
	run      inventory.Runner
	elevated bool
	listsDir string
}

// AptListsDir is where APT keeps downloaded package information.
const AptListsDir = "/var/lib/apt/lists"

// NewApt checks with "apt list --upgradable", which uses the package lists
// the system last downloaded, and updates with
// "apt-get install --only-upgrade", which asks for confirmation itself.
// listsDir is checked first so "no updates" is never reported when APT has
// nothing to compare against.
func NewApt(run inventory.Runner, elevated bool, listsDir string) core.PackageManager {
	return apt{run: run, elevated: elevated, listsDir: listsDir}
}

// HasAptLists reports whether any package index has been downloaded.
func HasAptLists(dir string) bool { return hasAptLists(dir) }

func hasAptLists(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "_Packages") {
			return true
		}
	}
	return false
}

func (apt) ID() string     { return ManagerApt }
func (apt) Label() string  { return "Debian/Ubuntu packages (APT)" }
func (apt) Detail() string { return "Runs: apt list --upgradable and apt-mark showhold" }

func (a apt) CheckUpdates(ctx context.Context) ([]core.Update, error) {
	out, err := a.run(ctx, "apt", "list", "--upgradable")
	if err != nil {
		return nil, err
	}
	if a.listsDir != "" && !hasAptLists(a.listsDir) {
		return nil, &core.SourceError{
			Message: "No package information downloaded yet, so updates can't be checked",
			Details: "run 'sudo apt update' (your system's normal update step), then check again",
		}
	}
	held := map[string]bool{}
	if h, err := a.run(ctx, "apt-mark", "showhold"); err == nil {
		for _, n := range strings.Fields(string(h)) {
			held[n] = true
		}
	}
	return parseAptUpgradable(string(out), held), nil
}

var aptLine = regexp.MustCompile(`^([^/\s]+)/(\S+)\s+(\S+)\s+(\S+)(?:\s+\[upgradable from: ([^\]]+)\])?`)

func parseAptUpgradable(out string, held map[string]bool) []core.Update {
	var ups []core.Update
	for _, l := range strings.Split(out, "\n") {
		m := aptLine.FindStringSubmatch(strings.TrimSpace(l))
		if m == nil {
			continue
		}
		u := core.Update{
			Manager:          ManagerApt,
			PackageID:        m[1],
			Name:             m[1],
			AvailableVersion: m[3],
			CurrentVersion:   m[5],
			Origin:           strings.Split(m[2], ",")[0],
			Scope:            "machine",
		}
		if held[m[1]] {
			u.Held, u.Note = true, "Held back with apt-mark hold"
		}
		ups = append(ups, u)
	}
	return ups
}

func (a apt) PlanUpdates(updates []core.Update) []core.UpdateStep {
	if len(updates) == 0 {
		return nil
	}
	cmd, elev := privileged(a.elevated, core.Command{
		Program: "apt-get",
		Args:    append([]string{"install", "--only-upgrade"}, ids(updates)...),
	})
	return []core.UpdateStep{{
		Manager: ManagerApt, Label: a.Label(), Updates: updates, Commands: []core.Command{cmd}, Elevation: elev,
		Rollback: "APT doesn't keep old versions. Going back means installing a specific version (sudo apt-get install <package>=<old version>) while the repository still offers it.",
	}}
}

// ---- dnf ----

type dnf struct {
	run      inventory.Runner
	elevated bool
}

// NewDnf checks with "dnf check-update" and updates with "dnf upgrade".
func NewDnf(run inventory.Runner, elevated bool) core.PackageManager {
	return dnf{run: run, elevated: elevated}
}

func (dnf) ID() string     { return ManagerDnf }
func (dnf) Label() string  { return "Fedora/RHEL packages (DNF)" }
func (dnf) Detail() string { return "Runs: dnf check-update -q" }

func (d dnf) CheckUpdates(ctx context.Context) ([]core.Update, error) {
	out, err := d.run(ctx, "dnf", "check-update", "-q")
	if err != nil {
		var ce *inventory.CommandError
		// Exit code 100 means "updates are available".
		if !errors.As(err, &ce) || ce.ExitCode != 100 {
			return nil, err
		}
	}
	return parseDnfCheckUpdate(string(out)), nil
}

var rpmArches = map[string]bool{"x86_64": true, "noarch": true, "i686": true, "aarch64": true, "ppc64le": true, "s390x": true, "armv7hl": true}

func parseDnfCheckUpdate(out string) []core.Update {
	var ups []core.Update
	for _, l := range strings.Split(out, "\n") {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "Obsoleting") || strings.HasPrefix(t, "Security:") {
			break
		}
		f := strings.Fields(t)
		if len(f) != 3 {
			continue
		}
		name := f[0]
		if i := strings.LastIndex(name, "."); i > 0 && rpmArches[name[i+1:]] {
			name = name[:i]
		}
		ups = append(ups, core.Update{Manager: ManagerDnf, PackageID: name, Name: name, AvailableVersion: f[1], Origin: f[2], Scope: "machine"})
	}
	return ups
}

func (d dnf) PlanUpdates(updates []core.Update) []core.UpdateStep {
	if len(updates) == 0 {
		return nil
	}
	cmd, elev := privileged(d.elevated, core.Command{Program: "dnf", Args: append([]string{"upgrade"}, ids(updates)...)})
	return []core.UpdateStep{{
		Manager: ManagerDnf, Label: d.Label(), Updates: updates, Commands: []core.Command{cmd}, Elevation: elev,
		Rollback: "dnf downgrade <package> can restore the previous version while the repository still has it.",
	}}
}

// ---- zypper (openSUSE, SLES) ----

type zypper struct {
	run      inventory.Runner
	elevated bool
}

// NewZypper checks with "zypper list-updates" (cached repository data, no
// refresh, changes nothing) and updates with "zypper update", which lists
// what will change and asks first.
func NewZypper(run inventory.Runner, elevated bool) core.PackageManager {
	return zypper{run: run, elevated: elevated}
}

func (zypper) ID() string    { return ManagerZypper }
func (zypper) Label() string { return "openSUSE/SUSE packages (Zypper)" }
func (zypper) Detail() string {
	return "Runs: zypper --non-interactive --no-refresh list-updates"
}

func (z zypper) CheckUpdates(ctx context.Context) ([]core.Update, error) {
	out, err := z.run(ctx, "zypper", "--non-interactive", "--no-refresh", "list-updates")
	if err != nil {
		return nil, err
	}
	return parseZypperListUpdates(string(out))
}

var errZypperNoTable = errors.New("zypper's output had no update table")

// parseZypperListUpdates reads zypper's table by its header names, so a
// column added or moved in another zypper version doesn't shift values.
func parseZypperListUpdates(out string) ([]core.Update, error) {
	cols := map[string]int{}
	var ups []core.Update
	inTable := false
	for _, l := range strings.Split(out, "\n") {
		// The rule under the header is "--+----+---": no "|" in it.
		if t := strings.TrimSpace(l); len(cols) > 0 && t != "" && strings.Trim(t, "-+") == "" {
			inTable = true
			continue
		}
		if !strings.Contains(l, "|") {
			if strings.Contains(l, "No updates found") {
				return nil, nil
			}
			continue
		}
		cells := strings.Split(l, "|")
		for i := range cells {
			cells[i] = strings.TrimSpace(cells[i])
		}
		if len(cols) == 0 {
			for i, c := range cells {
				cols[c] = i
			}
			if _, ok := cols["Name"]; !ok {
				cols = map[string]int{}
				continue
			}
			if _, ok := cols["Available Version"]; !ok {
				return nil, errZypperNoTable
			}
			continue
		}
		if !inTable {
			continue
		}
		get := func(name string) string {
			if i, ok := cols[name]; ok && i < len(cells) {
				return cells[i]
			}
			return ""
		}
		name := get("Name")
		if name == "" {
			continue
		}
		ups = append(ups, core.Update{Manager: ManagerZypper, PackageID: name, Name: name, CurrentVersion: get("Current Version"),
			AvailableVersion: get("Available Version"), Origin: get("Repository"), Scope: "machine"})
	}
	if len(cols) == 0 && strings.TrimSpace(out) != "" && !strings.Contains(out, "No updates found") {
		return nil, errZypperNoTable
	}
	return ups, nil
}

func (z zypper) PlanUpdates(updates []core.Update) []core.UpdateStep {
	if len(updates) == 0 {
		return nil
	}
	cmd, elev := privileged(z.elevated, core.Command{Program: "zypper", Args: append([]string{"update"}, ids(updates)...)})
	return []core.UpdateStep{{
		Manager: ManagerZypper, Label: z.Label(), Updates: updates, Commands: []core.Command{cmd}, Elevation: elev,
		Rollback: "zypper install --oldpackage <package>-<version> can restore the previous version while the repository still has it.",
	}}
}

// ---- RPM distributions ATLAS won't change ----

type rpmRefused struct{ reason string }

// NewRPMRefused stands in for the RPM tool on a distribution where ATLAS
// won't change RPM packages (yum only, native tool missing), so the update
// check says why instead of silently skipping them.
func NewRPMRefused(reason string) core.PackageManager { return rpmRefused{reason: reason} }

func (rpmRefused) ID() string     { return "rpm-refused" }
func (rpmRefused) Label() string  { return "RPM packages" }
func (rpmRefused) Detail() string { return "Not checked: no supported tool on this distribution" }
func (r rpmRefused) CheckUpdates(context.Context) ([]core.Update, error) {
	return nil, &core.SourceError{Message: "Not checked", Details: r.reason}
}
func (rpmRefused) PlanUpdates([]core.Update) []core.UpdateStep { return nil }

// ---- snap ----

type snap struct {
	run      inventory.Runner
	elevated bool
}

// NewSnap checks with "snap refresh --list" and updates with "snap refresh".
func NewSnap(run inventory.Runner, elevated bool) core.PackageManager {
	return snap{run: run, elevated: elevated}
}

func (snap) ID() string     { return ManagerSnap }
func (snap) Label() string  { return "Snap" }
func (snap) Detail() string { return "Runs: snap refresh --list" }

func (s snap) CheckUpdates(ctx context.Context) ([]core.Update, error) {
	out, err := s.run(ctx, "snap", "refresh", "--list")
	if err != nil {
		return nil, err
	}
	return parseSnapRefreshList(string(out)), nil
}

func parseSnapRefreshList(out string) []core.Update {
	var ups []core.Update
	header := true
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) == 0 {
			continue
		}
		if header {
			header = f[0] != "Name"
			continue
		}
		if len(f) < 2 {
			continue
		}
		ups = append(ups, core.Update{Manager: ManagerSnap, PackageID: f[0], Name: f[0], AvailableVersion: f[1], Scope: "machine"})
	}
	return ups
}

func (s snap) PlanUpdates(updates []core.Update) []core.UpdateStep {
	if len(updates) == 0 {
		return nil
	}
	cmd, elev := privileged(s.elevated, core.Command{Program: "snap", Args: append([]string{"refresh"}, ids(updates)...)})
	return []core.UpdateStep{{
		Manager: ManagerSnap, Label: s.Label(), Updates: updates, Commands: []core.Command{cmd}, Elevation: elev,
		Rollback: "Snap keeps the previous revision. To go back: sudo snap revert <name>",
	}}
}

// ---- flatpak ----

type flatpak struct{ run inventory.Runner }

// NewFlatpak checks with "flatpak remote-ls --updates" and updates with
// "flatpak update", which asks for confirmation and, for system-wide
// installs, may ask for a password through PolicyKit.
func NewFlatpak(run inventory.Runner) core.PackageManager { return flatpak{run: run} }

func (flatpak) ID() string    { return ManagerFlatpak }
func (flatpak) Label() string { return "Flatpak" }
func (flatpak) Detail() string {
	return "Runs: flatpak remote-ls --updates --columns=application,version,origin"
}

func (f flatpak) CheckUpdates(ctx context.Context) ([]core.Update, error) {
	out, err := f.run(ctx, "flatpak", "remote-ls", "--updates", "--columns=application,version,origin")
	if err != nil {
		return nil, err
	}
	return parseFlatpakUpdates(string(out)), nil
}

var spaces2 = regexp.MustCompile(`\s{2,}`)

func parseFlatpakUpdates(out string) []core.Update {
	var ups []core.Update
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) == "" {
			continue
		}
		cols := strings.Split(l, "\t")
		if len(cols) == 1 {
			cols = spaces2.Split(strings.TrimSpace(l), -1)
		}
		id := strings.TrimSpace(cols[0])
		if id == "" || strings.EqualFold(id, "Application ID") {
			continue
		}
		u := core.Update{Manager: ManagerFlatpak, PackageID: id, Name: id}
		if len(cols) > 1 {
			u.AvailableVersion = strings.TrimSpace(cols[1])
		}
		if len(cols) > 2 {
			u.Origin = strings.TrimSpace(cols[2])
		}
		if u.AvailableVersion == "" {
			u.AvailableVersion = "newer build"
		}
		ups = append(ups, u)
	}
	return ups
}

func (f flatpak) PlanUpdates(updates []core.Update) []core.UpdateStep {
	if len(updates) == 0 {
		return nil
	}
	return []core.UpdateStep{{
		Manager: ManagerFlatpak, Label: f.Label(), Updates: updates, Elevation: core.ElevationPolkit,
		Commands: []core.Command{{Program: "flatpak", Args: append([]string{"update"}, ids(updates)...)}},
		Rollback: "Flatpak can go back to an earlier build: flatpak update --commit=<hash> <app> (find hashes with flatpak remote-info --log)",
	}}
}
