//go:build !windows

package cleanup

import (
	"github.com/deepakraj1207/atlas/internal/analysis"
	"github.com/deepakraj1207/atlas/internal/inventory"
	"github.com/deepakraj1207/atlas/internal/platform"
)

// DefaultConfig returns the Linux removers and tidiers. RPM packages are
// removed with the distribution's own tool (DNF or Zypper), detected from
// /etc/os-release; DNF's tidiers are only offered where DNF is that tool.
func DefaultConfig(elevated bool) Config {
	run := inventory.ExecRunner
	rpm := platform.LinuxRPMRoute()
	tidiers := []Tidier{
		NewAptCache("/var/cache/apt/archives", elevated),
		NewAptAutoremove(run, elevated),
	}
	if rpm.Manager == "dnf" {
		tidiers = append(tidiers,
			NewDnfCache([]string{"/var/cache/dnf", "/var/cache/libdnf5"}, elevated),
			NewDnfAutoremove(run, elevated))
	}
	tidiers = append(tidiers, NewSnapOldRevisions(run, "/var/lib/snapd/snaps", elevated))
	return Config{
		Removers:        []Remover{NewAptRemover(elevated), NewDnfRemover(elevated), NewZypperRemover(elevated), NewSnapRemover(elevated), NewFlatpakRemover()},
		Tidiers:         tidiers,
		Deps:            []analysis.Source{analysis.NewDpkgDeps("/var/lib/dpkg/status"), analysis.NewRPMDeps(run)},
		DnfProtectedDir: "/etc/dnf/protected.d",
		RPM:             rpm,
	}
}
