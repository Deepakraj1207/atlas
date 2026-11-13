// Command catalogcheck confirms that every package ID in the Discover Apps
// catalogue still exists, by asking each package manager (or, for Flatpak,
// Flathub's public API). It only looks things up; it installs nothing.
//
//	go run ./tools/catalogcheck -managers winget,scoop,chocolatey
//	go run ./tools/catalogcheck -managers apt,snap,flatpak
//	go run ./tools/catalogcheck -managers dnf
//
// It prints a Markdown table and exits 1 if any ID wasn't found. Run it by
// hand or from the catalog-check workflow; ATLAS never does this at runtime.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/deepakraj1207/atlas/internal/discover"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

// Result of looking up one package.
type Result struct {
	Manager, Tool, ID string
	Status            string // "found", "missing" or "not checked"
	Detail            string
}

// Checker looks up package IDs. Fields are replaceable in tests.
type Checker struct {
	Run       inventory.Runner
	HTTPGet   func(ctx context.Context, url string) (int, error)
	ScoopRoot string
	Timeout   time.Duration
}

func main() {
	managers := flag.String("managers", "", "comma-separated managers to check, e.g. winget,scoop,chocolatey")
	listBuckets := flag.Bool("list-scoop-buckets", false, "print the Scoop buckets the catalogue uses, then exit")
	flag.Parse()

	cat, err := discover.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "catalogue:", err)
		os.Exit(2)
	}
	if *listBuckets {
		fmt.Println(strings.Join(scoopBuckets(cat), " "))
		return
	}
	if *managers == "" {
		fmt.Fprintln(os.Stderr, "usage: catalogcheck -managers winget,scoop,...")
		os.Exit(2)
	}
	c := Checker{Run: inventory.ExecRunner, HTTPGet: httpGet, ScoopRoot: scoopRoot(), Timeout: 2 * time.Minute}
	results := c.Check(context.Background(), cat, strings.Split(*managers, ","))
	missing := Report(os.Stdout, cat.AsOf, results)
	if missing > 0 {
		os.Exit(1)
	}
}

// Check looks up every catalogue package for the given managers.
func (c Checker) Check(ctx context.Context, cat discover.Catalog, managers []string) []Result {
	want := map[string]bool{}
	for _, m := range managers {
		want[strings.TrimSpace(m)] = true
	}
	var out []Result
	for _, e := range cat.Entries {
		for _, p := range e.Packages {
			if !want[p.Manager] {
				continue
			}
			r := Result{Manager: p.Manager, Tool: e.Name, ID: p.ID}
			lctx, cancel := context.WithTimeout(ctx, c.Timeout)
			r.Status, r.Detail = c.lookup(lctx, p.Manager, p.ID)
			cancel()
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Manager < out[j].Manager })
	return out
}

func (c Checker) lookup(ctx context.Context, manager, id string) (status, detail string) {
	found := func(ok bool, detail string) (string, string) {
		if ok {
			return "found", detail
		}
		return "missing", detail
	}
	switch manager {
	case "winget":
		out, err := c.Run(ctx, "winget", "show", "--id", id, "--exact", "--source", "winget",
			"--accept-source-agreements", "--disable-interactivity")
		if err != nil && !isExit(err) {
			return "not checked", err.Error()
		}
		return found(err == nil && strings.Contains(string(out), "["+id+"]"), firstLine(out))
	case "chocolatey":
		out, err := c.Run(ctx, "choco", "search", id, "--exact", "--limit-output")
		if err != nil && !isExit(err) {
			return "not checked", err.Error()
		}
		return found(strings.HasPrefix(strings.ToLower(strings.TrimSpace(string(out))), strings.ToLower(id)+"|"), firstLine(out))
	case "scoop":
		bucket, app, _ := strings.Cut(id, "/")
		dir := filepath.Join(c.ScoopRoot, "buckets", bucket)
		if _, err := os.Stat(dir); err != nil {
			return "not checked", "the " + bucket + " bucket isn't added here"
		}
		for _, p := range []string{filepath.Join(dir, "bucket", app+".json"), filepath.Join(dir, app+".json")} {
			if _, err := os.Stat(p); err == nil {
				return "found", ""
			}
		}
		return "missing", "no " + app + ".json in the " + bucket + " bucket"
	case "apt":
		out, err := c.Run(ctx, "apt-cache", "show", "--no-all-versions", id)
		if err != nil && !isExit(err) {
			return "not checked", err.Error()
		}
		return found(err == nil && strings.Contains(string(out), "Package: "+id), "")
	case "dnf":
		out, err := c.Run(ctx, "dnf", "-q", "info", id)
		if err != nil && !isExit(err) {
			return "not checked", err.Error()
		}
		return found(err == nil && strings.Contains(string(out), id), "")
	case "snap":
		out, err := c.Run(ctx, "snap", "info", id)
		if err != nil && !isExit(err) {
			return "not checked", err.Error()
		}
		return found(err == nil && strings.Contains(string(out), "name:"), "")
	case "flatpak":
		code, err := c.HTTPGet(ctx, "https://flathub.org/api/v2/appstream/"+id)
		if err != nil {
			return "not checked", err.Error()
		}
		return found(code == http.StatusOK, fmt.Sprintf("Flathub answered %d", code))
	}
	return "not checked", "unknown manager"
}

// Report writes a Markdown table and returns how many IDs were missing.
func Report(w io.Writer, asOf string, results []Result) int {
	fmt.Fprintf(w, "# Catalogue check\n\nCatalogue as of %s. Checked %s.\n\n", asOf, time.Now().UTC().Format("2006-01-02 15:04 MST"))
	fmt.Fprintln(w, "| Result | Manager | Tool | Package ID | Details |")
	fmt.Fprintln(w, "|---|---|---|---|---|")
	counts := map[string]int{}
	for _, r := range results {
		counts[r.Status]++
		mark := map[string]string{"found": "ok", "missing": "MISSING", "not checked": "n/a"}[r.Status]
		fmt.Fprintf(w, "| %s | %s | %s | `%s` | %s |\n", mark, r.Manager, r.Tool, r.ID, strings.ReplaceAll(r.Detail, "|", "/"))
	}
	fmt.Fprintf(w, "\n%d found, %d missing, %d not checked.\n", counts["found"], counts["missing"], counts["not checked"])
	return counts["missing"]
}

func scoopBuckets(cat discover.Catalog) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range cat.Entries {
		for _, p := range e.Packages {
			if b, _, ok := strings.Cut(p.ID, "/"); ok && p.Manager == "scoop" && !seen[b] {
				seen[b] = true
				out = append(out, b)
			}
		}
	}
	sort.Strings(out)
	return out
}

func scoopRoot() string {
	if s := os.Getenv("SCOOP"); s != "" {
		return s
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "scoop")
}

// isExit reports whether the program ran and exited with an error code
// (usually "not found"), as opposed to not running at all.
func isExit(err error) bool {
	var ce *inventory.CommandError
	return errors.As(err, &ce) && ce.ExitCode >= 0
}

func firstLine(b []byte) string {
	for _, l := range strings.Split(string(b), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.ContainsAny(l[:1], `-\|/`) {
			if len(l) > 80 {
				l = l[:80] + "…"
			}
			return l
		}
	}
	return ""
}

func httpGet(ctx context.Context, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "atlas-catalogcheck")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}
