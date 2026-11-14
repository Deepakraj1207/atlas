package updates

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/handover"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// DefaultTimeout bounds each manager's check unless overridden.
const DefaultTimeout = 60 * time.Second

// Service implements core.UpdateService.
type Service struct {
	managers []core.PackageManager
	store    core.Store
	log      *slog.Logger
	timeouts map[string]time.Duration
	now      func() time.Time
	verify   core.Verifier

	// Hooks for the terminal job; replaced in tests.
	start  handover.ProcessStarter
	notify func() (<-chan os.Signal, func())
}

var _ core.UpdateService = (*Service)(nil)

// NewService wires managers to a store. timeouts may be nil.
func NewService(managers []core.PackageManager, store core.Store, log *slog.Logger, timeouts map[string]time.Duration) *Service {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{managers: managers, store: store, log: log, timeouts: timeouts, now: time.Now, start: handover.ExecStarter, notify: handover.NotifyInterrupt}
}

// WithVerifier sets what re-reads installed versions after updates.
// Without one, no update is confirmed and the updates list is kept.
func (s *Service) WithVerifier(v core.Verifier) *Service { s.verify = v; return s }

// WithProcessStarter replaces how commands are started (tests).
func (s *Service) WithProcessStarter(p handover.ProcessStarter) *Service { s.start = p; return s }

// Managers lists the package managers in priority order.
func (s *Service) Managers() []core.SourceInfo {
	out := make([]core.SourceInfo, len(s.managers))
	for i, m := range s.managers {
		out[i] = core.SourceInfo{ID: m.ID(), Label: m.Label(), Detail: m.Detail()}
	}
	return out
}

func (s *Service) timeoutFor(id string) time.Duration {
	if d, ok := s.timeouts[id]; ok && d > 0 {
		return d
	}
	return DefaultTimeout
}

// Check asks every manager for updates in parallel.
func (s *Service) Check(ctx context.Context, progress func(core.SourceEvent)) (core.UpdateCheck, error) {
	if progress == nil {
		progress = func(core.SourceEvent) {}
	}
	s.log.Info("update check started", "managers", len(s.managers))
	check := core.UpdateCheck{StartedAt: s.now()}
	results, found := inventory.RunAll(ctx, s.Managers(), func(ctx context.Context, i int) ([]core.Update, error) {
		return s.managers[i].CheckUpdates(ctx)
	}, s.timeoutFor, progress)
	check.Sources = results
	for _, ups := range found {
		check.Updates = append(check.Updates, ups...)
	}
	check.FinishedAt = s.now()
	check.Status = inventory.OverallStatus(ctx, results)
	for _, r := range results {
		s.log.Info("update check source", "manager", r.Source, "status", string(r.Status), "count", r.Count,
			"duration_ms", r.Duration.Milliseconds(), "details", r.Details)
	}
	if !check.Status.Saved() {
		check.Updates = nil
	} else {
		s.enrich(context.WithoutCancel(ctx), check.Updates)
		slices.SortStableFunc(check.Updates, func(a, b core.Update) int {
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})
	}

	var saveErr error
	if check.Status.Saved() {
		if _, err := s.store.SaveUpdateCheck(context.WithoutCancel(ctx), check); err != nil {
			saveErr = err
			s.log.Error("save update check", "error", err)
		}
	}
	if _, err := s.store.AddActivity(context.WithoutCancel(ctx), CheckActivity(check, saveErr)); err != nil {
		s.log.Error("record update check activity", "error", err)
	}
	s.log.Info("update check finished", "status", string(check.Status), "updates", len(check.Updates))
	return check, saveErr
}

