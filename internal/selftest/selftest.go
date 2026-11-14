// Package selftest runs every read-only part of ATLAS against the real
// computer and writes a plain report. It exists to verify ATLAS on machines
// the developers can't drive interactively (for example a Windows PC, or a
// CI runner).
//
// It never changes the system and never touches the user's own ATLAS data:
// it uses a throwaway database in a temporary folder that is deleted
// afterwards. Removals and installs are planned (to exercise the safety
// checks) but never run.
package selftest

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/deepakraj1207/atlas/internal/analysis"
	"github.com/deepakraj1207/atlas/internal/cleanup"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/database"
	"github.com/deepakraj1207/atlas/internal/discover"
	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/platform"
	"github.com/deepakraj1207/atlas/internal/updates"
)

// Result summarises a run.
type Result struct {
	Passed, Warnings, Failed int
}

type report struct {
	w   io.Writer
	res Result
}

func (r *report) printf(format string, args ...any) { fmt.Fprintf(r.w, format, args...) }

// mark classifies a source result: unavailable is informational, failures
// and timeouts are failures.
func (r *report) mark(s core.SourceResult) string {
	switch s.Status {
	case core.SourceOK:
		r.res.Passed++
		return "PASS"
	case core.SourceUnavailable:
		return "n/a"
	default:
		r.res.Failed++
		return "FAIL"
	}
}

func (r *report) sources(title string, results []core.SourceResult) {
	r.printf("\n## %s\n\n| Result | Source | Status | Found | Time | Details |\n|---|---|---|---|---|---|\n", title)
	for _, s := range results {
		details := s.Message
		if s.Details != "" {
			details += ": " + s.Details
		}
		r.printf("| %s | %s | %s | %d | %s | %s |\n", r.mark(s), s.Label, s.Status, s.Count, s.Duration.Round(time.Millisecond), cell(details))
	}
}

func cell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 160 {
		s = s[:160] + "…"
	}
	return s
}

func (r *report) check(ok bool, warnOnly bool, text string) {
	switch {
	case ok:
		r.res.Passed++
		r.printf("- PASS: %s\n", text)
	case warnOnly:
		r.res.Warnings++
		r.printf("- WARN: %s\n", text)
	default:
		r.res.Failed++
		r.printf("- FAIL: %s\n", text)
	}
}

