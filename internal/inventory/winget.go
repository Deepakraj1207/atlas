package inventory

import (
	"context"
	"errors"
	"strings"

	"github.com/charmbracelet/x/ansi"
	"github.com/mattn/go-runewidth"

	"github.com/deepakraj1207/atlas/internal/core"
)

type wingetScanner struct{ run Runner }

// NewWingetScanner lists packages with "winget list". The flag
// --disable-interactivity keeps winget from waiting on prompts; ATLAS never
// accepts source agreements on the user's behalf.
func NewWingetScanner(run Runner) core.Scanner { return wingetScanner{run: run} }

func (wingetScanner) ID() string     { return SourceWinget }
func (wingetScanner) Label() string  { return "Windows Package Manager" }
func (wingetScanner) Detail() string { return "Runs: winget list --disable-interactivity" }

func (s wingetScanner) Scan(ctx context.Context) ([]core.SoftwareItem, error) {
	out, err := s.run(ctx, "winget", "list", "--disable-interactivity")
	if err != nil && (errors.Is(err, core.ErrSourceUnavailable) || ctx.Err() != nil) {
		return nil, err
	}
	items, perr := parseWingetList(string(out))
	if len(items) > 0 {
		// winget sometimes exits non-zero after printing a usable table
		// (for example when one source needs attention).
		return items, nil
	}
	if err != nil {
		return nil, WingetError(err)
	}
	return items, perr
}

var errNoTable = errors.New("winget output did not contain a package table")

// WingetRow is one row of a winget table. Available is empty for
// "winget list" rows that have no upgrade.
type WingetRow struct {
	Name, ID, Version, Available, Source string
}

// cleanWingetLines removes spinner frames, ANSI codes and backspaces.
func cleanWingetLines(out string) []string {
	raw := strings.Split(out, "\n")
	lines := make([]string, 0, len(raw))
	for _, l := range raw {
		// winget draws a spinner with carriage returns; keep what was
		// printed last on each line.
		if i := strings.LastIndex(l, "\r"); i >= 0 {
			if strings.TrimSpace(l[i+1:]) == "" {
				l = l[:i]
				if j := strings.LastIndex(l, "\r"); j >= 0 {
					l = l[j+1:]
				}
			} else {
				l = l[i+1:]
			}
		}
		l = strings.ReplaceAll(ansi.Strip(l), "\b", "")
		lines = append(lines, strings.TrimRight(l, " \t"))
	}
	return lines
}

// ParseWingetTables parses every fixed-width table in winget output.
// "winget upgrade" can print two: normal upgrades, then packages that
// "require explicit targeting". Column positions come from the header line
// above each dashed rule, so localized header words still work. Widths are
// measured in terminal cells so wide characters line up as winget printed.
func ParseWingetTables(out string) ([][]WingetRow, error) {
	lines := cleanWingetLines(out)
	var tables [][]WingetRow
	for i := 1; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if len(t) < 10 || strings.Trim(t, "-") != "" {
			continue
		}
		starts := columnStarts(lines[i-1])
		if len(starts) < 3 {
			continue
		}
		availCol, sourceCol := -1, -1
		switch {
		case len(starts) >= 5:
			availCol, sourceCol = 3, 4
		case len(starts) == 4:
			sourceCol = 3
		}
		var rows []WingetRow
		j := i + 1
		for ; j < len(lines); j++ {
			l := lines[j]
			if strings.TrimSpace(l) == "" {
				break
			}
			if runewidth.StringWidth(l) <= starts[1] {
				continue // footer such as "3 upgrades available."
			}
			cols := cutColumns(l, starts)
			if cols[0] == "" {
				continue
			}
			r := WingetRow{Name: cols[0], ID: cols[1], Version: cols[2]}
			if availCol >= 0 {
				r.Available = cols[availCol]
			}
			if sourceCol >= 0 {
				r.Source = cols[sourceCol]
			}
			rows = append(rows, r)
		}
		tables = append(tables, rows)
		i = j
	}
	if len(tables) == 0 {
		if strings.TrimSpace(strings.Join(lines, "")) == "" {
			return nil, nil
		}
		return nil, errNoTable
	}
	return tables, nil
}

// parseWingetList maps "winget list" rows to inventory items.
func parseWingetList(out string) ([]core.SoftwareItem, error) {
	tables, err := ParseWingetTables(out)
	if err != nil {
		return nil, err
	}
	var items []core.SoftwareItem
	for _, rows := range tables {
		for _, r := range rows {
			it := core.SoftwareItem{
				Name:    r.Name,
				Version: strings.TrimPrefix(strings.TrimPrefix(r.Version, "< "), "> "),
				Source:  SourceWinget,
				Kind:    classifyWindowsName(r.Name),
			}
			// IDs only help later when they map to an installable package.
			// Local IDs (ARP\..., MSIX\...) and truncated IDs are dropped.
			if r.Source != "" && !truncated(r.ID) {
				it.PackageID = r.ID
			}
			it.Origin = r.Source
			if r.Source == "" {
				if kind, _, ok := strings.Cut(r.ID, `\`); ok {
					it.Origin = strings.ToUpper(kind) // "ARP" or "MSIX"
				}
			}
			if strings.HasPrefix(r.ID, "MSIX\\") && it.Kind == core.KindApp && strings.HasPrefix(r.Name, "Microsoft.") {
				it.Kind = core.KindComponent
			}
			items = append(items, it)
		}
	}
	return items, nil
}

// Truncated reports whether winget shortened a value with an ellipsis.
func Truncated(s string) bool { return truncated(s) }

// columnStarts returns the cell offsets where header words begin.
func columnStarts(header string) []int {
	var starts []int
	col := 0
	prevSpace := true
	for _, r := range header {
		space := r == ' ' || r == '\t'
		if !space && prevSpace {
			starts = append(starts, col)
		}
		prevSpace = space
		col += runewidth.RuneWidth(r)
	}
	return starts
}

// cutColumns splits a line at the given cell offsets.
func cutColumns(line string, starts []int) []string {
	cols := make([]strings.Builder, len(starts))
	col := 0
	idx := 0
	for _, r := range line {
		for idx+1 < len(starts) && col >= starts[idx+1] {
			idx++
		}
		cols[idx].WriteRune(r)
		col += runewidth.RuneWidth(r)
	}
	out := make([]string, len(starts))
	for i := range cols {
		out[i] = strings.TrimSpace(cols[i].String())
	}
	return out
}

// wingetAgreementsPending is winget's exit code when a source's terms have
// not been accepted (APPINSTALLER_CLI_ERROR_SOURCE_AGREEMENTS_NOT_AGREED_TO).
const wingetAgreementsPending = 0x8A150046

// WingetError turns winget's own failures into plain explanations. ATLAS
// never accepts agreements on the user's behalf, so it says how to do it.
func WingetError(err error) error {
	var ce *CommandError
	if errors.As(err, &ce) && uint32(ce.ExitCode) == wingetAgreementsPending {
		return &core.SourceError{
			Message: "Windows Package Manager is waiting for you to accept its source agreements",
			Details: "open a terminal, run 'winget list' once and answer the question; ATLAS never accepts agreements for you (winget exit code 0x8A150046)",
		}
	}
	return err
}
