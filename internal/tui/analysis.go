package tui

import (
	"context"
	"errors"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/analysis"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/screens"
)

type analysisEventMsg struct{ ev core.SourceEvent }

type analysisDoneMsg struct {
	result core.AnalysisResult
	err    error
}

type analysisLoadedMsg struct {
	result *core.AnalysisResult
	err    error
}

// busy reports whether any background task is running.
func (m Model) busy() bool {
	return m.scan != nil || m.check != nil || m.analysis != nil || m.tidy != nil
}

func loadAnalysis(ana core.AnalysisService) tea.Cmd {
	if ana == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		r, ok, err := ana.Latest(ctx)
		out := analysisLoadedMsg{err: err}
		if ok {
			out.result = &r
		}
		return out
	}
}

// startAnalysis runs the analysis in the background.
func (m *Model) startAnalysis() tea.Cmd {
	switch {
	case m.ana == nil:
		return m.setStatus(components.Status{Text: "Analysis is turned off because ATLAS can't open its saved data. See Settings.", Kind: components.StatusError})
	case m.busy():
		return m.setStatus(components.Status{Text: "Wait for the running task to finish, or stop it with x.", Kind: components.StatusWarning})
	}
	sources := m.ana.Sources()
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan tea.Msg, 2*len(sources)+4)
	ana := m.ana
	go func() {
		defer close(ch)
		r, err := ana.Analyze(ctx, func(ev core.SourceEvent) { ch <- analysisEventMsg{ev: ev} })
		ch <- analysisDoneMsg{result: r, err: err}
	}()
	run := &scanRun{cancel: cancel, events: ch, progress: screens.ScanProgress{Started: time.Now()}}
	for _, s := range sources {
		run.progress.Sources = append(run.progress.Sources, screens.SourceProgress{Info: s})
	}
	m.analysis = run
	m.log.Info("analysis requested")
	return tea.Batch(listen(ch), m.spinner.Tick)
}

func analysisResultStatus(r core.AnalysisResult, err error) components.Status {
	switch {
	case errors.Is(err, analysis.ErrNoScan):
		return components.Status{Text: "Scan your system first: there is no software list to analyze yet.", Kind: components.StatusWarning}
	case err != nil:
		return components.Status{Text: "The analysis worked but couldn't be saved: " + err.Error(), Kind: components.StatusError}
	case r.Status == core.ScanCancelled:
		return components.Status{Text: "Analysis stopped. Your previous results are unchanged."}
	}
	att, info := r.Counts()
	var st components.Status
	switch {
	case att > 0:
		st = components.Status{Text: fmt.Sprintf("Analysis complete: %d worth your attention, %d worth knowing. See Smart Analysis (4).", att, info), Kind: components.StatusWarning}
	case info > 0:
		st = components.Status{Text: fmt.Sprintf("Analysis complete: nothing urgent, %d worth knowing. See Smart Analysis (4).", info), Kind: components.StatusSuccess}
	default:
		st = components.Status{Text: "Analysis complete: nothing needs your attention.", Kind: components.StatusSuccess}
	}
	if r.Status == core.ScanPartial {
		st.Text += " Some data couldn't be read; details are in Smart Analysis."
	}
	return st
}
