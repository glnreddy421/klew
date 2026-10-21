// Package history keeps a local, read-only record of Kubernetes Events so
// investigations can see events that have already expired from the API server.
//
// Data never leaves the machine. Event message bodies are stored but never logged.
package history

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

const schemaVersion = "1"

// Store is the SQLite-backed event history. Safe for concurrent use and for
// several Klew processes sharing one file (WAL + busy_timeout).
type Store struct {
	db   *sql.DB
	path string

	// writeMu serializes writers inside this process so they queue in Go
	// rather than spinning on SQLITE_BUSY.
	writeMu sync.Mutex
}

// EventRow is one stored Event. Times are UTC.
type EventRow struct {
	ClusterKey          string
	UID                 string
	Namespace           string
	RegardingKind       string
	RegardingName       string
	RegardingUID        string
	Type                string
	Reason              string
	Message             string
	ReportingController string
	Action              string
	Count               int32
	SeriesCount         int32
	FirstSeen           time.Time
	LastSeen            time.Time
	RecordedAt          time.Time
}

// WatchState is the resume point for one watch stream. Namespace is "" for a
// cluster-wide watch.
type WatchState struct {
	ClusterKey      string
	Namespace       string
	API             string
	ResourceVersion string
	UpdatedAt       time.Time
}

// Open creates or opens the database at path.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	q := url.Values{}
	// auto_vacuum must precede table creation to take effect on a new file.
	q.Add("_pragma", "auto_vacuum(INCREMENTAL)")
	q.Add("_pragma", "busy_timeout(5000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "synchronous(NORMAL)")
	q.Set("_txlock", "immediate")
	dsn := "file:" + path + "?" + q.Encode()
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	s := &Store{db: db, path: path}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("event history schema: %w", err)
	}
	_ = os.Chmod(path, 0o600)
	return s, nil
}

// Path returns the database file path.
func (s *Store) Path() string { return s.path }

