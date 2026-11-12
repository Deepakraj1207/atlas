package cleanup

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// Tidier finds one kind of clean-up and describes it exactly. Find is
// read-only; it returns no steps when there is nothing to do.
type Tidier interface {
	ID() string
	Label() string
	Detail() string
	Find(ctx context.Context) ([]core.UpdateStep, error)
}

// sizeOf returns the total size of regular files under path.
func sizeOf(path string) int64 {
	var n int64
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.Type().IsRegular() {
			if info, err := d.Info(); err == nil {
				n += info.Size()
			}
		}
		return nil
	})
	return n
}

// filesWithSuffix lists regular files ending in suffix directly in dir.
func filesWithSuffix(dir, suffix string) ([]string, int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0, err
	}
	var files []string
	var total int64
	for _, e := range entries {
		if e.Type().IsRegular() && strings.HasSuffix(e.Name(), suffix) {
			if info, err := e.Info(); err == nil {
				files = append(files, e.Name())
				total += info.Size()
			}
		}
	}
	return files, total, nil
}

// ---- APT downloaded packages ----

type aptCache struct {
	dir      string
	elevated bool
}

// NewAptCache offers "apt-get clean" when downloaded .deb files exist.
func NewAptCache(dir string, elevated bool) Tidier { return aptCache{dir: dir, elevated: elevated} }

func (aptCache) ID() string       { return "apt-cache" }
func (aptCache) Label() string    { return "Downloaded package files (APT)" }
func (a aptCache) Detail() string { return "Lists *.deb in " + a.dir }

func (a aptCache) Find(ctx context.Context) ([]core.UpdateStep, error) {
	files, total, err := filesWithSuffix(a.dir, ".deb")
	if errors.Is(err, fs.ErrNotExist) {
		return nil, core.ErrSourceUnavailable
	}
	if err != nil || len(files) == 0 {
		return nil, err
	}
	cmd, elev := privileged(a.elevated, core.Command{Program: "apt-get", Args: []string{"clean"}})
	return []core.UpdateStep{{
		Manager: "apt", Label: a.Label(), Action: core.ActionClean, Bytes: total, Elevation: elev,
		Updates:  []core.Update{{Manager: "apt", Name: inventory.Plural(len(files), "downloaded package file")}},
		Commands: []core.Command{cmd},
		Rollback: "Nothing you use is lost: APT downloads package files again when it needs them",
	}}, nil
}

// ---- APT autoremove ----

type aptAutoremove struct {
	run      inventory.Runner
	elevated bool
}

// NewAptAutoremove offers "apt-get autoremove". The exact list comes from
// APT's own simulation (apt-get -s autoremove), which needs no root.
func NewAptAutoremove(run inventory.Runner, elevated bool) Tidier {
	return aptAutoremove{run: run, elevated: elevated}
}

func (aptAutoremove) ID() string     { return "apt-autoremove" }
func (aptAutoremove) Label() string  { return "Packages no longer needed (APT)" }
func (aptAutoremove) Detail() string { return "Runs: apt-get -s autoremove (simulation only)" }

var aptRemv = regexp.MustCompile(`^Remv (\S+) \[([^\]]+)\]`)

func (a aptAutoremove) Find(ctx context.Context) ([]core.UpdateStep, error) {
	out, err := a.run(ctx, "apt-get", "-s", "autoremove")
	if err != nil {
		return nil, err
	}
	var ups []core.Update
	for _, l := range strings.Split(string(out), "\n") {
		if m := aptRemv.FindStringSubmatch(strings.TrimSpace(l)); m != nil {
			name := m[1]
			if i := strings.Index(name, ":"); i > 0 {
				name = name[:i]
			}
			ups = append(ups, core.Update{Manager: "apt", Name: name, PackageID: name, CurrentVersion: m[2], Origin: inventory.SourceDpkg})
		}
	}
	if len(ups) == 0 {
		return nil, nil
	}
	cmd, elev := privileged(a.elevated, core.Command{Program: "apt-get", Args: []string{"autoremove"}})
	return []core.UpdateStep{{
		Manager: "apt", Label: a.Label(), Action: core.ActionRemove, Elevation: elev, Updates: ups,
		Commands: []core.Command{cmd},
		Rollback: "APT installed these automatically for software that has since been removed. To get one back: sudo apt-get install <package>",
	}}, nil
}

// ---- DNF ----

type dnfCache struct {
	dirs     []string
	elevated bool
}

// NewDnfCache offers "dnf clean packages" when cached .rpm files exist.
func NewDnfCache(dirs []string, elevated bool) Tidier {
	return dnfCache{dirs: dirs, elevated: elevated}
}

