package screens

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
)

// AIStatus describes the optional AI explanations for screens.
type AIStatus struct {
	// Available is false when this build or session has no AI service.
	Available bool
	// Ready is false when no credentials were found; Reason says what to do.
	Ready        bool
	Reason       string
	Model        string
	Instructions string
}

// maxItemsSent caps how many items of a finding are sent.
const maxItemsSent = 15

// FindingQuestion is exactly what is sent to explain a Smart Analysis
// finding: what ATLAS shows, without technical details such as file paths.
func FindingQuestion(f core.Finding) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Smart Analysis observation: %s\n", f.Title)
	line := func(label, s string) {
		if s = strings.TrimSpace(s); s != "" {
			fmt.Fprintf(&b, "%s: %s\n", label, s)
		}
	}
	line("What ATLAS found", f.Summary)
	line("Why ATLAS thinks it matters", f.Why)
	line("ATLAS's suggestion", f.Suggestion)
	if len(f.Items) > 0 {
		b.WriteString("Items:\n")
		for i, it := range f.Items {
			if i == maxItemsSent {
				fmt.Fprintf(&b, "- …and %d more\n", len(f.Items)-maxItemsSent)
				break
			}
			s := core.NameAndVersion(it.Name, it.Version)
			if it.Note != "" {
				s += " (" + it.Note + ")"
			}
			b.WriteString("- " + s + "\n")
		}
	}
	return strings.TrimSpace(b.String())
}

// ItemQuestion is exactly what is sent to explain one installed item. It
// leaves out install locations, which can contain the user's name.
func ItemQuestion(ctx Context, it core.SoftwareItem) string {
	var b strings.Builder
	kind := "app"
	if it.Kind == core.KindComponent {
		kind = "system component"
	}
	fmt.Fprintf(&b, "Installed software (%s): %s\n", kind, it.Name)
	line := func(label, s string) {
		if s = strings.TrimSpace(s); s != "" {
			fmt.Fprintf(&b, "%s: %s\n", label, s)
		}
	}
	line("Version", it.Version)
	line("Publisher", it.Publisher)
	line("Package ID", it.PackageID)
	found := []string{ctx.SourceLabel(it.Source)}
	for _, s := range it.AlsoFoundIn {
		found = append(found, ctx.SourceLabel(s))
	}
	line("Found by", strings.Join(found, ", "))
	line("Description", it.Description)
	return strings.TrimSpace(b.String())
}

// ExplainPlan is the preview shown before anything is sent to the AI.
func ExplainPlan(ctx Context, subject, question string) core.ActionPlan {
	quoted := "    " + strings.ReplaceAll(question, "\n", "\n    ")
	return core.ActionPlan{
		Title:       "Ask Claude about " + subject,
		WhatHappens: "ATLAS will send the text below to Claude, Anthropic's AI, and show you its answer. This exact text is sent, nothing else from your computer:\n\n" + quoted,
		Why:         "To get a plain-language explanation of what this is and whether it matters, without searching the web yourself.",
		WhatChanges: []string{"The text above leaves your computer and goes to Anthropic (api.anthropic.com), using your API key. Nothing on your computer changes."},
		Safety: "Only the text shown is sent: no file paths, user names or the rest of your software list. Anthropic handles it under its usage policies and your account's settings. " +
			"AI answers can be wrong; check anything important before acting on it. Your Anthropic account may be charged a small amount. The question is recorded in Activity History.",
		Reversible: "Sending can't be undone. Nothing on your computer is changed, so there is nothing else to undo.",
		Technical: []string{
			"Model: " + ctx.AI.Model + " (low effort; if it declines, Anthropic's recommended fallback model answers)",
			"Instructions sent with every question: " + ctx.AI.Instructions,
		},
		Available: true,
	}
}

// requestExplain asks the root to preview an explanation.
func requestExplain(subject, question string) tea.Cmd {
	return send(RequestExplainMsg{Subject: subject, Question: question})
}

// explainHint is the key hint, shown only when AI explanations are on.
func explainHint(ctx Context) []components.KeyHint {
	if !ctx.Config.AI.Enabled {
		return nil
	}
	return []components.KeyHint{{Key: "e", Desc: "explain (AI)"}}
}
