package inventory

import (
	"context"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
)

type rpmScanner struct{ run Runner }

// NewRPMScanner queries the RPM database (Fedora, RHEL, openSUSE, ...).
func NewRPMScanner(run Runner) core.Scanner { return rpmScanner{run: run} }

func (rpmScanner) ID() string    { return SourceRPM }
func (rpmScanner) Label() string { return "RPM packages" }
func (rpmScanner) Detail() string {
	return "Runs: rpm -qa --qf '%{NAME}\\t%{VERSION}-%{RELEASE}\\t%{VENDOR}\\t%{SUMMARY}\\n'"
}

const rpmFormat = "%{NAME}\t%{VERSION}-%{RELEASE}\t%{VENDOR}\t%{SUMMARY}\n"

func (s rpmScanner) Scan(ctx context.Context) ([]core.SoftwareItem, error) {
	out, err := s.run(ctx, "rpm", "-qa", "--qf", rpmFormat)
	if err != nil {
		return nil, err
	}
	return parseRPM(string(out)), nil
}

func parseRPM(out string) []core.SoftwareItem {
	var items []core.SoftwareItem
	for _, l := range strings.Split(out, "\n") {
		f := strings.Split(strings.TrimRight(l, "\r"), "\t")
		if len(f) < 2 || strings.TrimSpace(f[0]) == "" {
			continue
		}
		get := func(i int) string {
			if i < len(f) && f[i] != "(none)" {
				return strings.TrimSpace(f[i])
			}
			return ""
		}
		items = append(items, core.SoftwareItem{
			Name:        get(0),
			Version:     get(1),
			Publisher:   get(2),
			Description: get(3),
			PackageID:   get(0),
			Source:      SourceRPM,
			Kind:        classifyLinuxPackage(get(0), "", "", false),
			Scope:       "machine",
		})
	}
	return items
}
