package tui

import "github.com/deepakraj1207/atlas/internal/tui/components"

// Key labels shared by the footer and the help overlay.
var (
	// globalHints follow a screen's own hints in the footer.
	globalHints = []components.KeyHint{
		{Key: "esc", Desc: "menu"},
		{Key: "/", Desc: "search"},
		{Key: "?", Desc: "help"},
		{Key: "q", Desc: "quit"},
	}

	// navHints are shown while the arrow keys move through the sections.
	navHints = []components.KeyHint{
		{Key: "↑↓", Desc: "navigate"},
		{Key: "enter", Desc: "select"},
		{Key: "/", Desc: "search"},
		{Key: "?", Desc: "help"},
		{Key: "q", Desc: "quit"},
		{Key: "0-9", Desc: "jump"},
	}

	navigationHelp = []components.KeyHint{
		{Key: "esc", Desc: "Back to the section list (Enter returns)"},
		{Key: "↑ ↓", Desc: "Move through the list"},
		{Key: "enter", Desc: "Choose the highlighted item"},
		{Key: "0 … 9", Desc: "Jump to a section"},
		{Key: "tab / shift+tab", Desc: "Next / previous section"},
		{Key: "/", Desc: "Search your software"},
		{Key: "pgup / pgdn", Desc: "Scroll a long screen"},
	}

	generalHelp = []components.KeyHint{
		{Key: "m", Desc: "Simple or Advanced mode"},
		{Key: "t", Desc: "Dark or light theme"},
		{Key: "?", Desc: "Open or close this help"},
		{Key: "esc", Desc: "Close, or cancel a preview"},
		{Key: "x", Desc: "Stop a running scan, check or search"},
		{Key: "r", Desc: "Confirm a removal (Enter never does)"},
		{Key: "q / ctrl+c", Desc: "Quit ATLAS"},
	}
)
