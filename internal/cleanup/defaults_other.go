//go:build !windows

package cleanup

import (
	"github.com/deepakraj1207/atlas/internal/analysis"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// DefaultConfig returns the Linux removers and tidiers.
func DefaultConfig(elevated bool) Config {
	run := inventory.ExecRunner
	return Config{
		Removers: []Remover{NewAptRemover(elevated), NewDnfRemover(elevated), NewSnapRemover(elevated), NewFlatpakRemover()},
		Tidiers: []Tidier{
			NewAptCache("/var/cache/apt/archives", elevated),
			NewAptAutoremove(run, elevated),
			NewDnfCache([]string{"/var/cache/dnf", "/var/cache/libdnf5"}, elevated),
			NewDnfAutoremove(run, elevated),
			NewSnapOldRevisions(run, "/var/lib/snapd/snaps", elevated),
		},
		Deps:            []analysis.Source{analysis.NewDpkgDeps("/var/lib/dpkg/status"), analysis.NewRPMDeps(run)},
		DnfProtectedDir: "/etc/dnf/protected.d",
	}
}
