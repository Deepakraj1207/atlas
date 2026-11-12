package screens

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/nav"
)

// Messages screens send to the root model.

// ShowActionPreviewMsg asks the root to show an action plan. If the plan is
// available and the user presses Enter, the root dispatches Confirm.
type ShowActionPreviewMsg struct {
	Plan    core.ActionPlan
	Confirm tea.Msg
}

// StartScanMsg asks the root to start a scan. Only sent after a preview.
type StartScanMsg struct{}

// StartCheckMsg asks the root to check for updates. Only sent after a preview.
type StartCheckMsg struct{}

// StartAnalysisMsg asks the root to analyze the saved software list. Only
// sent after a preview.
type StartAnalysisMsg struct{}

// StartTidyFindMsg asks the root to look for things to tidy. Only sent
// after a preview.
type StartTidyFindMsg struct{}

// RequestCleanupMsg asks the root to plan removing these items (with a
// fresh dependency check) plus these clean-up steps, and show the preview.
type RequestCleanupMsg struct {
	Items []core.SoftwareItem
	Tidy  []core.UpdateStep
}

// ApplyCleanupMsg asks the root to run removal and clean-up steps in the
// terminal. Only sent after the user confirms the preview with r.
type ApplyCleanupMsg struct{ Steps []core.UpdateStep }

// RequestApplyMsg asks the root to plan these updates and show the preview.
type RequestApplyMsg struct{ Updates []core.Update }

// ApplyUpdatesMsg asks the root to run planned steps in the terminal. Only
// sent after the user confirms the preview that lists them.
type ApplyUpdatesMsg struct{ Steps []core.UpdateStep }

// ClearInventoryMsg asks the root to delete the saved software list. Only
// sent after a preview.
type ClearInventoryMsg struct{}

// SetModeMsg asks the root to switch mode and persist it.
type SetModeMsg struct{ Mode config.Mode }

// SetThemeMsg asks the root to switch theme and persist it.
type SetThemeMsg struct{ Theme config.Theme }

// SetShowHintsMsg asks the root to toggle the hint row and persist it.
type SetShowHintsMsg struct{ Show bool }

// SetConfirmActionsMsg asks the root to toggle action confirmation and persist it.
type SetConfirmActionsMsg struct{ Confirm bool }

// StatusMsg asks the root to show a status-bar message.
type StatusMsg struct{ Status components.Status }

// NavigateMsg asks the root to open a section.
type NavigateMsg struct{ Section nav.Section }

func send(msg tea.Msg) tea.Cmd {
	return func() tea.Msg { return msg }
}

// requestScan builds the scan preview request.
func requestScan(ctx Context) tea.Cmd {
	return send(ShowActionPreviewMsg{Plan: ScanPlan(ctx), Confirm: StartScanMsg{}})
}

// requestCheck builds the update-check preview request.
func requestCheck(ctx Context) tea.Cmd {
	return send(ShowActionPreviewMsg{Plan: CheckPlan(ctx), Confirm: StartCheckMsg{}})
}

// requestAnalysis builds the analysis preview request.
func requestAnalysis(ctx Context) tea.Cmd {
	return send(ShowActionPreviewMsg{Plan: AnalyzePlan(ctx), Confirm: StartAnalysisMsg{}})
}

// requestTidyFind builds the "look for things to tidy" preview request.
func requestTidyFind(ctx Context) tea.Cmd {
	return send(ShowActionPreviewMsg{Plan: TidyFindPlan(ctx), Confirm: StartTidyFindMsg{}})
}
