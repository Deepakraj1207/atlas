package updates

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

type scoop struct {
	roots    []inventory.ScoopRoot
	elevated bool
}

// NewScoop compares installed versions with the bucket copies already on
// disk, so checking needs no network and runs no PowerShell. Updating runs
// "scoop update" (refresh buckets) and then "scoop update <apps>".
func NewScoop(roots []inventory.ScoopRoot, elevated bool) core.PackageManager {
	return scoop{roots: roots, elevated: elevated}
}

func (scoop) ID() string    { return ManagerScoop }
func (scoop) Label() string { return "Scoop" }
func (scoop) Detail() string {
	return "Compares <scoop>\\apps\\*\\current\\manifest.json with <scoop>\\buckets\\<bucket>\\bucket\\<app>.json"
}

type scoopVersion struct {
	Version string `json:"version"`
}

type scoopInstallInfo struct {
	Bucket string `json:"bucket"`
	Hold   bool   `json:"hold"`
}

func (s scoop) CheckUpdates(ctx context.Context) ([]core.Update, error) {
	found := false
	var ups []core.Update
	for _, root := range s.roots {
		apps := filepath.Join(root.Dir, "apps")
		entries, err := os.ReadDir(apps)
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		found = true
		for _, e := range entries {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			name := e.Name()
			if !e.IsDir() || name == "scoop" {
				continue
			}
			current := filepath.Join(apps, name, "current")
			var cur scoopVersion
			var info scoopInstallInfo
			if !readJSON(filepath.Join(current, "manifest.json"), &cur) || !readJSON(filepath.Join(current, "install.json"), &info) || info.Bucket == "" {
				continue // broken install, or installed from a URL with no bucket
			}
			var latest scoopVersion
			bucket := filepath.Join(root.Dir, "buckets", info.Bucket)
			if !readJSON(filepath.Join(bucket, "bucket", name+".json"), &latest) && !readJSON(filepath.Join(bucket, name+".json"), &latest) {
				continue
			}
			if latest.Version == "" || latest.Version == cur.Version || latest.Version == "nightly" {
				continue
			}
			u := core.Update{
				Manager:          ManagerScoop,
				PackageID:        name,
				Name:             name,
				CurrentVersion:   cur.Version,
				AvailableVersion: latest.Version,
				Origin:           info.Bucket,
				Scope:            root.Scope,
			}
			if info.Hold {
				u.Held, u.Note = true, "Held in Scoop (scoop hold)"
			}
			ups = append(ups, u)
		}
	}
	if !found {
		return nil, core.ErrSourceUnavailable
	}
	return ups, nil
}

func readJSON(path string, v any) bool {
	data, err := os.ReadFile(path)
	return err == nil && json.Unmarshal(data, v) == nil
}

func (s scoop) PlanUpdates(updates []core.Update) []core.UpdateStep {
	var user, global []core.Update
	for _, u := range updates {
		if u.Scope == "machine" {
			global = append(global, u)
		} else {
			user = append(user, u)
		}
	}
	rollback := "Scoop keeps the previous version until you run cleanup. To go back: scoop reset <app>@<old version>"
	var steps []core.UpdateStep
	if len(user) > 0 {
		steps = append(steps, core.UpdateStep{
			Manager: ManagerScoop, Label: s.Label(), Updates: user, Elevation: core.ElevationNone, Rollback: rollback,
			Commands: []core.Command{
				{Program: "scoop", Args: []string{"update"}},
				{Program: "scoop", Args: append([]string{"update"}, ids(user)...)},
			},
		})
	}
	if len(global) > 0 {
		st := core.UpdateStep{
			Manager: ManagerScoop, Label: s.Label() + " (all users)", Updates: global, Elevation: core.ElevationAdmin, Rollback: rollback,
			Commands: []core.Command{
				{Program: "scoop", Args: []string{"update"}},
				{Program: "scoop", Args: append([]string{"update", "--global"}, ids(global)...)},
			},
		}
		if !s.elevated {
			st.Blocked = "Scoop apps installed for all users can only be updated from an administrator terminal."
		}
		steps = append(steps, st)
	}
	return steps
}
