package inventory

import (
	"context"
	"strings"

	"github.com/deepakraj1207/atlas/internal/core"
)

// Snapshot runs every scanner now (read-only) and returns each source's
// result and own items, unmerged and unsaved. Removals and updates use it to
// check what really changed instead of trusting an exit code.
func (s *Service) Snapshot(ctx context.Context) core.Snapshot {
	infos := s.Sources()
	results, found := RunAll(ctx, infos, func(ctx context.Context, i int) ([]core.SoftwareItem, error) {
		return s.scanners[i].Scan(ctx)
	}, s.timeoutFor, func(core.SourceEvent) {})
	snap := core.Snapshot{Sources: results, BySource: map[string][]core.SoftwareItem{}}
	for i, r := range results {
		if r.Status == core.SourceOK {
			snap.BySource[r.Source] = found[i]
		}
	}
	return snap
}

// sameSoftware reports whether a snapshot item is the same software as it:
// the same package ID (when both have one) or the same display name.
func sameSoftware(a, b core.SoftwareItem) bool {
	if a.PackageID != "" && b.PackageID != "" && strings.EqualFold(a.PackageID, b.PackageID) {
		return true
	}
	return normName(a.Name) != "" && normName(a.Name) == normName(b.Name)
}

func label(snap core.Snapshot, source string) string {
	if r, ok := snap.Result(source); ok && r.Label != "" {
		return r.Label
	}
	return source
}

// VerifyRemoved checks that a removed item is gone. It is confirmed only
// when no source lists it any more and every source it came from was read;
// if a source still lists it, or one of its sources couldn't be read, the
// removal isn't confirmed and the saved record must be kept.
func VerifyRemoved(snap core.Snapshot, it core.SoftwareItem) core.Verification {
	v := core.Verification{Name: it.Name, Source: it.Source}
	var still []string
	for _, r := range snap.Sources {
		for _, other := range snap.BySource[r.Source] {
			if sameSoftware(it, other) {
				still = append(still, label(snap, r.Source))
				break
			}
		}
	}
	if len(still) > 0 {
		v.State, v.Detail = core.VerifyNotDone, "Still installed: listed by "+strings.Join(still, ", ")+"."
		return v
	}
	for _, src := range append([]string{it.Source}, it.AlsoFoundIn...) {
		r, ok := snap.Result(src)
		if !ok || r.Status != core.SourceOK {
			why := "wasn't checked"
			if ok && r.Message != "" {
				why = strings.ToLower(r.Message[:1]) + r.Message[1:]
			}
			v.State, v.Detail = core.VerifyUnknown, "Couldn't confirm: "+label(snap, src)+" "+why+"."
			return v
		}
	}
	v.State, v.Detail = core.VerifyConfirmed, "Confirmed: no longer installed."
	return v
}

// VerifyUpdated checks an update by reading the version source now lists.
// Confirmed when it shows the new version, or a version different from the
// one before; not done when it still shows the old one; unknown when the
// source couldn't be read or no longer lists the package.
func VerifyUpdated(snap core.Snapshot, source string, u core.Update) core.Verification {
	v := core.Verification{Name: u.Name, Source: source}
	r, ok := snap.Result(source)
	if !ok || r.Status != core.SourceOK {
		v.State, v.Detail = core.VerifyUnknown, "Couldn't confirm: "+label(snap, source)+" couldn't be read."
		return v
	}
	want := core.SoftwareItem{Name: u.Name, PackageID: u.PackageID}
	var now *core.SoftwareItem
	for i, it := range snap.BySource[source] {
		if sameSoftware(want, it) {
			now = &snap.BySource[source][i]
			break
		}
	}
	if now == nil {
		v.State, v.Detail = core.VerifyUnknown, "Couldn't confirm: "+label(snap, source)+" no longer lists it."
		return v
	}
	v.Version = now.Version
	same := func(a, b string) bool {
		// winget's "< 3.10.3" is a bound ("older than"), never equal to a version.
		if a == "" || b == "" || strings.ContainsAny(a[:1]+b[:1], "<>") {
			return false
		}
		return normVersion(a) == normVersion(b) || VersionsEquivalent(a, b)
	}
	switch {
	case same(now.Version, u.AvailableVersion):
		v.State, v.Detail = core.VerifyConfirmed, "Confirmed: now version "+now.Version+"."
	case same(now.Version, u.CurrentVersion):
		v.State, v.Detail = core.VerifyNotDone, "Not updated: still version "+now.Version+"."
	case u.CurrentVersion != "":
		v.State, v.Detail = core.VerifyConfirmed, "Confirmed: now version "+now.Version+" (was "+u.CurrentVersion+")."
	default:
		v.State, v.Detail = core.VerifyUnknown, "Couldn't confirm: version is "+now.Version+", expected "+u.AvailableVersion+"."
	}
	return v
}

// NotVerified is the outcome when nothing could re-read the computer.
func NotVerified(name, source string) core.Verification {
	return core.Verification{Name: name, Source: source, State: core.VerifyUnknown,
		Detail: "Couldn't confirm: ATLAS couldn't re-read your software."}
}

// AddVerification adds what checking the computer found to an activity,
// and lowers its outcome when not everything was confirmed.
func AddVerification(a *core.Activity, out core.RecordOutcome) {
	if len(out.Verifications) == 0 {
		return
	}
	var confirmed, notDone, unknown []string
	var lines []string
	for _, v := range out.Verifications {
		switch v.State {
		case core.VerifyConfirmed:
			confirmed = append(confirmed, v.Name)
		case core.VerifyNotDone:
			notDone = append(notDone, v.Name+" ("+strings.TrimSuffix(v.Detail, ".")+")")
		default:
			unknown = append(unknown, v.Name+" ("+strings.TrimSuffix(v.Detail, ".")+")")
		}
		lines = append(lines, "Verified "+v.Name+": "+string(v.State)+" - "+v.Detail)
	}
	var parts []string
	if len(confirmed) > 0 {
		parts = append(parts, "Checked afterwards and confirmed: "+strings.Join(confirmed, ", ")+".")
	}
	if len(notDone) > 0 {
		parts = append(parts, "Not confirmed: "+strings.Join(notDone, "; ")+".")
	}
	if len(unknown) > 0 {
		parts = append(parts, "Couldn't check: "+strings.Join(unknown, "; ")+". Their records are kept.")
	}
	a.Summary = strings.TrimSpace(a.Summary + " " + strings.Join(parts, " "))
	if a.Details != "" {
		a.Details += "\n"
	}
	a.Details += strings.Join(lines, "\n")
	if len(notDone)+len(unknown) > 0 && (a.Outcome == core.OutcomeSuccess || a.Outcome == core.OutcomeInfo) {
		a.Outcome = core.OutcomePartial
		a.Title += ", but not all confirmed"
	}
}
