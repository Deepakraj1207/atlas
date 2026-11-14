package ai

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/deepakraj1207/atlas/internal/core"
)

// fakeAnthropic answers like the Messages API and records the last request.
func fakeAnthropic(t *testing.T, status int, body string) (*httptest.Server, *map[string]any, *http.Header) {
	t.Helper()
	var got map[string]any
	var hdr http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		hdr = r.Header.Clone()
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &got)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &got, &hdr
}

func newTestClaude(url string) *Claude {
	return New("", option.WithBaseURL(url), option.WithAPIKey("test-key"), option.WithMaxRetries(0))
}

func TestExplainSendsWhatThePreviewShows(t *testing.T) {
	srv, got, hdr := fakeAnthropic(t, 200, `{"id":"msg_1","type":"message","role":"assistant","model":"claude-opus-5-5",
		"content":[{"type":"text","text":"Node.js runs JavaScript programs.\n"}],
		"stop_reason":"end_turn","usage":{"input_tokens":120,"output_tokens":40}}`)
	c := newTestClaude(srv.URL)
	ex, err := c.Explain(context.Background(), "Node.js 22 · support ends on 30 Apr 2027")
	if err != nil {
		t.Fatal(err)
	}
	if ex.Text != "Node.js runs JavaScript programs." || ex.Model != "claude-opus-5-5" || ex.InputTokens != 120 || ex.OutputTokens != 40 {
		t.Errorf("explanation = %+v", ex)
	}
	req := *got
	if req["model"] != "claude-opus-5-5" {
		t.Errorf("model = %v", req["model"])
	}
	if oc, _ := req["output_config"].(map[string]any); oc["effort"] != "low" {
		t.Errorf("output_config = %v", req["output_config"])
	}
	if req["fallbacks"] != "default" || !strings.Contains(hdr.Get("Anthropic-Beta"), "server-side-fallback-2026-07-01") {
		t.Errorf("refusal fallback not requested: fallbacks=%v beta=%q", req["fallbacks"], hdr.Get("Anthropic-Beta"))
	}
	msgs, _ := json.Marshal(req["messages"])
	if !strings.Contains(string(msgs), "Node.js 22 · support ends on 30 Apr 2027") {
		t.Errorf("question not sent as shown: %s", msgs)
	}
	sys, _ := json.Marshal(req["system"])
	if !strings.Contains(string(sys), "people who are not technical") {
		t.Errorf("instructions not sent: %s", sys)
	}
	if _, ok := req["thinking"]; ok {
		t.Error("thinking can't be turned off on this model; leave it unset")
	}
	if req["max_tokens"] != float64(DefaultMaxOutputTokens) {
		t.Errorf("max_tokens = %v, want the default %d", req["max_tokens"], DefaultMaxOutputTokens)
	}
}

func TestAnswerLimitIsConfigurable(t *testing.T) {
	srv, got, _ := fakeAnthropic(t, 200, `{"id":"m","type":"message","role":"assistant","model":"claude-opus-5-5",
		"content":[{"type":"text","text":"Node.js runs"}],"stop_reason":"max_tokens","usage":{"input_tokens":1,"output_tokens":2048}}`)
	c := newTestClaude(srv.URL).WithMaxOutputTokens(2048)
	ex, err := c.Explain(context.Background(), "x")
	if err != nil || (*got)["max_tokens"] != float64(2048) {
		t.Fatalf("max_tokens = %v, err %v", (*got)["max_tokens"], err)
	}
	if !ex.Truncated || ex.Text != "Node.js runs" {
		t.Errorf("a cut-short answer is kept and marked: %+v", ex)
	}
	if p := c.Provider(); p.MaxOutputTokens != 2048 || !strings.Contains(p.Billing, "capped at 2048 output tokens") {
		t.Errorf("provider = %+v", p)
	}

	srv, _, _ = fakeAnthropic(t, 200, `{"id":"m","type":"message","role":"assistant","model":"claude-opus-5-5",
		"content":[],"stop_reason":"max_tokens","usage":{"input_tokens":1,"output_tokens":4096}}`)
	_, err = newTestClaude(srv.URL).Explain(context.Background(), "x")
	var se *core.SourceError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "answer limit (4096 tokens)") || !strings.Contains(se.Message, "ai.max_output_tokens") {
		t.Errorf("no text at the limit: %v", err)
	}
}

// External use is billed: the preview must say so, and how much at most.
func TestClaudeBillingIsExplicit(t *testing.T) {
	p := New("").Provider()
	for _, want := range []string{"billed to your Anthropic API account", "doesn't cover it", "capped at 4096 output tokens", "claude-opus-5-5's published price", "usually much less"} {
		if !strings.Contains(p.Billing, want) {
			t.Errorf("billing text missing %q: %s", want, p.Billing)
		}
	}
	if p.Local || p.Host != "api.anthropic.com" || p.Name != "Claude" {
		t.Errorf("provider = %+v", p)
	}
	// 4096 output tokens at $20/M is $0.082; the longest question with the
	// instructions, at 3 characters per token, adds under half a cent.
	if !strings.Contains(p.Billing, "at most about US$0.09") {
		t.Errorf("worst case should be about US$0.09: %s", p.Billing)
	}
	if other := New("claude-sonnet-5-5").Provider().Billing; strings.Contains(other, "US$") || !strings.Contains(other, "see Anthropic's pricing for claude-sonnet-5-5") {
		t.Errorf("no made-up price for other models: %s", other)
	}
}

func TestExplainErrorsArePlainLanguage(t *testing.T) {
	for _, c := range []struct {
		status int
		body   string
		want   string
	}{
		{401, `{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"},"request_id":"req_1"}`, "didn't accept your API key"},
		{402, `{"type":"error","error":{"type":"billing_error","message":"credit"}}`, "needs credit"},
		{404, `{"type":"error","error":{"type":"not_found_error","message":"model"}}`, "doesn't know the model"},
		{429, `{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`, "Wait a minute"},
		{529, `{"type":"error","error":{"type":"overloaded_error","message":"busy"}}`, "busy or unavailable"},
		{200, `{"id":"m","type":"message","role":"assistant","model":"claude-opus-5-5","content":[],"stop_reason":"refusal","usage":{"input_tokens":1,"output_tokens":0}}`, "declined"},
		{200, `{"id":"m","type":"message","role":"assistant","model":"claude-opus-5-5","content":[],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":0}}`, "didn't return an answer"},
	} {
		srv, _, _ := fakeAnthropic(t, c.status, c.body)
		_, err := newTestClaude(srv.URL).Explain(context.Background(), "x")
		var se *core.SourceError
		if !errors.As(err, &se) || !strings.Contains(se.Message, c.want) {
			t.Errorf("HTTP %d: got %v, want a message containing %q", c.status, err, c.want)
		}
	}
}

func TestExplainUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	url := srv.URL
	srv.Close() // nothing listens any more
	_, err := newTestClaude(url).Explain(context.Background(), "x")
	var se *core.SourceError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "Couldn't reach Anthropic") {
		t.Errorf("got %v", err)
	}
}

func TestCredentialsAvailable(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", home+"/xdg")
	t.Setenv("APPDATA", home+"/appdata")
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("ANTHROPIC_AUTH_TOKEN", "")
	if CredentialsAvailable() {
		t.Error("no key and no profile: should be false")
	}
	t.Setenv("ANTHROPIC_API_KEY", "sk-test")
	if !CredentialsAvailable() {
		t.Error("an API key in the environment should count")
	}
}
