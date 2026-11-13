package tui

import (
	"context"
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
)

type discoveryLoadedMsg struct {
	d   core.Discovery
	err error
}

type installPlannedMsg struct {
	steps []core.UpdateStep
	err   error
}

type installFinishedMsg struct {
	results []core.StepResult
	err     error
}

type installRecordedMsg struct {
	results []core.StepResult
	err     error
}

// loadDiscovery assesses the catalogue against saved data (local only).
func loadDiscovery(dsc core.DiscoverService) tea.Cmd {
	if dsc == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		d, err := dsc.Discover(ctx)
		return discoveryLoadedMsg{d: d, err: err}
	}
}

func planInstall(dsc core.DiscoverService, ids []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		steps, err := dsc.PlanInstall(ctx, ids)
		return installPlannedMsg{steps: steps, err: err}
	}
}

func recordInstall(dsc core.DiscoverService, results []core.StepResult) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return installRecordedMsg{results: results, err: dsc.RecordResults(ctx, results)}
	}
}

// markInstalled remembers what was installed this session, so Discover
// shows it as installed before the next scan.
func (m *Model) markInstalled(results []core.StepResult) {
	if m.data.Discovery == nil {
		return
	}
	for _, r := range results {
		if r.Status != core.StepDone || len(r.Step.Updates) == 0 {
			continue
		}
		for _, it := range m.data.Discovery.Items {
			if it.Entry.Name == r.Step.Updates[0].Name {
				m.data.InstalledNow[it.Entry.ID] = true
			}
		}
	}
}

func installResultStatus(results []core.StepResult, recordErr error) components.Status {
	done, bad, stopped, restart := 0, 0, 0, false
	for _, r := range results {
		switch r.Status {
		case core.StepDone:
			done++
			restart = restart || r.Restart
		case core.StepFailed:
			bad++
		case core.StepStopped:
			bad++
			stopped++
		}
	}
	var st components.Status
	switch {
	case done == 0 && bad > 0 && stopped == bad:
		st = components.Status{Text: "Install cancelled. Details are in Activity History (8).", Kind: components.StatusWarning}
	case done > 0 && bad == 0:
		st = components.Status{Text: fmt.Sprintf("Installed %d. Scan again to add it to My Software.", done), Kind: components.StatusSuccess}
	case done > 0:
		st = components.Status{Text: fmt.Sprintf("%d installed, %d didn't finish. Details are in Activity History (8).", done, bad), Kind: components.StatusWarning}
	case bad > 0:
		st = components.Status{Text: "The install didn't finish. The package manager's message is in Activity History (8).", Kind: components.StatusError}
	default:
		st = components.Status{Text: "Nothing was installed.", Kind: components.StatusWarning}
	}
	if restart {
		st.Text += " Restart your computer to finish."
	}
	if recordErr != nil {
		st = components.Status{Text: st.Text + " (The result couldn't be recorded: " + recordErr.Error() + ")", Kind: components.StatusError}
	}
	return st
}
