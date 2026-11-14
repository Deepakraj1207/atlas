package ai

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/deepakraj1207/atlas/internal/core"
)

type fakeExplainer struct {
	answer core.Explanation
	err    error
	got    string
	local  bool
}

func (f *fakeExplainer) Model() string         { return "claude-opus-5-5" }
func (f *fakeExplainer) Instructions() string  { return "be plain" }
func (f *fakeExplainer) Ready() (bool, string) { return true, "" }
func (f *fakeExplainer) Provider() core.AIProvider {
	if f.local {
		return core.AIProvider{Name: "Ollama", Service: "Ollama on this computer", Host: "localhost:11434", Local: true, MaxOutputTokens: 4096}
	}
	return core.AIProvider{Name: "Claude", Service: "Anthropic", Host: "api.anthropic.com", MaxOutputTokens: 4096}
}
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
		!strings.Contains(a.Summary, "Sent 41 characters to Anthropic (api.anthropic.com).") || !strings.Contains(a.Details, "Answer limit: 4096 tokens") || !strings.Contains(a.Details, "It runs JavaScript.") {
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
	if ok, _ := s.Ready(); !ok {
		t.Error("should ask the explainer")
	}
	s.ready = func() (bool, string) { return false, "No Anthropic API key found. Set ANTHROPIC_API_KEY" }
	if ok, why := s.Ready(); ok || !strings.Contains(why, "ANTHROPIC_API_KEY") {
		t.Errorf("Ready = %v %q", ok, why)
	}
}

func TestServiceNamesTheProviderAndCutAnswers(t *testing.T) {
	rec := &recorder{}
	exp := &fakeExplainer{local: true, answer: core.Explanation{Text: "Partial", Model: "llama3.2", Truncated: true}}
	s := NewService(exp, rec, nil)
	if s.timeout < 5*time.Minute {
		t.Errorf("a local model gets more time: %v", s.timeout)
	}
	s.Explain(context.Background(), "Git", "Git 2.56.0")
	a := rec.acts[0]
	if a.Title != "Asked Ollama about Git" || !strings.Contains(a.Summary, "Sent 10 characters to Ollama on this computer (localhost:11434).") ||
		!strings.Contains(a.Summary, "cut short") {
		t.Errorf("activity = %+v", a)
	}
}

// Prepare is what the preview shows and what is sent: no user name, home
// folder or computer name, and a bounded length.
func TestPrepareRemovesIdentifyingDetails(t *testing.T) {
	s := NewService(&fakeExplainer{}, nil, nil)
	s.redact = redactor{home: `C:\Users\Priya.Raman`, user: "Priya.Raman", host: "DESKTOP-41IVS1K"}
	q := "Installed software (app): Dropbox - Priya.Raman\nFound by: C:\\Users\\priya.raman\\AppData\\Local\\Dropbox, c:/users/Priya.Raman/x\n" +
		"Description: sync for desktop-41ivs1k\nPublisher: Priya.Ramanathan Ltd"
	got := s.Prepare(q)
	for _, bad := range []string{"Priya.Raman\n", "priya.raman", "Users", "DESKTOP-41IVS1K", "desktop-41ivs1k"} {
		if strings.Contains(got, bad) {
			t.Errorf("%q left in:\n%s", bad, got)
		}
	}
	for _, want := range []string{"Dropbox - [user]", `~\AppData\Local\Dropbox`, "~/x", "sync for [computer]", "Priya.Ramanathan Ltd"} {
		if !strings.Contains(got, want) {
			t.Errorf("want %q in:\n%s", want, got)
		}
	}
	if s.Prepare(got) != got {
		t.Error("Prepare should leave prepared text alone")
	}

	// Generic or very short names aren't treated as identifying.
	s.redact = redactor{user: "root", host: "pc"}
	if q := "root certificate on pc"; s.Prepare(q) != q {
		t.Errorf("generic names changed: %q", s.Prepare(q))
	}

	long := "Description: " + strings.Repeat("a", 1000) + "\n" + strings.Repeat("Items: x\n", 400)
	got = s.Prepare(long)
	if n := utf8.RuneCountInString(got); n > MaxQuestionChars {
		t.Errorf("question has %d characters, want at most %d", n, MaxQuestionChars)
	}
	if first := strings.SplitN(got, "\n", 2)[0]; utf8.RuneCountInString(first) != maxLineChars || !strings.HasSuffix(first, "…") || !strings.HasSuffix(got, "\n…") {
		t.Errorf("long line and question should be shortened visibly: %q … %q", first[:20], got[len(got)-10:])
	}
}
