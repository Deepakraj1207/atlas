package inventory

import (
	"context"
	"regexp"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
)

type flatpakScanner struct{ run Runner }

// NewFlatpakScanner lists installed Flatpak applications and runtimes.
func NewFlatpakScanner(run Runner) core.Scanner { return flatpakScanner{run: run} }

func (flatpakScanner) ID() string    { return SourceFlatpak }
func (flatpakScanner) Label() string { return "Flatpak" }
func (flatpakScanner) Detail() string {
	return "Runs: flatpak list --app|--runtime --columns=application,name,version,origin,installation"
}

const flatpakColumns = "--columns=application,name,version,origin,installation"

func (s flatpakScanner) Scan(ctx context.Context) ([]core.SoftwareItem, error) {
	apps, err := s.run(ctx, "flatpak", "list", "--app", flatpakColumns)
	if err != nil {
		return nil, err
	}
	items := parseFlatpak(string(apps), core.KindApp)
	if runtimes, err := s.run(ctx, "flatpak", "list", "--runtime", flatpakColumns); err == nil {
		items = append(items, parseFlatpak(string(runtimes), core.KindComponent)...)
	} else if ctx.Err() != nil {
		return nil, err
	}
	return items, nil
}

var multiSpace = regexp.MustCompile(`\s{2,}`)

// parseFlatpak reads tab-separated columns (flatpak's non-terminal output),
// falling back to runs of spaces.
func parseFlatpak(out string, kind core.Kind) []core.SoftwareItem {
	var items []core.SoftwareItem
	for _, l := range strings.Split(out, "\n") {
		l = strings.TrimRight(l, "\r")
		if strings.TrimSpace(l) == "" {
			continue
		}
		f := strings.Split(l, "\t")
		if len(f) == 1 {
			f = multiSpace.Split(strings.TrimSpace(l), -1)
		}
		get := func(i int) string {
			if i < len(f) {
				return strings.TrimSpace(f[i])
			}
			return ""
		}
		id := get(0)
		if id == "" || strings.EqualFold(id, "Application ID") || strings.EqualFold(id, "Application") {
			continue
		}
		name := get(1)
		if name == "" {
			name = id
		}
		scope := get(4)
		if scope == "system" {
			scope = "machine"
		}
		items = append(items, core.SoftwareItem{
			Name:      name,
			Version:   get(2),
			PackageID: id,
			Publisher: get(3),
			Source:    SourceFlatpak,
			Kind:      kind,
			Scope:     scope,
		})
	}
	return items
}
