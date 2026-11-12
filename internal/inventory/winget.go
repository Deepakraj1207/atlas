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
		return nil, err
	}
	return items, perr
}

var errNoTable = errors.New("winget output did not contain a package table")

// parseWingetList parses winget's fixed-width table. Column positions come
// from the header line above the dashed rule, so localized header words
// still work. Widths are measured in terminal cells so wide characters line
// up the way winget printed them.
func parseWingetList(out string) ([]core.SoftwareItem, error) {
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

	rule := -1
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if len(t) >= 10 && strings.Trim(t, "-") == "" && i > 0 {
			rule = i
			break
		}
	}
	if rule < 0 {
		if strings.TrimSpace(strings.Join(lines, "")) == "" {
			return nil, nil
		}
		return nil, errNoTable
	}
	header := lines[rule-1]
	starts := columnStarts(header)
	if len(starts) < 3 {
		return nil, errNoTable
	}
	sourceCol := -1
	switch {
	case len(starts) >= 5:
		sourceCol = 4
	case len(starts) == 4:
		sourceCol = 3
	}

	var items []core.SoftwareItem
	for _, l := range lines[rule+1:] {
		if strings.TrimSpace(l) == "" || runewidth.StringWidth(l) <= starts[1] {
			continue
		}
		cols := cutColumns(l, starts)
		name := cols[0]
		if name == "" {
			continue
		}
		src := ""
		if sourceCol >= 0 {
			src = cols[sourceCol]
		}
		it := core.SoftwareItem{
			Name:    name,
			Version: strings.TrimPrefix(strings.TrimPrefix(cols[2], "< "), "> "),
			Source:  SourceWinget,
			Kind:    classifyWindowsName(name),
		}
		id := cols[1]
		// IDs only help later when they map to an installable package.
		// Local IDs (ARP\..., MSIX\...) and truncated IDs are dropped.
		if src != "" && !truncated(id) {
			it.PackageID = id
		}
		if strings.HasPrefix(id, "MSIX\\") && it.Kind == core.KindApp && strings.HasPrefix(name, "Microsoft.") {
			it.Kind = core.KindComponent
		}
		items = append(items, it)
	}
	return items, nil
}

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
