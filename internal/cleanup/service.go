package cleanup

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/deepakraj1207/atlas/internal/analysis"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/handover"
	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/platform"
)

// DefaultTimeout bounds each tidier and dependency source.
const DefaultTimeout = 60 * time.Second

// Service implements core.CleanupService.
type Service struct {
	removers     map[string]Remover
	tidiers      []Tidier
	deps         []analysis.Source
	depTimeout   time.Duration
	dnfProtected string
	rpm          platform.RPMRoute
	store        core.Store
	log          *slog.Logger
	now          func() time.Time
	start        handover.ProcessStarter
	notify       func() (<-chan os.Signal, func())
}

var _ core.CleanupService = (*Service)(nil)

// Config wires a Service.
type Config struct {
	Removers []Remover
	Tidiers  []Tidier
	// Deps are dependency sources read fresh for every removal plan.
	Deps []analysis.Source
	// DnfProtectedDir is DNF's protected-packages folder (optional).
	DnfProtectedDir string
	// RPM is the tool that may remove RPM packages on this distribution
	// ("dnf" or "zypper"), or why none may. The zero value refuses.
	RPM platform.RPMRoute
}

// NewService builds the cleanup service.
func NewService(cfg Config, store core.Store, log *slog.Logger) *Service {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	s := &Service{removers: map[string]Remover{}, tidiers: cfg.Tidiers, deps: cfg.Deps, depTimeout: DefaultTimeout, dnfProtected: cfg.DnfProtectedDir, rpm: cfg.RPM,
		store: store, log: log, now: time.Now, start: handover.ExecStarter, notify: handover.NotifyInterrupt}
	for _, r := range cfg.Removers {
		s.removers[r.ID()] = r
	}
	return s
}

// WithProcessStarter replaces how commands are started (tests).
func (s *Service) WithProcessStarter(p handover.ProcessStarter) *Service { s.start = p; return s }

// Tidiers lists what FindTidyUps examines.
func (s *Service) Tidiers() []core.SourceInfo {
	out := make([]core.SourceInfo, len(s.tidiers))
	for i, t := range s.tidiers {
		out[i] = core.SourceInfo{ID: t.ID(), Label: t.Label(), Detail: t.Detail()}
	}
	return out
}

// FindTidyUps runs every tidier (read-only) and records the activity.
func (s *Service) FindTidyUps(ctx context.Context, progress func(core.SourceEvent)) (core.TidyReport, error) {
	if progress == nil {
		progress = func(core.SourceEvent) {}
	}
	r := core.TidyReport{StartedAt: s.now()}
	results, found := inventory.RunAll(ctx, s.Tidiers(), func(ctx context.Context, i int) ([]core.UpdateStep, error) {
		return s.tidiers[i].Find(ctx)
	}, func(string) time.Duration { return DefaultTimeout }, progress)
	r.Sources = results
	for _, steps := range found {
		r.Steps = append(r.Steps, steps...)
	}
	r.Status = inventory.OverallStatus(ctx, results)
	r.FinishedAt = s.now()
	if r.Status == core.ScanCancelled || r.Status == core.ScanFailed || r.Status == core.ScanNothingToScan {
		r.Steps = nil
	}
	if _, err := s.store.AddActivity(context.WithoutCancel(ctx), TidyActivity(r)); err != nil {
		s.log.Error("record tidy activity", "error", err)
	}
	s.log.Info("looked for things to tidy", "status", string(r.Status), "steps", len(r.Steps), "bytes", r.Bytes())
	return r, nil
}

