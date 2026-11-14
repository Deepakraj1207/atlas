package screens

import (
	"fmt"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/nav"
)

// QuickAction is one entry of the Dashboard's "What would you like to do?"
// list. Plan actions open a preview first; the others open a section.
type QuickAction struct {
	Label string
	// Desc is a few words on what it does, shown beside the label.
	Desc    string
	Plan    core.ActionPlan
	Confirm any
	// Navigate opens Section instead of previewing a plan.
	Navigate bool
	Section  nav.Section
	// Search opens My Software with the search box active.
	Search bool
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
	case ctx.Data.Busy():
		p.Available = false
		p.Unavailability = "another task is running. Wait for it to finish, or stop it with x."
	}
	return p
}

// AnalyzePlan describes "Analyze My Software".
func AnalyzePlan(ctx Context) core.ActionPlan {
	happens := "ATLAS will review your saved software list for things worth knowing: runtimes whose support has ended or ends soon, apps installed twice or in several versions, and updates waiting. It also maps which runtimes other software needs, using your package managers' own records."
	reads := false
	for _, s := range ctx.AnalysisSources {
		if s.ID == "usage" {
			reads = true
		}
	}
	if reads {
		happens += " It also reads Windows' own record of when you last opened programs, to spot apps you haven't used in over a year."
	}
	p := core.ActionPlan{
		Title:       "Analyze My Software",
		WhatHappens: happens,
		Why:         "It helps you decide what to keep, update or replace, without reading technical documentation.",
		Safety:      "Safe. Analysis only reads information and never changes anything. Nothing leaves this computer.",
		Reversible:  "There is nothing to undo. Results are saved so you can come back to them.",
		Available:   true,
	}
	for _, s := range ctx.AnalysisSources {
		p.Technical = append(p.Technical, s.Label+": "+s.Detail)
	}
	p.Technical = append(p.Technical,
		"Support dates: built-in table for Python, Node.js, .NET and Visual C++ (as of 2026-11), with sources shown on each finding.",
		"Nothing is inferred from missing data: no usage record means no claim about usage.")
	switch {
	case !ctx.Data.AnalysisAvailable:
		p.Available, p.Unavailability = false, "ATLAS can't open its saved data. See Settings for details."
	case ctx.Data.Summary == nil:
		p.Available, p.Unavailability = false, "there is no software list to analyze yet. Scan your system first."
	case ctx.Data.Busy():
		p.Available, p.Unavailability = false, "another task is running. Wait for it to finish, or stop it with x."
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
	case ctx.Data.Busy():
		p.Available, p.Unavailability = false, "another task is running. Wait for it to finish, or stop it with x."
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
	case u.CurrentVersion != "" && core.NameAndVersion(u.Name, u.CurrentVersion) == u.Name:
		return u.Name + " → " + u.AvailableVersion
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
		WhatChanges: []string{what, "Smart Analysis results based on the list are removed too"},
		Safety:      "Safe. No software on your computer is installed, removed or changed. Activity History and update checks are kept.",
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

// quickActions are the Dashboard's actions. The first three preview a
// read-only task; the rest open the section that does the job.
func quickActions(ctx Context) []QuickAction {
	return []QuickAction{
		{Label: "Scan my system", Desc: "Find what's installed. Only reads", Plan: ScanPlan(ctx), Confirm: StartScanMsg{}},
		{Label: "Check for updates", Desc: "Newer versions from your package managers", Plan: CheckPlan(ctx), Confirm: StartCheckMsg{}},
		{Label: "Analyze installed software", Desc: "Support status, duplicates, dependencies", Plan: AnalyzePlan(ctx), Confirm: StartAnalysisMsg{}},
		{Label: "Find unused applications", Desc: "Not opened in a year · Smart Analysis", Navigate: true, Section: nav.SmartAnalysis},
		{Label: "Clean unnecessary files", Desc: "Caches, old versions, leftovers · Cleanup", Navigate: true, Section: nav.Cleanup},
		{Label: "Discover useful applications", Desc: "Free tools that fit what you use", Navigate: true, Section: nav.DiscoverApps},
		{Label: "Search software", Desc: "Find anything installed · /", Search: true},
	}
}

// TidyFindPlan describes looking for things to tidy.
func TidyFindPlan(ctx Context) core.ActionPlan {
	p := core.ActionPlan{
		Title:       "Look for things to tidy",
		WhatHappens: "ATLAS will look for package caches, old versions kept after updates, and packages your package manager says nothing needs any more. It lists exactly what it finds.",
		Why:         "These take up space and aren't needed by the software you use.",
		Safety:      "Safe. Looking only reads information; nothing is deleted. Removing anything is a separate step you confirm.",
		Reversible:  "There is nothing to undo.",
		Available:   true,
	}
	for _, s := range ctx.Tidiers {
		p.Technical = append(p.Technical, s.Label+": "+s.Detail)
	}
	switch {
	case !ctx.Data.CleanupAvailable:
		p.Available, p.Unavailability = false, "ATLAS can't open its saved data. See Settings for details."
	case len(ctx.Tidiers) == 0:
		p.Available, p.Unavailability = false, "ATLAS doesn't know any caches or leftovers to check on this computer."
	case ctx.Data.Busy():
		p.Available, p.Unavailability = false, "another task is running. Wait for it to finish, or stop it with x."
	}
	return p
}

// CleanupPlan describes removing software and tidying up. It is
// destructive, so it needs r (not Enter) to confirm.
func CleanupPlan(ctx Context, steps []core.UpdateStep) core.ActionPlan {
	var changes, blocked, technical, rollbacks []string
	seen := map[string]bool{}
	elevations := map[core.Elevation]bool{}
	removed, cleaned := 0, 0
	var bytes int64
	firstName := ""
	autoremove := false
	for _, st := range steps {
		if st.Blocked != "" {
			for _, u := range st.Updates {
				blocked = append(blocked, "Not removed: "+u.Name+". "+st.Blocked)
			}
			continue
		}
		elevations[st.Elevation] = true
		bytes += st.Bytes
		for _, u := range st.Updates {
			name := core.NameAndVersion(u.Name, u.CurrentVersion)
			if firstName == "" {
				firstName = u.Name
			}
			if st.Action == core.ActionClean {
				cleaned++
				changes = append(changes, "Deleted: "+name+" · "+st.Label)
			} else {
				removed++
				changes = append(changes, "Removed: "+name+" · "+st.Label)
			}
		}
		if st.Action == core.ActionRemove && st.Commands != nil && len(st.Commands[0].Args) > 0 && strings.Contains(st.Commands[0].String(), "autoremove") {
			autoremove = true
		}
		for _, c := range st.Commands {
			technical = append(technical, c.String())
		}
		if st.Rollback != "" && !seen[st.Rollback] {
			seen[st.Rollback] = true
			rollbacks = append(rollbacks, st.Label+": "+strings.TrimSuffix(st.Rollback, ".")+".")
		}
	}
	const maxLines = 14
	if len(changes) > maxLines {
		more := len(changes) - maxLines
		changes = append(changes[:maxLines], fmt.Sprintf("…and %d more", more))
	}
	if bytes > 0 {
		changes = append(changes, "Space freed: about "+humanBytes(bytes)+" (removed software may free more)")
	}

	var title string
	switch {
	case removed == 1 && cleaned == 0:
		title = "Remove " + firstName
	case removed > 0 && cleaned == 0:
		title = fmt.Sprintf("Remove %d items", removed)
	case removed == 0 && cleaned > 0:
		title = "Tidy up"
	case removed > 0:
		title = fmt.Sprintf("Remove %d items and tidy up", removed)
	default:
		title = "Remove software"
	}
	safety := []string{}
	if removed > 0 {
		safety = append(safety, "ATLAS checked dependencies just now: nothing listed here is needed by other installed software, as far as your package managers record. Each package manager shows what it will remove, and most ask you to confirm. Your documents aren't touched and settings are kept: ATLAS never purges.")
	}
	if cleaned > 0 {
		safety = append(safety, "Clean-ups only delete the caches and old versions listed above; the software you use isn't affected.")
	}
	if autoremove {
		safety = append(safety, "For packages no longer needed, the package manager works out the list again and shows it before asking.")
	}
	if elevations[core.ElevationSudo] {
		safety = append(safety, "Some steps run through sudo, which will ask for your password in the terminal. ATLAS never sees or stores it.")
	}
	if elevations[core.ElevationInstaller] {
		safety = append(safety, "A program's own uninstaller runs, and Windows may ask for permission. Approve only what you expect.")
	}
	if elevations[core.ElevationPolkit] {
		safety = append(safety, "Flatpak may ask for your password for apps installed for everyone.")
	}
	safety = append(safety, "Press Ctrl+C to stop; nothing further starts after that.")

	p := core.ActionPlan{
		Title:       title,
		WhatHappens: "ATLAS will hand this terminal to each package manager in turn, so you see exactly what it does and can answer its questions. When it's finished, you'll come back here and the result is recorded in Activity History.",
		Why:         "Software and files you don't use take space and still need updates.",
		WhatChanges: append(changes, blocked...),
		Safety:      strings.Join(safety, " "),
		Reversible:  "Removed software can be reinstalled. " + strings.Join(rollbacks, " "),
		Technical:   technical,
		Available:   removed+cleaned > 0,
		Destructive: true,
	}
	switch {
	case removed+cleaned == 0 && len(blocked) > 0:
		p.Unavailability = "nothing selected can be removed safely (see below)."
	case removed+cleaned == 0:
		p.Unavailability = "there is nothing to remove."
	case ctx.Data.Busy():
		p.Available = false
		p.Unavailability = "another task is running. Wait for it to finish, or stop it with x."
	}
	return p
}

func humanBytes(n int64) string {
	const unit = 1000
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "kMGTPE"[exp])
}

// InstallPlan describes installing tools from Discover Apps.
func InstallPlan(ctx Context, steps []core.UpdateStep) core.ActionPlan {
	var changes, blocked, technical, notes []string
	elevations := map[core.Elevation]bool{}
	first := ""
	n := 0
	for _, st := range steps {
		name := ""
		if len(st.Updates) > 0 {
			name = st.Updates[0].Name
		}
		if st.Blocked != "" {
			blocked = append(blocked, "Not included: "+name+". "+st.Blocked)
			continue
		}
		n++
		if first == "" {
			first = name
		}
		elevations[st.Elevation] = true
		changes = append(changes, "Installed: "+name+" · "+st.Label)
		for _, c := range st.Commands {
			technical = append(technical, c.String())
		}
		if st.Rollback != "" {
			notes = append(notes, name+": "+st.Rollback)
		}
	}
	title := fmt.Sprintf("Install %d apps", n)
	if n == 1 {
		title = "Install " + first
	}
	safety := []string{"Everything comes from the package manager's official source. The package manager shows what it will install (including anything it needs) and may ask you to confirm or to accept a licence; ATLAS never answers for you."}
	if elevations[core.ElevationSudo] {
		safety = append(safety, "sudo will ask for your password in the terminal. ATLAS never sees or stores it.")
	}
	if elevations[core.ElevationInstaller] {
		safety = append(safety, "Windows may ask for permission if the installer needs it. Approve only what you expect.")
	}
	if elevations[core.ElevationPolkit] {
		safety = append(safety, "Flatpak may ask for your password.")
	}
	if elevations[core.ElevationAdmin] {
		safety = append(safety, "ATLAS is running with administrator rights, so installers run with full rights.")
	}
	safety = append(safety, "Press Ctrl+C to stop; nothing further starts after that.")
	p := core.ActionPlan{
		Title:       title,
		WhatHappens: "ATLAS will hand this terminal to the package manager, so you see exactly what it does and can answer its questions. When it's finished, you'll come back here and the result is recorded in Activity History.",
		Why:         "You picked these in Discover Apps.",
		WhatChanges: append(changes, blocked...),
		Safety:      strings.Join(safety, " "),
		Reversible:  strings.Join(notes, " "),
		Technical:   technical,
		Available:   n > 0,
	}
	switch {
	case n == 0 && len(blocked) > 0:
		p.Unavailability = "none of these can be installed from here (see below)."
	case n == 0:
		p.Unavailability = "there is nothing to install."
	case ctx.Data.Busy():
		p.Available, p.Unavailability = false, "another task is running. Wait for it to finish, or stop it with x."
	}
	if p.Reversible == "" {
		p.Reversible = "Nothing is installed."
	}
	return p
}
