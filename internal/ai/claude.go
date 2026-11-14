// Package ai explains software in plain language, using Claude (Anthropic's
// AI, through the official Anthropic Go SDK) or a model running locally in
// Ollama. It is optional and off by default; the TUI previews exactly what
// will be sent before anything is. Answers are advice only: nothing here
// runs commands or changes the computer.
package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthropics/anthropic-sdk-go/v2"
	"github.com/anthropics/anthropic-sdk-go/v2/option"

	"github.com/deepakraj1207/atlas/internal/core"
)

// DefaultModel is the model asked unless settings name another.
const DefaultModel = "claude-opus-5-5"

// instructions are sent with every question and shown in Advanced mode.
const instructions = `You explain software to people who are not technical. You are part of ATLAS, a tool that shows what is installed on someone's computer and helps them keep it tidy and up to date. You will be given one item or one observation exactly as ATLAS shows it.

Answer in plain language, in at most 150 words:
- what it is, or what the observation means;
- whether it usually matters, and why;
- what a sensible next step would be, if any.

Rules: no headings or tables; short paragraphs or a few bullets. Don't invent version numbers, dates or facts you aren't sure of; say when you're unsure. Never suggest commands that delete data, disable security features or change system settings. If removing or updating something could break other software, say so.`

// DefaultMaxOutputTokens caps an answer unless settings say otherwise.
const DefaultMaxOutputTokens = 4096

// Anthropic's published price for DefaultModel, per million tokens, as of
// November 2026. Used only to tell the user the most a question can cost.
const (
	defaultModelInputPerMTok  = 4.0
	defaultModelOutputPerMTok = 20.0
)

// Claude implements core.Explainer.
type Claude struct {
	client    anthropic.Client
	model     string
	maxTokens int
}

// New returns an explainer that asks model (DefaultModel if empty). The SDK
// finds credentials itself: ANTHROPIC_API_KEY, ANTHROPIC_AUTH_TOKEN, or a
// profile saved by "ant auth login".
func New(model string, opts ...option.RequestOption) *Claude {
	if model == "" {
		model = DefaultModel
	}
	return &Claude{client: anthropic.NewClient(opts...), model: model, maxTokens: DefaultMaxOutputTokens}
}

// WithMaxOutputTokens caps each answer at n tokens (n <= 0 keeps the
// default). Claude's reasoning counts towards the cap.
func (c *Claude) WithMaxOutputTokens(n int) *Claude {
	if n > 0 {
		c.maxTokens = n
	}
	return c
}

// Provider says where questions go and that they are billed.
func (c *Claude) Provider() core.AIProvider {
	billing := fmt.Sprintf("Each question is billed to your Anthropic API account by the token. That is separate from any Claude.ai subscription, which doesn't cover it. Answers are capped at %d output tokens", c.maxTokens)
	if c.model == DefaultModel {
		// Upper bound: the longest question Prepare allows plus the
		// instructions, at a pessimistic 3 characters per token.
		in := float64(len(instructions)+MaxQuestionChars) / 3
		most := (in*defaultModelInputPerMTok + float64(c.maxTokens)*defaultModelOutputPerMTok) / 1e6
		billing += fmt.Sprintf("; at %s's published price (US$%g per million input tokens and US$%g per million output tokens, November 2026), one question costs at most about US$%.2f, usually much less", DefaultModel, defaultModelInputPerMTok, defaultModelOutputPerMTok, most)
	} else {
		billing += "; see Anthropic's pricing for " + c.model
	}
	return core.AIProvider{Name: "Claude", Service: "Anthropic", Host: "api.anthropic.com", Billing: billing + ".", MaxOutputTokens: c.maxTokens}
}

// Ready reports whether the SDK is likely to find credentials.
func (c *Claude) Ready() (bool, string) {
	if CredentialsAvailable() {
		return true, ""
	}
	return false, "No Anthropic API key found. Set ANTHROPIC_API_KEY (or sign in with \"ant auth login\") and restart ATLAS"
}

// Model is the model asked.
func (c *Claude) Model() string { return c.model }

// Instructions are the fixed instructions sent with every question.
func (c *Claude) Instructions() string { return instructions }

