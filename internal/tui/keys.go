package tui

import "github.com/deepakraj1207/atlas/internal/tui/components"

// Global key labels shared by the footer and the help overlay.
var (
	globalHints = []components.KeyHint{
		{Key: "?", Desc: "help"},
		{Key: "tab", Desc: "next section"},
		{Key: "1-9", Desc: "jump"},
		{Key: "m", Desc: "mode"},
		{Key: "t", Desc: "theme"},
		{Key: "q", Desc: "quit"},
	}

	navigationHelp = []components.KeyHint{
		{Key: "tab / shift+tab", Desc: "Next / previous section"},
		{Key: "1 … 9", Desc: "Jump straight to a section (shown next to each name)"},
		{Key: "↑ ↓ ← →", Desc: "Move within the current screen"},
		{Key: "enter", Desc: "Choose the highlighted item"},
		{Key: "pgup / pgdn", Desc: "Scroll the screen when it does not fit"},
	}

	generalHelp = []components.KeyHint{
		{Key: "m", Desc: "Switch between Simple and Advanced mode (remembered)"},
		{Key: "t", Desc: "Switch between dark and light theme (remembered)"},
		{Key: "?", Desc: "Open or close this help"},
		{Key: "esc", Desc: "Close an overlay, or cancel a preview without doing anything"},
		{Key: "x", Desc: "Stop a running scan (nothing is changed either way)"},
		{Key: "q / ctrl+c", Desc: "Quit ATLAS"},
	}
)