// enrich fills friendly names, current versions and the app/component split
// from the saved software list, matching by package ID within the same
// package manager.
func (s *Service) enrich(ctx context.Context, ups []core.Update) {
	items, err := s.store.Software(ctx)
	if err != nil || len(items) == 0 {
		return
	}
	type key struct{ source, id string }
	index := map[key]core.SoftwareItem{}
	for _, it := range items {
		if it.PackageID == "" {
			continue
		}
		for _, src := range append([]string{it.Source}, it.AlsoFoundIn...) {
			index[key{src, strings.ToLower(it.PackageID)}] = it
		}
	}
	for i := range ups {
		u := &ups[i]
		it, ok := index[key{inventorySource(u.Manager), strings.ToLower(u.PackageID)}]
		if !ok {
			continue
		}
		if u.Name == u.PackageID && it.Name != "" {
			u.Name = it.Name
		}
		if u.CurrentVersion == "" {
			u.CurrentVersion = it.Version
		}
		u.Component = it.Kind == core.KindComponent
	}
}

// LatestCheck passes through to the store.
func (s *Service) LatestCheck(ctx context.Context) (core.UpdateCheckSummary, []core.Update, bool, error) {
	return s.store.LatestUpdateCheck(ctx)
}

// Plan groups updates by manager, in manager order, leaving out held ones.
func (s *Service) Plan(ups []core.Update) []core.UpdateStep {
	var steps []core.UpdateStep
	for _, m := range s.managers {
		var mine []core.Update
		for _, u := range ups {
			if u.Manager == m.ID() && !u.Held {
				mine = append(mine, u)
			}
		}
		if len(mine) > 0 {
			steps = append(steps, m.PlanUpdates(mine)...)
		}
	}
	return steps
}

// NewJob prepares a terminal job. Nothing runs until the TUI calls Run.
func (s *Service) NewJob(steps []core.UpdateStep) core.TerminalJob {
	return handover.New(steps, s.start, s.notify)
}

// RecordResults writes the activity entry and removes applied updates.
func (s *Service) RecordResults(ctx context.Context, results []core.StepResult) (core.RecordOutcome, error) {
	var done []core.Update
	for _, r := range results {
		s.log.Info("update step finished", "manager", r.Step.Manager, "status", string(r.Status),
			"exit_code", r.ExitCode, "command", r.Command, "duration_ms", r.Duration.Milliseconds(), "note", r.Note)
		if r.Status == core.StepDone {
			done = append(done, r.Step.Updates...)
		}
	}
	// An exit code of 0 isn't proof: read the installed versions again and
	// drop an update from the list only when the new version is confirmed.
	var out core.RecordOutcome
	var applied []int64
	if len(done) > 0 {
		var snap *core.Snapshot
		if s.verify != nil {
			sn := s.verify.Snapshot(ctx)
			snap = &sn
		}
		for _, u := range done {
			src := inventorySource(u.Manager)
			v := inventory.NotVerified(u.Name, src)
			if snap != nil {
				v = inventory.VerifyUpdated(*snap, src, u)
			}
			s.log.Info("update verified", "name", u.Name, "manager", u.Manager, "state", string(v.State), "detail", v.Detail)
			out.Verifications = append(out.Verifications, v)
			if v.State == core.VerifyConfirmed && u.ID != 0 {
				applied = append(applied, u.ID)
			}
		}
	}
	a := ApplyActivity(results, s.now())
	inventory.AddVerification(&a, out)
	if _, err := s.store.AddActivity(ctx, a); err != nil {
		s.log.Error("record update activity", "error", err)
		return out, err
	}
	return out, s.store.RemoveUpdates(ctx, applied)
}

