// Package app is the composition root: it resolves paths, loads
// configuration, starts logging and runs the TUI.
package app

import (
	"fmt"
	"path/filepath"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/logging"
	"github.com/deepakraj1207/atlas/internal/platform"
	"github.com/deepakraj1207/atlas/internal/tui"
)

// Options come from the command line.
type Options struct {
	// ConfigPath overrides the default settings file location.
	ConfigPath string
	Version    string
}

// Run starts ATLAS and blocks until the user quits.
func Run(opts Options) error {
	cfgPath := opts.ConfigPath
	if cfgPath == "" {
		dir, err := platform.ConfigDir()
		if err != nil {
			return fmt.Errorf("find settings folder: %w", err)
		}
		cfgPath = filepath.Join(dir, config.FileName)
	}
	res := config.Load(cfgPath)

	logger := logging.Discard()
	logPath := ""
	logErr := ""
	if dir, err := platform.LogDir(); err != nil {
		logErr = err.Error()
	} else {
		logPath = filepath.Join(dir, logging.FileName)
		l, closeFn, err := logging.Setup(dir, res.Config.Logging.Level)
		if err != nil {
			logErr = err.Error()
		} else {
			logger = l
			defer closeFn()
		}
	}

	logger.Info("atlas starting",
		"version", opts.Version,
		"os", runtime.GOOS,
		"arch", runtime.GOARCH,
		"config", cfgPath,
		"config_found", res.Found,
		"mode", string(res.Config.UI.Mode),
		"theme", string(res.Config.UI.Theme),
	)
	if res.Err != nil {
		logger.Warn("settings file problem", "error", res.Err.Details, "malformed", res.Err.Malformed)
	}
	for _, w := range res.Warnings {
		logger.Warn("settings warning", "warning", w)
	}
	model := tui.New(tui.Options{
		ConfigResult: res,
		LogPath:      logPath,
		LogError:     logErr,
		Logger:       logger,
		Version:      opts.Version,
	})
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		logger.Error("tui exited with error", "error", err)
		return err
	}
	logger.Info("atlas exited")
	return nil
}
