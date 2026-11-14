package components

import (
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/tui/theme"
)

// ActionPreview renders a core.ActionPlan so the user sees what an action
// would do before anything happens. Unavailable plans say so explicitly.
// The result may be taller than the screen; the caller scrolls it.
func ActionPreview(t theme.Theme, width int, p core.ActionPlan, advanced bool) string {
	var b strings.Builder
	w := max(width-2, 10)
	section := func(title, body string) {
		b.WriteString(t.Heading.Render(title))
		b.WriteString("\n  ")
		b.WriteString(strings.ReplaceAll(t.Body.Render(Wrap(body, w-2)), "\n", "\n  "))
		b.WriteString("\n\n")
	}

	b.WriteString(t.Logo.Render("›") + " " + t.Title.Render(p.Title))
	b.WriteString("\n")
	if !p.Available {
		note := "Not available. Nothing has been changed on your computer."
		if p.Unavailability != "" {
			note = "Not available: " + p.Unavailability + " Nothing has been changed on your computer."
		}
		b.WriteString(t.Warning.Render(Wrap(note, w)))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	section("What will happen", p.WhatHappens)
	section("Why", p.Why)
	if p.ChangesSystem() {
		b.WriteString(t.Heading.Render("What will change"))
		b.WriteString("\n  ")
		b.WriteString(strings.ReplaceAll(Bullets(t, w-2, p.WhatChanges), "\n", "\n  "))
		b.WriteString("\n\n")
	} else {
		section("What will change", "Nothing on your computer changes. This only reads information.")
	}
	section("Is it safe?", p.Safety)
	section("Can it be undone?", p.Reversible)

	if advanced && len(p.Technical) > 0 {
		b.WriteString(t.Heading.Render("Technical details"))
		b.WriteString("\n  ")
		b.WriteString(strings.ReplaceAll(Bullets(t, w-2, p.Technical), "\n", "\n  "))
		b.WriteString("\n\n")
	} else if len(p.Technical) > 0 {
		b.WriteString(t.Subtle.Render("Switch to Advanced mode (m) to see technical details."))
		b.WriteString("\n\n")
	}

	switch {
	case p.Available && p.Destructive:
		b.WriteString(t.Warning.Render("Press r to go ahead · Esc to cancel"))
	case p.Available:
		b.WriteString(t.Subtle.Render("Enter to continue · Esc to cancel"))
	default:
		b.WriteString(t.Subtle.Render("Press Esc or Enter to close."))
	}
	return b.String()
}
