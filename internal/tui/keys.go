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
		{Key: "1-9", Desc: "jump"},
	}

	navigationHelp = []components.KeyHint{
		{Key: "esc", Desc: "Back: leave the screen's list and move through the sections with ↑ ↓ (Esc or Enter returns)"},
		{Key: "↑ ↓", Desc: "Move through the screen's list, or through the sections after Esc"},
		{Key: "enter", Desc: "Choose the highlighted item or section"},
		{Key: "1 … 9", Desc: "Jump straight to a section (shown next to each name)"},
		{Key: "tab / shift+tab", Desc: "Next / previous section"},
		{Key: "/", Desc: "Search your software from anywhere"},
		{Key: "pgup / pgdn", Desc: "Scroll the screen when it does not fit"},
	}

	generalHelp = []components.KeyHint{
		{Key: "m", Desc: "Switch between Simple and Advanced mode (remembered)"},
		{Key: "t", Desc: "Switch between dark and light theme (remembered)"},
		{Key: "?", Desc: "Open or close this help"},
		{Key: "esc", Desc: "Close an overlay, or cancel a preview without doing anything"},
		{Key: "x", Desc: "Stop a running scan, update check, analysis or search (nothing is changed either way)"},
		{Key: "r", Desc: "In a removal or clean-up preview: go ahead (Enter alone never confirms a deletion)"},
		{Key: "q / ctrl+c", Desc: "Quit ATLAS"},
	}
)