// Close closes the database.
func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) migrate() error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS events (
			cluster_key TEXT NOT NULL,
			event_uid TEXT NOT NULL,
			namespace TEXT NOT NULL DEFAULT '',
			regarding_kind TEXT NOT NULL DEFAULT '',
			regarding_name TEXT NOT NULL DEFAULT '',
			regarding_uid TEXT NOT NULL DEFAULT '',
			type TEXT NOT NULL DEFAULT '',
			reason TEXT NOT NULL DEFAULT '',
			message TEXT NOT NULL DEFAULT '',
			reporting_controller TEXT NOT NULL DEFAULT '',
			action TEXT NOT NULL DEFAULT '',
			count INTEGER NOT NULL DEFAULT 1,
			series_count INTEGER NOT NULL DEFAULT 0,
			first_seen INTEGER NOT NULL,
			last_seen INTEGER NOT NULL,
			recorded_at INTEGER NOT NULL,
			PRIMARY KEY (cluster_key, event_uid)
		)`,
		`CREATE INDEX IF NOT EXISTS events_ns_last ON events (cluster_key, namespace, last_seen)`,
		`CREATE INDEX IF NOT EXISTS events_last ON events (last_seen)`,
		`CREATE TABLE IF NOT EXISTS watch_state (
			cluster_key TEXT NOT NULL,
			namespace TEXT NOT NULL DEFAULT '',
			api TEXT NOT NULL,
			resource_version TEXT NOT NULL,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY (cluster_key, namespace)
		)`,
		`CREATE TABLE IF NOT EXISTS coverage (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			cluster_key TEXT NOT NULL,
			scope TEXT NOT NULL DEFAULT '',
			started_at INTEGER NOT NULL,
			ended_at INTEGER NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS coverage_key_start ON coverage (cluster_key, started_at)`,
		`INSERT OR IGNORE INTO meta (key, value) VALUES ('schema_version', '` + schemaVersion + `')`,
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return err
		}
	}
	return nil
}

func ms(t time.Time) int64 {
	if t.IsZero() {
		return 0
	}
	return t.UTC().UnixMilli()
}

func fromMS(v int64) time.Time {
	if v == 0 {
		return time.Time{}
	}
	return time.UnixMilli(v).UTC()
}

const upsertEventSQL = `INSERT INTO events (
	cluster_key, event_uid, namespace, regarding_kind, regarding_name, regarding_uid,
	type, reason, message, reporting_controller, action, count, series_count,
	first_seen, last_seen, recorded_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (cluster_key, event_uid) DO UPDATE SET
	first_seen = MIN(events.first_seen, excluded.first_seen),
	last_seen = MAX(events.last_seen, excluded.last_seen),
	count = MAX(events.count, excluded.count),
	series_count = MAX(events.series_count, excluded.series_count),
	message = CASE WHEN excluded.last_seen >= events.last_seen THEN excluded.message ELSE events.message END,
	type = excluded.type,
	reason = excluded.reason,
	action = excluded.action,
	reporting_controller = excluded.reporting_controller,
	recorded_at = excluded.recorded_at`

// WriteBatch upserts events and saves watch resume points in one transaction,
// so a crash never leaves a resourceVersion ahead of the events it covers.
func (s *Store) WriteBatch(ctx context.Context, rows []EventRow, states []WatchState) error {
	if len(rows) == 0 && len(states) == 0 {
		return nil
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if len(rows) > 0 {
		stmt, err := tx.PrepareContext(ctx, upsertEventSQL)
		if err != nil {
			return err
		}
		defer stmt.Close()
		for _, r := range rows {
			if r.ClusterKey == "" || r.UID == "" {
				continue
			}
			first, last := r.FirstSeen, r.LastSeen
			if first.IsZero() {
				first = last
			}
			if last.IsZero() {
				last = first
			}
			if first.IsZero() {
				first = r.RecordedAt
				last = r.RecordedAt
			}
			if last.Before(first) {
				last = first
			}
			count := r.Count
			if count <= 0 {
				count = 1
			}
			if _, err := stmt.ExecContext(ctx,
				r.ClusterKey, r.UID, r.Namespace, r.RegardingKind, r.RegardingName, r.RegardingUID,
				r.Type, r.Reason, r.Message, r.ReportingController, r.Action, count, r.SeriesCount,
				ms(first), ms(last), ms(r.RecordedAt),
			); err != nil {
				return err
			}
		}
	}
	for _, st := range states {
		if st.ResourceVersion == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO watch_state (cluster_key, namespace, api, resource_version, updated_at)
			VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (cluster_key, namespace) DO UPDATE SET api = excluded.api,
				resource_version = excluded.resource_version, updated_at = excluded.updated_at`,
			st.ClusterKey, st.Namespace, st.API, st.ResourceVersion, ms(st.UpdatedAt)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// UpsertEvents is WriteBatch without watch state.
func (s *Store) UpsertEvents(ctx context.Context, rows []EventRow) error {
	return s.WriteBatch(ctx, rows, nil)
}

// LoadWatchState returns the saved resume point, if any.
func (s *Store) LoadWatchState(ctx context.Context, clusterKey, namespace string) (WatchState, bool, error) {
	var st WatchState
	var updated int64
	err := s.db.QueryRowContext(ctx, `SELECT cluster_key, namespace, api, resource_version, updated_at
		FROM watch_state WHERE cluster_key = ? AND namespace = ?`, clusterKey, namespace).
		Scan(&st.ClusterKey, &st.Namespace, &st.API, &st.ResourceVersion, &updated)
	if errors.Is(err, sql.ErrNoRows) {
		return WatchState{}, false, nil
	}
	if err != nil {
		return WatchState{}, false, err
	}
	st.UpdatedAt = fromMS(updated)
	return st, true, nil
}

// ClearWatchState drops resume points for a cluster (forces a relist).
func (s *Store) ClearWatchState(ctx context.Context, clusterKey string) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM watch_state WHERE cluster_key = ?`, clusterKey)
	return err
}

// EventQuery selects stored events.
type EventQuery struct {
	ClusterKey string
	// Namespace filters by event namespace; "" means all namespaces.
	Namespace string
	// Since/Until bound last_seen; zero means unbounded.
	Since time.Time
	Until time.Time
	// RegardingName limits to one object; NamePrefix also matches "<name>-*".
	RegardingName string
	NamePrefix    bool
	// Limit caps rows (newest last_seen first); <= 0 means 1000.
	Limit int
}

// Events returns stored events, newest last_seen first.
func (s *Store) Events(ctx context.Context, q EventQuery) ([]EventRow, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 1000
	}
	where := `cluster_key = ?`
	args := []any{q.ClusterKey}
	if q.Namespace != "" {
		where += ` AND namespace = ?`
		args = append(args, q.Namespace)
	}
	if !q.Since.IsZero() {
		where += ` AND last_seen >= ?`
		args = append(args, ms(q.Since))
	}
	if !q.Until.IsZero() {
		where += ` AND first_seen <= ?`
		args = append(args, ms(q.Until))
	}
	if q.RegardingName != "" {
		if q.NamePrefix {
			where += ` AND (regarding_name = ? OR regarding_name LIKE ? ESCAPE '\')`
			args = append(args, q.RegardingName, likeEscape(q.RegardingName)+"-%")
		} else {
			where += ` AND regarding_name = ?`
			args = append(args, q.RegardingName)
		}
	}
	args = append(args, limit)
	rows, err := s.db.QueryContext(ctx, `SELECT cluster_key, event_uid, namespace, regarding_kind, regarding_name,
		regarding_uid, type, reason, message, reporting_controller, action, count, series_count,
		first_seen, last_seen, recorded_at FROM events WHERE `+where+` ORDER BY last_seen DESC LIMIT ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []EventRow
	for rows.Next() {
		var r EventRow
		var first, last, rec int64
		if err := rows.Scan(&r.ClusterKey, &r.UID, &r.Namespace, &r.RegardingKind, &r.RegardingName,
			&r.RegardingUID, &r.Type, &r.Reason, &r.Message, &r.ReportingController, &r.Action,
			&r.Count, &r.SeriesCount, &first, &last, &rec); err != nil {
			return nil, err
		}
		r.FirstSeen, r.LastSeen, r.RecordedAt = fromMS(first), fromMS(last), fromMS(rec)
		out = append(out, r)
	}
	return out, rows.Err()
}

// CountEvents returns the number of stored events for a cluster ("" = all).
func (s *Store) CountEvents(ctx context.Context, clusterKey string) (int64, error) {
	var n int64
	var err error
	if clusterKey == "" {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&n)
	} else {
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE cluster_key = ?`, clusterKey).Scan(&n)
	}
	return n, err
}

