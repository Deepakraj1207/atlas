// Command release prepares release files that aren't Go builds.
//
//	go run ./tools/release winget -version 0.1.0 -date 2026-11-13 \
//	    -x64 dist/atlas_0.1.0_windows_amd64.zip -arm64 dist/atlas_0.1.0_windows_arm64.zip \
//	    -out dist/winget
//
// writes the three winget manifests (version, installer, en-US locale) under
// <out>/manifests/d/Deepakraj1207/ATLAS/<version>/, the layout the
// microsoft/winget-pkgs repository uses. Nothing is uploaded or submitted.
package main

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"
)

// Package identity. The identifier is permanent once submitted to winget.
const (
	identifier = "Deepakraj1207.ATLAS"
	publisher  = "Deepakraj1207"
	repo       = "Deepakraj1207/atlas"
	exeName    = "atlas.exe"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "winget" {
		fmt.Fprintln(os.Stderr, "usage: release winget -version X -date YYYY-MM-DD -x64 FILE.zip [-arm64 FILE.zip] -out DIR")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("winget", flag.ExitOnError)
	version := fs.String("version", "", "package version, e.g. 0.1.0")
	date := fs.String("date", time.Now().UTC().Format("2006-01-02"), "release date, YYYY-MM-DD")
	x64 := fs.String("x64", "", "Windows amd64 zip")
	arm64 := fs.String("arm64", "", "Windows arm64 zip (optional)")
	out := fs.String("out", "dist/winget", "output folder")
	fs.Parse(os.Args[2:])

	dir, err := writeWinget(Options{Version: *version, Date: *date, X64Zip: *x64, Arm64Zip: *arm64, Out: *out})
	if err != nil {
		fmt.Fprintln(os.Stderr, "release:", err)
		os.Exit(1)
	}
	fmt.Println("winget manifests written to", dir)
}

// Options describe one release.
type Options struct {
	Version, Date    string
	X64Zip, Arm64Zip string
	Out              string
}

type installer struct {
	Arch, URL, SHA256 string
}

var (
	reVersion = regexp.MustCompile(`^\d+\.\d+\.\d+([-+.][0-9A-Za-z.-]+)?$`)
	reDate    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

func writeWinget(o Options) (string, error) {
	if !reVersion.MatchString(o.Version) {
		return "", fmt.Errorf("version %q isn't like 0.1.0", o.Version)
	}
	if !reDate.MatchString(o.Date) {
		return "", fmt.Errorf("date %q isn't YYYY-MM-DD", o.Date)
	}
	if o.X64Zip == "" {
		return "", errors.New("the x64 zip is required")
	}
	var installers []installer
	for _, z := range []struct{ arch, path string }{{"x64", o.X64Zip}, {"arm64", o.Arm64Zip}} {
		if z.path == "" {
			continue
		}
		if err := checkZip(z.path); err != nil {
			return "", err
		}
		sum, err := sha256File(z.path)
		if err != nil {
			return "", err
		}
		installers = append(installers, installer{
			Arch:   z.arch,
			URL:    fmt.Sprintf("https://github.com/%s/releases/download/v%s/%s", repo, o.Version, filepath.Base(z.path)),
			SHA256: sum,
		})
	}

	dir := filepath.Join(o.Out, "manifests", strings.ToLower(identifier[:1]), strings.ReplaceAll(identifier, ".", string(filepath.Separator)), o.Version)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data := map[string]any{
		"ID": identifier, "Version": o.Version, "Date": o.Date, "Publisher": publisher,
		"Repo": repo, "Exe": exeName, "Installers": installers,
	}
	for name, tmpl := range map[string]*template.Template{
		identifier + ".yaml":              versionTmpl,
		identifier + ".installer.yaml":    installerTmpl,
		identifier + ".locale.en-US.yaml": localeTmpl,
	} {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			return "", err
		}
		err = tmpl.Execute(f, data)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
	}
	return dir, nil
}

// checkZip makes sure the manifest's NestedInstallerFiles path exists: the
// zip must hold atlas.exe at its top level.
func checkZip(path string) error {
	r, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name == exeName {
			return nil
		}
	}
	return fmt.Errorf("%s has no %s at its top level", path, exeName)
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(h.Sum(nil))), nil
}

const schema = "# yaml-language-server: $schema=https://aka.ms/winget-manifest.%s.1.6.0.schema.json\n\n"

var versionTmpl = template.Must(template.New("version").Parse(fmt.Sprintf(schema, "version") + `PackageIdentifier: {{.ID}}
PackageVersion: {{.Version}}
DefaultLocale: en-US
ManifestType: version
ManifestVersion: 1.6.0
`))

var installerTmpl = template.Must(template.New("installer").Parse(fmt.Sprintf(schema, "installer") + `PackageIdentifier: {{.ID}}
PackageVersion: {{.Version}}
InstallerType: zip
NestedInstallerType: portable
NestedInstallerFiles:
- RelativeFilePath: {{.Exe}}
  PortableCommandAlias: atlas
Commands:
- atlas
ReleaseDate: {{.Date}}
Installers:
{{- range .Installers}}
- Architecture: {{.Arch}}
  InstallerUrl: {{.URL}}
  InstallerSha256: {{.SHA256}}
{{- end}}
ManifestType: installer
ManifestVersion: 1.6.0
`))

var localeTmpl = template.Must(template.New("locale").Parse(fmt.Sprintf(schema, "defaultLocale") + `PackageIdentifier: {{.ID}}
PackageVersion: {{.Version}}
PackageLocale: en-US
Publisher: {{.Publisher}}
PublisherUrl: https://github.com/{{.Publisher}}
PublisherSupportUrl: https://github.com/{{.Repo}}/issues
PackageName: ATLAS
PackageUrl: https://github.com/{{.Repo}}
License: MIT
LicenseUrl: https://github.com/{{.Repo}}/blob/v{{.Version}}/LICENSE
ShortDescription: Understand, update and tidy the software on your computer, in plain language.
Description: |-
  ATLAS is a terminal app that shows what is installed on your computer and helps you keep it
  tidy and up to date. It reads Windows' installed-programs list, winget, Chocolatey and Scoop,
  checks for updates, points out runtimes that no longer get security fixes, duplicates and
  unused apps, and installs or removes software through your own package managers. Nothing
  happens without a preview you confirm, and every action is recorded.
Moniker: atlas
Tags:
- cleanup
- inventory
- package-manager
- terminal
- tui
- updates
ReleaseNotesUrl: https://github.com/{{.Repo}}/releases/tag/v{{.Version}}
ManifestType: defaultLocale
ManifestVersion: 1.6.0
`))
