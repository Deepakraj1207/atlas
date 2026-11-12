package analysis

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// Inputs is everything the rules look at. Analyze is a pure function of it.
type Inputs struct {
	Now           time.Time
	Items         []core.SoftwareItem
	Scan          *core.ScanSummary
	UpdateSummary *core.UpdateCheckSummary
	Updates       []core.Update
	// Graphs holds dependency data keyed by inventory source ID.
	Graphs      map[string]Graph
	GraphLabels map[string]string
	// UsageAvailable is true when usage records were read successfully.
	UsageAvailable bool
	// UsageNote explains why usage isn't available, when it isn't.
	UsageNote string
	Usage     []UsageRecord
}

// Thresholds, exported so the UI and docs can quote them.
const (
	UnusedAfter      = 365 * 24 * time.Hour
	StaleScanAfter   = 30 * 24 * time.Hour
	StaleCheckAfter  = 14 * 24 * time.Hour
	maxItemsListed   = 50
	minFamilyKeySize = 3
)

func ref(it core.SoftwareItem) core.ItemRef {
	return core.ItemRef{Name: it.Name, Version: it.Version, Source: it.Source, PackageID: it.PackageID}
}

// Analyze applies every rule and returns findings (most important first),
// runtimes, and a summary of each check.
func Analyze(in Inputs) ([]core.Finding, []core.Runtime, []core.CheckSummary) {
	runtimes := detectRuntimes(in)
	var findings []core.Finding
	var checks []core.CheckSummary

	ended, ending, vc := supportFindings(runtimes, in.Now)
	findings = append(findings, ended...)
	findings = append(findings, ending...)
	findings = append(findings, vc...)
	checks = append(checks,
		core.CheckSummary{Name: "Runtimes past end of support", Found: len(ended) + len(vc), Available: true,
			Note: "Python, Node.js, .NET and Visual C++, using support dates as of " + SupportDataAsOf + ". Java isn't judged because its support depends on the vendor."},
		core.CheckSummary{Name: "Runtimes whose support ends within 6 months", Found: len(ending), Available: true})

	dups, versions := duplicateFindings(in.Items)
	findings = append(findings, dups...)
	findings = append(findings, versions...)
	checks = append(checks,
		core.CheckSummary{Name: "Same app installed by two different tools", Found: len(dups), Available: true},
		core.CheckSummary{Name: "Several versions of the same app", Found: len(versions), Available: true})

	unused, matched := unusedFinding(in)
	if unused != nil {
		findings = append(findings, *unused)
	}
	uc := core.CheckSummary{Name: "Apps not opened in over a year", Available: in.UsageAvailable}
	if in.UsageAvailable {
		if unused != nil {
			uc.Found = len(unused.Items)
		}
		uc.Note = fmt.Sprintf("Windows had a record for %d of your apps. It only records programs opened from the Start menu, taskbar or desktop, so apps without a record aren't listed.", matched)
	} else {
		uc.Note = in.UsageNote
	}
	checks = append(checks, uc)

	unneeded := unneededFindings(runtimes, in.Items)
	findings = append(findings, unneeded...)
	anyGraph := false
	for _, r := range runtimes {
		anyGraph = anyGraph || r.DepsKnown
	}
	nc := core.CheckSummary{Name: "Runtimes nothing else needs", Found: len(unneeded), Available: anyGraph}
	if !anyGraph {
		nc.Note = "No dependency records were available for the runtimes found."
	}
	checks = append(checks, nc)

	fresh := freshnessFindings(in)
	findings = append(findings, fresh...)
	pending := 0
	for _, u := range in.Updates {
		if !u.Held {
			pending++
		}
	}
	checks = append(checks, core.CheckSummary{Name: "Updates waiting", Found: pending, Available: in.UpdateSummary != nil,
		Note: map[bool]string{true: "", false: "Updates haven't been checked yet."}[in.UpdateSummary != nil]})

	slices.SortStableFunc(findings, func(a, b core.Finding) int {
		if a.Severity != b.Severity {
			if a.Severity == core.SeverityAttention {
				return -1
			}
			return 1
		}
		return 0
	})
	return findings, runtimes, checks
}

