package history

import (
	"context"
	"errors"
	"sort"
	"time"
)

// GapTolerance is the largest hole between recording sessions that is not
// reported as a gap (covers reconnects and the coverage heartbeat interval).
const GapTolerance = 2 * time.Minute

// Session is one contiguous recording period for a cluster.
type Session struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Scope string    `json:"scope,omitempty"`
}

// Gap is a period inside the history window with no recording.
type Gap struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// Coverage summarises what part of a time range was recorded.
type Coverage struct {
	ClusterKey string `json:"clusterKey"`
	// Since is the start of the earliest recorded session in range (zero when
	// nothing was recorded).
	Since    time.Time `json:"since"`
	Until    time.Time `json:"until"`
	Sessions []Session `json:"sessions,omitempty"`
	Gaps     []Gap     `json:"gaps,omitempty"`
}

// Recorded reports whether any part of the range is covered.
func (c Coverage) Recorded() bool { return !c.Since.IsZero() }

// GapsOverlapping returns gaps that intersect [from, to], clipped to that range.
func (c Coverage) GapsOverlapping(from, to time.Time) []Gap {
	var out []Gap
	for _, g := range c.Gaps {
		if !to.IsZero() && !g.From.Before(to) {
			continue
		}
		if !from.IsZero() && !g.To.After(from) {
			continue
		}
		cg := g
		if !from.IsZero() && cg.From.Before(from) {
			cg.From = from
		}
		if !to.IsZero() && cg.To.After(to) {
			cg.To = to
		}
		out = append(out, cg)
	}
	return out
}

// BeginSession opens a coverage row; End starts equal to start and is moved
// forward by TouchSession, so a crash leaves an honest end time.
func (s *Store) BeginSession(ctx context.Context, clusterKey, scope string, start time.Time) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.ExecContext(ctx, `INSERT INTO coverage (cluster_key, scope, started_at, ended_at) VALUES (?, ?, ?, ?)`,
		clusterKey, scope, ms(start), ms(start))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ErrNoSession means the coverage row is gone (history was cleared, possibly
// from another window); the caller should begin a new session.
var ErrNoSession = errors.New("coverage session not found")

// TouchSession extends a coverage row to t.
func (s *Store) TouchSession(ctx context.Context, id int64, t time.Time) error {
	if id == 0 {
		return ErrNoSession
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.ExecContext(ctx, `UPDATE coverage SET ended_at = MAX(ended_at, ?) WHERE id = ?`, ms(t), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoSession
	}
	return nil
}

// Coverage returns recorded sessions and gaps for clusterKey in [since, until].
// until zero means now. A trailing gap is reported only when the last session
// ended more than GapTolerance before until.
func (s *Store) Coverage(ctx context.Context, clusterKey string, since, until time.Time) (Coverage, error) {
	if until.IsZero() {
		until = time.Now().UTC()
	}
	cov := Coverage{ClusterKey: clusterKey, Until: until}
	args := []any{clusterKey, ms(until)}
	where := `cluster_key = ? AND started_at <= ?`
	if !since.IsZero() {
		where += ` AND ended_at >= ?`
		args = append(args, ms(since))
	}
	rows, err := s.db.QueryContext(ctx, `SELECT scope, started_at, ended_at FROM coverage WHERE `+where+` ORDER BY started_at`, args...)
	if err != nil {
		return cov, err
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var scope string
		var st, en int64
		if err := rows.Scan(&scope, &st, &en); err != nil {
			return cov, err
		}
		sessions = append(sessions, Session{Start: fromMS(st), End: fromMS(en), Scope: scope})
	}
	if err := rows.Err(); err != nil {
		return cov, err
	}
	cov.Sessions = mergeSessions(sessions)
	cov.Gaps = gapsBetween(cov.Sessions, until)
	if len(cov.Sessions) > 0 {
		cov.Since = cov.Sessions[0].Start
		if !since.IsZero() && cov.Since.Before(since) {
			cov.Since = since
		}
	}
	return cov, nil
}

// HistoryStart is the earliest recorded moment for clusterKey (zero if none).
func (s *Store) HistoryStart(ctx context.Context, clusterKey string) (time.Time, error) {
	var v *int64
	if err := s.db.QueryRowContext(ctx, `SELECT MIN(started_at) FROM coverage WHERE cluster_key = ?`, clusterKey).Scan(&v); err != nil {
		return time.Time{}, err
	}
	if v == nil {
		return time.Time{}, nil
	}
	return fromMS(*v), nil
}

// mergeSessions sorts and joins sessions that overlap or sit within GapTolerance.
func mergeSessions(in []Session) []Session {
	if len(in) == 0 {
		return nil
	}
	sort.Slice(in, func(i, j int) bool { return in[i].Start.Before(in[j].Start) })
	out := []Session{in[0]}
	for _, s := range in[1:] {
		last := &out[len(out)-1]
		if !s.Start.After(last.End.Add(GapTolerance)) {
			if s.End.After(last.End) {
				last.End = s.End
			}
			continue
		}
		out = append(out, s)
	}
	return out
}

func gapsBetween(sessions []Session, until time.Time) []Gap {
	var gaps []Gap
	for i := 1; i < len(sessions); i++ {
		gaps = append(gaps, Gap{From: sessions[i-1].End, To: sessions[i].Start})
	}
	if n := len(sessions); n > 0 && until.Sub(sessions[n-1].End) > GapTolerance {
		gaps = append(gaps, Gap{From: sessions[n-1].End, To: until})
	}
	return gaps
}