// PlanRemoval checks every item against the safety rules with fresh
// dependency data and groups the allowed ones into steps. Blocked items get
// a step of their own carrying the reason.
func (s *Service) PlanRemoval(ctx context.Context, items []core.SoftwareItem) ([]core.UpdateStep, error) {
	g := gate{deps: map[string]depRecords{}, removing: map[string]bool{}, dnfProtected: map[string]bool{}}
	if s.dnfProtected != "" && s.rpm.Manager == "dnf" {
		g.dnfProtected = readDnfProtected(s.dnfProtected)
	}
	infos := make([]core.SourceInfo, len(s.deps))
	for i, d := range s.deps {
		infos[i] = core.SourceInfo{ID: d.ID(), Label: d.Label(), Detail: d.Detail()}
	}
	results, found := inventory.RunAll(ctx, infos, func(ctx context.Context, i int) ([]analysis.Input, error) {
		in, err := s.deps[i].Read(ctx)
		if err != nil {
			return nil, err
		}
		return []analysis.Input{in}, nil
	}, func(string) time.Duration { return s.depTimeout }, func(core.SourceEvent) {})
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// Keep every source's outcome, not just the graphs that arrived: a
	// source that failed or timed out must not look like one that keeps no
	// records (the gate then refuses rather than guesses).
	for i, d := range s.deps {
		ds, ok := d.(analysis.DependencySource)
		if !ok {
			continue
		}
		rec := depRecords{result: results[i]}
		for _, in := range found[i] {
			if in.GraphFor == ds.Covers() {
				rec.graph, rec.read = in.Graph, true
			}
		}
		if results[i].Status == core.SourceOK && !rec.read {
			rec.result.Status, rec.result.Message = core.SourceFailed, "returned no dependency records"
		}
		g.deps[ds.Covers()] = rec
		s.log.Info("dependency source read for removal", "source", d.ID(), "status", string(rec.result.Status), "packages", len(rec.graph.Nodes))
	}
	for _, it := range items {
		for _, src := range append([]string{it.Source}, it.AlsoFoundIn...) {
			g.removing[itemKey(src, analysis.GraphKey(src, it))] = true
		}
	}

	groups := map[string][]core.SoftwareItem{}
	var order []string
	var steps []core.UpdateStep
	blockedStep := func(it core.SoftwareItem, manager, reason string) {
		label := "Not removable"
		if r, ok := s.removers[manager]; ok {
			label = r.Label()
		}
		steps = append(steps, core.UpdateStep{Manager: manager, Label: label, Action: core.ActionRemove,
			Updates: []core.Update{target(manager, it)}, Blocked: reason})
	}
	for _, it := range items {
		manager, why := removerFor(it, s.rpm)
		// Safety reasons come first: they matter more than which tool would run.
		if reason, _ := g.check(it); reason != "" {
			blockedStep(it, manager, reason)
			continue
		}
		if why != "" {
			blockedStep(it, manager, why)
			continue
		}
		if _, ok := s.removers[manager]; !ok {
			blockedStep(it, manager, "Removing software with "+manager+" isn't available on this computer.")
			continue
		}
		if _, ok := groups[manager]; !ok {
			order = append(order, manager)
		}
		groups[manager] = append(groups[manager], it)
	}
	var allowed []core.UpdateStep
	for _, m := range order {
		allowed = append(allowed, s.removers[m].PlanRemove(groups[m])...)
	}
	s.log.Info("removal planned", "items", len(items), "steps", len(allowed), "blocked", len(steps))
	return append(allowed, steps...), nil
}

// NewJob prepares a terminal job. Nothing runs until the TUI calls Run.
func (s *Service) NewJob(steps []core.UpdateStep) core.TerminalJob {
	return handover.New(steps, s.start, s.notify)
}

// RecordResults records the outcome and drops removed software from the
// saved list.
func (s *Service) RecordResults(ctx context.Context, results []core.StepResult) error {
	var gone []core.ItemRef
	for _, r := range results {
		s.log.Info("cleanup step finished", "manager", r.Step.Manager, "action", r.Step.Action, "status", string(r.Status),
			"exit_code", r.ExitCode, "command", r.Command, "duration_ms", r.Duration.Milliseconds())
		if r.Status != core.StepDone || r.Step.Action != core.ActionRemove {
			continue
		}
		for _, u := range r.Step.Updates {
			if u.Origin != "" {
				gone = append(gone, core.ItemRef{Name: u.Name, Version: u.CurrentVersion, Source: u.Origin})
			}
		}
	}
	if _, err := s.store.AddActivity(ctx, Activity(results, s.now())); err != nil {
		return err
	}
	return s.store.DeleteSoftware(ctx, gone)
}

// HumanBytes renders a size for people.
func HumanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}

