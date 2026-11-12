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

// startScan launches the scan goroutine and returns the first listen.
func (m *Model) startScan() tea.Cmd {
	if m.inv == nil {
		return m.setStatus(components.Status{Text: "Scanning is turned off because ATLAS can't open its saved data. See Settings.", Kind: components.StatusError})
	}
	if m.scan != nil {
		return m.setStatus(components.Status{Text: "A scan is already running.", Kind: components.StatusWarning})
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

func (m *Model) applyScanEvent(ev core.SourceEvent) {
	if m.scan == nil {
		return
	}
	for i := range m.scan.progress.Sources {
		sp := &m.scan.progress.Sources[i]
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

func (m *Model) cancelScan() {
	if m.scan == nil || m.scan.progress.Cancelling {
		return
	}
	m.scan.cancel()
	m.scan.progress.Cancelling = true
	m.log.Info("scan cancel requested")
}

// scanningStatus is the persistent status line while a scan runs.
func (m Model) scanningStatus() components.Status {
	p := m.scan.progress
	if p.Cancelling {
		return components.Status{Text: "Stopping the scan… nothing has been changed."}
	}
	return components.Status{Text: fmt.Sprintf("Scanning… %d of %d sources finished. Press x to stop.", p.Finished(), len(p.Sources))}
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