// Explain sends one question.
func (c *Claude) Explain(ctx context.Context, question string) (core.Explanation, error) {
	resp, err := c.client.Beta.Messages.New(ctx, anthropic.BetaMessageNewParams{
		Model:     c.model,
		MaxTokens: int64(c.maxTokens),
		System:    []anthropic.BetaTextBlockParam{{Text: instructions}},
		Messages:  []anthropic.BetaMessageParam{anthropic.NewBetaUserMessage(anthropic.NewBetaTextBlock(question))},
		// Short, everyday explanations: low effort is enough and quicker.
		OutputConfig: anthropic.BetaOutputConfigParam{Effort: anthropic.BetaOutputConfigEffortLow},
		// If a safety check declines, Anthropic passes the question to its
		// recommended fallback model instead of returning nothing.
		Fallbacks: anthropic.BetaFallbacksParamOfDefault(),
		Betas:     []anthropic.AnthropicBeta{anthropic.AnthropicBetaServerSideFallback2026_07_01},
	})
	if err != nil {
		return core.Explanation{}, describe(err)
	}
	if resp.StopReason == anthropic.BetaStopReasonRefusal {
		return core.Explanation{}, &core.SourceError{Message: "Claude declined to answer this one", Details: "stop reason: refusal"}
	}
	var parts []string
	for _, block := range resp.Content {
		if t, ok := block.AsAny().(anthropic.BetaTextBlock); ok && strings.TrimSpace(t.Text) != "" {
			parts = append(parts, strings.TrimSpace(t.Text))
		}
	}
	cut := resp.StopReason == anthropic.BetaStopReasonMaxTokens
	if len(parts) == 0 {
		if cut {
			return core.Explanation{}, &core.SourceError{
				Message: fmt.Sprintf("Claude reached the answer limit (%d tokens) before answering. You can raise ai.max_output_tokens in the settings file", c.maxTokens),
				Details: "stop reason: max_tokens"}
		}
		return core.Explanation{}, &core.SourceError{Message: "Claude didn't return an answer", Details: "stop reason: " + string(resp.StopReason)}
	}
	return core.Explanation{
		Text:         strings.Join(parts, "\n\n"),
		Model:        string(resp.Model),
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
		Truncated:    cut,
	}, nil
}

// describe turns an SDK error into a plain-language one.
func describe(err error) error {
	if errors.Is(err, context.Canceled) {
		return &core.SourceError{Message: "Stopped. Nothing more was sent", Details: err.Error()}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &core.SourceError{Message: "Claude took too long to answer. Try again later", Details: err.Error()}
	}
	var apierr *anthropic.Error
	if !errors.As(err, &apierr) {
		return &core.SourceError{Message: "Couldn't reach Anthropic (api.anthropic.com). Check your internet connection", Details: err.Error()}
	}
	details := fmt.Sprintf("HTTP %d %s (request %s)", apierr.StatusCode, apierr.Type(), apierr.RequestID)
	msg := "Anthropic returned an error"
	switch {
	case apierr.StatusCode == http.StatusUnauthorized:
		msg = "Anthropic didn't accept your API key. Check ANTHROPIC_API_KEY"
	case apierr.StatusCode == http.StatusPaymentRequired:
		msg = "Your Anthropic account needs credit before it can answer"
	case apierr.StatusCode == http.StatusForbidden:
		msg = "Your Anthropic API key isn't allowed to use this model"
	case apierr.StatusCode == http.StatusNotFound:
		msg = "Anthropic doesn't know the model in your settings (ai.model)"
	case apierr.StatusCode == http.StatusTooManyRequests:
		msg = "Too many questions in a short time. Wait a minute and try again"
	case apierr.StatusCode >= 500:
		msg = "Claude is busy or unavailable right now. Try again in a few minutes"
	}
	return &core.SourceError{Message: msg, Details: details}
}

// CredentialsAvailable reports whether the SDK is likely to find
// credentials: an API key or token in the environment, or a saved profile.
func CredentialsAvailable() bool {
	for _, k := range []string{"ANTHROPIC_API_KEY", "ANTHROPIC_AUTH_TOKEN"} {
		if os.Getenv(k) != "" {
			return true
		}
	}
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".config", "anthropic"))
	}
	if cfg, err := os.UserConfigDir(); err == nil {
		dirs = append(dirs, filepath.Join(cfg, "anthropic"))
	}
	for _, d := range dirs {
		if _, err := os.Stat(d); err == nil {
			return true
		}
	}
	return false
}