func (dnfCache) ID() string       { return "dnf-cache" }
func (dnfCache) Label() string    { return "Downloaded package files (DNF)" }
func (d dnfCache) Detail() string { return "Lists *.rpm under " + strings.Join(d.dirs, ", ") }

func (d dnfCache) Find(ctx context.Context) ([]core.UpdateStep, error) {
	found := false
	n, total := 0, int64(0)
	for _, dir := range d.dirs {
		if _, err := os.Stat(dir); err != nil {
			continue
		}
		found = true
		_ = filepath.WalkDir(dir, func(p string, e fs.DirEntry, err error) error {
			if err == nil && e.Type().IsRegular() && strings.HasSuffix(p, ".rpm") && filepath.Base(filepath.Dir(p)) == "packages" {
				if info, err := e.Info(); err == nil {
					n++
					total += info.Size()
				}
			}
			return ctx.Err()
		})
	}
	if !found {
		return nil, core.ErrSourceUnavailable
	}
	if n == 0 {
		return nil, ctx.Err()
	}
	cmd, elev := privileged(d.elevated, core.Command{Program: "dnf", Args: []string{"clean", "packages"}})
	return []core.UpdateStep{{
		Manager: "dnf", Label: d.Label(), Action: core.ActionClean, Bytes: total, Elevation: elev,
		Updates:  []core.Update{{Manager: "dnf", Name: inventory.Plural(n, "downloaded package file")}},
		Commands: []core.Command{cmd},
		Rollback: "Nothing you use is lost: DNF downloads package files again when it needs them",
	}}, nil
}

type dnfAutoremove struct {
	run      inventory.Runner
	elevated bool
}

// NewDnfAutoremove offers "dnf autoremove", listing exactly what DNF
// considers unneeded (dnf repoquery --unneeded, read-only).
func NewDnfAutoremove(run inventory.Runner, elevated bool) Tidier {
	return dnfAutoremove{run: run, elevated: elevated}
}

func (dnfAutoremove) ID() string     { return "dnf-autoremove" }
func (dnfAutoremove) Label() string  { return "Packages no longer needed (DNF)" }
func (dnfAutoremove) Detail() string { return "Runs: dnf repoquery --unneeded" }

func (d dnfAutoremove) Find(ctx context.Context) ([]core.UpdateStep, error) {
	out, err := d.run(ctx, "dnf", "repoquery", "--unneeded", "--qf", "%{name}\n")
	if err != nil {
		return nil, err
	}
	var ups []core.Update
	seen := map[string]bool{}
	for _, l := range strings.Split(string(out), "\n") {
		n := strings.TrimSpace(l)
		if n == "" || strings.ContainsAny(n, " :") || seen[n] {
			continue
		}
		seen[n] = true
		ups = append(ups, core.Update{Manager: "dnf", Name: n, PackageID: n, Origin: inventory.SourceRPM})
	}
	if len(ups) == 0 {
		return nil, nil
	}
	cmd, elev := privileged(d.elevated, core.Command{Program: "dnf", Args: []string{"autoremove"}})
	return []core.UpdateStep{{
		Manager: "dnf", Label: d.Label(), Action: core.ActionRemove, Elevation: elev, Updates: ups,
		Commands: []core.Command{cmd},
		Rollback: "To get one back: sudo dnf install <package>",
	}}, nil
}

// ---- snap old revisions ----

type snapOld struct {
	run      inventory.Runner
	dir      string
	elevated bool
}

// NewSnapOldRevisions offers removing disabled snap revisions, which snapd
// keeps after updates. The current version of each app is untouched.
func NewSnapOldRevisions(run inventory.Runner, dir string, elevated bool) Tidier {
	return snapOld{run: run, dir: dir, elevated: elevated}
}

func (snapOld) ID() string     { return "snap-old" }
func (snapOld) Label() string  { return "Old snap versions" }
func (snapOld) Detail() string { return "Runs: snap list --all (disabled revisions)" }

func (s snapOld) Find(ctx context.Context) ([]core.UpdateStep, error) {
	out, err := s.run(ctx, "snap", "list", "--all")
	if err != nil {
		return nil, err
	}
	var ups []core.Update
	var cmds []core.Command
	var total int64
	elev := core.ElevationNone
	for _, l := range strings.Split(string(out), "\n") {
		f := strings.Fields(l)
		if len(f) < 6 || f[0] == "Name" || !strings.Contains(f[5], "disabled") {
			continue
		}
		name, version, rev := f[0], f[1], f[2]
		if info, err := os.Stat(filepath.Join(s.dir, name+"_"+rev+".snap")); err == nil {
			total += info.Size()
		}
		ups = append(ups, core.Update{Manager: "snap", Name: name, CurrentVersion: version + " (revision " + rev + ")"})
		var c core.Command
		c, elev = privileged(s.elevated, core.Command{Program: "snap", Args: []string{"remove", name, "--revision=" + rev}})
		cmds = append(cmds, c)
	}
	if len(ups) == 0 {
		return nil, nil
	}
	return []core.UpdateStep{{
		Manager: "snap", Label: s.Label(), Action: core.ActionClean, Bytes: total, Elevation: elev, Updates: ups,
		Commands: cmds,
		Rollback: "Old revisions can't be brought back once removed. The version you use now is untouched",
	}}, nil
}

