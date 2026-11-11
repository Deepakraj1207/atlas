// Package layout turns a terminal size into layout decisions. Everything
// here is a pure function so the responsive behaviour is unit-testable.
package layout

// Breakpoint classifies the terminal width.
type Breakpoint int

const (
	// TooSmall means the UI cannot render usefully; show a short notice.
	TooSmall Breakpoint = iota
	// Narrow: no sidebar, abbreviated top tabs, one-column content.
	Narrow
	// Medium: no sidebar, full-name top tabs, one-column content.
	Medium
	// Wide: sidebar navigation and two-column content where useful.
	Wide
)

// String returns a short label for diagnostics.
func (b Breakpoint) String() string {
	switch b {
	case TooSmall:
		return "too-small"
	case Narrow:
		return "narrow"
	case Medium:
		return "medium"
	default:
		return "wide"
	}
}

// Thresholds. Exported so screens and tests share them.
const (
	MinWidth  = 40
	MinHeight = 10

	NarrowMax = 79  // widths below 80 are Narrow
	MediumMax = 109 // widths below 110 are Medium

	SidebarWidth = 26
	HeaderHeight = 2 // title line + blank spacer
	FooterHeight = 2 // message line + hints line
	// TabBarHeight is used instead of the sidebar on Narrow/Medium.
	TabBarHeight = 2
)

// Layout is the set of decisions derived from a terminal size.
type Layout struct {
	Width, Height int
	Breakpoint    Breakpoint

	// ShowSidebar is true on Wide terminals. Otherwise a tab bar is shown.
	ShowSidebar bool
	// ShowTabBar is the inverse of ShowSidebar when not TooSmall.
	ShowTabBar bool
	// ShowHeader is false on very short terminals to preserve content rows.
	ShowHeader bool
	// ShowHints is false when the footer must collapse to a single line.
	ShowHints bool
	// TwoColumn suggests side-by-side panels in the content area.
	TwoColumn bool
	// UseShortTitles tells tab bars to use abbreviated section names.
	UseShortTitles bool

	// ContentWidth and ContentHeight are the space left for the active screen.
	ContentWidth, ContentHeight int
	// SidebarWidth is 0 when the sidebar is hidden.
	SidebarWidth int
}

// Compute derives a Layout from the terminal size. It never returns negative
// content dimensions.
func Compute(width, height int) Layout {
	l := Layout{Width: width, Height: height}

	if width < MinWidth || height < MinHeight {
		l.Breakpoint = TooSmall
		l.ContentWidth = max(width, 0)
		l.ContentHeight = max(height, 0)
		return l
	}

	switch {
	case width <= NarrowMax:
		l.Breakpoint = Narrow
	case width <= MediumMax:
		l.Breakpoint = Medium
	default:
		l.Breakpoint = Wide
	}

	l.ShowSidebar = l.Breakpoint == Wide
	l.ShowTabBar = !l.ShowSidebar
	l.UseShortTitles = l.Breakpoint == Narrow
	l.ShowHeader = height >= 16
	l.ShowHints = height >= 14
	l.TwoColumn = l.Breakpoint == Wide && height >= 20

	used := FooterHeight
	if !l.ShowHints {
		used = 1
	}
	if l.ShowHeader {
		used += HeaderHeight
	}
	if l.ShowTabBar {
		used += TabBarHeight
	}
	l.ContentHeight = max(height-used, 1)

	if l.ShowSidebar {
		l.SidebarWidth = SidebarWidth
		l.ContentWidth = max(width-SidebarWidth, 1)
	} else {
		l.ContentWidth = width
	}
	return l
}
