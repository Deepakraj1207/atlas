package tui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/components"
	"github.com/deepakraj1207/atlas/internal/tui/screens"
)

type extensionsLoadedMsg struct {
	r   core.ExtensionReport
	err error
}

type extensionChangedMsg struct {
	res core.ExtensionResult
	err error
}

func loadExtensions(svc core.ExtensionService) tea.Cmd {
	if svc == nil {
		return nil
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		r, err := svc.ListExtensions(ctx)
		return extensionsLoadedMsg{r: r, err: err}
	}
}

func changeExtension(svc core.ExtensionService, msg screens.ChangeExtensionMsg) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		var res core.ExtensionResult
		var err error
		if msg.Remove {
			res, err = svc.RemoveExtension(ctx, msg.Ext)
		} else {
			res, err = svc.SetExtensionEnabled(ctx, msg.Ext, msg.On)
		}
		return extensionChangedMsg{res: res, err: err}
	}
}

// updateExtensions handles AI Tools messages; ok is false for others.
func (m Model) updateExtensions(msg tea.Msg) (Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case screens.RefreshExtensionsMsg:
		return m, loadExtensions(m.ext), true
	case extensionsLoadedMsg:
		if msg.err != nil {
			m.log.Error("read AI tools", "error", msg.err)
			m.data.ExtensionsErr = msg.err.Error()
			return m, nil, true
		}
		m.data.ExtensionsErr = ""
		m.data.Extensions = &msg.r
		return m, nil, true
	case screens.ChangeExtensionMsg:
		if m.ext == nil {
			return m, nil, true
		}
		return m, changeExtension(m.ext, msg), true
	case extensionChangedMsg:
		st := components.Status{Text: msg.res.Summary, Kind: components.StatusSuccess}
		if msg.err != nil {
			st = components.Status{Text: "Nothing was changed: " + msg.err.Error(), Kind: components.StatusError}
		}
		return m, tea.Batch(m.setStatus(st), loadExtensions(m.ext), loadData(m.inv)), true
	}
	return m, nil, false
}
