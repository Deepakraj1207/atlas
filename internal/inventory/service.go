package inventory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
)

// DefaultTimeout bounds each scanner unless overridden.
const DefaultTimeout = 45 * time.Second

// Service implements core.InventoryService.
type Service struct {
	scanners []core.Scanner
	store    core.Store
	log      *slog.Logger
	timeouts map[string]time.Duration
	now      func() time.Time
}

var _ core.InventoryService = (*Service)(nil)

// NewService wires scanners to a store. timeouts may be nil.
func NewService(scanners []core.Scanner, store core.Store, log *slog.Logger, timeouts map[string]time.Duration) *Service {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{scanners: scanners, store: store, log: log, timeouts: timeouts, now: time.Now}
}

// Sources lists the scanners in priority order.
func (s *Service) Sources() []core.SourceInfo {
	out := make([]core.SourceInfo, len(s.scanners))
	for i, sc := range s.scanners {
		out[i] = core.SourceInfo{ID: sc.ID(), Label: sc.Label(), Detail: sc.Detail()}
	}
	return out
}

func (s *Service) timeoutFor(id string) time.Duration {
	if d, ok := s.timeouts[id]; ok && d > 0 {
		return d
	}
	return DefaultTimeout
}

// Scan runs all scanners, saves the result if it is complete or partial,
// and records an activity entry in every case.
func (s *Service) Scan(ctx context.Context, progress func(core.SourceEvent)) (core.ScanReport, error) {
	if progress == nil {
		progress = func(core.SourceEvent) {}
	}
	s.log.Info("scan started", "sources", len(s.scanners))
	report := RunScanners(ctx, s.scanners, s.timeoutFor, progress, s.now)
	for _, src := range report.Sources {
		s.log.Info("scan source finished", "source", src.Source, "status", string(src.Status),
			"count", src.Count, "duration_ms", src.Duration.Milliseconds(), "details", src.Details)
	}

	var saveErr error
	if report.Status.Saved() {
		// Saving must finish even if the user cancels at the last moment.
		if _, err := s.store.SaveScan(context.WithoutCancel(ctx), report); err != nil {
			saveErr = err
			s.log.Error("save scan", "error", err)
		}
	}
	apps, comps := report.Counts()
	s.log.Info("scan finished", "status", string(report.Status), "apps", apps, "components", comps, "saved", report.Status.Saved() && saveErr == nil)

	if _, err := s.store.AddActivity(context.WithoutCancel(ctx), ScanActivity(report, saveErr)); err != nil {
		s.log.Error("record scan activity", "error", err)
	}
	return report, saveErr
}

// ClearInventory deletes the saved list and records that it happened.
func (s *Service) ClearInventory(ctx context.Context) error {
	sum, had, _ := s.store.LatestScan(ctx)
	err := s.store.ClearInventory(ctx)
	a := core.Activity{At: s.now(), Kind: "inventory.clear", Title: "Cleared saved software list"}
	if err != nil {
		a.Outcome = core.OutcomeFailed
		a.Title = "Couldn't clear saved software list"
		a.Summary = "The saved list could not be deleted. Nothing else was affected."
		a.Details = err.Error()
	} else {
		a.Outcome = core.OutcomeSuccess
		a.Summary = "ATLAS deleted its own saved list. No software on this computer was touched. Run a scan to rebuild it."
		if had {
			a.Details = fmt.Sprintf("Removed %d apps and %d components from the database.", sum.AppCount, sum.ComponentCount)
		}
	}
	s.log.Info("clear inventory", "ok", err == nil)
	if _, aerr := s.store.AddActivity(context.WithoutCancel(ctx), a); aerr != nil {
		s.log.Error("record clear activity", "error", aerr)
	}
	return err
}

// LatestScan passes through to the store.
func (s *Service) LatestScan(ctx context.Context) (core.ScanSummary, bool, error) {
	return s.store.LatestScan(ctx)
}

// Software passes through to the store.
func (s *Service) Software(ctx context.Context) ([]core.SoftwareItem, error) {
	return s.store.Software(ctx)
}

// RecentActivity passes through to the store.
func (s *Service) RecentActivity(ctx context.Context, limit int) ([]core.Activity, error) {
	return s.store.RecentActivity(ctx, limit)
}

// RunScanners runs every scanner concurrently, each with its own timeout,
// and returns the merged report.
func RunScanners(ctx context.Context, scanners []core.Scanner, timeoutFor func(string) time.Duration,
	progress func(core.SourceEvent), now func() time.Time) core.ScanReport {

	report := core.ScanReport{StartedAt: now()}
	infos := make([]core.SourceInfo, len(scanners))
	for i, sc := range scanners {
		infos[i] = core.SourceInfo{ID: sc.ID(), Label: sc.Label(), Detail: sc.Detail()}
	}
	results, found := RunAll(ctx, infos, func(ctx context.Context, i int) ([]core.SoftwareItem, error) {
		return scanners[i].Scan(ctx)
	}, timeoutFor, progress)

	report.Sources = results
	var all []core.SoftwareItem
	for _, items := range found {
		all = append(all, items...)
	}
	report.Items = Merge(all)
	report.FinishedAt = now()
	report.Status = OverallStatus(ctx, results)
	if !report.Status.Saved() {
		report.Items = nil
	}
	return report
}

