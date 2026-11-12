package analysis

import (
	"encoding/binary"
	"path/filepath"
	"strings"
	"time"
)

// UsageRecord is one program Windows recorded as opened.
type UsageRecord struct {
	Path     string
	RunCount int
	LastRun  time.Time
}

// rot13 decodes UserAssist value names, which Windows stores ROT13-encoded.
func rot13(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return 'a' + (r-'a'+13)%26
		case r >= 'A' && r <= 'Z':
			return 'A' + (r-'A'+13)%26
		}
		return r
	}, s)
}

// filetimeToTime converts a Windows FILETIME (100 ns since 1601).
func filetimeToTime(ft uint64) time.Time {
	const epochDiff = 116444736000000000 // 1601-01-01 to 1970-01-01 in 100 ns
	if ft <= epochDiff {
		return time.Time{}
	}
	ns := (ft - epochDiff) * 100
	return time.Unix(0, int64(ns)).UTC()
}

// parseUserAssistValue decodes the 72-byte value Windows 7 and later use:
// run count at offset 4, last-run FILETIME at offset 60.
func parseUserAssistValue(b []byte) (count int, last time.Time, ok bool) {
	if len(b) < 68 {
		return 0, time.Time{}, false
	}
	count = int(binary.LittleEndian.Uint32(b[4:8]))
	last = filetimeToTime(binary.LittleEndian.Uint64(b[60:68]))
	return count, last, !last.IsZero()
}

// knownFolders maps the folder GUIDs UserAssist uses to environment
// variables or relative paths.
var knownFolders = map[string]func(env func(string) string) string{
	"{6D809377-6AF0-444B-8957-A3773F02200E}": func(env func(string) string) string { // Program Files
		if v := env("ProgramW6432"); v != "" {
			return v
		}
		return env("ProgramFiles")
	},
	"{7C5A40EF-A0FB-4BFC-874A-C0F2E0B9FA8E}": func(env func(string) string) string { return env("ProgramFiles(x86)") },
	"{F38BF404-1D43-42F2-9305-67DE0B28FC23}": func(env func(string) string) string { return env("SystemRoot") },
	"{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}": func(env func(string) string) string { return env("SystemRoot") + `\System32` },
	"{D65231B0-B2F1-4857-A4CE-A8E7C6EA7D27}": func(env func(string) string) string { return env("SystemRoot") + `\SysWOW64` },
}

// resolveUserAssistPath expands a leading known-folder GUID. Paths that
// can't be resolved are returned unchanged.
func resolveUserAssistPath(name string, env func(string) string) string {
	if strings.HasPrefix(name, "{") {
		if i := strings.Index(name, "}"); i > 0 {
			if f, ok := knownFolders[strings.ToUpper(name[:i+1])]; ok {
				if base := f(env); base != "" {
					return base + name[i+1:]
				}
			}
		}
	}
	return name
}

// lastOpened returns the latest recorded run of anything under dir.
func lastOpened(dir string, usage []UsageRecord) (time.Time, bool) {
	dir = strings.TrimRight(strings.ToLower(filepath.Clean(strings.ReplaceAll(dir, "/", `\`))), `\`)
	if len(dir) < 4 { // never match a drive root
		return time.Time{}, false
	}
	var last time.Time
	for _, u := range usage {
		p := strings.ToLower(strings.ReplaceAll(u.Path, "/", `\`))
		if strings.HasPrefix(p, dir+`\`) && u.LastRun.After(last) {
			last = u.LastRun
		}
	}
	return last, !last.IsZero()
}