func detectRuntimes(in Inputs) []core.Runtime {
	names := map[[2]string]string{} // (source, package id) -> display name
	for _, it := range in.Items {
		if it.PackageID != "" {
			names[[2]string{it.Source, key(it.PackageID)}] = it.Name
		}
	}
	var out []core.Runtime
	for _, it := range in.Items {
		fam, branch, ok := DetectRuntime(it)
		if !ok {
			continue
		}
		rt := core.Runtime{
			Family: fam, Branch: branch, Name: it.Name, Version: it.Version,
			Source: it.Source, PackageID: it.PackageID,
			Support: LookupSupport(fam, branch, distroSource(it.Source), in.Now),
		}
		for _, src := range append([]string{it.Source}, it.AlsoFoundIn...) {
			g, ok := in.Graphs[src]
			pkg := it.PackageID
			if pkg == "" {
				pkg = it.Name
			}
			if !ok || !g.Has(pkg) {
				continue
			}
			direct, indirect := g.Dependents(pkg)
			rt.DepsKnown, rt.DepsSource, rt.Indirect = true, in.GraphLabels[src], indirect
			for _, d := range direct {
				if n, ok := names[[2]string{src, d}]; ok {
					rt.Dependents = append(rt.Dependents, n)
				} else {
					rt.Dependents = append(rt.Dependents, d)
				}
			}
			break
		}
		out = append(out, rt)
	}
	slices.SortStableFunc(out, func(a, b core.Runtime) int {
		if c := strings.Compare(a.Family, b.Family); c != 0 {
			return c
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out
}

func runtimeLabel(family, branch string) string { return family + " " + branch }

func supportFindings(runtimes []core.Runtime, now time.Time) (ended, ending, vc []core.Finding) {
	type group struct {
		family, branch string
		support        core.Support
		items          []core.ItemRef
		distro         bool
	}
	groups := map[string]*group{}
	var order []string
	for _, r := range runtimes {
		if r.Support.State != "ended" && r.Support.State != "ending" {
			continue
		}
		k := r.Family + "|" + r.Branch + "|" + fmt.Sprint(r.Support.DistroManaged)
		g, ok := groups[k]
		if !ok {
			g = &group{family: r.Family, branch: r.Branch, support: r.Support, distro: r.Support.DistroManaged}
			groups[k] = g
			order = append(order, k)
		}
		g.items = append(g.items, core.ItemRef{Name: r.Name, Version: r.Version, Source: r.Source, PackageID: r.PackageID})
	}
	var oldVC []core.ItemRef
	var vcDetails []string
	for _, k := range order {
		g := groups[k]
		label := runtimeLabel(g.family, g.branch)
		when := FormatEnd(g.support.EndDate)
		details := []string{fmt.Sprintf("%s: end of support %s (%s)", label, g.support.EndDate, g.support.State)}
		switch {
		case g.family == "Visual C++" && g.support.State == "ended":
			oldVC = append(oldVC, g.items...)
			vcDetails = append(vcDetails, details...)
		case g.support.State == "ending":
			ending = append(ending, core.Finding{
				Key: "ending:" + key(label), Kind: "support", Severity: core.SeverityInfo,
				Title:      fmt.Sprintf("Support for %s ends in %s", label, when),
				Summary:    fmt.Sprintf("After %s, %s stops receiving fixes, including security fixes.", when, label),
				Why:        "Planning a move to a newer version now avoids a rush later.",
				Suggestion: "When convenient, install a newer supported version. Dependencies (5) shows what uses this one.",
				Items:      g.items, Details: details, Reference: g.support.Reference,
			})
		case g.distro:
			ended = append(ended, core.Finding{
				Key: "eol-distro:" + key(label), Kind: "support", Severity: core.SeverityInfo,
				Title:      fmt.Sprintf("%s is past its upstream end of support", label),
				Summary:    fmt.Sprintf("The %s project stopped supporting %s in %s. Your Linux distribution may still provide security fixes for its package.", g.family, g.branch, when),
				Why:        "Distribution maintainers often keep patching older versions for the life of the release, but not always.",
				Suggestion: "Check your distribution's support policy. Upgrading the distribution moves you to newer versions.",
				Items:      g.items, Details: details, Reference: g.support.Reference,
			})
		default:
			ended = append(ended, core.Finding{
				Key: "eol:" + key(label), Kind: "support", Severity: core.SeverityAttention,
				Title:      fmt.Sprintf("%s no longer gets security fixes", label),
				Summary:    fmt.Sprintf("Support for %s ended on %s.", label, when),
				Why:        "Software that no longer gets fixes can have known security problems that will never be patched.",
				Suggestion: "Install a supported version (the newest is usually best). Before removing this one, check Dependencies (5) for anything that still needs it; Cleanup (6) checks again before removing anything.",
				Items:      g.items, Details: details, Reference: g.support.Reference,
			})
		}
	}
	if len(oldVC) > 0 {
		vc = append(vc, core.Finding{
			Key: "eol:visual-c++", Kind: "support", Severity: core.SeverityInfo,
			Title:      "Old Visual C++ runtimes are installed",
			Summary:    "Microsoft no longer supports these Visual C++ Redistributables, but older programs may still need them to run.",
			Why:        "Windows doesn't record which programs use them, so removing one can break an app with no warning.",
			Suggestion: "Leave them installed unless you're sure nothing uses them. Keeping the newest (2015-2022) version updated matters most.",
			Items:      oldVC, Details: vcDetails, Reference: supportRefs["Visual C++"],
		})
	}
	return ended, ending, vc
}

var (
	reParens = regexp.MustCompile(`\([^)]*\)`)
	// Dotted versions anywhere, and standalone numbers; "7" in "7-Zip" stays.
	reVersions = regexp.MustCompile(`\b[vV]?\d+(?:[._]\d+)+\b|(?:^|\s)[vV]?\d+(?:\s|$)`)
	reArchWord = regexp.MustCompile(`(?i)\b(x64|x86|arm64|amd64|64-bit|32-bit|64bit|32bit)\b`)
	reNonAlnum = regexp.MustCompile(`[^a-z0-9]+`)
)

// familyKey reduces a display name to what identifies the product, so
// "Git 2.44 (64-bit)" and "git" compare equal.
func familyKey(name string) string {
	s := reParens.ReplaceAllString(name, " ")
	s = reArchWord.ReplaceAllString(s, " ")
	s = reVersions.ReplaceAllString(s, " ")
	return reNonAlnum.ReplaceAllString(strings.ToLower(s), "")
}

func duplicateFindings(items []core.SoftwareItem) (dups, versions []core.Finding) {
	groups := map[string][]core.SoftwareItem{}
	var order []string
	for _, it := range items {
		if it.Kind == core.KindComponent {
			continue
		}
		if _, _, rt := DetectRuntime(it); rt {
			continue // runtimes are covered by support and Dependencies
		}
		// Ubuntu's "firefox" deb is a stub that installs the snap.
		if it.Source == inventory.SourceDpkg && strings.Contains(it.Version, "snap") {
			continue
		}
		// Distro package names are exact identifiers that often carry a
		// version on purpose (clang-18, python3.12-venv); compare them as is.
		k := familyKey(it.Name)
		if distroSource(it.Source) {
			k = reNonAlnum.ReplaceAllString(strings.ToLower(it.Name), "")
		}
		if len(k) < minFamilyKeySize {
			continue
		}
		if _, ok := groups[k]; !ok {
			order = append(order, k)
		}
		groups[k] = append(groups[k], it)
	}
	for _, k := range order {
		g := groups[k]
		if len(g) < 2 {
			continue
		}
		sources := map[string]bool{}
		var refs []core.ItemRef
		var details []string
		for _, it := range g {
			sources[it.Source] = true
			r := ref(it)
			refs = append(refs, r)
			loc := it.InstallLocation
			if loc == "" {
				loc = "location unknown"
			}
			details = append(details, fmt.Sprintf("%s %s from %s (%s)", it.Name, it.Version, it.Source, loc))
		}
		name := g[0].Name
		if len(sources) >= 2 {
			dups = append(dups, core.Finding{
				Key: "duplicate:" + k, Kind: "duplicate", Severity: core.SeverityInfo,
				Title:      fmt.Sprintf("%s is installed more than once", name),
				Summary:    fmt.Sprintf("%d copies were installed by different tools.", len(g)),
				Why:        "Separate copies are updated separately and can disagree about which one runs when you type its name or open it.",
				Suggestion: "Keeping one copy is usually simpler. You can remove the one you don't use from Cleanup (6).",
				Items:      refs, Details: details,
			})
		} else {
			versions = append(versions, core.Finding{
				Key: "versions:" + k, Kind: "versions", Severity: core.SeverityInfo,
				Title:      fmt.Sprintf("Several versions of %s are installed", name),
				Summary:    fmt.Sprintf("%d versions are installed side by side.", len(g)),
				Why:        "Sometimes this is deliberate, for example to test with an older version. Often the old one is left over from an upgrade.",
				Suggestion: "If you don't need the older version, you can remove it from Cleanup (6).",
				Items:      refs, Details: details,
			})
		}
	}
	return dups, versions
}

func unusedFinding(in Inputs) (*core.Finding, int) {
	if !in.UsageAvailable {
		return nil, 0
	}
	var refs []core.ItemRef
	var details []string
	matched := 0
	for _, it := range in.Items {
		if it.Kind == core.KindComponent || it.InstallLocation == "" {
			continue
		}
		last, ok := lastOpened(it.InstallLocation, in.Usage)
		if !ok {
			continue
		}
		matched++
		if in.Now.Sub(last) > UnusedAfter {
			r := ref(it)
			r.Note = "last opened " + last.Local().Format("2 Jan 2006")
			refs = append(refs, r)
			details = append(details, it.Name+": last run recorded "+last.Format(time.RFC3339)+" under "+it.InstallLocation)
		}
	}
	if len(refs) == 0 {
		return nil, matched
	}
	return &core.Finding{
		Key: "unused", Kind: "unused", Severity: core.SeverityInfo,
		Title:      fmt.Sprintf("%d apps haven't been opened in over a year", len(refs)),
		Summary:    "Windows recorded when you last opened these, and it was more than a year ago.",
		Why:        "Unused software still takes space and may still need updates.",
		Suggestion: "If you don't need them any more, you can remove them from Cleanup (6).",
		Items:      capItems(refs), Details: details,
		Reference: "Windows UserAssist records (stay on this computer)",
	}, matched
}

func unneededFindings(runtimes []core.Runtime, items []core.SoftwareItem) []core.Finding {
	kinds := map[[2]string]core.Kind{}
	for _, it := range items {
		kinds[[2]string{it.Source, it.Name}] = it.Kind
	}
	var out []core.Finding
	for _, r := range runtimes {
		if !r.DepsKnown || len(r.Dependents) > 0 || r.Indirect > 0 || kinds[[2]string{r.Source, r.Name}] != core.KindComponent {
			continue
		}
		label := runtimeLabel(r.Family, r.Branch)
		out = append(out, core.Finding{
			Key: "unneeded:" + key(r.Source+":"+r.Name), Kind: "unneeded", Severity: core.SeverityInfo,
			Title:      fmt.Sprintf("%s (%s) may no longer be needed", label, r.Name),
			Summary:    "Nothing else installed lists it as a requirement, and it was installed automatically rather than by you.",
			Why:        "Runtimes left behind after the software that needed them was removed only take up space.",
			Suggestion: "If you don't use it directly, you can remove it from Cleanup (6), which checks again first.",
			Items:      []core.ItemRef{{Name: r.Name, Version: r.Version, Source: r.Source, PackageID: r.PackageID}},
			Details:    []string{"Dependency data: " + r.DepsSource + ". Only hard requirements (Depends/Requires) are counted; packages that merely recommend it aren't."},
		})
	}
	return out
}

func freshnessFindings(in Inputs) []core.Finding {
	var out []core.Finding
	pending := 0
	var refs []core.ItemRef
	for _, u := range in.Updates {
		if !u.Held {
			pending++
			refs = append(refs, core.ItemRef{Name: u.Name, Version: u.CurrentVersion + " → " + u.AvailableVersion, Source: u.Manager, PackageID: u.PackageID})
		}
	}
	switch {
	case in.UpdateSummary == nil:
		out = append(out, core.Finding{
			Key: "updates:never", Kind: "updates", Severity: core.SeverityInfo,
			Title:      "Updates haven't been checked yet",
			Summary:    "ATLAS doesn't know yet whether newer versions of your software exist.",
			Why:        "Newer versions fix bugs and security problems.",
			Suggestion: "Check for updates from Home or Updates (3). Checking only reads information.",
		})
	case pending > 0:
		out = append(out, core.Finding{
			Key: "updates:pending", Kind: "updates", Severity: core.SeverityAttention,
			Title:      fmt.Sprintf("%d updates are waiting", pending),
			Summary:    "Your last update check found newer versions that haven't been installed.",
			Why:        "Updates often include security fixes.",
			Suggestion: "Review them in Updates (3). Nothing installs until you confirm.",
			Items:      capItems(refs),
		})
	case in.Now.Sub(in.UpdateSummary.FinishedAt) > StaleCheckAfter:
		out = append(out, core.Finding{
			Key: "updates:stale", Kind: "updates", Severity: core.SeverityInfo,
			Title:      "Your last update check is more than two weeks old",
			Summary:    "New versions may have come out since then.",
			Why:        "Updates often include security fixes.",
			Suggestion: "Check for updates again from Home or Updates (3).",
		})
	}
	if in.Scan != nil && in.Now.Sub(in.Scan.FinishedAt) > StaleScanAfter {
		out = append(out, core.Finding{
			Key: "scan:stale", Kind: "freshness", Severity: core.SeverityInfo,
			Title:      "Your software list is more than a month old",
			Summary:    "This analysis is based on a scan from " + in.Scan.FinishedAt.Local().Format("2 Jan 2006") + ".",
			Why:        "Software installed or removed since then isn't reflected here.",
			Suggestion: "Scan again from Home, then analyze again.",
		})
	}
	return out
}

func capItems(refs []core.ItemRef) []core.ItemRef {
	if len(refs) > maxItemsListed {
		return refs[:maxItemsListed]
	}
	return refs
}