// TidyActivity describes a search for things to tidy.
func TidyActivity(r core.TidyReport) core.Activity {
	a := core.Activity{At: r.FinishedAt, Kind: "cleanup.find", Title: "Looked for things to tidy", Outcome: core.OutcomeSuccess}
	switch {
	case r.Status == core.ScanCancelled:
		a.Title, a.Outcome, a.Summary = "Search for things to tidy stopped", core.OutcomeCancelled, "You stopped the search."
	case r.Status == core.ScanFailed:
		a.Outcome, a.Summary = core.OutcomeFailed, "None of the places ATLAS checks could be read."
	case len(r.Steps) == 0:
		a.Summary = "Nothing to tidy right now."
	default:
		a.Summary = fmt.Sprintf("Found %s to tidy", inventory.Plural(len(r.Steps), "thing"))
		if b := r.Bytes(); b > 0 {
			a.Summary += ", freeing about " + HumanBytes(b)
		}
		a.Summary += ". Nothing was changed."
		if r.Status == core.ScanPartial {
			a.Outcome = core.OutcomePartial
		}
	}
	var lines []string
	for _, src := range r.Sources {
		lines = append(lines, inventory.SourceLine(src))
	}
	a.Details = strings.Join(lines, "\n")
	return a
}

// Activity describes a removal or clean-up run.
func Activity(results []core.StepResult, at time.Time) core.Activity {
	var done, failed, stopped, skipped []string
	var freed int64
	restart := false
	removals := 0
	for _, r := range results {
		var names []string
		for _, u := range r.Step.Updates {
			names = append(names, u.Name)
		}
		if r.Step.Action == core.ActionRemove {
			removals++
		}
		switch r.Status {
		case core.StepDone:
			done = append(done, names...)
			freed += r.Step.Bytes
			restart = restart || r.Restart
		case core.StepFailed:
			failed = append(failed, names...)
		case core.StepStopped:
			stopped = append(stopped, names...)
		case core.StepSkipped:
			skipped = append(skipped, names...)
		}
	}
	a := core.Activity{At: at, Kind: "cleanup.run"}
	noun := "Cleanup"
	if removals == len(results) {
		noun = "Removal"
	}
	switch {
	case len(done) > 0 && len(failed)+len(stopped) == 0:
		a.Outcome = core.OutcomeSuccess
		a.Title = noun + " finished"
		if noun == "Removal" {
			a.Title = "Removed " + inventory.Plural(len(done), "item")
		}
	case len(done) > 0:
		a.Title, a.Outcome = noun+" partly finished", core.OutcomePartial
	case len(stopped) > 0:
		a.Title, a.Outcome = noun+" stopped", core.OutcomeCancelled
	case len(failed) > 0:
		a.Title, a.Outcome = noun+" didn't work", core.OutcomeFailed
	default:
		a.Title, a.Outcome = "Nothing could be removed", core.OutcomeInfo
	}
	var parts []string
	add := func(label string, names []string) {
		if len(names) == 0 {
			return
		}
		shown := names
		more := ""
		if len(shown) > 5 {
			shown, more = shown[:5], fmt.Sprintf(" and %d more", len(names)-5)
		}
		parts = append(parts, label+": "+strings.Join(shown, ", ")+more+".")
	}
	add("Done", done)
	add("Didn't work", failed)
	add("Stopped before finishing", stopped)
	add("Not removed (blocked)", skipped)
	if freed > 0 {
		parts = append(parts, "Freed about "+HumanBytes(freed)+".")
	}
	if restart {
		parts = append(parts, "Restart your computer to finish.")
	}
	a.Summary = strings.Join(parts, " ")
	var lines []string
	for _, r := range results {
		line := fmt.Sprintf("%s: %s", r.Step.Label, r.Status)
		if r.Status == core.StepFailed {
			line += fmt.Sprintf(" (exit code %s)", core.FormatExitCode(r.ExitCode))
		}
		if r.Command != "" {
			line += " — " + r.Command
		}
		if r.Note != "" {
			line += " — " + r.Note
		}
		lines = append(lines, line)
	}
	a.Details = strings.Join(lines, "\n")
	return a
}