// ---- Scoop ----

type scoopOld struct {
	roots    []inventory.ScoopRoot
	elevated bool
}

// NewScoopOldVersions offers "scoop cleanup" for versions Scoop keeps after
// updating. The current version is untouched.
func NewScoopOldVersions(roots []inventory.ScoopRoot, elevated bool) Tidier {
	return scoopOld{roots: roots, elevated: elevated}
}

func (scoopOld) ID() string    { return "scoop-old" }
func (scoopOld) Label() string { return "Old Scoop versions" }
func (scoopOld) Detail() string {
	return "Lists <scoop>\\apps\\<app>\\<version> folders other than the current one"
}

func (s scoopOld) Find(ctx context.Context) ([]core.UpdateStep, error) {
	found := false
	var steps []core.UpdateStep
	for _, root := range s.roots {
		apps := filepath.Join(root.Dir, "apps")
		entries, err := os.ReadDir(apps)
		if err != nil {
			continue
		}
		found = true
		var ups []core.Update
		var names []string
		var total int64
		for _, e := range entries {
			if !e.IsDir() || e.Name() == "scoop" {
				continue
			}
			var m struct {
				Version string `json:"version"`
			}
			data, err := os.ReadFile(filepath.Join(apps, e.Name(), "current", "manifest.json"))
			if err != nil || json.Unmarshal(data, &m) != nil || m.Version == "" {
				continue
			}
			vers, _ := os.ReadDir(filepath.Join(apps, e.Name()))
			var old []string
			for _, v := range vers {
				if v.IsDir() && v.Name() != "current" && v.Name() != m.Version {
					old = append(old, v.Name())
					total += sizeOf(filepath.Join(apps, e.Name(), v.Name()))
				}
			}
			if len(old) > 0 {
				names = append(names, e.Name())
				ups = append(ups, core.Update{Manager: "scoop", Name: e.Name(), CurrentVersion: strings.Join(old, ", ")})
			}
		}
		if len(ups) == 0 {
			continue
		}
		args := append([]string{"cleanup"}, names...)
		st := core.UpdateStep{Manager: "scoop", Label: s.Label(), Action: core.ActionClean, Bytes: total, Elevation: core.ElevationNone, Updates: ups,
			Rollback: "Old versions are deleted. To use one again: scoop install <app>@<version>"}
		if root.Scope == "machine" {
			args = append([]string{"cleanup", "--global"}, names...)
			st.Label += " (all users)"
			st.Elevation = core.ElevationAdmin
			if !s.elevated {
				st.Blocked = "Scoop apps installed for all users can only be cleaned up from an administrator terminal."
			}
		}
		st.Commands = []core.Command{{Program: "scoop", Args: args}}
		steps = append(steps, st)
	}
	if !found {
		return nil, core.ErrSourceUnavailable
	}
	return steps, nil
}

type scoopCache struct{ roots []inventory.ScoopRoot }

// NewScoopCache offers "scoop cache rm *" for downloaded installers.
func NewScoopCache(roots []inventory.ScoopRoot) Tidier { return scoopCache{roots: roots} }

func (scoopCache) ID() string     { return "scoop-cache" }
func (scoopCache) Label() string  { return "Downloaded installers (Scoop)" }
func (scoopCache) Detail() string { return "Lists files in <scoop>\\cache" }

func (s scoopCache) Find(ctx context.Context) ([]core.UpdateStep, error) {
	for _, root := range s.roots {
		if root.Scope != "user" {
			continue
		}
		dir := filepath.Join(root.Dir, "cache")
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		n, total := 0, int64(0)
		for _, e := range entries {
			if e.Type().IsRegular() {
				if info, err := e.Info(); err == nil {
					n++
					total += info.Size()
				}
			}
		}
		if n == 0 {
			return nil, nil
		}
		return []core.UpdateStep{{
			Manager: "scoop", Label: s.Label(), Action: core.ActionClean, Bytes: total, Elevation: core.ElevationNone,
			Updates:  []core.Update{{Manager: "scoop", Name: inventory.Plural(n, "downloaded installer")}},
			Commands: []core.Command{{Program: "scoop", Args: []string{"cache", "rm", "*"}}},
			Rollback: "Nothing you use is lost: Scoop downloads installers again when it needs them",
		}}, nil
	}
	return nil, core.ErrSourceUnavailable
}
