package screens

import (
	"fmt"

	"github.com/deepakraj1207/atlas/internal/core"
)

// QuickAction is one of the Home screen buttons.
type QuickAction struct {
	Label   string
	Plan    core.ActionPlan
	Confirm any
}

// ScanPlan describes "Scan My System" for the current state.
func ScanPlan(ctx Context) core.ActionPlan {
	p := core.ActionPlan{
		Title:       "Scan My System",
		WhatHappens: "ATLAS will read this computer's list of installed programs and the package managers it supports, then save a list you can search in My Software.",
		Why:         "You can't keep software tidy or up to date without knowing what is there.",
		Safety:      "Safe. A scan only reads information. It never installs, removes or changes software, and it doesn't ask for administrator rights.",
		Reversible:  "There is nothing to undo on your computer. ATLAS keeps the list in its own database; you can clear it from Settings at any time.",
		Available:   true,
	}
	for _, s := range ctx.Sources {
		p.Technical = append(p.Technical, s.Label+": "+s.Detail)
	}
	p.Technical = append(p.Technical,
		"Sources run in parallel, each with a time limit. A source that fails is reported and left out.",
		"Results are saved to "+ctx.DBPath+". Nothing leaves this computer.")
	switch {
	case !ctx.Data.Available:
		p.Available = false
		p.Unavailability = "ATLAS can't open its saved data, so results couldn't be kept. See Settings for details."
	case ctx.Data.Scan != nil:
		p.Available = false
		p.Unavailability = "a scan is already running."
	}
	return p
}

// ClearPlan describes clearing the saved software list.
func ClearPlan(ctx Context) core.ActionPlan {
	what := "ATLAS's saved software list is deleted from its own database"
	if s := ctx.Data.Summary; s != nil {
		what = fmt.Sprintf("ATLAS's saved list of %d apps and %d system components is deleted from its own database", s.AppCount, s.ComponentCount)
	}
	p := core.ActionPlan{
		Title:       "Clear saved software list",
		WhatHappens: "ATLAS will delete the list of software it saved from your last scan.",
		Why:         "Useful if you want to start fresh, or don't want ATLAS to keep this list.",
		WhatChanges: []string{what},
		Safety:      "Safe. No software on your computer is installed, removed or changed. Activity History is kept.",
		Reversible:  "Not directly, but running a new scan rebuilds the list.",
		Technical:   []string{"Deletes all rows from the software, scans and scan_sources tables in " + ctx.DBPath},
		Available:   true,
	}
	switch {
	case !ctx.Data.Available:
		p.Available, p.Unavailability = false, "ATLAS can't open its saved data. See Settings for details."
	case ctx.Data.Scan != nil:
		p.Available, p.Unavailability = false, "a scan is running. Wait for it to finish or stop it with x."
	case ctx.Data.Summary == nil:
		p.Available, p.Unavailability = false, "there is no saved list to clear."
	}
	return p
}

// quickActions are the Home buttons. Only scanning is available in Phase 2;
// the others explain what they will do and when they arrive.
func quickActions(ctx Context) []QuickAction {
	return []QuickAction{
		{Label: "Scan My System", Plan: ScanPlan(ctx), Confirm: StartScanMsg{}},
		{
			Label: "Check for Updates",
			Plan: core.ActionPlan{
				Title:       "Check for Updates",
				WhatHappens: "ATLAS will ask each supported package source whether newer versions exist for the software it found.",
				Why:         "Newer versions fix bugs and security problems.",
				Safety:      "Safe. Checking only reads version information. Installing an update is a separate step that always asks first.",
				Reversible:  "There is nothing to undo; no update is installed by this action.",
				Technical: []string{
					"winget upgrade --list, scoop status, choco outdated",
					"apt list --upgradable, snap refresh --list, flatpak remote-ls --updates",
				},
				Unavailability: "update checks arrive in Phase 3.",
			},
		},
		{
			Label: "Analyze My Software",
			Plan: core.ActionPlan{
				Title:          "Analyze My Software",
				WhatHappens:    "ATLAS will review your software list and explain, in plain language, what looks duplicated, unused or risky.",
				Why:            "It helps you decide what to keep without reading technical documentation.",
				Safety:         "Safe. Analysis only reads the saved list and never changes anything.",
				Reversible:     "There is nothing to undo.",
				Technical:      []string{"Rules-based analysis first; optional AI explanations later, off by default"},
				Unavailability: "analysis arrives in Phase 4.",
			},
		},
		{
			Label: "Find New Tools",
			Plan: core.ActionPlan{
				Title:          "Find New Tools",
				WhatHappens:    "ATLAS will suggest well-regarded tools related to the kinds of software you already use.",
				Why:            "Discovering useful tools is hard when you don't know their names.",
				Safety:         "Safe. Suggestions are only shown; nothing is downloaded or installed unless you choose to, and that will always ask first.",
				Reversible:     "There is nothing to undo.",
				Technical:      []string{"Curated catalogue matched against installed categories; no telemetry"},
				Unavailability: "discovery arrives in Phase 6.",
			},
		},
	}
}
