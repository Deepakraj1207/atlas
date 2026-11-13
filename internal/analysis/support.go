package analysis

import (
	"strconv"
	"strings"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
)

// SupportDataAsOf is when the table below was last reviewed. Shown to users
// so they can judge how current it is.
const SupportDataAsOf = "2026-11"

// supportEnds lists published end-of-support dates (YYYY-MM-DD, or YYYY-MM
// when only the month is announced) by family and branch.
var supportEnds = map[string]map[string]string{
	"Python": {
		"2.7": "2020-01-01", "3.5": "2020-09-13", "3.6": "2021-12-23", "3.7": "2023-06-27",
		"3.8": "2024-10-07", "3.9": "2025-10", "3.10": "2026-10", "3.11": "2027-10",
		"3.12": "2028-10", "3.13": "2029-10", "3.14": "2030-10",
	},
	"Node.js": {
		"12": "2022-04-30", "14": "2023-04-30", "16": "2023-09-11", "17": "2022-06-01",
		"18": "2025-04-30", "19": "2023-06-01", "20": "2026-04-30", "21": "2024-06-01",
		"22": "2027-04-30", "23": "2025-06-01", "24": "2028-04-30", "25": "2026-06-01",
	},
	".NET": {
		"2.1": "2021-08-21", "3.1": "2022-12-13", "5.0": "2022-05-10", "6.0": "2024-11-12",
		"7.0": "2024-05-14", "8.0": "2026-11-10", "9.0": "2026-11-10", "10.0": "2028-11-14",
	},
	"Visual C++": {
		"2005": "2016-04-12", "2008": "2018-04-10", "2010": "2020-07-14",
		"2012": "2023-01-10", "2013": "2024-04-09",
	},
}

var supportRefs = map[string]string{
	"Python":     "Python release status, devguide.python.org/versions",
	"Node.js":    "Node.js release schedule, nodejs.org/en/about/previous-releases",
	".NET":       ".NET support policy, dotnet.microsoft.com/platform/support/policy/dotnet-core",
	"Visual C++": "Visual C++ Redistributable support, learn.microsoft.com/cpp/windows/latest-supported-vc-redist",
}

// endingWindow is how far ahead "support ends soon" looks.
const endingWindow = 183 * 24 * time.Hour

// parseEnd returns the last moment of a YYYY-MM-DD or YYYY-MM date.
func parseEnd(s string) (time.Time, bool) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Add(24*time.Hour - time.Nanosecond), true
	}
	if t, err := time.Parse("2006-01", s); err == nil {
		return t.AddDate(0, 1, 0).Add(-time.Nanosecond), true
	}
	return time.Time{}, false
}

// FormatEnd renders an end date for people: "7 Oct 2024" or "Oct 2026".
func FormatEnd(s string) string {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format("2 Jan 2006")
	}
	if t, err := time.Parse("2006-01", s); err == nil {
		return t.Format("Jan 2006")
	}
	return s
}

// EndPhrase is FormatEnd with the right preposition: "on 7 Oct 2024" or
// "in Oct 2026".
func EndPhrase(s string) string {
	if _, err := time.Parse("2006-01-02", s); err == nil {
		return "on " + FormatEnd(s)
	}
	return "in " + FormatEnd(s)
}

// LookupSupport returns the support status of a runtime branch at now.
func LookupSupport(family, branch string, distro bool, now time.Time) core.Support {
	s := core.Support{State: "unknown", Reference: supportRefs[family], DistroManaged: distro}
	switch family {
	case "Java":
		s.Note = "Java support depends on who provides it (Oracle, Eclipse Temurin, Microsoft, your distribution…), so ATLAS doesn't judge it."
		return s
	case "Visual C++":
		year := strings.SplitN(branch, "-", 2)[0]
		year = strings.Fields(year)[0]
		if y, err := strconv.Atoi(year); err == nil && y >= 2015 {
			s.State = "supported"
			s.Note = "Visual C++ 2015 and later share one runtime that Microsoft still supports."
			return s
		}
		branch = year
	}
	end, ok := supportEnds[family][branch]
	if !ok {
		s.Note = "ATLAS's support data (" + SupportDataAsOf + ") doesn't include this version."
		return s
	}
	s.EndDate = end
	t, _ := parseEnd(end)
	switch {
	case now.After(t):
		s.State = "ended"
	case t.Sub(now) <= endingWindow:
		s.State = "ending"
	default:
		s.State = "supported"
	}
	return s
}
