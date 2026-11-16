package handover

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/deepakraj1207/atlas/internal/core"
)

// journalStore is the part of core.Store the journal needs.
type journalStore interface {
	AddActivity(ctx context.Context, a core.Activity) (int64, error)
	RemoveActivity(ctx context.Context, id int64) error
}

// Journal keeps ATLAS's promise that nothing happens silently even when a
// run is cut short. Just before the terminal is handed to a package manager
// it writes a "started" entry to Activity History. When the run's real
// outcome is recorded, that entry is removed. If ATLAS is closed in between
// (the terminal window is shut, the computer restarts), the entry stays and
// says what was started and that the result is unknown.
type Journal struct {
	mu    sync.Mutex
	store journalStore
	log   *slog.Logger
	now   func() time.Time
	id    int64
}

// NewJournal returns a journal writing to store.
func NewJournal(store journalStore, log *slog.Logger) *Journal {
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Journal{store: store, log: log, now: time.Now}
}

// Started notes that the steps are about to run. kind is the activity kind
// ("updates.apply"…), doing is "installing updates for" / "removing" /
// "installing".
func (j *Journal) Started(kind, doing string, steps []core.UpdateStep) {
	var names []string
	var commands []string
	for _, st := range steps {
		if st.Blocked != "" {
			continue
		}
		for _, u := range st.Updates {
			names = append(names, u.Name)
		}
		for _, c := range st.Commands {
			commands = append(commands, c.String())
		}
	}
	if len(names) == 0 {
		return
	}
	a := core.Activity{
		At:      j.now(),
		Kind:    kind,
		Title:   "Started " + doing + " " + list(names),
		Outcome: core.OutcomeStarted,
		Summary: "ATLAS handed the terminal to your package manager, and was closed before the result came back. " +
			"It doesn't know whether this finished. Scan again (Dashboard) to see what is really installed now.",
		Details: "Commands that were about to run:\n" + strings.Join(commands, "\n"),
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	id, err := j.store.AddActivity(context.Background(), a)
	if err != nil {
		j.log.Error("note that a run started", "error", err)
		return
	}
	j.id = id
}

// Finished removes the "started" note; the caller records the real outcome.
func (j *Journal) Finished(ctx context.Context) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.id == 0 {
		return
	}
	if err := j.store.RemoveActivity(context.WithoutCancel(ctx), j.id); err != nil {
		j.log.Error("clear started note", "error", err)
	}
	j.id = 0
}

// list is "a", "a and b", or "a, b and 3 more".
func list(names []string) string {
	switch len(names) {
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	default:
		return fmt.Sprintf("%s, %s and %d more", names[0], names[1], len(names)-2)
	}
}
