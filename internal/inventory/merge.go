package inventory

import (
	"slices"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
)

// normName lowercases, trims and collapses whitespace, and drops a trailing
// ellipsis that winget adds when it truncates a column.
func normName(s string) string {
	s = strings.TrimSuffix(strings.TrimSpace(s), "…")
	return strings.ToLower(strings.Join(strings.Fields(s), " "))
}

func truncated(s string) bool { return strings.HasSuffix(strings.TrimSpace(s), "…") }

func normVersion(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "< ")
	v = strings.TrimPrefix(v, "> ")
	return strings.ToLower(strings.TrimSuffix(v, "…"))
}

func namesMatch(a, b core.SoftwareItem) bool {
	na, nb := normName(a.Name), normName(b.Name)
	if na == nb {
		return true
	}
	if truncated(a.Name) && strings.HasPrefix(nb, na) {
		return true
	}
	return truncated(b.Name) && strings.HasPrefix(na, nb)
}

func versionsMatch(a, b core.SoftwareItem) bool {
	va, vb := normVersion(a.Version), normVersion(b.Version)
	if va == "" || vb == "" || va == vb {
		return true
	}
	if truncated(a.Version) && strings.HasPrefix(vb, va) {
		return true
	}
	return truncated(b.Version) && strings.HasPrefix(va, vb)
}

// Merge combines reports of the same software from different sources.
// Items are processed in order, so earlier items (higher-priority sources)
// provide the primary record and later ones fill gaps. Two items merge when
// their names match and their versions match or one is unknown; the same
// program at two different versions stays as two entries.
func Merge(items []core.SoftwareItem) []core.SoftwareItem {
	var out []core.SoftwareItem
	for _, it := range items {
		idx := -1
		for i := range out {
			if namesMatch(out[i], it) && versionsMatch(out[i], it) {
				idx = i
				break
			}
		}
		if idx < 0 {
			out = append(out, it)
			continue
		}
		mergeInto(&out[idx], it)
	}
	slices.SortStableFunc(out, func(a, b core.SoftwareItem) int {
		if c := strings.Compare(normName(a.Name), normName(b.Name)); c != 0 {
			return c
		}
		return strings.Compare(a.Version, b.Version)
	})
	return out
}

func mergeInto(dst *core.SoftwareItem, src core.SoftwareItem) {
	if truncated(dst.Name) && !truncated(src.Name) {
		dst.Name = src.Name
	}
	if dst.Version == "" || (truncated(dst.Version) && !truncated(src.Version)) {
		dst.Version = src.Version
	}
	fill := func(d *string, s string) {
		if *d == "" {
			*d = s
		}
	}
	fill(&dst.Publisher, src.Publisher)
	fill(&dst.PackageID, src.PackageID)
	fill(&dst.InstallLocation, src.InstallLocation)
	fill(&dst.InstallDate, src.InstallDate)
	fill(&dst.Scope, src.Scope)
	fill(&dst.Description, src.Description)
	if src.Kind == core.KindApp {
		dst.Kind = core.KindApp
	}
	for _, s := range append([]string{src.Source}, src.AlsoFoundIn...) {
		if s != dst.Source && !slices.Contains(dst.AlsoFoundIn, s) {
			dst.AlsoFoundIn = append(dst.AlsoFoundIn, s)
		}
	}
}
