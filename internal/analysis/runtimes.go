package analysis

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

var (
	rePyName     = regexp.MustCompile(`(?i)^python\s*(\d+)\.(\d+)`)
	rePyPkg      = regexp.MustCompile(`^python(\d)\.(\d+)$`)
	reMajorMinor = regexp.MustCompile(`(\d+)\.(\d+)`)
	reNodeName   = regexp.MustCompile(`(?i)^(node\.?js|nodejs)(-lts)?$`)
	reDotnetPkg  = regexp.MustCompile(`^(?:dotnet|aspnetcore)-(?:runtime|sdk|hostfxr|host|targeting-pack|apphost-pack)-(\d+)\.(\d+)$`)
	reDotnetName = regexp.MustCompile(`(?i)(?:\.net(?: core)?|asp\.net core|windows desktop runtime)\b[^0-9]*(\d+)\.(\d+)\.\d+`)
	reDebJava    = regexp.MustCompile(`^openjdk-(\d+)-(?:jre|jdk)(?:-headless)?$`)
	reRPMJava    = regexp.MustCompile(`^java-(?:1\.(\d+)\.0|(\d+))-openjdk(?:-headless|-devel)?$`)
	reJavaName   = regexp.MustCompile(`(?i)\b(java|jdk|jre|openjdk|temurin|zulu|corretto)`)
	reFirstNum   = regexp.MustCompile(`(\d+)(?:\.(\d+))?`)
	reVCName     = regexp.MustCompile(`(?i)visual c\+\+ (\d{4}(?:-\d{4})?)\b.*redistributable`)
	reArch       = regexp.MustCompile(`(?i)\b(x64|x86|arm64)\b`)
)

func distroSource(src string) bool {
	return src == inventory.SourceDpkg || src == inventory.SourceRPM
}

// majorOf returns the leading number of a version, skipping a Debian epoch.
func majorOf(v string) string {
	if i := strings.Index(v, ":"); i >= 0 {
		v = v[i+1:]
	}
	m := reFirstNum.FindStringSubmatch(v)
	if m == nil {
		return ""
	}
	return m[1]
}

// DetectRuntime reports whether an item is a runtime other software may
// need, and its support branch.
func DetectRuntime(it core.SoftwareItem) (family, branch string, ok bool) {
	name := strings.TrimSpace(it.Name)
	low := strings.ToLower(name)
	distro := distroSource(it.Source)

	// Python
	switch {
	case distro && rePyPkg.MatchString(name):
		m := rePyPkg.FindStringSubmatch(name)
		return "Python", m[1] + "." + m[2], true
	case it.Source == inventory.SourceRPM && (low == "python3" || low == "python"):
		if m := reMajorMinor.FindStringSubmatch(it.Version); m != nil {
			return "Python", m[1] + "." + m[2], true
		}
	case !distro && (low == "python" || low == "python3"):
		if m := reMajorMinor.FindStringSubmatch(it.Version); m != nil {
			return "Python", m[1] + "." + m[2], true
		}
	case !distro && rePyName.MatchString(name):
		m := rePyName.FindStringSubmatch(name)
		return "Python", m[1] + "." + m[2], true
	}

	// Node.js
	if reNodeName.MatchString(name) {
		if major := majorOf(it.Version); major != "" {
			return "Node.js", major, true
		}
	}

	// .NET (not the Windows-bundled .NET Framework)
	if distro {
		if m := reDotnetPkg.FindStringSubmatch(name); m != nil {
			return ".NET", m[1] + "." + m[2], true
		}
	} else if !strings.Contains(low, ".net framework") {
		if m := reDotnetName.FindStringSubmatch(name); m != nil {
			return ".NET", m[1] + "." + m[2], true
		}
	}

	// Java
	if distro {
		if m := reDebJava.FindStringSubmatch(name); m != nil {
			return "Java", m[1], true
		}
		if m := reRPMJava.FindStringSubmatch(name); m != nil {
			return "Java", m[1] + m[2], true
		}
	} else if reJavaName.MatchString(name) && !strings.Contains(low, "javascript") {
		if m := reFirstNum.FindStringSubmatch(name[reJavaName.FindStringIndex(name)[0]:]); m != nil {
			major := m[1]
			if major == "1" && m[2] != "" {
				major = m[2] // 1.8 means Java 8
			}
			if n, err := strconv.Atoi(major); err == nil && n >= 5 && n <= 40 {
				return "Java", major, true
			}
		}
	}

	// Visual C++ Redistributable
	if m := reVCName.FindStringSubmatch(name); m != nil {
		branch := m[1]
		if a := reArch.FindStringSubmatch(name); a != nil {
			branch += " " + strings.ToLower(a[1])
		}
		return "Visual C++", branch, true
	}
	return "", "", false
}
