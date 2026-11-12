package inventory

import (
	"bufio"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
)

type dpkgScanner struct{ statusPath, extendedStatesPath string }

// NewDpkgScanner reads dpkg's status database directly (the same file
// dpkg-query reads) and apt's extended_states to tell packages the user
// asked for from those installed automatically as dependencies.
func NewDpkgScanner(statusPath, extendedStatesPath string) core.Scanner {
	return dpkgScanner{statusPath: statusPath, extendedStatesPath: extendedStatesPath}
}

func (dpkgScanner) ID() string    { return SourceDpkg }
func (dpkgScanner) Label() string { return "Debian/Ubuntu packages" }
func (s dpkgScanner) Detail() string {
	return "Reads " + s.statusPath + " and " + s.extendedStatesPath
}

func (s dpkgScanner) Scan(ctx context.Context) ([]core.SoftwareItem, error) {
	f, err := os.Open(s.statusPath)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, core.ErrSourceUnavailable
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()

	auto := map[string]bool{}
	if ef, err := os.Open(s.extendedStatesPath); err == nil {
		auto = parseExtendedStates(ef)
		ef.Close()
	}
	return parseDpkgStatus(ctx, f, auto)
}

// stanzas reads RFC 822 style paragraphs, keeping only the first line of
// multi-line fields.
func stanzas(r io.Reader, each func(map[string]string) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 4*1024*1024)
	cur := map[string]string{}
	flush := func() error {
		if len(cur) == 0 {
			return nil
		}
		err := each(cur)
		cur = map[string]string{}
		return err
	}
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			if err := flush(); err != nil {
				return err
			}
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			continue // continuation line
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		cur[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	if err := sc.Err(); err != nil {
		return err
	}
	return flush()
}

func parseExtendedStates(r io.Reader) map[string]bool {
	auto := map[string]bool{}
	_ = stanzas(r, func(f map[string]string) error {
		if f["Auto-Installed"] == "1" && f["Package"] != "" {
			auto[f["Package"]] = true
			if a := f["Architecture"]; a != "" {
				auto[f["Package"]+":"+a] = true
			}
		}
		return nil
	})
	return auto
}

func parseDpkgStatus(ctx context.Context, r io.Reader, auto map[string]bool) ([]core.SoftwareItem, error) {
	var items []core.SoftwareItem
	err := stanzas(r, func(f map[string]string) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		status := strings.Fields(f["Status"])
		if len(status) < 3 || status[2] != "installed" || f["Package"] == "" {
			return nil
		}
		pkg := f["Package"]
		kind := classifyLinuxPackage(pkg, f["Section"], f["Priority"], f["Essential"] == "yes")
		if auto[pkg] || auto[pkg+":"+f["Architecture"]] {
			kind = core.KindComponent
		}
		publisher := f["Maintainer"]
		if i := strings.Index(publisher, "<"); i > 0 {
			publisher = strings.TrimSpace(publisher[:i])
		}
		items = append(items, core.SoftwareItem{
			Name:        pkg,
			Version:     f["Version"],
			Publisher:   publisher,
			PackageID:   pkg,
			Description: f["Description"],
			Source:      SourceDpkg,
			Kind:        kind,
			Scope:       "machine",
		})
		return nil
	})
	return items, err
}
