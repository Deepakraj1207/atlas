package tui

import (
	"context"
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/screens"
	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// answerView is Claude's answer, shown in an overlay until closed.
type answerView struct {
	subject string
	ex      core.Explanation
	sent    int
}

type explainDoneMsg struct {
	subject  string
	question string
	ex       core.Explanation
	err      error
}

func (m Model) aiStatus() screens.AIStatus {
	if m.explain == nil {
		return screens.AIStatus{}
	}
	ready, reason := m.explain.Ready()
	return screens.AIStatus{Available: true, Ready: ready, Reason: reason, Model: m.explain.Model(), Instructions: m.explain.Instructions()}
}

// requestExplain shows the preview, or says why nothing can be sent.
func (m Model) requestExplain(msg screens.RequestExplainMsg) (tea.Model, tea.Cmd) {
	warn := func(text string) (tea.Model, tea.Cmd) {
		return m, m.setStatus(components.Status{Text: text, Kind: components.StatusWarning})
	}
	switch {
	case !m.cfg.AI.Enabled:
		return warn("AI explanations are off. Turn them on in Settings (9) if you want them. Nothing was sent.")
	case m.explain == nil:
		return warn("AI explanations aren't available in this session. Nothing was sent.")
	case m.explaining != nil:
		return warn("Already asking Claude. Wait for the answer, or press x to stop.")
	}
	if ok, reason := m.explain.Ready(); !ok {
		return warn(reason + ". Nothing was sent.")
	}
	plan := screens.ExplainPlan(m.ctx(), msg.Subject, msg.Question)
	// Sending text off the computer is always previewed: the plan lists it
	// under "What will change", so "preview read-only actions" can't skip it.
	return m.Update(screens.ShowActionPreviewMsg{Plan: plan, Confirm: screens.StartExplainMsg(msg)})
}

// startExplain sends the question the user just confirmed.
func (m Model) startExplain(msg screens.StartExplainMsg) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(context.Background())
	m.explaining = cancel
	svc := m.explain
	m.log.Info("asking the AI", "subject", msg.Subject)
	ask := func() tea.Msg {
		ex, err := svc.Explain(ctx, msg.Subject, msg.Question)
		return explainDoneMsg{subject: msg.Subject, question: msg.Question, ex: ex, err: err}
	}
	return m, tea.Batch(m.setStatus(components.Status{Text: "Asking Claude about " + msg.Subject + "… Press x to stop."}), ask)
}

func (m Model) explainDone(msg explainDoneMsg) (tea.Model, tea.Cmd) {
	if m.explaining != nil {
		m.explaining()
		m.explaining = nil
	}
	reload := loadData(m.inv) // Activity History has a new entry
	if msg.err != nil {
		text := "Claude couldn't answer"
		var se *core.SourceError
		if errors.As(msg.err, &se) {
			text = se.Message
		}
		kind := components.StatusError
		if errors.Is(msg.err, context.Canceled) || strings.HasPrefix(text, "Stopped") {
			text, kind = "Stopped. Nothing more was sent", components.StatusWarning
		}
		return m, tea.Batch(m.setStatus(components.Status{Text: text + ". See Activity History (8).", Kind: kind}), reload)
	}
	m.answer = &answerView{subject: msg.subject, ex: msg.ex, sent: len([]rune(msg.question))}
	m.overlayScroll = 0
	return m, tea.Batch(m.setStatus(components.Status{Text: "Claude answered. Esc to close.", Kind: components.StatusSuccess}), reload)
}

// renderAnswer shows the answer, clearly labelled as written by AI.
func renderAnswer(t theme.Theme, width int, a answerView, advanced bool) string {
	w := max(width-2, 10)
	var b strings.Builder
	b.WriteString(t.Title.Render("Claude's explanation: " + a.subject))
	b.WriteString("\n")
	b.WriteString(t.Warning.Render(components.Wrap("Written by AI, not by ATLAS. It can be wrong: check anything important before acting on it.", w)))
	b.WriteString("\n\n")
	text := strings.ReplaceAll(a.ex.Text, "**", "")
	b.WriteString(t.Body.Render(components.Wrap(text, w)))
	b.WriteString("\n\n")
	note := "The question and this answer are recorded in Activity History (8)."
	if advanced {
		note += " Model: " + a.ex.Model + "."
	}
	b.WriteString(t.Muted.Render(components.Wrap(note, w)))
	b.WriteString("\n")
	b.WriteString(t.Subtle.Render("Esc to close"))
	return b.String()
}
