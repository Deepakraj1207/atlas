package ai

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/deepakraj1207/atlas/internal/core"
)

// Service implements core.ExplainService: it asks the explainer and records
// every question, answered or not, in Activity History.
type Service struct {
	exp     core.Explainer
	rec     core.ActivityRecorder
	ready   func() (bool, string)
	redact  redactor
	log     *slog.Logger
	now     func() time.Time
	timeout time.Duration
}

// NewService wires an explainer to Activity History. rec may be nil when
// ATLAS's database is unavailable; the question is then logged only.
func NewService(exp core.Explainer, rec core.ActivityRecorder, log *slog.Logger) *Service {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	timeout := 2 * time.Minute
	if exp.Provider().Local {
		timeout = 5 * time.Minute // a local model can be slow on an ordinary PC
	}
	return &Service{exp: exp, rec: rec, ready: exp.Ready, redact: currentRedactor(), log: log, now: time.Now, timeout: timeout}
}

func (s *Service) Model() string             { return s.exp.Model() }
func (s *Service) Instructions() string      { return s.exp.Instructions() }
func (s *Service) Provider() core.AIProvider { return s.exp.Provider() }

// Ready reports whether the explainer can be used, or what to do first.
func (s *Service) Ready() (bool, string) { return s.ready() }

// Prepare removes the user's home folder, user name and computer name, and
// caps the length. The TUI previews the result, and that is what is sent.
func (s *Service) Prepare(question string) string {
	return shorten(s.redact.apply(question))
}

// Explain sends question and records the attempt.
func (s *Service) Explain(ctx context.Context, subject, question string) (core.Explanation, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	start := s.now()
	ex, err := s.exp.Explain(ctx, question)

	p := s.exp.Provider()
	a := core.Activity{At: start, Kind: "ai.explain", Title: "Asked " + p.Name + " about " + subject}
	sent := fmt.Sprintf("Sent %d characters to %s (%s).", utf8.RuneCountInString(question), p.Service, p.Host)
	details := []string{"Model asked: " + s.exp.Model(), fmt.Sprintf("Answer limit: %d tokens", p.MaxOutputTokens), "Sent:", question}
	switch {
	case err == nil:
		a.Outcome = core.OutcomeSuccess
		a.Summary = sent + " " + p.Name + " answered; the answer is below in Advanced mode."
		if ex.Truncated {
			a.Summary += " The answer reached the limit and was cut short."
		}
		details = append(details, fmt.Sprintf("Answered by %s (%d input and %d output tokens):", ex.Model, ex.InputTokens, ex.OutputTokens), ex.Text)
	case errors.Is(ctx.Err(), context.Canceled):
		a.Outcome = core.OutcomeCancelled
		a.Summary = sent + " You stopped it before the answer arrived."
	default:
		a.Outcome = core.OutcomeFailed
		a.Summary = sent + " No answer: " + plainMessage(err) + "."
		details = append(details, "Error: "+err.Error())
	}
	a.Details = strings.Join(details, "\n")
	s.log.Info("ai explain", "subject", subject, "outcome", string(a.Outcome), "chars", utf8.RuneCountInString(question), "provider", p.Service, "model", s.exp.Model())
	if s.rec != nil {
		if _, rerr := s.rec.AddActivity(context.WithoutCancel(ctx), a); rerr != nil {
			s.log.Error("record ai explain", "error", rerr)
		}
	}
	return ex, err
}

func plainMessage(err error) string {
	var se *core.SourceError
	if errors.As(err, &se) {
		return se.Message
	}
	return err.Error()
}

var _ core.ExplainService = (*Service)(nil)
