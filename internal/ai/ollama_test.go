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

	"github.com/deepakraj1207/atlas/internal/core"
)

func fakeOllama(t *testing.T, status int, body string) (*httptest.Server, *map[string]any) {
	t.Helper()
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/chat" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &got)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, body)
	}))
	t.Cleanup(srv.Close)
	return srv, &got
}

func newTestOllama(t *testing.T, url string) *Ollama {
	t.Helper()
	o, err := NewOllama(url+"/", "llama3.2", 1024)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestOllamaSendsWhatThePreviewShows(t *testing.T) {
	srv, got := fakeOllama(t, 200, `{"model":"llama3.2","message":{"role":"assistant","content":"<think>hmm</think>\nGit tracks changes to files.\n"},
		"done":true,"done_reason":"stop","prompt_eval_count":150,"eval_count":30}`)
	o := newTestOllama(t, srv.URL)
	ex, err := o.Explain(context.Background(), "Git 2.56.0 · Windows Package Manager")
	if err != nil {
		t.Fatal(err)
	}
	if ex.Text != "Git tracks changes to files." || ex.Model != "llama3.2" || ex.InputTokens != 150 || ex.OutputTokens != 30 || ex.Truncated {
		t.Errorf("explanation = %+v", ex)
	}
	req := *got
	msgs, _ := json.Marshal(req["messages"])
	if req["model"] != "llama3.2" || req["stream"] != false || !strings.Contains(string(msgs), "people who are not technical") ||
		!strings.Contains(string(msgs), "Git 2.56.0 · Windows Package Manager") {
		t.Errorf("request = %v", req)
	}
	if opts, _ := req["options"].(map[string]any); opts["num_predict"] != float64(1024) {
		t.Errorf("answer limit not sent: %v", req["options"])
	}
	if len(req) != 4 {
		t.Errorf("only model, messages, stream and options are sent: %v", req)
	}
}

func TestOllamaProblemsArePlainLanguage(t *testing.T) {
	for _, c := range []struct {
		status int
		body   string
		want   string
	}{
		{404, `{"error":"model \"llama3.2\" not found, try pulling it first"}`, `Run "ollama pull llama3.2" first`},
		{500, `{"error":"out of memory"}`, "Ollama returned an error"},
		{200, `<html>not ollama</html>`, "Is that address really Ollama?"},
		{200, `{"model":"llama3.2","message":{"role":"assistant","content":""},"done":true,"done_reason":"length"}`, "answer limit (1024 tokens)"},
		{200, `{"model":"llama3.2","message":{"role":"assistant","content":"  "},"done":true,"done_reason":"stop"}`, "didn't return an answer"},
	} {
		srv, _ := fakeOllama(t, c.status, c.body)
		_, err := newTestOllama(t, srv.URL).Explain(context.Background(), "x")
		var se *core.SourceError
		if !errors.As(err, &se) || !strings.Contains(se.Message, c.want) {
			t.Errorf("HTTP %d %s: got %v, want %q", c.status, c.body, err, c.want)
		}
	}

	srv, _ := fakeOllama(t, 200, `{"model":"llama3.2","message":{"role":"assistant","content":"Git is"},"done":true,"done_reason":"length"}`)
	if ex, err := newTestOllama(t, srv.URL).Explain(context.Background(), "x"); err != nil || !ex.Truncated {
		t.Errorf("cut-short answer: %+v %v", ex, err)
	}

	down := httptest.NewServer(http.NotFoundHandler())
	url := down.URL
	down.Close()
	_, err := newTestOllama(t, url).Explain(context.Background(), "x")
	var se *core.SourceError
	if !errors.As(err, &se) || !strings.Contains(se.Message, "Couldn't reach Ollama on this computer") || !strings.Contains(se.Message, "Is Ollama running?") {
		t.Errorf("not running: %v", err)
	}
}

// The question goes only to the address shown: no redirects elsewhere.
func TestOllamaDoesNotFollowRedirects(t *testing.T) {
	elsewhere := false
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { elsewhere = true }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/api/chat", http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	_, err := newTestOllama(t, srv.URL).Explain(context.Background(), "x")
	if err == nil || elsewhere {
		t.Errorf("redirect followed (elsewhere=%v, err=%v)", elsewhere, err)
	}
}

func TestOllamaProviderAndReady(t *testing.T) {
	for _, c := range []struct {
		url   string
		local bool
	}{{"http://localhost:11434", true}, {"http://127.0.0.1:11434", true}, {"http://[::1]:11434", true}, {"http://gpu-box.lan:11434", false}} {
		o, err := NewOllama(c.url, "llama3.2", 0)
		if err != nil {
			t.Fatal(err)
		}
		p := o.Provider()
		if p.Local != c.local || p.Name != "Ollama" || p.MaxOutputTokens != DefaultMaxOutputTokens {
			t.Errorf("%s: %+v", c.url, p)
		}
		if c.local && (!strings.Contains(p.Billing, "Free") || p.Service != "Ollama on this computer") {
			t.Errorf("%s: %+v", c.url, p)
		}
		if !c.local && (strings.Contains(p.Billing, "Free") || !strings.Contains(p.Service, "gpu-box.lan")) {
			t.Errorf("%s: a remote Ollama must not be described as local: %+v", c.url, p)
		}
	}
	o, _ := NewOllama("http://localhost:11434", "", 0)
	if ok, why := o.Ready(); ok || !strings.Contains(why, "ai.model") {
		t.Errorf("no model: %v %q", ok, why)
	}
	if _, err := NewOllama("ftp://x", "m", 0); err == nil {
		t.Error("only http and https addresses")
	}
}
