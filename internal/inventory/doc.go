// Package inventory finds installed software. Every scanner is read-only:
// it reads files, the Windows registry, or the output of a package
// manager's "list" command. Nothing here installs, removes, updates or
// changes anything, and no scanner asks for elevated privileges.
//
// Parsing is kept in pure functions so the Windows parsers are tested on
// any platform. Only the registry enumeration is Windows-specific.
package inventory

// Source IDs. They are stored in the database, so never rename them.
const (
	SourceRegistry   = "windows-registry"
	SourceWinget     = "winget"
	SourceScoop      = "scoop"
	SourceChocolatey = "chocolatey"
	SourceDpkg       = "dpkg"
	SourceRPM        = "rpm"
	SourceSnap       = "snap"
	SourceFlatpak    = "flatpak"
)
