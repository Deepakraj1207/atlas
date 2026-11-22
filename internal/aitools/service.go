// Package aitools finds the MCP servers, plugins and skills set up in AI
// apps (Claude Code, Claude Desktop, Cursor, VS Code, Windsurf, Gemini CLI,
// Codex) by reading their own configuration files, and makes the few
// changes that are safe to make from outside: removing an MCP server
// (keeping a backup of the file), turning a Claude Code plugin on or off,
// and moving a skill folder aside so it can be put back.
//
// It never reads secrets into memory it keeps: environment variables and
// headers of MCP servers are skipped.
package aitools

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
)

// Service implements core.ExtensionService.
type Service struct {
	env Env
	rec core.ActivityRecorder // may be nil
	log *slog.Logger
	now func() time.Time
}

// NewService creates the service. rec may be nil when ATLAS's database is
// unavailable; changes then still work but aren't recorded.
func NewService(env Env, rec core.ActivityRecorder, log *slog.Logger) *Service {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Service{env: env, rec: rec, log: log, now: time.Now}
}

// ListExtensions reads every known configuration. It only reads.
func (s *Service) ListExtensions(ctx context.Context) (core.ExtensionReport, error) {
	r := &reader{env: s.env}
	r.readAll()
	return core.ExtensionReport{At: s.now(), Extensions: r.exts, Sources: r.sources}, ctx.Err()
}

// kindName is "MCP server", "plugin" or "skill".
func kindName(k core.ExtensionKind) string {
	switch k {
	case core.ExtMCP:
		return "MCP server"
	case core.ExtPlugin:
		return "plugin"
	}
	return "skill"
}

// RemoveExtension removes an MCP server or a skill.
func (s *Service) RemoveExtension(ctx context.Context, e core.Extension) (core.ExtensionResult, error) {
	if !e.CanRemove {
		return core.ExtensionResult{}, errors.New("ATLAS can't remove this " + kindName(e.Kind) + ". " + e.Note)
	}
	var res core.ExtensionResult
	var err error
	switch e.Kind {
	case core.ExtMCP:
		res, err = s.removeServer(e)
	case core.ExtSkill:
		res, err = s.removeSkill(e)
	default:
		err = errors.New("plugins are turned off, not removed, from ATLAS")
	}
	s.record(ctx, "extension.remove", fmt.Sprintf("Removed the %s %s from %s", kindName(e.Kind), e.Name, e.App), e, res, err)
	return res, err
}

// SetExtensionEnabled turns a Claude Code plugin on or off.
func (s *Service) SetExtensionEnabled(ctx context.Context, e core.Extension, on bool) (core.ExtensionResult, error) {
	verb := map[bool]string{true: "on", false: "off"}[on]
	var res core.ExtensionResult
	err := func() error {
		if !e.CanToggle || e.Kind != core.ExtPlugin || len(e.Locator) != 1 {
			return errors.New("ATLAS can't turn this " + kindName(e.Kind) + " on or off. " + e.Note)
		}
		n, raw, exists, err := readJSON(e.Path)
		switch {
		case err != nil:
			return err
		case !exists:
			n = &node{kind: 'o'}
		case n.kind != 'o':
			return errors.New(e.Path + " isn't a JSON object")
		}
		if exists && !isStrictJSON(raw) {
			return errors.New(e.Path + " has comments, which ATLAS would lose; change it in " + e.App + " instead")
		}
		plugins := n.get("enabledPlugins")
		if plugins == nil || plugins.kind != 'o' {
			plugins = &node{kind: 'o'}
			n.set("enabledPlugins", plugins)
		}
		plugins.set(e.Locator[0], boolNode(on))
		backup, err := s.writeWithBackup(e.Path, raw, exists, n.encode())
		if err != nil {
			return err
		}
		res = core.ExtensionResult{Summary: fmt.Sprintf("Turned the plugin %s %s. Restart %s to apply it.", e.Name, verb, e.App), Backup: backup}
		return nil
	}()
	s.record(ctx, "extension."+map[bool]string{true: "enable", false: "disable"}[on],
		fmt.Sprintf("Turned the plugin %s %s in %s", e.Name, verb, e.App), e, res, err)
	return res, err
}

func isStrictJSON(raw []byte) bool {
	_, err := parseJSON(raw)
	return err == nil || len(strings.TrimSpace(string(raw))) == 0
}

