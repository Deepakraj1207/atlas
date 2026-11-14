package app

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/deepakraj1207/atlas/internal/config"
)

func TestNewExplainerFollowsSettings(t *testing.T) {
	log := slog.New(slog.DiscardHandler)

	claude := newExplainer(config.AIConfig{}, log)
	if p := claude.Provider(); p.Name != "Claude" || p.Local || p.MaxOutputTokens != config.DefaultAIMaxOutputTokens {
		t.Errorf("default: %+v", p)
	}
	if p := newExplainer(config.AIConfig{MaxOutputTokens: 2048}, log).Provider(); p.MaxOutputTokens != 2048 {
		t.Errorf("limit not applied: %+v", p)
	}

	local := newExplainer(config.AIConfig{Provider: config.AIProviderOllama, Model: "llama3.2"}, log)
	if p := local.Provider(); p.Name != "Ollama" || !p.Local || p.Host != "localhost:11434" || local.Model() != "llama3.2" {
		t.Errorf("ollama: %+v %s", p, local.Model())
	}
	if ok, _ := local.Ready(); !ok {
		t.Error("ollama with a model should be ready")
	}

	// An unusable address never falls back to Anthropic.
	bad := newExplainer(config.AIConfig{Provider: config.AIProviderOllama, Model: "m", OllamaURL: "::bad"}, log)
	if ok, why := bad.Ready(); ok || !strings.Contains(why, "ai.ollama_url") || bad.Provider().Name == "Claude" {
		t.Errorf("bad address: ready=%v %q %+v", ok, why, bad.Provider())
	}
}
