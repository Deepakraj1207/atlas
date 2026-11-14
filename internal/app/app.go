// Package app is the composition root: it resolves paths, loads
// configuration, starts logging and runs the TUI.
package app

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/deepakraj1207/atlas/internal/ai"
	"github.com/deepakraj1207/atlas/internal/analysis"
	"github.com/deepakraj1207/atlas/internal/cleanup"
	"github.com/deepakraj1207/atlas/internal/config"
	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/database"
	"github.com/deepakraj1207/atlas/internal/discover"
	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/logging"
	"github.com/deepakraj1207/atlas/internal/platform"
	"github.com/deepakraj1207/atlas/internal/tui"
	"github.com/deepakraj1207/atlas/internal/updates"
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
	// The database is optional: if it cannot be opened, ATLAS still starts
	// and explains in Settings why scanning is off.
	var inv core.InventoryService
	var upd core.UpdateService
	var ana core.AnalysisService
	var cln core.CleanupService
	var dsc core.DiscoverService
	var rec core.ActivityRecorder
	elevated := platform.IsElevated()
	dbPath, storeErr := "", ""
	if dir, err := platform.DataDir(); err != nil {
		storeErr = "find data folder: " + err.Error()
	} else {
		dbPath = filepath.Join(dir, database.FileName)
		db, err := database.Open(context.Background(), dbPath)
		if err != nil {
			storeErr = err.Error()
		} else {
			defer db.Close()
			rec = db
			invSvc := inventory.NewService(inventory.DefaultScanners(), db, logger, inventory.DefaultTimeouts())
			inv = invSvc
			// After removals and updates, the inventory re-reads every
			// source to confirm what really changed.
			upd = updates.NewService(updates.DefaultManagers(elevated), db, logger, updates.DefaultTimeouts()).WithVerifier(invSvc)
			ana = analysis.NewService(analysis.DefaultSources(), db, logger, analysis.UsageSupported)
			clnCfg := cleanup.DefaultConfig(elevated)
			clnCfg.Verifier = invSvc
			cln = cleanup.NewService(clnCfg, db, logger)
			if d, err := discover.NewService(discover.DefaultConfig(elevated), db, logger); err != nil {
				logger.Error("discover catalogue", "error", err)
			} else {
				dsc = d
			}
		}
	}
	if storeErr != "" {
		logger.Error("database unavailable", "path", dbPath, "error", storeErr)
	} else {
		logger.Info("database opened", "path", dbPath, "schema", database.SchemaVersion(), "elevated", elevated)
	}

	// AI explanations: the service exists either way, but nothing is sent
	// unless the setting is on and the user confirms a preview.
	explain := ai.NewService(ai.New(res.Config.AI.Model), rec, logger)

	model := tui.New(tui.Options{
		ConfigResult: res,
		LogPath:      logPath,
		LogError:     logErr,
		Logger:       logger,
		Version:      opts.Version,
		Inventory:    inv,
		Updates:      upd,
		Analysis:     ana,
		Cleanup:      cln,
		Discover:     dsc,
		Explain:      explain,
		Elevated:     elevated,
		StoreErr:     storeErr,
		DBPath:       dbPath,
	})
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		logger.Error("tui exited with error", "error", err)
		return err
	}
	logger.Info("atlas exited")
	return nil
}
