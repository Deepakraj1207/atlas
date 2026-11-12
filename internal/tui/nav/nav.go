// Package nav defines the application's sections and the navigation state
// that moves between them. It is pure data so it can be unit tested without
// Bubble Tea.
package nav

// Section identifies one top-level area of the application.
type Section int

// Sections in display order. The order is the navigation order and the
// number shortcut (Home = 1 ... Settings = 9).
const (
	Home Section = iota
	MySoftware
	Updates
	SmartAnalysis
	Dependencies
	Cleanup
	DiscoverApps
	ActivityHistory
	Settings
	sectionCount
)

// Availability tells the UI whether a section has real functionality.
type Availability int

const (
	// Available sections have working screens.
	Available Availability = iota
	// ComingSoon sections render an honest empty state describing the plan.
	ComingSoon
)

// Info is the static metadata for a section.
type Info struct {
	Title string
	// Short is used when horizontal space is tight.
	Short string
	// Summary is one plain-language sentence about what the section is for.
	Summary string
	// Plan describes what the section will do, for empty states.
	Plan []string
	// Phase is the delivery phase for ComingSoon sections (0 if available).
	Phase int
	Availability
}

var infos = [sectionCount]Info{
	Home: {
		Title:   "Home",
		Short:   "Home",
		Summary: "Your starting point: quick actions and what needs attention.",
	},
	MySoftware: {
		Title:   "My Software",
		Short:   "Software",
		Summary: "Everything installed on this computer, in one searchable list.",
	},
	Updates: {
		Title:   "Updates",
		Short:   "Updates",
		Summary: "See which applications have newer versions and update them safely.",
	},
	SmartAnalysis: {
		Title:   "Smart Analysis",
		Short:   "Analysis",
		Summary: "Plain-language insight into what you have and what you might not need.",
	},
	Dependencies: {
		Title:   "Dependencies",
		Short:   "Depends",
		Summary: "Understand which tools rely on which runtimes and libraries.",
	},
	Cleanup: {
		Title:   "Cleanup",
		Short:   "Cleanup",
		Summary: "Reclaim space from leftovers, caches and unused software.",
	},
	DiscoverApps: {
		Title:   "Discover Apps",
		Short:   "Discover",
		Summary: "Find useful tools based on what you already use.",
		Plan: []string{
			"Suggest well-regarded tools for the kinds of work you do",
			"Explain what each suggestion is for before you install anything",
		},
		Phase:        6,
		Availability: ComingSoon,
	},
	ActivityHistory: {
		Title:   "Activity History",
		Short:   "History",
		Summary: "A record of everything ATLAS has done, so nothing happens silently.",
	},
	Settings: {
		Title:   "Settings",
		Short:   "Settings",
		Summary: "Choose Simple or Advanced mode, the theme, and other preferences.",
	},
}

// All returns every section in display order.
func All() []Section {
	out := make([]Section, sectionCount)
	for i := range out {
		out[i] = Section(i)
	}
	return out
}

// Count is the number of sections.
func Count() int { return int(sectionCount) }

// Info returns the section's metadata.
func (s Section) Info() Info {
	if s < 0 || s >= sectionCount {
		return Info{Title: "Unknown", Short: "?"}
	}
	return infos[s]
}

// Title is the full display name.
func (s Section) Title() string { return s.Info().Title }

// Shortcut is the single-digit key that jumps to the section.
func (s Section) Shortcut() string {
	if s < 0 || s >= sectionCount {
		return ""
	}
	return string(rune('1' + int(s)))
}

// Valid reports whether s is a real section.
func (s Section) Valid() bool { return s >= 0 && s < sectionCount }

// State tracks the current section.
type State struct {
	current Section
}

// NewState starts at Home.
func NewState() State { return State{current: Home} }

// Current returns the active section.
func (s State) Current() Section { return s.current }

// Next moves forward, wrapping to Home after Settings.
func (s *State) Next() {
	s.current = Section((int(s.current) + 1) % int(sectionCount))
}

// Prev moves backward, wrapping to Settings before Home.
func (s *State) Prev() {
	s.current = Section((int(s.current) + int(sectionCount) - 1) % int(sectionCount))
}

// Go jumps to a section. Invalid sections are ignored and false is returned.
func (s *State) Go(to Section) bool {
	if !to.Valid() {
		return false
	}
	s.current = to
	return true
}

// GoShortcut jumps to the section bound to a digit key ("1".."9").
func (s *State) GoShortcut(key string) bool {
	if len(key) != 1 || key[0] < '1' || key[0] > '9' {
		return false
	}
	return s.Go(Section(key[0] - '1'))
}
