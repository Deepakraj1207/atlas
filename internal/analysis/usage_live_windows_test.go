//go:build windows

package analysis

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
)

// Reads this user's real UserAssist records. A service account may have
// none, which is reported as unavailable, never as an error.
func TestUserAssistReadsThisComputer(t *testing.T) {
	in, err := NewUsageSource().Read(context.Background())
	if errors.Is(err, core.ErrSourceUnavailable) {
		t.Skip("no UserAssist key for this user (normal for service accounts)")
	}
	if err != nil {
		t.Fatalf("UserAssist: %v", err)
	}
	for _, u := range in.Usage {
		if u.Path == "" || u.LastRun.Before(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)) || u.LastRun.After(time.Now().Add(24*time.Hour)) {
			t.Errorf("implausible record: %+v", u)
		}
	}
	t.Logf("%d usage records", len(in.Usage))
}
