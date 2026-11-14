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

// activityLimit bounds how much history is loaded into memory.
const activityLimit = 200

// scanRun is a scan in progress.
type scanRun struct {
	cancel   context.CancelFunc
	events   chan tea.Msg
	progress screens.ScanProgress
}

type scanEventMsg struct{ ev core.SourceEvent }

type scanDoneMsg struct {
	report core.ScanReport
	err    error
}

type dataLoadedMsg struct {
	summary  *core.ScanSummary
	items    []core.SoftwareItem
	activity []core.Activity
	err      error
}

type inventoryClearedMsg struct{ err error }

type checkEventMsg struct{ ev core.SourceEvent }

type checkDoneMsg struct {
	check core.UpdateCheck
	err   error
}

type updatesLoadedMsg struct {
	summary *core.UpdateCheckSummary
	updates []core.Update
	err     error
}

// updatesFinishedMsg arrives when the terminal job returns control.
type updatesFinishedMsg struct {
	results []core.StepResult
	err     error
}

type updatesRecordedMsg struct {
	results []core.StepResult
	outcome core.RecordOutcome
	err     error
}

// listen waits for the next event from a running scan.
func listen(ch <-chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		msg, ok := <-ch
		if !ok {
			return nil
		}
		return msg
	}
}

// loadData reads the saved list and history in the background.
func loadData(inv core.InventoryService) tea.Cmd {
	if inv == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		var out dataLoadedMsg
		sum, ok, err := inv.LatestScan(ctx)
		if err != nil {
			out.err = err
			return out
		}
		if ok {
			out.summary = &sum
			if out.items, err = inv.Software(ctx); err != nil {
				out.err = err
				return out
			}
		}
		out.activity, out.err = inv.RecentActivity(ctx, activityLimit)
		return out
	}
}

// loadUpdates reads the latest update check in the background.
func loadUpdates(upd core.UpdateService) tea.Cmd {
	if upd == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		sum, ups, ok, err := upd.LatestCheck(ctx)
		out := updatesLoadedMsg{updates: ups, err: err}
		if ok {
			out.summary = &sum
		}
		return out
	}
}

// startScan launches the scan goroutine and returns the first listen.
func (m *Model) startScan() tea.Cmd {
	if m.inv == nil {
		return m.setStatus(components.Status{Text: "Scanning is turned off because ATLAS can't open its saved data. See Settings.", Kind: components.StatusError})
	}
	if m.scan != nil {
		return m.setStatus(components.Status{Text: "A scan is already running.", Kind: components.StatusWarning})
	}
	if m.busy() {
		return m.setStatus(components.Status{Text: "Wait for the running task to finish, or stop it with x.", Kind: components.StatusWarning})
	}
	sources := m.inv.Sources()
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan tea.Msg, 2*len(sources)+4)
	inv := m.inv
	go func() {
		defer close(ch)
		report, err := inv.Scan(ctx, func(ev core.SourceEvent) { ch <- scanEventMsg{ev: ev} })
		ch <- scanDoneMsg{report: report, err: err}
	}()

	run := &scanRun{cancel: cancel, events: ch, progress: screens.ScanProgress{Started: time.Now()}}
	for _, s := range sources {
		run.progress.Sources = append(run.progress.Sources, screens.SourceProgress{Info: s})
	}
	m.scan = run
	m.log.Info("scan requested")
	return tea.Batch(listen(ch), m.spinner.Tick)
}

// startCheck launches an update check in the background.
func (m *Model) startCheck() tea.Cmd {
	if m.upd == nil {
		return m.setStatus(components.Status{Text: "Update checks are turned off because ATLAS can't open its saved data. See Settings.", Kind: components.StatusError})
	}
	if m.check != nil {
		return m.setStatus(components.Status{Text: "An update check is already running.", Kind: components.StatusWarning})
	}
	if m.busy() {
		return m.setStatus(components.Status{Text: "Wait for the running task to finish, or stop it with x.", Kind: components.StatusWarning})
	}
	managers := m.upd.Managers()
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan tea.Msg, 2*len(managers)+4)
	upd := m.upd
	go func() {
		defer close(ch)
		check, err := upd.Check(ctx, func(ev core.SourceEvent) { ch <- checkEventMsg{ev: ev} })
		ch <- checkDoneMsg{check: check, err: err}
	}()
	run := &scanRun{cancel: cancel, events: ch, progress: screens.ScanProgress{Started: time.Now()}}
	for _, s := range managers {
		run.progress.Sources = append(run.progress.Sources, screens.SourceProgress{Info: s})
	}
	m.check = run
	m.log.Info("update check requested")
	return tea.Batch(listen(ch), m.spinner.Tick)
}

