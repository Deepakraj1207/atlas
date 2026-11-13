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
	ready   func() bool
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
	return &Service{exp: exp, rec: rec, ready: CredentialsAvailable, log: log, now: time.Now, timeout: 2 * time.Minute}
}

func (s *Service) Model() string        { return s.exp.Model() }
func (s *Service) Instructions() string { return s.exp.Instructions() }

// Ready reports whether credentials were found.
func (s *Service) Ready() (bool, string) {
	if s.ready() {
		return true, ""
	}
	return false, "No Anthropic API key found. Set ANTHROPIC_API_KEY (or sign in with \"ant auth login\") and restart ATLAS"
}

// Explain sends question and records the attempt.
func (s *Service) Explain(ctx context.Context, subject, question string) (core.Explanation, error) {
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	start := s.now()
	ex, err := s.exp.Explain(ctx, question)

	a := core.Activity{At: start, Kind: "ai.explain", Title: "Asked Claude about " + subject}
	sent := fmt.Sprintf("Sent %d characters to Anthropic.", utf8.RuneCountInString(question))
	details := []string{"Model asked: " + s.exp.Model(), "Sent:", question}
	switch {
	case err == nil:
		a.Outcome = core.OutcomeSuccess
		a.Summary = sent + " Claude answered; the answer is below in Advanced mode."
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
	s.log.Info("ai explain", "subject", subject, "outcome", string(a.Outcome), "chars", utf8.RuneCountInString(question), "model", s.exp.Model())
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
