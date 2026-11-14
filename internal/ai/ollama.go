package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
)

// Ollama implements core.Explainer with a model running in Ollama
// (https://ollama.com), normally on this computer: nothing is billed and,
// when Ollama is local, the question never leaves the computer.
type Ollama struct {
	base      *url.URL
	model     string
	maxTokens int
	client    *http.Client
}

// NewOllama returns an explainer for model at baseURL (for example
// http://localhost:11434). The address must already be validated.
func NewOllama(baseURL, model string, maxTokens int) (*Ollama, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("not an Ollama address: %q", baseURL)
	}
	if maxTokens <= 0 {
		maxTokens = DefaultMaxOutputTokens
	}
	return &Ollama{base: u, model: model, maxTokens: maxTokens, client: &http.Client{
		// Straight to the address shown in the preview: no proxy, and no
		// following redirects somewhere else.
		Transport:     &http.Transport{Proxy: nil, DialContext: (&net.Dialer{}).DialContext},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (o *Ollama) Model() string        { return o.model }
func (o *Ollama) Instructions() string { return instructions }

// local reports whether the address is this computer.
func (o *Ollama) local() bool {
	h := o.base.Hostname()
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.IsLoopback()
}

// Provider says where questions go. A remote Ollama is reported as such.
func (o *Ollama) Provider() core.AIProvider {
	p := core.AIProvider{Name: "Ollama", Host: o.base.Host, Local: o.local(), MaxOutputTokens: o.maxTokens}
	if p.Local {
		p.Service = "Ollama on this computer"
		p.Billing = "Free: Ollama runs the model on your own computer, so nothing is billed and nothing is sent over the internet."
	} else {
		p.Service = "Ollama at " + o.base.Host
		p.Billing = "ATLAS doesn't charge anything; whoever runs the Ollama server at " + o.base.Host + " decides what it costs and what happens to the text."
	}
	return p
}

// Ready needs only a model name; whether Ollama is running is found out
// when asking, and explained then.
func (o *Ollama) Ready() (bool, string) {
	if o.model == "" {
		return false, "No Ollama model chosen. Set ai.model in the settings file to a model you've pulled (for example llama3.2), then restart ATLAS"
	}
	return true, ""
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
	Options  map[string]any  `json:"options"`
}

type ollamaResponse struct {
	Model      string        `json:"model"`
	Message    ollamaMessage `json:"message"`
	Done       bool          `json:"done"`
	DoneReason string        `json:"done_reason"`
	PromptEval int64         `json:"prompt_eval_count"`
	Eval       int64         `json:"eval_count"`
	Error      string        `json:"error"`
}

// reThink removes the reasoning some local models write before answering.
var reThink = regexp.MustCompile(`(?s)<think>.*?</think>`)

// Explain sends one question to Ollama's chat API.
func (o *Ollama) Explain(ctx context.Context, question string) (core.Explanation, error) {
	body, _ := json.Marshal(ollamaRequest{
		Model: o.model,
		Messages: []ollamaMessage{
			{Role: "system", Content: instructions},
			{Role: "user", Content: question},
		},
		Options: map[string]any{"num_predict": o.maxTokens},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.base.JoinPath("api", "chat").String(), bytes.NewReader(body))
	if err != nil {
		return core.Explanation{}, &core.SourceError{Message: "Couldn't prepare the question for Ollama", Details: err.Error()}
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := o.client.Do(req)
	if err != nil {
		return core.Explanation{}, o.describe(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return core.Explanation{}, o.describe(err)
	}
	var out ollamaResponse
	jerr := json.Unmarshal(raw, &out)
	details := fmt.Sprintf("HTTP %d from %s", resp.StatusCode, o.base.Host)
	if out.Error != "" {
		details += ": " + out.Error
	}
	switch {
	case resp.StatusCode == http.StatusNotFound && strings.Contains(strings.ToLower(out.Error), "not found"):
		return core.Explanation{}, &core.SourceError{Message: fmt.Sprintf("Ollama doesn't have the model %q. Run \"ollama pull %s\" first", o.model, o.model), Details: details}
	case resp.StatusCode != http.StatusOK:
		return core.Explanation{}, &core.SourceError{Message: "Ollama returned an error", Details: details}
	case jerr != nil:
		return core.Explanation{}, &core.SourceError{Message: "Ollama's answer couldn't be read. Is that address really Ollama?", Details: details + ": " + jerr.Error()}
	}
	text := strings.TrimSpace(reThink.ReplaceAllString(out.Message.Content, ""))
	cut := out.DoneReason == "length"
	if text == "" {
		if cut {
			return core.Explanation{}, &core.SourceError{
				Message: fmt.Sprintf("The model reached the answer limit (%d tokens) before answering. You can raise ai.max_output_tokens in the settings file", o.maxTokens),
				Details: details + ", done_reason: length"}
		}
		return core.Explanation{}, &core.SourceError{Message: "Ollama didn't return an answer", Details: details + ", done_reason: " + out.DoneReason}
	}
	model := out.Model
	if model == "" {
		model = o.model
	}
	return core.Explanation{Text: text, Model: model, InputTokens: out.PromptEval, OutputTokens: out.Eval, Truncated: cut}, nil
}

func (o *Ollama) describe(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return &core.SourceError{Message: "Stopped. Nothing more was sent", Details: err.Error()}
	case errors.Is(err, context.DeadlineExceeded):
		return &core.SourceError{Message: "The model took too long to answer. A smaller model may be quicker", Details: err.Error()}
	}
	where := "on this computer"
	if !o.local() {
		where = "at " + o.base.Host
	}
	return &core.SourceError{Message: fmt.Sprintf("Couldn't reach Ollama %s (%s). Is Ollama running?", where, o.base.Host), Details: err.Error()}
}

var _ core.Explainer = (*Ollama)(nil)
