package inventory

import (
	"context"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
)

type snapScanner struct{ run Runner }

// NewSnapScanner lists installed snaps.
func NewSnapScanner(run Runner) core.Scanner { return snapScanner{run: run} }

func (snapScanner) ID() string     { return SourceSnap }
func (snapScanner) Label() string  { return "Snap" }
func (snapScanner) Detail() string { return "Runs: snap list" }

func (s snapScanner) Scan(ctx context.Context) ([]core.SoftwareItem, error) {
	out, err := s.run(ctx, "snap", "list")
	if err != nil {
		return nil, err
	}
	return parseSnapList(string(out)), nil
}

var snapComponentPrefixes = []string{"core", "snapd", "bare", "gnome-", "gtk-common-themes", "kf5-", "kde-frameworks", "mesa-", "gpu-"}

func parseSnapList(out string) []core.SoftwareItem {
	var items []core.SoftwareItem
	header := true
	for _, l := range strings.Split(out, "\n") {
		f := strings.Fields(l)
		if len(f) == 0 {
			continue
		}
		if header {
			if f[0] == "Name" {
				header = false
			}
			continue
		}
		if len(f) < 2 {
			continue
		}
		it := core.SoftwareItem{
			Name:      f[0],
			Version:   f[1],
			PackageID: f[0],
			Source:    SourceSnap,
			Kind:      core.KindApp,
			Scope:     "machine",
		}
		if len(f) >= 5 {
			it.Publisher = strings.TrimRight(f[4], "✓*")
		}
		notes := ""
		if len(f) >= 6 {
			notes = f[5]
		}
		if strings.Contains(notes, "base") || strings.Contains(notes, "snapd") || strings.Contains(notes, "core") {
			it.Kind = core.KindComponent
		}
		for _, p := range snapComponentPrefixes {
			if strings.HasPrefix(it.Name, p) {
				it.Kind = core.KindComponent
			}
		}
		items = append(items, it)
	}
	return items
}
