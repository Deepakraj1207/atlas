package updates

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

type winget struct {
	run      inventory.Runner
	elevated bool
}

// NewWinget checks with "winget upgrade" and updates one package at a time
// by exact ID. No agreement flags are passed: winget asks the user itself.
func NewWinget(run inventory.Runner, elevated bool) core.PackageManager {
	return winget{run: run, elevated: elevated}
}

func (winget) ID() string     { return ManagerWinget }
func (winget) Label() string  { return "Windows Package Manager" }
func (winget) Detail() string { return "Runs: winget upgrade --disable-interactivity" }

func (w winget) CheckUpdates(ctx context.Context) ([]core.Update, error) {
	out, err := w.run(ctx, "winget", "upgrade", "--disable-interactivity")
	if err != nil && (errors.Is(err, core.ErrSourceUnavailable) || ctx.Err() != nil) {
		return nil, err
	}
	ups, perr := parseWingetUpgrade(string(out))
	if perr == nil {
		return ups, nil
	}
	if err != nil {
		return nil, inventory.WingetError(err)
	}
	return nil, perr
}

var errWingetNoTable = errors.New("winget output did not contain an upgrade table")

func parseWingetUpgrade(out string) ([]core.Update, error) {
	tables, err := inventory.ParseWingetTables(out)
	if err != nil {
		low := strings.ToLower(out)
		if strings.Contains(low, "no applicable") || strings.Contains(low, "no installed package found") ||
			strings.Contains(low, "no available upgrade") {
			return nil, nil
		}
		return nil, errWingetNoTable
	}
	var ups []core.Update
	for ti, rows := range tables {
		for _, r := range rows {
			if r.Available == "" {
				continue
			}
			u := core.Update{
				Manager:          ManagerWinget,
				PackageID:        r.ID,
				Name:             r.Name,
				CurrentVersion:   strings.TrimPrefix(strings.TrimPrefix(r.Version, "< "), "> "),
				AvailableVersion: r.Available,
				Origin:           r.Source,
			}
			if inventory.Truncated(r.ID) {
				u.PackageID = ""
				u.Note = "winget shortened this package's ID to fit the terminal"
			}
			if ti > 0 {
				// The second table lists packages that "require explicit
				// targeting", usually because they are pinned.
				u.Held = true
				u.Note = "winget only updates this when asked for it by name (it may be pinned)"
			}
			ups = append(ups, u)
		}
	}
	return ups, nil
}

func (w winget) PlanUpdates(updates []core.Update) []core.UpdateStep {
	elev := core.ElevationInstaller
	if w.elevated {
		elev = core.ElevationAdmin
	}
	var steps []core.UpdateStep
	for _, u := range updates {
		args := []string{"upgrade", "--id", u.PackageID, "--exact"}
		if u.Origin != "" {
			args = append(args, "--source", u.Origin)
		}
		st := core.UpdateStep{
			Manager:   ManagerWinget,
			Label:     w.Label(),
			Updates:   []core.Update{u},
			Commands:  []core.Command{{Program: "winget", Args: args}},
			Elevation: elev,
			Rollback:  "winget doesn't keep the old version. To go back, reinstall it from the same source",
		}
		if u.CurrentVersion != "" && u.PackageID != "" {
			st.Rollback = fmt.Sprintf("winget doesn't keep the old version. To go back: winget install --id %s --version %s --force (if the source still offers it)", u.PackageID, u.CurrentVersion)
		}
		if u.PackageID == "" {
			st.Blocked = "winget shortened this package's ID, so ATLAS can't target it safely. Make the terminal wider and check again."
			st.Commands = nil
		}
		steps = append(steps, st)
	}
	return steps
}
