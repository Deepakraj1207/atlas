package inventory

import (
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
)

// windowsComponentHints mark supporting pieces rather than applications.
var windowsComponentHints = []string{
	"redist", "runtime", "language pack", "update for", "hotfix",
	"security update", "service pack", "driver", "sdk arm", "targeting pack",
	"shared framework", "host fx resolver", "webview2",
}

// classifyWindowsName guesses whether a Windows display name is an app.
func classifyWindowsName(name string) core.Kind {
	n := strings.ToLower(name)
	for _, h := range windowsComponentHints {
		if strings.Contains(n, h) {
			return core.KindComponent
		}
	}
	// Package-style names such as "Microsoft.UI.Xaml.2.8" are frameworks.
	if !strings.Contains(name, " ") && strings.Count(name, ".") >= 2 {
		return core.KindComponent
	}
	return core.KindApp
}

// classifyLinuxPackage guesses whether a distro package is a component.
func classifyLinuxPackage(name, section, priority string, essential bool) core.Kind {
	n := strings.ToLower(name)
	if essential || priority == "required" || priority == "important" {
		return core.KindComponent
	}
	if i := strings.LastIndex(section, "/"); i >= 0 {
		section = section[i+1:]
	}
	switch section {
	case "libs", "libdevel", "oldlibs", "kernel", "fonts", "localization", "metapackages":
		return core.KindComponent
	}
	if strings.HasPrefix(n, "lib") || strings.HasPrefix(n, "linux-") || strings.HasPrefix(n, "kernel") ||
		strings.HasPrefix(n, "gpg-pubkey") || strings.HasPrefix(n, "fonts-") || strings.HasPrefix(n, "language-pack") {
		return core.KindComponent
	}
	for _, suffix := range []string{"-libs", "-devel", "-dev", "-common", "-data", "-doc", "-filesystem", "-locale", "-l10n"} {
		if strings.HasSuffix(n, suffix) {
			return core.KindComponent
		}
	}
	return core.KindApp
}