// checkResultStatus explains a finished update check in one line.
func checkResultStatus(c core.UpdateCheck, err error) components.Status {
	if err != nil {
		return components.Status{Text: "The check worked but its results couldn't be saved: " + err.Error(), Kind: components.StatusError}
	}
	n := 0
	for _, u := range c.Updates {
		if !u.Held {
			n++
		}
	}
	switch c.Status {
	case core.ScanComplete:
		if n == 0 {
			return components.Status{Text: "Update check complete: everything checked is up to date.", Kind: components.StatusSuccess}
		}
		return components.Status{Text: fmt.Sprintf("Update check complete: %d updates available. Review them in Updates (3).", n), Kind: components.StatusSuccess}
	case core.ScanPartial:
		return components.Status{Text: fmt.Sprintf("Found %d updates, but some package managers couldn't be checked. See Updates (3).", n), Kind: components.StatusWarning}
	case core.ScanFailed:
		text := "The update check didn't work: no package manager could be checked."
		for _, s := range c.Sources {
			if s.Status == core.SourceFailed || s.Status == core.SourceTimeout {
				text = "The update check didn't work. " + s.Label + ": " + s.Message + ". See Updates (3)."
				break
			}
		}
		return components.Status{Text: text, Kind: components.StatusError}
	case core.ScanNothingToScan:
		return components.Status{Text: "No package manager ATLAS supports was found on this computer.", Kind: components.StatusWarning}
	default:
		return components.Status{Text: "Update check stopped. Your previous results are unchanged."}
	}
}

// applyResultStatus explains a finished update run in one line.
func applyResultStatus(results []core.StepResult, out core.RecordOutcome, recordErr error) components.Status {
	count := map[core.StepStatus]int{}
	restart := false
	for _, r := range results {
		count[r.Status] += len(r.Step.Updates)
		restart = restart || (r.Status == core.StepDone && r.Restart)
	}
	var st components.Status
	switch {
	case count[core.StepDone] > 0 && count[core.StepFailed]+count[core.StepStopped] == 0:
		st = components.Status{Text: fmt.Sprintf("Installed %d updates. Scan again to refresh version numbers in My Software.", count[core.StepDone]), Kind: components.StatusSuccess}
	case count[core.StepDone] > 0:
		st = components.Status{Text: fmt.Sprintf("%d updates installed, %d didn't finish. Details are in Activity History (8).", count[core.StepDone], count[core.StepFailed]+count[core.StepStopped]), Kind: components.StatusWarning}
	case count[core.StepStopped] > 0:
		st = components.Status{Text: "Updates stopped. Anything that didn't start was left as it was. See Activity History (8)."}
	case count[core.StepFailed] > 0:
		st = components.Status{Text: "The updates didn't work. The package manager's message is in Activity History (8).", Kind: components.StatusError}
	default:
		st = components.Status{Text: "Nothing was updated.", Kind: components.StatusWarning}
	}
	if n := len(out.Verifications); n > 0 {
		confirmed := out.Count(core.VerifyConfirmed)
		if confirmed < n {
			st = components.Status{Kind: components.StatusWarning, Text: fmt.Sprintf(
				"Checked afterwards: %d of %d updates confirmed. The others still show the old version or couldn't be checked; they stay in Updates (3).",
				confirmed, n)}
		} else if st.Kind == components.StatusSuccess {
			st.Text = fmt.Sprintf("Installed and confirmed %s. Scan again to refresh version numbers in My Software.", components.Plural(confirmed, "update"))
		}
	}
	if restart {
		st.Text += " Restart your computer to finish."
	}
	if recordErr != nil {
		st = components.Status{Text: st.Text + " (The result couldn't be recorded: " + recordErr.Error() + ")", Kind: components.StatusError}
	}
	return st
}

