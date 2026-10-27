package history

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/glnreddy421/klew/internal/model"
)

// Reader reads recorded events without recording, pruning or locking. It is
// for processes that only consume history the desktop app wrote (klew-mcp).
type Reader struct {
	store *Store
	dir   string
	now   func() time.Time
}

// OpenReader opens the history under dir. It returns nil, nil when nothing
// has been recorded yet, so callers can treat history as optional.
func OpenReader(dir string) (*Reader, error) {
	if dir == "" {
		return nil, nil
	}
	path := filepath.Join(dir, "events.db")
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	store, err := Open(path)
	if err != nil {
		return nil, err
	}
	return &Reader{store: store, dir: dir, now: func() time.Time { return time.Now().UTC() }}, nil
}

// Path returns the database file path.
func (r *Reader) Path() string {
	if r == nil {
		return ""
	}
	return r.store.Path()
}

// Close closes the database.
func (r *Reader) Close() error {
	if r == nil {
		return nil
	}
	return r.store.Close()
}

// RecordedEvents implements kube.EventHistory. Settings are re-read on every
// call so retention changes made in the desktop app apply immediately.
func (r *Reader) RecordedEvents(ctx context.Context, contextName, serverURL string, q model.EventHistoryQuery) ([]model.EventRecord, model.EventHistoryCoverage, error) {
	if r == nil {
		return nil, model.EventHistoryCoverage{}, nil
	}
	return recordedEvents(ctx, r.store, loadSettings(r.dir), r.now(), contextName, serverURL, q)
}
