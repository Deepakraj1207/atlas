package nav

import "testing"

func TestSectionOrderAndTitles(t *testing.T) {
	want := []string{
		"Home", "My Software", "Updates", "Smart Analysis", "Dependencies",
		"Cleanup", "Discover Apps", "Activity History", "Settings",
	}
	all := All()
	if len(all) != len(want) {
		t.Fatalf("got %d sections, want %d", len(all), len(want))
	}
	for i, s := range all {
		if s.Title() != want[i] {
			t.Errorf("section %d title = %q, want %q", i, s.Title(), want[i])
		}
		if s.Shortcut() != string(rune('1'+i)) {
			t.Errorf("section %d shortcut = %q", i, s.Shortcut())
		}
		if s.Info().Summary == "" {
			t.Errorf("section %q has no summary", s.Title())
		}
	}
}

func TestComingSoonSectionsHavePlanAndPhase(t *testing.T) {
	for _, s := range All() {
		info := s.Info()
		switch info.Availability {
		case Available:
			switch s {
			case Home, MySoftware, Updates, SmartAnalysis, Dependencies, Cleanup, DiscoverApps, ActivityHistory, Settings:
			default:
				t.Errorf("%q marked Available but is not built yet", s.Title())
			}
		case ComingSoon:
			if len(info.Plan) == 0 || info.Phase < 6 {
				t.Errorf("%q is ComingSoon but lacks a plan or phase: %+v", s.Title(), info)
			}
		}
	}
}

func TestNextPrevWrap(t *testing.T) {
	st := NewState()
	if st.Current() != Home {
		t.Fatal("should start at Home")
	}
	st.Prev()
	if st.Current() != Settings {
		t.Errorf("Prev from Home = %v, want Settings", st.Current())
	}
	st.Next()
	if st.Current() != Home {
		t.Errorf("Next from Settings = %v, want Home", st.Current())
	}
	for i := 0; i < Count(); i++ {
		st.Next()
	}
	if st.Current() != Home {
		t.Errorf("full cycle should return to Home, got %v", st.Current())
	}
}

func TestGoAndShortcut(t *testing.T) {
	st := NewState()
	if !st.GoShortcut("3") || st.Current() != Updates {
		t.Errorf("shortcut 3 should open Updates, got %v", st.Current())
	}
	if st.GoShortcut("0") || st.GoShortcut("x") || st.GoShortcut("12") {
		t.Error("invalid shortcuts must be rejected")
	}
	if st.Current() != Updates {
		t.Error("rejected shortcut must not move")
	}
	if st.Go(Section(42)) {
		t.Error("invalid section must be rejected")
	}
	if !st.Go(Settings) || st.Current() != Settings {
		t.Error("Go(Settings) failed")
	}
	if Section(-1).Valid() || Section(99).Valid() {
		t.Error("out-of-range sections must be invalid")
	}
	if Section(99).Shortcut() != "" {
		t.Error("invalid section must have no shortcut")
	}
}
