package screens

import (
	"fmt"
	"strings"

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
	case ctx.Data.Check != nil:
		p.Available = false
		p.Unavailability = "an update check is running. Wait for it to finish, or stop it with x."
	}
	return p
}

// CheckPlan describes "Check for Updates".
func CheckPlan(ctx Context) core.ActionPlan {
	p := core.ActionPlan{
		Title:       "Check for Updates",
		WhatHappens: "ATLAS will ask each package manager on this computer which installed software has a newer version, and save the answer so you can review it in Updates.",
		Why:         "Newer versions fix bugs and security problems.",
		Safety:      "Safe. Checking only reads version information; nothing is downloaded or installed. Some package managers contact their online catalogue to answer.",
		Reversible:  "There is nothing to undo. Installing an update is a separate step that always shows you exactly what it will do first.",
		Available:   true,
	}
	for _, m := range ctx.Managers {
		p.Technical = append(p.Technical, m.Label+": "+m.Detail)
	}
	p.Technical = append(p.Technical,
		"APT and DNF use the package information your system last downloaded. Scoop compares with its local bucket copies.",
		"Each package manager has a time limit. One that fails is reported and left out.")
	switch {
	case !ctx.Data.UpdatesAvailable:
		p.Available, p.Unavailability = false, "ATLAS can't open its saved data. See Settings for details."
	case ctx.Data.Check != nil:
		p.Available, p.Unavailability = false, "an update check is already running."
	case ctx.Data.Scan != nil:
		p.Available, p.Unavailability = false, "a scan is running. Wait for it to finish, or stop it with x."
	}
	return p
}

// ApplyPlan describes installing the planned update steps. It lists every
// change, how permission will be asked for, and how to go back.
func ApplyPlan(ctx Context, steps []core.UpdateStep) core.ActionPlan {
	var runnable []core.Update
	var changes, blocked, technical []string
	elevations := map[core.Elevation]bool{}
	var rollbacks []string
	seenRollback := map[string]bool{}
	for _, st := range steps {
		if st.Blocked != "" {
			for _, u := range st.Updates {
				blocked = append(blocked, "Not included: "+u.Name+". "+st.Blocked)
			}
			continue
		}
		elevations[st.Elevation] = true
		for _, u := range st.Updates {
			runnable = append(runnable, u)
			changes = append(changes, updateLine(u)+" · "+st.Label)
		}
		for _, c := range st.Commands {
			technical = append(technical, c.String())
		}
		if st.Rollback != "" && !seenRollback[st.Label] {
			seenRollback[st.Label] = true
			rollbacks = append(rollbacks, st.Label+": "+strings.TrimSuffix(st.Rollback, ".")+".")
		}
	}

	const maxLines = 12
	if len(changes) > maxLines {
		more := len(changes) - maxLines
		changes = append(changes[:maxLines], fmt.Sprintf("…and %d more", more))
	}
	title := fmt.Sprintf("Install %d updates", len(runnable))
	if len(runnable) == 1 {
		title = "Update " + runnable[0].Name
	}

	safety := []string{"Updates come from the same place the software was installed from. An installer may close an app while it updates, so save your work first."}
	if elevations[core.ElevationSudo] {
		safety = append(safety, "Some steps run through sudo, which will ask for your password in the terminal. ATLAS never sees or stores it.")
	}
	if elevations[core.ElevationInstaller] {
		safety = append(safety, "Windows may ask for permission (a User Account Control prompt) if an installer needs it. Approve only what you expect.")
	}
	if elevations[core.ElevationPolkit] {
		safety = append(safety, "Flatpak may ask for your password to update apps installed for everyone.")
	}
	if elevations[core.ElevationAdmin] {
		safety = append(safety, "ATLAS is running with administrator rights, so these updates run with full rights.")
	}
	safety = append(safety, "Each package manager shows its own messages and asks its own questions. Press Ctrl+C to stop; nothing further starts after that.")

	p := core.ActionPlan{
		Title:       title,
		WhatHappens: "ATLAS will hand this terminal to each package manager in turn, so you see exactly what it does and can answer its questions. When it's finished, you'll come back here and the result is recorded in Activity History.",
		Why:         "Updates fix bugs and security problems.",
		WhatChanges: append(changes, blocked...),
		Safety:      strings.Join(safety, " "),
		Reversible:  "Not automatically. " + strings.Join(rollbacks, " "),
		Technical:   technical,
		Available:   len(runnable) > 0,
	}
	switch {
	case len(runnable) == 0 && len(blocked) > 0:
		p.Unavailability = "none of these updates can be installed from here (see below)."
	case len(runnable) == 0:
		p.Unavailability = "there is nothing to update."
	case ctx.Data.Busy():
		p.Available = false
		p.Unavailability = "a scan or update check is running. Wait for it to finish, or stop it with x."
	}
	return p
}

func updateLine(u core.Update) string {
	switch {
	case u.CurrentVersion != "":
		return u.Name + ": " + u.CurrentVersion + " → " + u.AvailableVersion
	default:
		return u.Name + ": → " + u.AvailableVersion
	}
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
	case ctx.Data.Busy():
		p.Available, p.Unavailability = false, "a scan or update check is running. Wait for it to finish or stop it with x."
	case ctx.Data.Summary == nil:
		p.Available, p.Unavailability = false, "there is no saved list to clear."
	}
	return p
}

// quickActions are the Home buttons. Scanning and checking for updates
// work; the others explain what they will do and when they arrive.
func quickActions(ctx Context) []QuickAction {
	return []QuickAction{
		{Label: "Scan My System", Plan: ScanPlan(ctx), Confirm: StartScanMsg{}},
		{Label: "Check for Updates", Plan: CheckPlan(ctx), Confirm: StartCheckMsg{}},
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
