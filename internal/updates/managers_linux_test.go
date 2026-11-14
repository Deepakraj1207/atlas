//go:build !windows

package updates

import (
	"testing"

	"github.com/deepakraj1207/atlas/internal/platform"
)

func ids_(t *testing.T, rpm platform.RPMRoute) []string {
	t.Helper()
	var out []string
	for _, m := range Managers(false, rpm) {
		out = append(out, m.ID())
	}
	return out
}

func TestManagersFollowTheDistribution(t *testing.T) {
	for _, c := range []struct {
		name string
		rpm  platform.RPMRoute
		want string
	}{
		{"fedora", platform.RPMRoute{Manager: "dnf", RPMBased: true}, "apt dnf snap flatpak"},
		{"opensuse", platform.RPMRoute{Manager: "zypper", RPMBased: true}, "apt zypper snap flatpak"},
		{"centos7", platform.RPMRoute{RPMBased: true, Reason: "uses yum"}, "apt rpm-refused snap flatpak"},
		{"ubuntu", platform.RPMRoute{Reason: "doesn't manage its software with RPM"}, "apt snap flatpak"},
	} {
		got := ""
		for i, id := range ids_(t, c.rpm) {
			if i > 0 {
				got += " "
			}
			got += id
		}
		if got != c.want {
			t.Errorf("%s: managers %q, want %q", c.name, got, c.want)
		}
	}
}
