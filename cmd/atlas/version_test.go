package main

import "testing"

func TestVersionLine(t *testing.T) {
	for _, c := range []struct {
		version, commit, date string
		settings              map[string]string
		want                  string
	}{
		{"0.1.0", "1a2b3c4d5e6f", "2026-11-13", nil, "atlas 0.1.0 (commit 1a2b3c4, built 2026-11-13)"},
		{"dev", "", "", map[string]string{"vcs.revision": "abcdef0123456", "vcs.time": "2026-11-13T10:41:51Z", "vcs.modified": "true"},
			"atlas dev (commit abcdef0, with uncommitted changes, built 2026-11-13)"},
		{"dev", "", "", map[string]string{}, "atlas dev"},
	} {
		if got := versionLine(c.version, c.commit, c.date, c.settings); got != c.want {
			t.Errorf("got %q, want %q", got, c.want)
		}
	}
}