// CheckActivity describes a finished update check.
func CheckActivity(c core.UpdateCheck, saveErr error) core.Activity {
	a := core.Activity{At: c.FinishedAt, Kind: "updates.check"}
	n := 0
	for _, u := range c.Updates {
		if !u.Held {
			n++
		}
	}
	var failed []string
	for _, s := range c.Sources {
		if s.Status == core.SourceFailed || s.Status == core.SourceTimeout {
			failed = append(failed, s.Label)
		}
	}
	switch c.Status {
	case core.ScanComplete:
		a.Title, a.Outcome = "Checked for updates", core.OutcomeSuccess
		a.Summary = "Found " + inventory.Plural(n, "update") + "."
		if n == 0 {
			a.Summary = "Everything checked is up to date."
		}
	case core.ScanPartial:
		a.Title, a.Outcome = "Update check finished with gaps", core.OutcomePartial
		a.Summary = fmt.Sprintf("Found %s, but couldn't check: %s.", inventory.Plural(n, "update"), strings.Join(failed, ", "))
	case core.ScanFailed:
		a.Title, a.Outcome = "Update check didn't work", core.OutcomeFailed
		a.Summary = "None of the package managers could be checked. Your previous results were kept."
	case core.ScanNothingToScan:
		a.Title, a.Outcome = "No package managers to check", core.OutcomeInfo
		a.Summary = "ATLAS didn't find a package manager it supports on this computer."
	case core.ScanCancelled:
		a.Title, a.Outcome = "Update check cancelled", core.OutcomeCancelled
		a.Summary = "You stopped the check. Your previous results were kept."
	}
	if saveErr != nil {
		a.Title, a.Outcome = "Update check couldn't be saved", core.OutcomeFailed
		a.Summary = "The check worked, but its results couldn't be saved. Your previous results were kept."
	}
	var lines []string
	for _, s := range c.Sources {
		lines = append(lines, inventory.SourceLine(s))
	}
	if saveErr != nil {
		lines = append(lines, "Save error: "+saveErr.Error())
	}
	a.Details = strings.Join(lines, "\n")
	return a
}

// ApplyActivity describes an update run.
func ApplyActivity(results []core.StepResult, at time.Time) core.Activity {
	var done, failed, skipped, stopped []string
	restart := false
	for _, r := range results {
		names := updateNames(r.Step.Updates)
		switch r.Status {
		case core.StepDone:
			done = append(done, names...)
			restart = restart || r.Restart
		case core.StepFailed:
			failed = append(failed, names...)
		case core.StepSkipped:
			skipped = append(skipped, names...)
		case core.StepStopped:
			stopped = append(stopped, names...)
		}
	}
	a := core.Activity{At: at, Kind: "updates.apply"}
	switch {
	case len(done) > 0 && len(failed)+len(stopped) == 0:
		a.Title, a.Outcome = "Installed "+inventory.Plural(len(done), "update"), core.OutcomeSuccess
	case len(done) > 0:
		a.Title, a.Outcome = "Some updates didn't finish", core.OutcomePartial
	case len(stopped) > 0:
		a.Title, a.Outcome = "Updates stopped", core.OutcomeCancelled
	case len(failed) > 0:
		a.Title, a.Outcome = "Updates didn't work", core.OutcomeFailed
	default:
		a.Title, a.Outcome = "No updates could run", core.OutcomeInfo
	}
	var parts []string
	add := func(label string, names []string) {
		if len(names) > 0 {
			parts = append(parts, label+": "+listNames(names)+".")
		}
	}
	add("Updated", done)
	add("Failed", failed)
	add("Stopped before finishing", stopped)
	add("Skipped", skipped)
	if restart {
		parts = append(parts, "Restart your computer to finish some updates.")
	}
	a.Summary = strings.Join(parts, " ")

	var lines []string
	for _, r := range results {
		line := fmt.Sprintf("%s: %s", r.Step.Label, r.Status)
		if r.Duration > 0 {
			line += " in " + r.Duration.Round(100*time.Millisecond).String()
		}
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

func updateNames(ups []core.Update) []string {
	out := make([]string, 0, len(ups))
	for _, u := range ups {
		n := u.Name
		if u.CurrentVersion != "" {
			n += " " + u.CurrentVersion + " → " + u.AvailableVersion
		} else if u.AvailableVersion != "" {
			n += " → " + u.AvailableVersion
		}
		out = append(out, n)
	}
	return out
}

func listNames(names []string) string {
	const show = 5
	if len(names) <= show {
		return strings.Join(names, ", ")
	}
	return strings.Join(names[:show], ", ") + fmt.Sprintf(" and %d more", len(names)-show)
}
