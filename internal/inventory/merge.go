package inventory

import (
	"regexp"
	"slices"
	"strconv"
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
	out = mergeInstallerRecords(out)
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

var (
	reParens   = regexp.MustCompile(`\([^)]*\)`)
	reVersions = regexp.MustCompile(`\b[vV]?\d+(?:[._]\d+)+\b|(?:^|\s)[vV]?\d+(?:\s|$)`)
	reArchWord = regexp.MustCompile(`(?i)\b(x64|x86|arm64|amd64|64-bit|32-bit)\b`)
	reNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)
)

// ProductKey reduces a display name to the product it names: "7-Zip 26.05
// (x64)" and "7-Zip (Install)" both become "7zip".
func ProductKey(name string) string {
	s := strings.ReplaceAll(strings.ToLower(name), "+", "plus")
	s = reParens.ReplaceAllString(s, " ")
	s = reArchWord.ReplaceAllString(s, " ")
	s = reVersions.ReplaceAllString(s, " ")
	return reNonAlnum.ReplaceAllString(s, "")
}

// VersionsEquivalent reports whether two versions are the same release
// written differently: "26.05" and "26.5.0", or "2.39.3" and "2.39.3.0".
func VersionsEquivalent(a, b string) bool {
	pa, oka := numericParts(a)
	pb, okb := numericParts(b)
	if !oka || !okb {
		return false
	}
	for len(pa) < len(pb) {
		pa = append(pa, 0)
	}
	for len(pb) < len(pa) {
		pb = append(pb, 0)
	}
	return slices.Equal(pa, pb)
}

func numericParts(v string) ([]int, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, false
	}
	var out []int
	for _, p := range strings.Split(v, ".") {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

// mergeInstallerRecords joins a Windows "installed programs" entry with the
// Chocolatey or winget record of the same install. Those tools run normal
// installers, so one program appears in both places, often with a
// different display name ("7-Zip (Install)") and version format ("26.5.0"
// vs "26.05"). Scoop apps are portable copies and are never joined this way.
func mergeInstallerRecords(items []core.SoftwareItem) []core.SoftwareItem {
	used := make([]bool, len(items))
	for i := range items {
		if items[i].Source != SourceRegistry {
			continue
		}
		key := ProductKey(items[i].Name)
		if len(key) < 3 {
			continue
		}
		for j := range items {
			if used[j] || i == j {
				continue
			}
			b := items[j]
			if b.Source != SourceChocolatey && b.Source != SourceWinget {
				continue
			}
			if ProductKey(b.Name) == key && VersionsEquivalent(items[i].Version, b.Version) {
				mergeInto(&items[i], b)
				used[j] = true
				break
			}
		}
	}
	out := items[:0:0]
	for i, it := range items {
		if !used[i] {
			out = append(out, it)
		}
	}
	return out
}
