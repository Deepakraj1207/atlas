package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/screens"
)

type tidyEventMsg struct{ ev core.SourceEvent }

type tidyDoneMsg struct{ report core.TidyReport }

type cleanupPlannedMsg struct {
	steps []core.UpdateStep
	err   error
}

type cleanupFinishedMsg struct {
	results []core.StepResult
	err     error
}

type cleanupRecordedMsg struct {
	results []core.StepResult
	err     error
}

// startTidy looks for things to tidy in the background.
func (m *Model) startTidy() tea.Cmd {
	switch {
	case m.cln == nil:
		return m.setStatus(components.Status{Text: "Cleanup is unavailable because ATLAS can't open its saved data.", Kind: components.StatusError})
	case m.busy():
		return m.setStatus(components.Status{Text: "Wait for the running task to finish, or stop it with x.", Kind: components.StatusWarning})
	}
	sources := m.cln.Tidiers()
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan tea.Msg, 2*len(sources)+4)
	cln := m.cln
	go func() {
		defer close(ch)
		r, _ := cln.FindTidyUps(ctx, func(ev core.SourceEvent) { ch <- tidyEventMsg{ev: ev} })
		ch <- tidyDoneMsg{report: r}
	}()
	run := &scanRun{cancel: cancel, events: ch, progress: screens.ScanProgress{Started: time.Now()}}
	for _, s := range sources {
		run.progress.Sources = append(run.progress.Sources, screens.SourceProgress{Info: s})
	}
	m.tidy = run
	m.log.Info("tidy search requested")
	return tea.Batch(listen(ch), m.spinner.Tick)
}

// planCleanup checks dependencies fresh and plans removals, then adds the
// chosen clean-up steps.
func planCleanup(cln core.CleanupService, req screens.RequestCleanupMsg) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		steps, err := cln.PlanRemoval(ctx, req.Items)
		return cleanupPlannedMsg{steps: append(steps, req.Tidy...), err: err}
	}
}

func recordCleanup(cln core.CleanupService, results []core.StepResult) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return cleanupRecordedMsg{results: results, err: cln.RecordResults(ctx, results)}
	}
}

// pruneTidy drops clean-up steps that ran successfully from the session's
// list, so they aren't offered again.
func (m *Model) pruneTidy(results []core.StepResult) {
	if m.data.Tidy == nil {
		return
	}
	done := map[string]bool{}
	for _, r := range results {
		if r.Status == core.StepDone {
			done[r.Step.Manager+"|"+r.Step.Label] = true
		}
	}
	r := *m.data.Tidy
	var keep []core.UpdateStep
	for _, st := range r.Steps {
		if !done[st.Manager+"|"+st.Label] {
			keep = append(keep, st)
		}
	}
	r.Steps = keep
	m.data.Tidy = &r
}

func tidyResultStatus(r core.TidyReport) components.Status {
	switch {
	case r.Status == core.ScanCancelled:
		return components.Status{Text: "Stopped looking. Nothing was changed."}
	case r.Status == core.ScanFailed:
		return components.Status{Text: "None of the places ATLAS checks could be read. See Activity History (8).", Kind: components.StatusError}
	case len(r.Steps) == 0:
		return components.Status{Text: "Nothing to tidy right now.", Kind: components.StatusSuccess}
	}
	text := "Found " + components.Plural(len(r.Steps), "thing") + " to tidy"
	if b := r.Bytes(); b > 0 {
		text += fmt.Sprintf(", about %s", humanBytes(b))
	}
	return components.Status{Text: text + ". Nothing was changed; review them in Cleanup (6).", Kind: components.StatusSuccess}
}

func cleanupResultStatus(results []core.StepResult, recordErr error) components.Status {
	count := map[core.StepStatus]int{}
	var freed int64
	for _, r := range results {
		count[r.Status] += max(len(r.Step.Updates), 1)
		if r.Status == core.StepDone {
			freed += r.Step.Bytes
		}
	}
	var st components.Status
	switch {
	case count[core.StepDone] > 0 && count[core.StepFailed]+count[core.StepStopped] == 0:
		st = components.Status{Text: "Cleanup finished. My Software is up to date.", Kind: components.StatusSuccess}
		if freed > 0 {
			st.Text = "Cleanup finished, freeing about " + humanBytes(freed) + ". My Software is up to date."
		}
	case count[core.StepDone] > 0:
		st = components.Status{Text: "Cleanup partly finished. Details are in Activity History (8).", Kind: components.StatusWarning}
	case count[core.StepStopped] > 0:
		st = components.Status{Text: "Cleanup stopped. Anything that didn't start was left as it was."}
	case count[core.StepFailed] > 0:
		st = components.Status{Text: "Cleanup didn't work. The package manager's message is in Activity History (8).", Kind: components.StatusError}
	default:
		st = components.Status{Text: "Nothing was removed.", Kind: components.StatusWarning}
	}
	if recordErr != nil {
		st = components.Status{Text: st.Text + " (The result couldn't be recorded: " + recordErr.Error() + ")", Kind: components.StatusError}
	}
	return st
}

func humanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}