// Run executes the self-test and writes a Markdown report to w.
func Run(ctx context.Context, w io.Writer, version string) (Result, error) {
	r := &report{w: w}
	tmp, err := os.MkdirTemp("", "atlas-selftest-")
	if err != nil {
		return r.res, err
	}
	defer os.RemoveAll(tmp)
	db, err := database.Open(ctx, filepath.Join(tmp, "selftest.db"))
	if err != nil {
		return r.res, fmt.Errorf("open throwaway database: %w", err)
	}
	defer db.Close()
	elevated := platform.IsElevated()

	r.printf("# ATLAS self-test\n\nRead-only. Nothing on this computer was changed, and your own ATLAS data was not used.\n\n")
	r.printf("| | |\n|---|---|\n| Version | %s |\n| Date | %s |\n| Platform | %s/%s |\n| Administrator/root | %v |\n| Terminal | %s |\n",
		version, time.Now().Format(time.RFC3339), runtime.GOOS, runtime.GOARCH, elevated, cell(terminalInfo()))
	if runtime.GOOS == "linux" {
		distro := "unknown (no /etc/os-release)"
		if d, err := platform.ReadOSRelease(); err == nil {
			distro = strings.TrimSpace(d.Name + " (" + d.ID + ")")
		}
		route := platform.LinuxRPMRoute()
		rpm := route.Manager
		if rpm == "" {
			rpm = "not changed: " + route.Reason
		}
		r.printf("| Linux distribution | %s |\n| RPM packages changed with | %s |\n", cell(distro), cell(rpm))
	}
	for _, p := range []struct {
		name string
		fn   func() (string, error)
	}{{"Settings folder", platform.ConfigDir}, {"Data folder", platform.DataDir}, {"Log folder", platform.LogDir}} {
		dir, err := p.fn()
		if err != nil {
			dir = "error: " + err.Error()
		}
		r.printf("| %s | %s |\n", p.name, cell(dir))
	}

	// 1. Inventory
	inv := inventory.NewService(inventory.DefaultScanners(), db, nil, inventory.DefaultTimeouts())
	scan, err := inv.Scan(ctx, nil)
	if err != nil {
		return r.res, fmt.Errorf("save scan: %w", err)
	}
	r.sources("Inventory sources", scan.Sources)
	apps, comps := scan.Counts()
	r.printf("\nOverall: %s, %d apps and %d system components after merging.\n\n", scan.Status, apps, comps)
	r.check(scan.Status == core.ScanComplete || scan.Status == core.ScanPartial, false, "the scan produced a software list")
	r.check(apps > 0, true, fmt.Sprintf("found at least one app (%d)", apps))
	empty := 0
	for _, it := range scan.Items {
		if strings.TrimSpace(it.Name) == "" {
			empty++
		}
	}
	r.check(empty == 0, false, fmt.Sprintf("every item has a name (%d without)", empty))
	r.printf("\nSample apps by source (first 5 each, to check names and versions parse correctly):\n\n")
	seen := map[string]int{}
	for _, it := range scan.Items {
		if it.Kind == core.KindApp && seen[it.Source] < 5 {
			seen[it.Source]++
			id := ""
			if it.PackageID != "" {
				id = " [" + it.PackageID + "]"
			}
			also := ""
			if len(it.AlsoFoundIn) > 0 {
				also = " (also: " + strings.Join(it.AlsoFoundIn, ", ") + ")"
			}
			r.printf("- %s: %s%s%s\n", it.Source, core.NameAndVersion(it.Name, it.Version), id, also)
		}
	}

	// 2. Updates
	upd := updates.NewService(updates.DefaultManagers(elevated), db, nil, updates.DefaultTimeouts())
	check, err := upd.Check(ctx, nil)
	if err != nil {
		return r.res, fmt.Errorf("save update check: %w", err)
	}
	r.sources("Update checks", check.Sources)
	r.printf("\nOverall: %s, %d updates.\n\n", check.Status, len(check.Updates))
	for i, u := range check.Updates {
		if i == 5 {
			r.printf("- …and %d more\n", len(check.Updates)-5)
			break
		}
		r.printf("- %s: %s [%s]%s\n", u.Manager, core.NameAndVersion(u.Name, strings.TrimSpace(u.CurrentVersion+" → "+u.AvailableVersion)), u.PackageID, map[bool]string{true: " (held)"}[u.Held])
	}
	if len(check.Updates) > 0 {
		steps := upd.Plan(check.Updates[:1])
		for _, st := range steps {
			r.printf("\nPlanned (not run) for the first update: `%s` — %s%s\n", commands(st), elevationPhrase(st.Elevation), blocked(st))
		}
	}

	// 3. Analysis
	ana := analysis.NewService(analysis.DefaultSources(), db, nil, analysis.UsageSupported)
	res, err := ana.Analyze(ctx, nil)
	if err != nil {
		return r.res, fmt.Errorf("analysis: %w", err)
	}
	r.sources("Analysis data sources", res.Sources)
	att, info := res.Counts()
	r.printf("\nOverall: %s, %d worth attention, %d worth knowing, %d runtimes.\n\n", res.Status, att, info, len(res.Runtimes))
	for _, rt := range res.Runtimes {
		deps := "not recorded"
		if rt.DepsKnown {
			deps = fmt.Sprintf("needed by %d direct + %d indirect", len(rt.Dependents), rt.Indirect)
		}
		r.printf("- Runtime %s %s: %s — support %s — %s\n", rt.Family, rt.Branch, core.NameAndVersion(rt.Name, rt.Version), strings.TrimSpace(string(rt.Support.State)+" "+rt.Support.EndDate), deps)
	}
	for _, f := range res.Findings {
		r.printf("- Finding [%s] %s (%d items)\n", f.Severity, f.Title, len(f.Items))
	}
	for _, c := range res.Checks {
		if !c.Available {
			r.printf("- Not checked: %s — %s\n", c.Name, c.Note)
		}
	}

	// 4. Cleanup
	cln := cleanup.NewService(cleanup.DefaultConfig(elevated), db, nil)
	tidy, _ := cln.FindTidyUps(ctx, nil)
	r.sources("Clean-up finders (nothing deleted)", tidy.Sources)
	for _, st := range tidy.Steps {
		r.printf("- Would offer: %s · %d targets · %s · `%s`%s\n", st.Label, len(st.Updates), cleanup.HumanBytes(st.Bytes), commands(st), blocked(st))
	}
	var sample []core.SoftwareItem
	for _, it := range scan.Items {
		if _, _, isRT := analysis.DetectRuntime(it); isRT && len(sample) < 2 {
			sample = append(sample, it)
		}
	}
	for _, it := range scan.Items {
		if it.Kind == core.KindApp && len(sample) < 5 {
			if _, _, isRT := analysis.DetectRuntime(it); !isRT {
				sample = append(sample, it)
			}
		}
	}
	r.printf("\nRemoval safety check on sample items (planned only, never run):\n\n")
	steps, err := cln.PlanRemoval(ctx, sample)
	r.check(err == nil, false, "removal planning works")
	// One line per planned step: a package manager removes several items in
	// one command, so listing that command against each item would read as
	// though removing one item also removed the others.
	for _, st := range steps {
		names := make([]string, 0, len(st.Updates))
		for _, u := range st.Updates {
			names = append(names, u.Name)
		}
		switch {
		case st.Blocked != "":
			r.printf("- %s: blocked — %s\n", strings.Join(names, ", "), st.Blocked)
		case len(names) > 1:
			r.printf("- %s: allowed, removed together in one step — `%s`\n", strings.Join(names, ", "), commands(st))
		default:
			r.printf("- %s: allowed — `%s`\n", strings.Join(names, ", "), commands(st))
		}
	}

	// 5. Discover
	r.printf("\n## Discover Apps\n\n")
	dsc, err := discover.NewService(discover.DefaultConfig(elevated), db, nil)
	r.check(err == nil, false, "the built-in catalogue loads and validates")
	if err == nil {
		d, err := dsc.Discover(ctx)
		r.check(err == nil, false, "the catalogue can be assessed for this computer")
		installed, installable, suggested := 0, 0, 0
		via := map[string]int{}
		for _, it := range d.Items {
			switch {
			case it.Installed:
				installed++
				r.printf("- Recognised as installed: %s (as %s)\n", it.Entry.Name, it.InstalledAs)
			case it.Via != "":
				installable++
				via[it.Via]++
			}
			if len(it.Because) > 0 {
				suggested++
			}
		}
		r.printf("\n%d entries for this platform: %d installed, %d installable%s, %d suggested. Package managers here: %s.\n",
			len(d.Items), installed, installable, viaPhrase(via), suggested, strings.Join(d.ManagersHere, ", "))
	}

	r.printf("\n## Summary\n\n%d passed, %d warnings, %d failed.\n", r.res.Passed, r.res.Warnings, r.res.Failed)
	if r.res.Failed > 0 {
		r.printf("\nA failure means a tool exists here but ATLAS couldn't read it. The Details column says why.\n")
	}
	return r.res, nil
}

