package inventory

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/deepakraj1207/atlas/internal/core"
)

func snapshot(results map[string]core.SourceStatus, items map[string][]core.SoftwareItem) core.Snapshot {
	s := core.Snapshot{BySource: map[string][]core.SoftwareItem{}}
	for src, st := range results {
		s.Sources = append(s.Sources, core.SourceResult{Source: src, Label: "Label " + src, Status: st, Message: map[core.SourceStatus]string{core.SourceTimeout: "Took longer than 1m and was stopped", core.SourceFailed: "Couldn't be read"}[st]})
		if st == core.SourceOK {
			s.BySource[src] = items[src]
		}
	}
	return s
}

func TestVerifyRemoved(t *testing.T) {
	jq := core.SoftwareItem{Name: "jq", PackageID: "jqlang.jq", Version: "1.8.2", Source: SourceRegistry, AlsoFoundIn: []string{SourceWinget}}
	ok := map[string]core.SourceStatus{SourceRegistry: core.SourceOK, SourceWinget: core.SourceOK, SourceScoop: core.SourceOK}

	if v := VerifyRemoved(snapshot(ok, nil), jq); v.State != core.VerifyConfirmed {
		t.Errorf("gone everywhere: %+v", v)
	}
	// Still listed by winget (by package ID) or by another source (by name).
	still := map[string][]core.SoftwareItem{SourceWinget: {{Name: "jq", PackageID: "jqlang.jq"}}}
	if v := VerifyRemoved(snapshot(ok, still), jq); v.State != core.VerifyNotDone || !strings.Contains(v.Detail, "Label winget") {
		t.Errorf("still in winget: %+v", v)
	}
	scoopCopy := map[string][]core.SoftwareItem{SourceScoop: {{Name: "JQ", PackageID: "main/jq"}}}
	if v := VerifyRemoved(snapshot(ok, scoopCopy), jq); v.State != core.VerifyNotDone {
		t.Errorf("another copy found by name must not count as removed: %+v", v)
	}
	// One of its sources couldn't be read: can't confirm.
	for _, st := range []core.SourceStatus{core.SourceFailed, core.SourceTimeout, core.SourceUnavailable} {
		res := map[string]core.SourceStatus{SourceRegistry: core.SourceOK, SourceWinget: st}
		if v := VerifyRemoved(snapshot(res, nil), jq); v.State != core.VerifyUnknown || !strings.Contains(v.Detail, "Label winget") {
			t.Errorf("winget %s: %+v", st, v)
		}
	}
	// A source missing from the snapshot entirely: can't confirm.
	if v := VerifyRemoved(snapshot(map[string]core.SourceStatus{SourceRegistry: core.SourceOK}, nil), jq); v.State != core.VerifyUnknown {
		t.Errorf("winget not checked: %+v", v)
	}
	if v := NotVerified("jq", SourceWinget); v.State != core.VerifyUnknown {
		t.Errorf("NotVerified: %+v", v)
	}
}

func TestVerifyUpdated(t *testing.T) {
	u := core.Update{Name: "Git", PackageID: "Git.Git", CurrentVersion: "2.56.0", AvailableVersion: "2.56.0.4"}
	ok := map[string]core.SourceStatus{SourceWinget: core.SourceOK}
	at := func(v string) map[string][]core.SoftwareItem {
		return map[string][]core.SoftwareItem{SourceWinget: {{Name: "Git", PackageID: "Git.Git", Version: v}}}
	}
	for _, c := range []struct {
		now   string
		state core.VerifyState
	}{
		{"2.56.0.4", core.VerifyConfirmed}, // the new version
		{"2.56.0", core.VerifyNotDone},     // unchanged
		{"2.57.1", core.VerifyConfirmed},   // changed from before (newer than offered)
		{"2.56.0.0", core.VerifyNotDone},   // same as before, written differently
	} {
		v := VerifyUpdated(snapshot(ok, at(c.now)), SourceWinget, u)
		if v.State != c.state || v.Version != c.now {
			t.Errorf("now %s: %+v, want %s", c.now, v, c.state)
		}
	}
	if v := VerifyUpdated(snapshot(ok, nil), SourceWinget, u); v.State != core.VerifyUnknown || !strings.Contains(v.Detail, "no longer lists it") {
		t.Errorf("gone from the list: %+v", v)
	}
	if v := VerifyUpdated(snapshot(map[string]core.SourceStatus{SourceWinget: core.SourceTimeout}, nil), SourceWinget, u); v.State != core.VerifyUnknown {
		t.Errorf("source timed out: %+v", v)
	}
	// No previous version known (winget's "< x"): only the offered version confirms.
	blank := core.Update{Name: "Safe Exam Browser", PackageID: "ETHZurich.SafeExamBrowser", AvailableVersion: "3.10.3"}
	seb := func(v string) map[string][]core.SoftwareItem {
		return map[string][]core.SoftwareItem{SourceWinget: {{Name: "Safe Exam Browser", PackageID: "ETHZurich.SafeExamBrowser", Version: v}}}
	}
	if v := VerifyUpdated(snapshot(ok, seb("3.10.3")), SourceWinget, blank); v.State != core.VerifyConfirmed {
		t.Errorf("offered version: %+v", v)
	}
	if v := VerifyUpdated(snapshot(ok, seb("< 3.10.3")), SourceWinget, blank); v.State != core.VerifyUnknown {
		t.Errorf("still older than offered: %+v", v)
	}
}

// winget prints "< 3.10.3" when it only knows a version is older than its
// catalogue's oldest; the scanner must keep that, or a failed update would
// look like it reached 3.10.3.
func TestWingetKeepsOlderThanMarker(t *testing.T) {
	out := "Name              Id                        Version  Source\n" +
		"---------------------------------------------------------------\n" +
		"Safe Exam Browser ETHZurich.SafeExamBrowser < 3.10.3 winget\n"
	items, err := parseWingetList(out)
	if err != nil || len(items) != 1 || items[0].Version != "< 3.10.3" {
		t.Fatalf("got %+v %v", items, err)
	}
	// Merging still pairs it with the Windows entry for the same install.
	reg := core.SoftwareItem{Name: "Safe Exam Browser", Version: "3.10.2.920", Source: SourceRegistry, Kind: core.KindApp}
	items[0].Origin = "winget"
	if m := Merge([]core.SoftwareItem{reg, items[0]}); len(m) != 1 {
		t.Errorf("merge: %+v", m)
	}
}

func TestSnapshotReadsEverySourceWithoutSaving(t *testing.T) {
	svc, db := newTestService(t,
		fakeScanner{id: SourceDpkg, items: []core.SoftwareItem{{Name: "jq", PackageID: "jq", Version: "1.7", Source: SourceDpkg}}},
		fakeScanner{id: SourceSnap, err: errors.New("snapd not responding")},
		fakeScanner{id: SourceFlatpak, err: core.ErrSourceUnavailable},
	)
	snap := svc.Snapshot(context.Background())
	if len(snap.Sources) != 3 || len(snap.BySource[SourceDpkg]) != 1 {
		t.Fatalf("snapshot = %+v", snap)
	}
	if r, _ := snap.Result(SourceSnap); r.Status != core.SourceFailed {
		t.Errorf("snap: %+v", r)
	}
	if _, ok := snap.BySource[SourceSnap]; ok {
		t.Error("a failed source contributes no items")
	}
	if _, saved, _ := db.LatestScan(context.Background()); saved {
		t.Error("a snapshot must not be saved as a scan")
	}
}
