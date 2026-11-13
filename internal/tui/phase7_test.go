package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/screens"
)

type fakeExplain struct {
	ready bool
	err   error
	asked []string
}

func (f *fakeExplain) Model() string        { return "claude-opus-5-5" }
func (f *fakeExplain) Instructions() string { return "Explain plainly." }
func (f *fakeExplain) Ready() (bool, string) {
	if f.ready {
		return true, ""
	}
	return false, "No Anthropic API key found. Set ANTHROPIC_API_KEY"
}
func (f *fakeExplain) Explain(ctx context.Context, subject, q string) (core.Explanation, error) {
	f.asked = append(f.asked, q)
	if f.err != nil {
		return core.Explanation{}, f.err
	}
	return core.Explanation{Text: "Alpha Editor is a **text editor**.", Model: "claude-opus-5-5"}, nil
}

func bootAI(t *testing.T, svc core.ExplainService, cfgYAML string) Model {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if cfgYAML != "" {
		os.WriteFile(path, []byte(cfgYAML), 0o644)
	}
	inv, _ := newTestInventory(t)
	m := New(Options{ConfigResult: config.Load(path), Version: "test", Inventory: inv, Explain: svc, DBPath: "test.db"})
	next, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m = drain(t, next.(Model), m.Init())
	m = scanDirect(t, m)
	if m.PreviewOpen() {
		m = press(t, m, "enter") // the scan preview
	}
	if m.Data().Summary == nil {
		t.Fatal("scan didn't run")
	}
	return press(t, m, "2") // My Software, cursor on Alpha Editor
}

func TestAIExplanationIsOffByDefault(t *testing.T) {
	svc := &fakeExplain{ready: true}
	m := bootAI(t, svc, "")
	if strings.Contains(m.View(), "explain (AI)") {
		t.Error("the e hint must not show while AI explanations are off")
	}
	m = press(t, m, "e")
	if m.PreviewOpen() || len(svc.asked) != 0 || !strings.Contains(m.Status().Text, "AI explanations are off") {
		t.Errorf("off: preview=%v asked=%v status=%q", m.PreviewOpen(), svc.asked, m.Status().Text)
	}
}

func TestAIExplanationNeedsAKey(t *testing.T) {
	svc := &fakeExplain{ready: false}
	m := bootAI(t, svc, "ai:\n  enabled: true\n")
	m = press(t, m, "e")
	if m.PreviewOpen() || len(svc.asked) != 0 || !strings.Contains(m.Status().Text, "ANTHROPIC_API_KEY") || !strings.Contains(m.Status().Text, "Nothing was sent") {
		t.Errorf("no key: preview=%v asked=%v status=%q", m.PreviewOpen(), svc.asked, m.Status().Text)
	}
}

func TestAIExplanationPreviewsExactlyWhatIsSent(t *testing.T) {
	svc := &fakeExplain{ready: true}
	// Read-only previews off: sending text off the computer must still preview.
	m := bootAI(t, svc, "ui:\n  confirm_actions: false\nai:\n  enabled: true\n")
	if !strings.Contains(m.View(), "explain (AI)") {
		t.Error("the e hint should show when AI explanations are on")
	}
	m = press(t, m, "e")
	if !m.PreviewOpen() {
		t.Fatal("e must open a preview before anything is sent")
	}
	v := m.View()
	flat := strings.Join(strings.Fields(v), " ")
	for _, want := range []string{"Ask Claude about Alpha Editor 1.0", "exact text is sent, nothing else from your computer", "Installed software (app): Alpha Editor",
		"Publisher: Alpha Co", "Package ID: Alpha.Editor", "leaves your computer", "check anything important before acting on it"} {
		if !strings.Contains(flat, want) {
			t.Errorf("preview missing %q:\n%s", want, v)
		}
	}
	if len(svc.asked) != 0 {
		t.Fatal("nothing may be sent before confirming")
	}

	m = press(t, m, "esc")
	if len(svc.asked) != 0 || m.PreviewOpen() {
		t.Fatal("Esc must send nothing")
	}

	m = press(t, m, "e")
	m = press(t, m, "enter")
	if len(svc.asked) != 1 || !strings.HasPrefix(svc.asked[0], "Installed software (app): Alpha Editor\nVersion: 1.0") {
		t.Fatalf("sent = %q", svc.asked)
	}
	v = m.View()
	for _, want := range []string{"Claude's explanation: Alpha Editor 1.0", "Written by AI, not by ATLAS", "Alpha Editor is a text editor."} {
		if !strings.Contains(v, want) {
			t.Errorf("answer missing %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "**") {
		t.Error("Markdown bold markers should be removed")
	}
	m = press(t, m, "esc")
	if strings.Contains(m.View(), "Written by AI") {
		t.Error("Esc should close the answer")
	}
}

func TestAIExplanationErrorIsExplained(t *testing.T) {
	svc := &fakeExplain{ready: true, err: &core.SourceError{Message: "Anthropic didn't accept your API key. Check ANTHROPIC_API_KEY", Details: "HTTP 401"}}
	m := bootAI(t, svc, "ai:\n  enabled: true\n")
	m = press(t, m, "e")
	m = press(t, m, "enter")
	if st := m.Status(); st.Kind != components.StatusError || !strings.Contains(st.Text, "didn't accept your API key") {
		t.Errorf("status = %+v", st)
	}
	if strings.Contains(m.View(), "Written by AI") {
		t.Error("no answer overlay after an error")
	}
}

func TestSettingsTurnsAIOnAndOff(t *testing.T) {
	svc := &fakeExplain{ready: true}
	m := bootAI(t, svc, "")
	m = press(t, m, "9")
	for i := 0; i < 5; i++ {
		m = press(t, m, "down")
	}
	if !strings.Contains(m.View(), "› AI explanations") {
		t.Fatalf("cursor should be on AI explanations:\n%s", m.View())
	}
	m = press(t, m, "enter")
	if !m.cfg.AI.Enabled || !strings.Contains(m.Status().Text, "AI explanations on") {
		t.Errorf("on: %+v %q", m.cfg.AI, m.Status().Text)
	}
	if !config.Load(m.cfgRes.Path).Config.AI.Enabled {
		t.Error("the setting must be saved")
	}
	m = press(t, m, "enter")
	if m.cfg.AI.Enabled || !strings.Contains(m.Status().Text, "nothing will be sent") {
		t.Errorf("off: %+v %q", m.cfg.AI, m.Status().Text)
	}
}

func TestFindingQuestionLeavesOutTechnicalDetails(t *testing.T) {
	f := core.Finding{Title: "Several versions of Node.js are installed", Summary: "2 versions", Why: "Old ones miss fixes",
		Items:   []core.ItemRef{{Name: "Node.js", Version: "22.1.0", Note: "from winget"}},
		Details: []string{`C:\Users\someone\AppData\Local\Programs\node`}}
	q := screensFindingQuestion(f)
	if strings.Contains(q, `C:\Users`) || !strings.Contains(q, "- Node.js 22.1.0 (from winget)") || !strings.Contains(q, "Why ATLAS thinks it matters: Old ones miss fixes") {
		t.Errorf("question = %q", q)
	}
}

var screensFindingQuestion = screens.FindingQuestion