func commands(st core.UpdateStep) string {
	var parts []string
	for _, c := range st.Commands {
		parts = append(parts, c.String())
	}
	return strings.Join(parts, " && ")
}

func blocked(st core.UpdateStep) string {
	if st.Blocked == "" {
		return ""
	}
	return " — blocked: " + st.Blocked
}

// terminalInfo names the terminal from environment hints, without querying it.
func terminalInfo() string {
	var parts []string
	switch {
	case os.Getenv("WT_SESSION") != "":
		parts = append(parts, "Windows Terminal")
	case os.Getenv("TERM_PROGRAM") != "":
		parts = append(parts, os.Getenv("TERM_PROGRAM"))
	case runtime.GOOS == "windows" && os.Getenv("TERM") == "":
		parts = append(parts, "Windows console (conhost or unknown)")
	}
	if t := os.Getenv("TERM"); t != "" {
		parts = append(parts, "TERM="+t)
	}
	if os.Getenv("CI") != "" {
		parts = append(parts, "CI")
	}
	if len(parts) == 0 {
		return "unknown"
	}
	return strings.Join(parts, ", ")
}

// viaPhrase turns {"winget": 22} into " (22 via winget)" for the report.
func viaPhrase(via map[string]int) string {
	if len(via) == 0 {
		return ""
	}
	keys := make([]string, 0, len(via))
	for k := range via {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%d via %s", via[k], k))
	}
	return " (" + strings.Join(parts, ", ") + ")"
}

// elevationPhrase says in plain words how a step gets the rights it needs.
func elevationPhrase(e core.Elevation) string {
	switch e {
	case core.ElevationSudo:
		return "runs through sudo, which asks for your password"
	case core.ElevationInstaller:
		return "Windows may ask for permission if the installer needs it"
	case core.ElevationPolkit:
		return "your system may ask for your password"
	case core.ElevationAdmin:
		return "ATLAS is running as administrator"
	default:
		return "runs with your normal rights"
	}
}