func (s *Service) removeServer(e core.Extension) (core.ExtensionResult, error) {
	if len(e.Locator) == 0 {
		return core.ExtensionResult{}, errors.New("don't know where this server is set up")
	}
	n, raw, exists, err := readJSON(e.Path)
	switch {
	case err != nil:
		return core.ExtensionResult{}, err
	case !exists:
		return core.ExtensionResult{}, errors.New(e.Path + " is gone; nothing to remove")
	case !isStrictJSON(raw):
		return core.ExtensionResult{}, errors.New(e.Path + " has comments, which ATLAS would lose; remove the server in " + e.App + " instead")
	}
	parent := n.at(e.Locator[:len(e.Locator)-1]...)
	if !parent.remove(e.Locator[len(e.Locator)-1]) {
		return core.ExtensionResult{}, errors.New("the server " + e.Name + " isn't in " + e.Path + " any more")
	}
	backup, err := s.writeWithBackup(e.Path, raw, true, n.encode())
	if err != nil {
		return core.ExtensionResult{}, err
	}
	// Check: read the file back and make sure the entry is gone.
	if again, _, _, err := readJSON(e.Path); err != nil || again.at(e.Locator...) != nil {
		return core.ExtensionResult{Backup: backup}, errors.New("the file was written but still lists the server; the original is kept at " + backup)
	}
	return core.ExtensionResult{
		Summary: fmt.Sprintf("Removed the MCP server %s from %s. Restart %s to apply it.", e.Name, e.App, e.App),
		Backup:  backup,
	}, nil
}

// writeWithBackup keeps a copy of the old file next to it, then replaces it
// in one step so a crash can't leave half a file.
func (s *Service) writeWithBackup(path string, old []byte, exists bool, data []byte) (string, error) {
	mode := fs.FileMode(0o600)
	backup := ""
	if exists {
		if st, err := os.Stat(path); err == nil {
			mode = st.Mode().Perm()
		}
		backup = path + ".atlas-backup-" + s.now().Format("20060102-150405")
		if err := os.WriteFile(backup, old, mode); err != nil {
			return "", fmt.Errorf("couldn't keep a backup, so nothing was changed: %w", err)
		}
	} else if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	tmp := path + ".atlas-tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return backup, fmt.Errorf("couldn't write %s, so nothing was changed: %w", path, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return backup, fmt.Errorf("couldn't replace %s, so nothing was changed: %w", path, err)
	}
	return backup, nil
}

// removeSkill moves the skill's folder into ATLAS's data folder, where it
// can be moved back.
func (s *Service) removeSkill(e core.Extension) (core.ExtensionResult, error) {
	if s.env.DataDir == "" {
		return core.ExtensionResult{}, errors.New("ATLAS has no data folder to keep the skill in, so nothing was changed")
	}
	if _, err := os.Stat(filepath.Join(e.Path, "SKILL.md")); err != nil {
		return core.ExtensionResult{}, errors.New(e.Path + " isn't a skill folder any more; nothing was changed")
	}
	dest := filepath.Join(s.env.DataDir, "removed-skills", s.now().Format("20060102-150405"), filepath.Base(e.Path))
	if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
		return core.ExtensionResult{}, err
	}
	if err := os.Rename(e.Path, dest); err != nil {
		// Different drive: copy, then delete the original only if the copy worked.
		if cerr := copyDir(e.Path, dest); cerr != nil {
			_ = os.RemoveAll(dest)
			return core.ExtensionResult{}, fmt.Errorf("couldn't move the skill, so nothing was changed: %w", cerr)
		}
		if rerr := os.RemoveAll(e.Path); rerr != nil {
			return core.ExtensionResult{Backup: dest}, fmt.Errorf("copied the skill to %s but couldn't delete the original: %w", dest, rerr)
		}
	}
	return core.ExtensionResult{
		Summary: fmt.Sprintf("Removed the skill %s from %s. It is kept in %s; move it back to restore it.", e.Name, e.App, dest),
		Backup:  dest,
	}, nil
}

func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !d.Type().IsRegular() {
			return nil
		}
		in, err := os.Open(p)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.Create(target)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, in); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	})
}

func (s *Service) record(ctx context.Context, kind, title string, e core.Extension, res core.ExtensionResult, err error) {
	a := core.Activity{At: s.now(), Kind: kind, Title: title, Outcome: core.OutcomeSuccess, Summary: res.Summary}
	details := []string{"App: " + e.App, "Where: " + e.Path}
	if res.Backup != "" {
		details = append(details, "Previous version kept at: "+res.Backup)
	}
	if err != nil {
		a.Outcome, a.Summary = core.OutcomeFailed, "Not changed: "+err.Error()
		s.log.Error(kind, "name", e.Name, "app", e.App, "error", err)
	} else {
		s.log.Info(kind, "name", e.Name, "app", e.App, "backup", res.Backup)
	}
	a.Details = strings.Join(details, "\n")
	if s.rec != nil {
		if _, rerr := s.rec.AddActivity(context.WithoutCancel(ctx), a); rerr != nil {
			s.log.Error("record activity", "error", rerr)
		}
	}
}

var _ core.ExtensionService = (*Service)(nil)
