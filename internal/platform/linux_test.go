package platform

import (
	"strings"
	"testing"
)

// Real os-release contents (trimmed) from each distribution.
var osReleases = map[string]string{
	"fedora41": `NAME="Fedora Linux"
VERSION_ID=41
ID=fedora
PRETTY_NAME="Fedora Linux 41 (Workstation Edition)"`,
	"rhel9": `NAME="Red Hat Enterprise Linux"
ID="rhel"
ID_LIKE="fedora"
VERSION_ID="9.4"
PRETTY_NAME="Red Hat Enterprise Linux 9.4 (Plow)"`,
	"rocky9": `NAME="Rocky Linux"
ID="rocky"
ID_LIKE="rhel centos fedora"
VERSION_ID="9.4"
PRETTY_NAME="Rocky Linux 9.4 (Blue Onyx)"`,
	"amzn2023": `NAME="Amazon Linux"
ID="amzn"
ID_LIKE="fedora"
VERSION_ID="2023"
PRETTY_NAME="Amazon Linux 2023"`,
	"centos7": `NAME="CentOS Linux"
ID="centos"
ID_LIKE="rhel fedora"
VERSION_ID="7"
PRETTY_NAME="CentOS Linux 7 (Core)"`,
	"tumbleweed": `NAME="openSUSE Tumbleweed"
ID="opensuse-tumbleweed"
ID_LIKE="opensuse suse"
PRETTY_NAME="openSUSE Tumbleweed"`,
	"leap15": `NAME="openSUSE Leap"
VERSION_ID="15.6"
ID="opensuse-leap"
ID_LIKE="suse opensuse"
PRETTY_NAME="openSUSE Leap 15.6"`,
	"sles15": `NAME="SLES"
VERSION_ID="15.6"
ID="sles"
ID_LIKE="suse"
PRETTY_NAME="SUSE Linux Enterprise Server 15 SP6"`,
	"ubuntu24": `PRETTY_NAME="Ubuntu 24.04.5 LTS"
NAME="Ubuntu"
VERSION_ID="24.04"
ID=ubuntu
ID_LIKE=debian`,
	"arch": `NAME="Arch Linux"
PRETTY_NAME="Arch Linux"
ID=arch`,
	"mystery": `NAME="Mystery OS"
ID=mystery`,
}

func tools(names ...string) func(string) bool {
	return func(p string) bool {
		for _, n := range names {
			if n == p {
				return true
			}
		}
		return false
	}
}

func TestParseOSRelease(t *testing.T) {
	d := ParseOSRelease("# comment\n" + osReleases["rocky9"] + "\nBROKEN LINE\n")
	if d.ID != "rocky" || d.VersionID != "9.4" || d.Name != "Rocky Linux 9.4 (Blue Onyx)" || len(d.Like) != 3 || !d.Is("fedora") || d.Is("suse") {
		t.Errorf("rocky = %+v", d)
	}
	if d := ParseOSRelease("ID='opensuse-leap'\nNAME=openSUSE"); d.ID != "opensuse-leap" || d.Name != "openSUSE" {
		t.Errorf("single quotes / NAME fallback: %+v", d)
	}
}

func TestRouteRPMPerDistribution(t *testing.T) {
	for _, c := range []struct {
		distro  string
		tools   []string
		manager string // "" means refused
		reason  string
	}{
		{"fedora41", []string{"dnf", "rpm"}, "dnf", ""},
		{"rhel9", []string{"dnf"}, "dnf", ""},
		{"rocky9", []string{"dnf"}, "dnf", ""},
		{"amzn2023", []string{"dnf"}, "dnf", ""},
		{"centos7", []string{"yum"}, "", "uses yum"},
		{"fedora41", nil, "", "should have DNF"},
		{"tumbleweed", []string{"zypper"}, "zypper", ""},
		{"leap15", []string{"zypper"}, "zypper", ""},
		{"sles15", []string{"zypper"}, "zypper", ""},
		// openSUSE with DNF installed still uses Zypper: go by the distribution.
		{"tumbleweed", []string{"zypper", "dnf"}, "zypper", ""},
		{"leap15", []string{"dnf"}, "", "Zypper, which isn't installed"},
		// RPM tools on a non-RPM distribution are never used to change packages.
		{"ubuntu24", []string{"rpm", "dnf"}, "", "doesn't manage its software with RPM"},
		{"arch", []string{"dnf"}, "", "doesn't manage its software with RPM"},
		{"mystery", []string{"dnf", "zypper"}, "", "doesn't recognise Mystery OS"},
	} {
		r := RouteRPM(ParseOSRelease(osReleases[c.distro]), tools(c.tools...))
		if r.Manager != c.manager || (c.reason != "" && !strings.Contains(r.Reason, c.reason)) || (c.manager == "" && r.Reason == "") {
			t.Errorf("%s with %v: %+v, want manager %q reason ~%q", c.distro, c.tools, r, c.manager, c.reason)
		}
	}
	if r := RouteRPM(Distro{}, tools("dnf")); r.Manager != "" || !strings.Contains(r.Reason, "couldn't tell") {
		t.Errorf("no os-release: %+v", r)
	}
}
