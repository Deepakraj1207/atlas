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
