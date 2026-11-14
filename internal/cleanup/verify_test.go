package cleanup

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/deepakraj1207/atlas/internal/core"
	"github.com/deepakraj1207/atlas/internal/inventory"
)

type fakeVerifier struct{ snap core.Snapshot }

func (f fakeVerifier) Snapshot(context.Context) core.Snapshot { return f.snap }

func dpkgSnapshot(status core.SourceStatus, items ...core.SoftwareItem) core.Snapshot {
	snap := core.Snapshot{Sources: []core.SourceResult{{Source: inventory.SourceDpkg, Label: "Debian packages", Status: status, Message: "Couldn't be read"}},
		BySource: map[string][]core.SoftwareItem{}}
	if status == core.SourceOK {
		snap.BySource[inventory.SourceDpkg] = items
	}
	return snap
}

// removeLonelyTool runs a removal whose command reports success (exit 0),
// then records it with the given verifier (nil: none).
func removeLonelyTool(t *testing.T, v core.Verifier) (core.RecordOutcome, []core.SoftwareItem, core.Activity) {
	t.Helper()
	s, db := newSvc(t, linuxCfg(t))
	if v != nil {
		s.verify = v
	}
	ctx := context.Background()
	db.SaveScan(ctx, core.ScanReport{Status: core.ScanComplete, Items: []core.SoftwareItem{deb("lonely-tool", core.KindApp), deb("meld", core.KindApp)}})
	allowed, _ := plan(t, s, deb("lonely-tool", core.KindApp))
	s.WithProcessStarter(func(c core.Command, in io.Reader, out, errw io.Writer) (int, error) { return 0, nil })
	job := s.NewJob(allowed)
	job.SetStdin(strings.NewReader("\n"))
	job.SetStdout(io.Discard)
	job.Run()
	out, err := s.RecordResults(ctx, job.Results())
	if err != nil {
		t.Fatal(err)
	}
	items, _ := db.Software(ctx)
	acts, _ := db.RecentActivity(ctx, 1)
	return out, items, acts[0]
}

// The audit's P0: an exit code of 0 was taken as proof of removal, and the
// saved record was deleted. Now the computer is checked again first.
func TestRemovalIsVerifiedBeforeTheRecordGoes(t *testing.T) {
	stillThere := fakeVerifier{snap: dpkgSnapshot(core.SourceOK, deb("lonely-tool", core.KindApp), deb("meld", core.KindApp))}
	out, items, act := removeLonelyTool(t, stillThere)
	if out.Count(core.VerifyNotDone) != 1 || len(items) != 2 {
		t.Errorf("still installed: the record must be kept: %+v / %d items", out, len(items))
	}
	if act.Outcome != core.OutcomePartial || !strings.Contains(act.Title, "not all confirmed") || !strings.Contains(act.Summary, "Still installed: listed by Debian packages") {
		t.Errorf("activity must say it wasn't confirmed: %+v", act)
	}

	unreadable := fakeVerifier{snap: dpkgSnapshot(core.SourceFailed)}
	out, items, act = removeLonelyTool(t, unreadable)
	if out.Count(core.VerifyUnknown) != 1 || len(items) != 2 || !strings.Contains(act.Summary, "Couldn't check") || !strings.Contains(act.Summary, "records are kept") {
		t.Errorf("source unreadable: keep the record and say so: %+v / %+v", out, act)
	}

	out, items, act = removeLonelyTool(t, nil)
	if out.Count(core.VerifyUnknown) != 1 || len(items) != 2 || act.Outcome != core.OutcomePartial {
		t.Errorf("no verifier: nothing is confirmed: %+v / %+v", out, act)
	}

	gone := fakeVerifier{snap: dpkgSnapshot(core.SourceOK, deb("meld", core.KindApp))}
	out, items, act = removeLonelyTool(t, gone)
	if out.Count(core.VerifyConfirmed) != 1 || len(items) != 1 || items[0].Name != "meld" || act.Outcome != core.OutcomeSuccess ||
		!strings.Contains(act.Details, "Verified lonely-tool: confirmed") {
		t.Errorf("confirmed: %+v / %+v / %+v", out, items, act)
	}
}
