package selftest

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestViaPhraseIsPlainLanguage(t *testing.T) {
	if got := viaPhrase(nil); got != "" {
		t.Errorf("empty: got %q", got)
	}
	got := viaPhrase(map[string]int{"winget": 22, "scoop": 3})
	if got != " (3 via scoop, 22 via winget)" {
		t.Errorf("got %q", got)
	}
}

// The self-test must run on any machine without touching the user's data and
// must always produce a readable report, even where tools are missing.
func TestRunWritesAReport(t *testing.T) {
	if testing.Short() {
		t.Skip("reads this computer's real software list")
	}
	var buf bytes.Buffer
	res, err := Run(context.Background(), &buf, "test")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()
	for _, want := range []string{"## Summary", "## Discover Apps"} {
		if !strings.Contains(out, want) {
			t.Errorf("report is missing %q", want)
		}
	}
	if strings.Contains(out, "map[") {
		t.Error("report shows raw Go map formatting")
	}
	if res.Passed == 0 {
		t.Error("expected at least one passing check")
	}
}