// RunAll runs one read-only job per source concurrently, each with its own
// timeout, and reports progress. Results keep source order so callers are
// deterministic. A panicking job is reported as failed, not fatal. It is
// shared by inventory scans and update checks.
func RunAll[T any](ctx context.Context, sources []core.SourceInfo, job func(context.Context, int) ([]T, error),
	timeoutFor func(string) time.Duration, progress func(core.SourceEvent)) ([]core.SourceResult, [][]T) {

	results := make([]core.SourceResult, len(sources))
	found := make([][]T, len(sources))
	var mu sync.Mutex // serialises progress callbacks
	emit := func(ev core.SourceEvent) {
		mu.Lock()
		defer mu.Unlock()
		progress(ev)
	}

	var wg sync.WaitGroup
	for i, src := range sources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			emit(core.SourceEvent{Source: src.ID})
			start := time.Now()
			timeout := timeoutFor(src.ID)
			sctx, cancel := context.WithTimeout(ctx, timeout)
			items, err := safeRun(sctx, i, job)
			cancel()

			res := core.SourceResult{Source: src.ID, Label: src.Label, Duration: time.Since(start)}
			switch {
			case ctx.Err() != nil:
				res.Status, res.Message = core.SourceCancelled, "Stopped"
			case err == nil:
				res.Status, res.Count = core.SourceOK, len(items)
				found[i] = items
			case errors.Is(err, core.ErrSourceUnavailable):
				res.Status, res.Message = core.SourceUnavailable, "Not on this computer"
			case errors.Is(err, context.DeadlineExceeded) || sctx.Err() == context.DeadlineExceeded:
				res.Status = core.SourceTimeout
				res.Message = fmt.Sprintf("Took longer than %s and was stopped", timeout.Round(time.Second))
			default:
				res.Status, res.Message, res.Details = core.SourceFailed, "Couldn't be read", err.Error()
				var se *core.SourceError
				if errors.As(err, &se) {
					res.Message, res.Details = se.Message, se.Details
				}
			}
			results[i] = res
			emit(core.SourceEvent{Source: src.ID, Result: &res})
		}()
	}
	wg.Wait()
	return results, found
}

func safeRun[T any](ctx context.Context, i int, job func(context.Context, int) ([]T, error)) (items []T, err error) {
	defer func() {
		if r := recover(); r != nil {
			items, err = nil, fmt.Errorf("unexpected internal problem: %v", r)
		}
	}()
	return job(ctx, i)
}

// OverallStatus summarises per-source results.
func OverallStatus(ctx context.Context, results []core.SourceResult) core.ScanStatus {
	if ctx.Err() != nil {
		return core.ScanCancelled
	}
	present, ok := 0, 0
	for _, r := range results {
		if r.Status == core.SourceUnavailable {
			continue
		}
		present++
		if r.Status == core.SourceOK {
			ok++
		}
	}
	switch {
	case present == 0:
		return core.ScanNothingToScan
	case ok == present:
		return core.ScanComplete
	case ok > 0:
		return core.ScanPartial
	default:
		return core.ScanFailed
	}
}

// ScanActivity describes a finished scan for the Activity History.
func ScanActivity(r core.ScanReport, saveErr error) core.Activity {
	apps, comps := r.Counts()
	a := core.Activity{At: r.FinishedAt, Kind: "scan"}
	var failed []string
	for _, s := range r.Sources {
		if s.Status == core.SourceFailed || s.Status == core.SourceTimeout {
			failed = append(failed, s.Label)
		}
	}
	switch r.Status {
	case core.ScanComplete:
		a.Title, a.Outcome = "Scanned your system", core.OutcomeSuccess
		a.Summary = fmt.Sprintf("Found %s and %s.", plural(apps, "app"), plural(comps, "system component"))
	case core.ScanPartial:
		a.Title, a.Outcome = "Scan finished with gaps", core.OutcomePartial
		a.Summary = fmt.Sprintf("Found %s, but couldn't read: %s. The list may be incomplete.", plural(apps, "app"), strings.Join(failed, ", "))
	case core.ScanFailed:
		a.Title, a.Outcome = "Scan didn't work", core.OutcomeFailed
		a.Summary = "None of the software sources could be read. Your previous list was kept."
	case core.ScanNothingToScan:
		a.Title, a.Outcome = "Nothing to scan", core.OutcomeInfo
		a.Summary = "ATLAS didn't find any software sources it supports on this computer yet."
	case core.ScanCancelled:
		a.Title, a.Outcome = "Scan cancelled", core.OutcomeCancelled
		a.Summary = "You stopped the scan. Your previous list was kept."
	}
	if saveErr != nil {
		a.Title, a.Outcome = "Scan results couldn't be saved", core.OutcomeFailed
		a.Summary = "The scan worked, but the results couldn't be saved. Your previous list was kept."
	}

	var lines []string
	for _, s := range r.Sources {
		lines = append(lines, SourceLine(s))
	}
	if saveErr != nil {
		lines = append(lines, "Save error: "+saveErr.Error())
	}
	a.Details = strings.Join(lines, "\n")
	return a
}

// SourceLine is a one-line technical description of a source result.
func SourceLine(s core.SourceResult) string {
	switch s.Status {
	case core.SourceOK:
		return fmt.Sprintf("%s: %d found in %s", s.Label, s.Count, s.Duration.Round(10*time.Millisecond))
	case core.SourceFailed:
		msg := "couldn't be read"
		if s.Message != "" {
			msg = strings.ToLower(s.Message[:1]) + s.Message[1:]
		}
		if s.Details != "" {
			return fmt.Sprintf("%s: %s (%s)", s.Label, msg, s.Details)
		}
		return s.Label + ": " + msg
	default:
		if s.Message == "" {
			return s.Label + ": " + string(s.Status)
		}
		return s.Label + ": " + strings.ToLower(s.Message[:1]) + s.Message[1:]
	}
}

// Plural formats a count with a simple English plural.
func Plural(n int, word string) string { return plural(n, word) }

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}