// Clear deletes all recorded history (events, coverage, resume points) and
// compacts the file.
func (s *Store) Clear(ctx context.Context) error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	for _, stmt := range []string{`DELETE FROM events`, `DELETE FROM coverage`, `DELETE FROM watch_state`} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}
	_, _ = s.db.ExecContext(ctx, `VACUUM`)
	_, _ = s.db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
	return nil
}

// SizeBytes is the on-disk size of the database including its WAL.
func (s *Store) SizeBytes() int64 {
	var total int64
	for _, p := range []string{s.path, s.path + "-wal", s.path + "-shm"} {
		if st, err := os.Stat(p); err == nil {
			total += st.Size()
		}
	}
	return total
}

// usedBytes is the space held by live pages (excludes free pages awaiting vacuum).
func (s *Store) usedBytes(ctx context.Context) (int64, error) {
	var pages, free, size int64
	if err := s.db.QueryRowContext(ctx, `PRAGMA page_count`).Scan(&pages); err != nil {
		return 0, err
	}
	if err := s.db.QueryRowContext(ctx, `PRAGMA freelist_count`).Scan(&free); err != nil {
		return 0, err
	}
	if err := s.db.QueryRowContext(ctx, `PRAGMA page_size`).Scan(&size); err != nil {
		return 0, err
	}
	return (pages - free) * size, nil
}

// Prune drops events whose last_seen is older than maxAge, then the oldest
// events until the live data fits in maxBytes. Coverage older than maxAge is
// trimmed too. Returns the number of events removed.
func (s *Store) Prune(ctx context.Context, now time.Time, maxAge time.Duration, maxBytes int64) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	var removed int64
	if maxAge > 0 {
		cutoff := ms(now.Add(-maxAge))
		res, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE last_seen < ?`, cutoff)
		if err != nil {
			return removed, err
		}
		n, _ := res.RowsAffected()
		removed += n
		if _, err := s.db.ExecContext(ctx, `DELETE FROM coverage WHERE ended_at < ?`, cutoff); err != nil {
			return removed, err
		}
		if _, err := s.db.ExecContext(ctx, `UPDATE coverage SET started_at = ? WHERE started_at < ?`, cutoff, cutoff); err != nil {
			return removed, err
		}
	}
	sizePruned := false
	if maxBytes > 0 {
		// Estimate bytes per row and keep the newest rows that fit in 90% of
		// the budget. The file is compacted afterwards (below) so the next
		// prune measures real usage instead of half-empty pages.
		used, err := s.usedBytes(ctx)
		if err != nil {
			return removed, err
		}
		if used > maxBytes {
			var rows int64
			if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events`).Scan(&rows); err != nil {
				return removed, err
			}
			if rows > 0 {
				perRow := used / rows
				if perRow < 1 {
					perRow = 1
				}
				keep := (maxBytes * 9 / 10) / perRow
				if drop := rows - keep; drop > 0 {
					res, err := s.db.ExecContext(ctx, `DELETE FROM events WHERE rowid IN (
						SELECT rowid FROM events ORDER BY last_seen ASC LIMIT ?)`, drop)
					if err != nil {
						return removed, err
					}
					n, _ := res.RowsAffected()
					removed += n
					sizePruned = n > 0
				}
			}
		}
	}
	switch {
	case sizePruned:
		// Only when over the size limit: rebuilds the file so freed space is
		// returned and usage estimates stay stable.
		_, _ = s.db.ExecContext(ctx, `VACUUM`)
		_, _ = s.db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`)
	case removed > 0:
		_, _ = s.db.ExecContext(ctx, `PRAGMA incremental_vacuum`)
	}
	return removed, nil
}

func likeEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}
