package updates

import (
	"context"
	"errors"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

type chocolatey struct {
	run      inventory.Runner
	elevated bool
}

// NewChocolatey checks with "choco outdated" and updates with
// "choco upgrade". Chocolatey needs an administrator terminal to install,
// so updates are blocked (and explained) when ATLAS isn't elevated.
func NewChocolatey(run inventory.Runner, elevated bool) core.PackageManager {
	return chocolatey{run: run, elevated: elevated}
}

func (chocolatey) ID() string     { return ManagerChocolatey }
func (chocolatey) Label() string  { return "Chocolatey" }
func (chocolatey) Detail() string { return "Runs: choco outdated -r --ignore-unfound" }

func (c chocolatey) CheckUpdates(ctx context.Context) ([]core.Update, error) {
	out, err := c.run(ctx, "choco", "outdated", "-r", "--ignore-unfound")
	if err != nil {
		var ce *inventory.CommandError
		// With "enhanced exit codes" on, 2 means "some packages are outdated".
		if !errors.As(err, &ce) || ce.ExitCode != 2 {
			return nil, err
		}
	}
	return parseChocoOutdated(string(out)), nil
}

func parseChocoOutdated(out string) []core.Update {
	var ups []core.Update
	for _, l := range strings.Split(out, "\n") {
		f := strings.Split(strings.TrimSpace(l), "|")
		if len(f) < 3 || f[0] == "" || f[2] == "" {
			continue
		}
		u := core.Update{
			Manager:          ManagerChocolatey,
			PackageID:        f[0],
			Name:             f[0],
			CurrentVersion:   f[1],
			AvailableVersion: f[2],
			Origin:           "chocolatey",
			Scope:            "machine",
		}
		if len(f) >= 4 && strings.EqualFold(f[3], "true") {
			u.Held = true
			u.Note = "Pinned in Chocolatey (choco pin)"
		}
		ups = append(ups, u)
	}
	return ups
}

func (c chocolatey) PlanUpdates(updates []core.Update) []core.UpdateStep {
	if len(updates) == 0 {
		return nil
	}
	st := core.UpdateStep{
		Manager:   ManagerChocolatey,
		Label:     c.Label(),
		Updates:   updates,
		Elevation: core.ElevationAdmin,
		Commands: []core.Command{{
			Program:      "choco",
			Args:         append([]string{"upgrade"}, ids(updates)...),
			RestartCodes: []int{1641, 3010},
		}},
		Rollback: "Chocolatey can reinstall an older version: choco install <package> --version <old version> --allow-downgrade",
	}
	if !c.elevated {
		st.Blocked = "Chocolatey only installs updates from an administrator terminal. Start ATLAS with “Run as administrator” to update these, or run the command yourself in one."
	}
	return []core.UpdateStep{st}
}
