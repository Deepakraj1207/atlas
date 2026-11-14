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
// provide the primary record and later ones fill gaps.
//
// Merging is deliberately conservative: when ATLAS can't tell whether two
// entries are one install, it keeps both. Two entries merge only when:
//   - their sources can report the same installed copy (sameInstallSources):
//     a Scoop, Snap or Flatpak app, or a dpkg/rpm package, is its own copy and
//     never merges with another manager's entry;
//   - they don't contradict each other: one source never merges two different
//     package IDs or two different publishers (distinctWithinSource);
//   - their names and versions match. The same program at two different
//     versions stays as two entries.
//
// An entry whose name was cut short by winget ("…") or that has no version
// is a weak match. It's joined only when exactly one entry fits it, so
// "Microsoft Visual C++ 2015-2022 Redistributable…" stays on its own when
// both the x64 and x86 editions are installed.
func Merge(items []core.SoftwareItem) []core.SoftwareItem {
	var out []core.SoftwareItem
	var first []int // input position of each out entry's primary record
	var weak []int
	for k, it := range items {
		if weakEntry(it) {
			weak = append(weak, k)
			continue
		}
		idx := -1
		for i := range out {
			if canMerge(out[i], it) {
				idx = i
				break
			}
		}
		if idx < 0 {
			out = append(out, it)
			first = append(first, k)
			continue
		}
		mergeInto(&out[idx], it)
	}
	for _, k := range weak {
		it := items[k]
		var cands []int
		for i := range out {
			if canMerge(out[i], it) {
				cands = append(cands, i)
			}
		}
		if len(cands) != 1 {
			out = append(out, it)
			first = append(first, k)
			continue
		}
		c := cands[0]
		if k < first[c] { // the weak entry came from a higher-priority source
			primary := it
			mergeInto(&primary, out[c])
			out[c], first[c] = primary, k
			continue
		}
		mergeInto(&out[c], it)
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

// weakEntry reports whether an entry lacks enough detail to be matched on
// its own: a truncated name, or a missing or truncated version.
func weakEntry(it core.SoftwareItem) bool {
	return truncated(it.Name) || normVersion(it.Version) == "" || truncated(it.Version)
}

func canMerge(a, b core.SoftwareItem) bool {
	return sameInstallSources(a, b) && !distinctWithinSource(a, b) && namesMatch(a, b) && versionsMatch(a, b)
}

// windowsInstallerSource reports sources that describe programs installed
// by a normal installer: the Windows "installed programs" list, and winget
// and Chocolatey, which run those installers and so report the same copy.
func windowsInstallerSource(s string) bool {
	return s == SourceRegistry || s == SourceWinget || s == SourceChocolatey
}

// sameInstallSources reports whether a and b could be two reports of one
// installed copy. A source can list the same copy twice (Windows keeps
// 32-bit, 64-bit and per-user lists); across sources only the Windows
// installer sources overlap. Scoop apps are portable copies, and Linux
// package managers (dpkg, rpm, Snap, Flatpak) each install their own copy.
func sameInstallSources(a, b core.SoftwareItem) bool {
	sa, sb := a.Source, b.Source
	return sa == sb || (windowsInstallerSource(sa) && windowsInstallerSource(sb))
}

// distinctWithinSource reports whether a and b, from the same source, are
// plainly different software: different package IDs, or different
// publishers. (Across sources these fields aren't comparable: Chocolatey
// reports a package's authors, not the program's publisher.)
func distinctWithinSource(a, b core.SoftwareItem) bool {
	if a.Source != b.Source {
		return false
	}
	if a.PackageID != "" && b.PackageID != "" && !strings.EqualFold(a.PackageID, b.PackageID) {
		return true
	}
	pa, pb := normPublisher(a.Publisher), normPublisher(b.Publisher)
	return pa != "" && pb != "" && pa != pb
}

var publisherSuffixes = []string{"inc", "incorporated", "corp", "corporation", "co", "company", "ltd", "limited", "llc", "gmbh", "ag", "sa", "bv", "srl", "plc"}

// normPublisher lets "Devolutions inc." and "Devolutions Inc" compare equal.
func normPublisher(p string) string {
	words := strings.Fields(reNonAlnum.ReplaceAllString(strings.ToLower(p), " "))
	for len(words) > 1 && slices.Contains(publisherSuffixes, words[len(words)-1]) {
		words = words[:len(words)-1]
	}
	return strings.Join(words, " ")
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

// joinWingetReadsOfRegistry joins a winget entry with the Windows
// "installed programs" entry it was read from. For desktop programs (not
// Windows app packages, which winget reads from elsewhere), winget list
// reads those same entries but shows
// the version from its own catalogue: PyCharm's "262.10315.174" becomes
// "2026.2.2", "9.4.0.129" becomes "9.4.0", "3.10.2.920" becomes "< 3.10.3".
// So versions are not compared. To avoid joining two real installs, the
// name must belong to exactly one unjoined entry on each side.
func joinWingetReadsOfRegistry(items []core.SoftwareItem, used []bool) {
	isWingetARP := func(it core.SoftwareItem) bool {
		return it.Source == SourceWinget && it.Origin != "MSIX" && it.Origin != "msstore"
	}
	for i := range items {
		if used[i] || items[i].Source != SourceRegistry || len(normName(items[i].Name)) < 3 {
			continue
		}
		var regs, wgs []int
		for j := range items {
			if used[j] || !namesMatch(items[i], items[j]) {
				continue
			}
			switch {
			case items[j].Source == SourceRegistry:
				regs = append(regs, j)
			case isWingetARP(items[j]):
				wgs = append(wgs, j)
			}
		}
		if len(regs) == 1 && len(wgs) == 1 && registryNameMatches(items, used, wgs[0]) == 1 {
			mergeInto(&items[i], items[wgs[0]])
			used[wgs[0]] = true
		}
	}
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
		var cands []int
		for j := range items {
			if used[j] || i == j {
				continue
			}
			b := items[j]
			if b.Source != SourceChocolatey && b.Source != SourceWinget {
				continue
			}
			if ProductKey(b.Name) == key && VersionsEquivalent(items[i].Version, b.Version) {
				cands = append(cands, j)
			}
		}
		// Join only a one-to-one pair: if two installer records fit, or the
		// record also fits another Windows entry (the x64 and x86 editions),
		// ATLAS can't tell which install it describes.
		if len(cands) != 1 || installerKeyMatches(items, used, cands[0]) != 1 {
			continue
		}
		mergeInto(&items[i], items[cands[0]])
		used[cands[0]] = true
	}
	joinWingetReadsOfRegistry(items, used)
	out := items[:0:0]
	for i, it := range items {
		if !used[i] {
			out = append(out, it)
		}
	}
	return out
}

// installerKeyMatches counts the unjoined Windows entries that the installer
// record items[j] fits by product name and version.
func installerKeyMatches(items []core.SoftwareItem, used []bool, j int) int {
	key, n := ProductKey(items[j].Name), 0
	for i := range items {
		if !used[i] && items[i].Source == SourceRegistry && ProductKey(items[i].Name) == key &&
			VersionsEquivalent(items[i].Version, items[j].Version) {
			n++
		}
	}
	return n
}

// registryNameMatches counts the unjoined Windows entries whose name fits
// items[j]: a name winget cut short can fit several.
func registryNameMatches(items []core.SoftwareItem, used []bool, j int) int {
	n := 0
	for i := range items {
		if !used[i] && items[i].Source == SourceRegistry && namesMatch(items[i], items[j]) {
			n++
		}
	}
	return n
}
