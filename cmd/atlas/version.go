package main

import (
	"runtime/debug"
	"strings"
)

// buildSettings returns the VCS details Go embeds in builds made from a Git
// checkout (vcs.revision, vcs.time, vcs.modified).
func buildSettings() map[string]string {
	out := map[string]string{}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, s := range info.Settings {
			out[s.Key] = s.Value
		}
	}
	return out
}

// versionLine is what "atlas -version" prints, for example
// "atlas 0.1.0 (commit 1a2b3c4, built 2026-11-13)". Values set at build time
// win; otherwise Go's recorded Git details are used.
func versionLine(version, commit, date string, settings map[string]string) string {
	if commit == "" {
		commit = settings["vcs.revision"]
	}
	if date == "" {
		date = settings["vcs.time"]
	}
	if len(commit) > 7 {
		commit = commit[:7]
	}
	if i := strings.IndexByte(date, 'T'); i > 0 {
		date = date[:i]
	}
	var parts []string
	if commit != "" {
		c := "commit " + commit
		if settings["vcs.modified"] == "true" {
			c += ", with uncommitted changes"
		}
		parts = append(parts, c)
	}
	if date != "" {
		parts = append(parts, "built "+date)
	}
	line := "atlas " + version
	if len(parts) > 0 {
		line += " (" + strings.Join(parts, ", ") + ")"
	}
	return line
}
