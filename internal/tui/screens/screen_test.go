package screens

import (
	"testing"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
)

// Windows clocks can give two quick scans the same timestamp; staleness
// must not depend on timestamps differing.
func TestAnalysisStaleWithIdenticalTimestamps(t *testing.T) {
	at := time.Date(2026, 11, 13, 2, 0, 0, 0, time.UTC)
	d := DataView{
		Summary:  &core.ScanSummary{ID: 2, FinishedAt: at},
		Analysis: &core.AnalysisResult{BasedOnScanID: 1, BasedOnScan: at, FinishedAt: at},
	}
	if !d.AnalysisStale() {
		t.Error("a newer scan with the same timestamp must still make the analysis stale")
	}
	d.Analysis.BasedOnScanID = 2
	if d.AnalysisStale() {
		t.Error("an analysis of the latest scan is not stale")
	}
	d.RemovedSinceAnalysis = true
	if !d.AnalysisStale() {
		t.Error("removing software after the analysis makes it stale")
	}
}
