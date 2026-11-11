package screens

import "github.com/deepakraj1207/atlas/internal/core"

// QuickAction is one of the Home screen buttons.
type QuickAction struct {
	Label string
	Plan  core.ActionPlan
}

// quickActions are the Phase 1 Home buttons. None is available yet; each
// plan explains what the action will do when it arrives and states that
// nothing runs today.
func quickActions() []QuickAction {
	return []QuickAction{
		{
			Label: "Scan My System",
			Plan: core.ActionPlan{
				Title:       "Scan My System",
				WhatHappens: "ATLAS will look through this computer for installed applications and developer tools and build a list you can search.",
				Why:         "You cannot keep software tidy or up to date without knowing what is there.",
				Safety:      "Safe. A scan only reads information; it never installs, removes or changes anything.",
				Reversible:  "There is nothing to undo. You can delete the saved list from Settings at any time.",
				Technical: []string{
					"Windows: Uninstall registry keys (HKLM/HKCU), Start Menu entries, winget list, scoop list, choco list",
					"Linux: dpkg/apt, snap, flatpak, pip and npm global packages",
					"Results stored locally in SQLite; nothing leaves this computer",
				},
				Available:      false,
				Unavailability: "scanning arrives in Phase 2.",
			},
		},
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
				Available:      false,
				Unavailability: "update checks arrive in Phase 3.",
			},
		},
		{
			Label: "Analyze My Software",
			Plan: core.ActionPlan{
				Title:       "Analyze My Software",
				WhatHappens: "ATLAS will review your software list and explain, in plain language, what looks duplicated, unused or risky.",
				Why:         "It helps you decide what to keep without reading technical documentation.",
				Safety:      "Safe. Analysis only reads the saved list and never changes anything.",
				Reversible:  "There is nothing to undo.",
				Technical: []string{
					"Rules-based analysis first; optional AI explanations later, off by default",
				},
				Available:      false,
				Unavailability: "analysis arrives in Phase 4.",
			},
		},
		{
			Label: "Find New Tools",
			Plan: core.ActionPlan{
				Title:       "Find New Tools",
				WhatHappens: "ATLAS will suggest well-regarded tools related to the kinds of software you already use.",
				Why:         "Discovering useful tools is hard when you do not know their names.",
				Safety:      "Safe. Suggestions are only shown; nothing is downloaded or installed unless you choose to, and that will always ask first.",
				Reversible:  "There is nothing to undo.",
				Technical: []string{
					"Curated catalogue matched against installed categories; no telemetry",
				},
				Available:      false,
				Unavailability: "discovery arrives in Phase 6.",
			},
		},
	}
}
