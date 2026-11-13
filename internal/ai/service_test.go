package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/deepakraj1207/atlas/internal/core"
)

type fakeExplainer struct {
	answer core.Explanation
	err    error
	got    string
}

func (f *fakeExplainer) Model() string        { return "claude-opus-5-5" }
func (f *fakeExplainer) Instructions() string { return "be plain" }
func (f *fakeExplainer) Explain(ctx context.Context, q string) (core.Explanation, error) {
	f.got = q
	return f.answer, f.err
}

type recorder struct{ acts []core.Activity }

func (r *recorder) AddActivity(ctx context.Context, a core.Activity) (int64, error) {
	r.acts = append(r.acts, a)
	return int64(len(r.acts)), nil
}

func TestServiceRecordsEveryQuestion(t *testing.T) {
	rec := &recorder{}
	exp := &fakeExplainer{answer: core.Explanation{Text: "It runs JavaScript.", Model: "claude-opus-5-5", InputTokens: 10, OutputTokens: 5}}
	s := NewService(exp, rec, nil)
	ex, err := s.Explain(context.Background(), "Node.js 24.19.0", "Node.js 24.19.0 · Windows Package Manager")
	if err != nil || ex.Text != "It runs JavaScript." || exp.got != "Node.js 24.19.0 · Windows Package Manager" {
		t.Fatalf("ex=%+v err=%v sent=%q", ex, err, exp.got)
	}
	a := rec.acts[0]
	if a.Kind != "ai.explain" || a.Outcome != core.OutcomeSuccess || a.Title != "Asked Claude about Node.js 24.19.0" ||
		!strings.Contains(a.Summary, "Sent 41 characters to Anthropic") || !strings.Contains(a.Details, "It runs JavaScript.") {
		t.Errorf("activity = %+v", a)
	}

	exp.err = &core.SourceError{Message: "Anthropic didn't accept your API key", Details: "HTTP 401"}
	s.Explain(context.Background(), "Git", "Git 2.56.0")
	if a := rec.acts[1]; a.Outcome != core.OutcomeFailed || !strings.Contains(a.Summary, "No answer: Anthropic didn't accept your API key.") {
		t.Errorf("failed activity = %+v", a)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	exp.err = context.Canceled
	s.Explain(ctx, "Git", "Git 2.56.0")
	if a := rec.acts[2]; a.Outcome != core.OutcomeCancelled {
		t.Errorf("cancelled activity = %+v", a)
	}
}

func TestServiceReady(t *testing.T) {
	s := NewService(&fakeExplainer{}, nil, nil)
	s.ready = func() bool { return false }
	if ok, why := s.Ready(); ok || !strings.Contains(why, "ANTHROPIC_API_KEY") {
		t.Errorf("Ready = %v %q", ok, why)
	}
	s.ready = func() bool { return true }
	if ok, _ := s.Ready(); !ok {
		t.Error("should be ready")
	}
}