func recordResults(upd core.UpdateService, results []core.StepResult) tea.Cmd {
	return func() tea.Msg {
		// Long enough to re-read every source and confirm the new versions.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		out, err := upd.RecordResults(ctx, results)
		return updatesRecordedMsg{results: results, outcome: out, err: err}
	}
}

func applyEvent(run *scanRun, ev core.SourceEvent) {
	if run == nil {
		return
	}
	for i := range run.progress.Sources {
		sp := &run.progress.Sources[i]
		if sp.Info.ID != ev.Source {
			continue
		}
		if ev.Result == nil {
			sp.Running = true
		} else {
			r := *ev.Result
			sp.Running, sp.Result = false, &r
		}
	}
}

// cancelTasks stops a running scan or update check.
func (m *Model) cancelTasks() {
	for _, run := range []*scanRun{m.scan, m.check, m.analysis, m.tidy} {
		if run == nil || run.progress.Cancelling {
			continue
		}
		run.cancel()
		run.progress.Cancelling = true
	}
	m.log.Info("cancel requested")
}

// busyStatus is the persistent status line while a scan or check runs.
func (m Model) busyStatus() components.Status {
	if m.scan != nil {
		p := m.scan.progress
		if p.Cancelling {
			return components.Status{Text: "Stopping the scan… nothing has been changed."}
		}
		return components.Status{Text: fmt.Sprintf("Scanning… %d of %d sources finished. Press x to stop.", p.Finished(), len(p.Sources))}
	}
	if m.tidy != nil {
		p := m.tidy.progress
		if p.Cancelling {
			return components.Status{Text: "Stopping… nothing has been changed."}
		}
		return components.Status{Text: fmt.Sprintf("Looking for things to tidy… %d of %d checked. Press x to stop.", p.Finished(), len(p.Sources))}
	}
	if m.analysis != nil {
		p := m.analysis.progress
		if p.Cancelling {
			return components.Status{Text: "Stopping the analysis… nothing has been changed."}
		}
		return components.Status{Text: fmt.Sprintf("Analyzing… %d of %d data sources read. Press x to stop.", p.Finished(), len(p.Sources))}
	}
	p := m.check.progress
	if p.Cancelling {
		return components.Status{Text: "Stopping the update check… nothing has been changed."}
	}
	return components.Status{Text: fmt.Sprintf("Checking for updates… %d of %d package managers finished. Press x to stop.", p.Finished(), len(p.Sources))}
}

// scanResultStatus explains a finished scan in one line.
func scanResultStatus(r core.ScanReport, err error, advanced bool) components.Status {
	apps, comps := r.Counts()
	if err != nil {
		return components.Status{Text: "The scan worked but its results couldn't be saved: " + err.Error(), Kind: components.StatusError}
	}
	extra := ""
	if advanced {
		extra = fmt.Sprintf(" and %d system components", comps)
	}
	switch r.Status {
	case core.ScanComplete:
		return components.Status{Text: fmt.Sprintf("Scan complete: found %d apps%s. See My Software (2).", apps, extra), Kind: components.StatusSuccess}
	case core.ScanPartial:
		return components.Status{Text: fmt.Sprintf("Scan finished with gaps: found %d apps%s, but some sources couldn't be read. See Activity History (8).", apps, extra), Kind: components.StatusWarning}
	case core.ScanFailed:
		return components.Status{Text: "The scan didn't work: no software source could be read. Your previous list is unchanged.", Kind: components.StatusError}
	case core.ScanNothingToScan:
		return components.Status{Text: "Nothing to scan: ATLAS doesn't support any software source on this computer yet.", Kind: components.StatusWarning}
	default:
		return components.Status{Text: "Scan stopped. Your previous list is unchanged."}
	}
}

func clearInventory(inv core.InventoryService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return inventoryClearedMsg{err: inv.ClearInventory(ctx)}
	}
}
