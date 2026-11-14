package platform

import (
	"errors"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
)

// Distro is what /etc/os-release says about a Linux system.
type Distro struct {
	ID        string   // e.g. "fedora", "opensuse-tumbleweed", "ubuntu"
	Like      []string // ID_LIKE, e.g. ["rhel", "centos", "fedora"]
	Name      string   // PRETTY_NAME or NAME, for messages
	VersionID string
}

// ParseOSRelease reads the KEY=value format of os-release(5).
func ParseOSRelease(data string) Distro {
	v := map[string]string{}
	for _, l := range strings.Split(data, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		k, val, ok := strings.Cut(l, "=")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		v[strings.TrimSpace(k)] = val
	}
	d := Distro{ID: strings.ToLower(v["ID"]), Name: v["PRETTY_NAME"], VersionID: v["VERSION_ID"]}
	if d.Name == "" {
		d.Name = v["NAME"]
	}
	for _, l := range strings.Fields(strings.ToLower(v["ID_LIKE"])) {
		d.Like = append(d.Like, l)
	}
	return d
}

// ReadOSRelease reads /etc/os-release, falling back to /usr/lib/os-release.
func ReadOSRelease() (Distro, error) {
	for _, p := range []string{"/etc/os-release", "/usr/lib/os-release"} {
		if data, err := os.ReadFile(p); err == nil {
			return ParseOSRelease(string(data)), nil
		}
	}
	return Distro{}, errors.New("no os-release file")
}

// Is reports whether the distribution is family or derives from it.
func (d Distro) Is(family string) bool {
	return d.ID == family || slices.Contains(d.Like, family)
}

func (d Distro) suse() bool {
	return d.Is("suse") || d.Is("opensuse") || strings.HasPrefix(d.ID, "opensuse") || d.ID == "sles" || d.ID == "sled"
}

func (d Distro) redHat() bool {
	return d.Is("fedora") || d.Is("rhel") || d.Is("centos")
}

func (d Distro) name() string {
	if d.Name != "" {
		return d.Name
	}
	if d.ID != "" {
		return d.ID
	}
	return "this system"
}

// RPMRoute says which tool may change RPM packages on this system: "dnf",
// "zypper", or none, with Reason explaining why ATLAS refuses.
type RPMRoute struct {
	Manager string
	Reason  string
	// RPMBased is true on distributions whose own software is RPM
	// (Fedora, RHEL and relatives, openSUSE, SLES), even when ATLAS refuses.
	RPMBased bool
}

// RouteRPM picks the distribution's own tool for RPM packages. It goes by
// the distribution, not by whichever tool happens to be installed: openSUSE
// with DNF installed still uses Zypper. When the native tool is missing or
// unsupported, or the distribution isn't recognised, it refuses rather than
// guesses. has reports whether a program is on PATH.
func RouteRPM(d Distro, has func(string) bool) RPMRoute {
	switch {
	case d.suse():
		if has("zypper") {
			return RPMRoute{Manager: "zypper", RPMBased: true}
		}
		return RPMRoute{RPMBased: true, Reason: d.name() + " manages packages with Zypper, which isn't installed, so ATLAS won't change RPM packages here."}
	case d.redHat():
		if has("dnf") {
			return RPMRoute{Manager: "dnf", RPMBased: true}
		}
		if has("yum") {
			return RPMRoute{RPMBased: true, Reason: d.name() + " uses yum. ATLAS only changes RPM packages with DNF or Zypper, so it won't change them here."}
		}
		return RPMRoute{RPMBased: true, Reason: d.name() + " should have DNF, but it isn't installed, so ATLAS won't change RPM packages here."}
	case d.ID == "":
		return RPMRoute{Reason: "ATLAS couldn't tell which Linux distribution this is, so it won't guess which tool should change RPM packages."}
	case d.Is("debian") || d.Is("ubuntu") || d.Is("arch") || d.Is("alpine"):
		return RPMRoute{Reason: d.name() + " doesn't manage its software with RPM, so ATLAS won't change RPM packages here."}
	}
	return RPMRoute{Reason: "ATLAS doesn't recognise " + d.name() + ", so it won't guess which tool should change RPM packages."}
}

var (
	rpmOnce  sync.Once
	rpmRoute RPMRoute
)

// LinuxRPMRoute detects this computer's RPM route once.
func LinuxRPMRoute() RPMRoute {
	rpmOnce.Do(func() {
		d, err := ReadOSRelease()
		if err != nil {
			rpmRoute = RPMRoute{Reason: "ATLAS couldn't read /etc/os-release, so it won't guess which tool should change RPM packages."}
			return
		}
		rpmRoute = RouteRPM(d, func(p string) bool { _, err := exec.LookPath(p); return err == nil })
	})
	return rpmRoute
}
