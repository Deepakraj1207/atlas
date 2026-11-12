package analysis

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// ErrNoScan is returned when there is no saved software list to analyze.
var ErrNoScan = errors.New("there is no saved software list yet; scan your system first")

// DefaultTimeout bounds each analysis source.
const DefaultTimeout = 30 * time.Second

// Store is what the service needs from the database.
type Store interface {
	core.Store
	core.AnalysisStore
}

// Service implements core.AnalysisService.
type Service struct {
	sources        []Source
	store          Store
	log            *slog.Logger
	now            func() time.Time
	usageSupported bool
}

var _ core.AnalysisService = (*Service)(nil)

// NewService wires sources to the store. usageSupported says whether this
// platform can record program use at all.
func NewService(sources []Source, store Store, log *slog.Logger, usageSupported bool) *Service {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{sources: sources, store: store, log: log, now: time.Now, usageSupported: usageSupported}
}

// WithClock replaces the clock (tests).
func (s *Service) WithClock(now func() time.Time) *Service { s.now = now; return s }

// Sources lists the extra data sources.
func (s *Service) Sources() []core.SourceInfo {
	out := make([]core.SourceInfo, len(s.sources))
	for i, src := range s.sources {
		out[i] = core.SourceInfo{ID: src.ID(), Label: src.Label(), Detail: src.Detail()}
	}
	return out
}

// Latest returns the saved analysis.
func (s *Service) Latest(ctx context.Context) (core.AnalysisResult, bool, error) {
	return s.store.LatestAnalysis(ctx)
}

// Analyze reads the extra sources and applies the rules.
func (s *Service) Analyze(ctx context.Context, progress func(core.SourceEvent)) (core.AnalysisResult, error) {
	if progress == nil {
		progress = func(core.SourceEvent) {}
	}
	res := core.AnalysisResult{StartedAt: s.now(), SupportDataAsOf: SupportDataAsOf}
	// Reading ATLAS's own database is quick and must not be interrupted;
	// stopping applies to the external sources below.
	dbCtx := context.WithoutCancel(ctx)
	scan, ok, err := s.store.LatestScan(dbCtx)
	if err != nil {
		return res, err
	}
	if !ok {
		return res, ErrNoScan
	}
	items, err := s.store.Software(dbCtx)
	if err != nil {
		return res, err
	}
	in := Inputs{Items: items, Scan: &scan, Graphs: map[string]Graph{}, GraphLabels: map[string]string{}}
	if us, ups, ok, err := s.store.LatestUpdateCheck(dbCtx); err == nil && ok {
		in.UpdateSummary, in.Updates = &us, ups
	}

	s.log.Info("analysis started", "items", len(items), "sources", len(s.sources))
	results, found := inventory.RunAll(ctx, s.Sources(), func(ctx context.Context, i int) ([]Input, error) {
		in, err := s.sources[i].Read(ctx)
		if err != nil {
			return nil, err
		}
		return []Input{in}, nil
	}, func(string) time.Duration { return DefaultTimeout }, progress)
	res.Sources = results

	in.UsageNote = "Linux doesn't keep a record of when programs are opened."
	if s.usageSupported {
		in.UsageNote = "Windows' record of opened programs couldn't be read."
	}
	for i, r := range results {
		for _, f := range found[i] {
			if f.IsUsage {
				in.UsageAvailable, in.Usage = true, f.Usage
				continue
			}
			if f.GraphFor != "" {
				in.Graphs[f.GraphFor] = f.Graph
				in.GraphLabels[f.GraphFor] = r.Label
			}
		}
		// Explain usage failures precisely.
		if s.usageSupported && r.Source == "usage" && r.Status != core.SourceOK {
			in.UsageNote = "Windows' record of opened programs couldn't be read: " + strings.ToLower(r.Message) + "."
		}
	}

	res.Status = inventory.OverallStatus(ctx, results)
	switch res.Status {
	case core.ScanCancelled:
		res.FinishedAt = s.now()
		s.record(ctx, res, nil)
		return res, nil
	case core.ScanNothingToScan:
		// The rules still run on the saved list; extra sources are optional.
		res.Status = core.ScanComplete
	case core.ScanFailed:
		res.Status = core.ScanPartial
	}

	in.Now = s.now()
	res.Findings, res.Runtimes, res.Checks = Analyze(in)
	res.BasedOnScan = scan.FinishedAt
	res.FinishedAt = s.now()

	var saveErr error
	if err := s.store.SaveAnalysis(context.WithoutCancel(ctx), res); err != nil {
		saveErr = err
		s.log.Error("save analysis", "error", err)
	}
	s.record(ctx, res, saveErr)
	att, info := res.Counts()
	s.log.Info("analysis finished", "status", string(res.Status), "attention", att, "info", info, "runtimes", len(res.Runtimes))
	return res, saveErr
}

func (s *Service) record(ctx context.Context, r core.AnalysisResult, saveErr error) {
	if _, err := s.store.AddActivity(context.WithoutCancel(ctx), Activity(r, saveErr)); err != nil {
		s.log.Error("record analysis activity", "error", err)
	}
}

// Activity describes an analysis for the Activity History.
func Activity(r core.AnalysisResult, saveErr error) core.Activity {
	a := core.Activity{At: r.FinishedAt, Kind: "analysis"}
	att, info := r.Counts()
	switch r.Status {
	case core.ScanCancelled:
		a.Title, a.Outcome, a.Summary = "Analysis cancelled", core.OutcomeCancelled, "You stopped the analysis. Your previous results were kept."
	default:
		a.Title, a.Outcome = "Analyzed your software", core.OutcomeSuccess
		if r.Status == core.ScanPartial {
			a.Title, a.Outcome = "Analysis finished with gaps", core.OutcomePartial
		}
		switch {
		case att+info == 0:
			a.Summary = "Nothing needs your attention."
		default:
			a.Summary = fmt.Sprintf("Found %s worth your attention and %s worth knowing.", inventory.Plural(att, "thing"), inventory.Plural(info, "thing"))
		}
	}
	if saveErr != nil {
		a.Title, a.Outcome = "Analysis couldn't be saved", core.OutcomeFailed
		a.Summary = "The analysis worked, but its results couldn't be saved."
	}
	var lines []string
	for _, src := range r.Sources {
		lines = append(lines, inventory.SourceLine(src))
	}
	for _, c := range r.Checks {
		if c.Available {
			lines = append(lines, fmt.Sprintf("%s: %d", c.Name, c.Found))
		} else {
			lines = append(lines, c.Name+": not checked")
		}
	}
	if saveErr != nil {
		lines = append(lines, "Save error: "+saveErr.Error())
	}
	a.Details = strings.Join(lines, "\n")
	return a
}
