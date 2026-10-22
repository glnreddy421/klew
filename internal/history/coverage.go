package history

import (
	"context"
	"errors"
	"sort"
	"time"
)

// GapTolerance is the largest hole between recording sessions that is not
// reported as a gap (heartbeat granularity and quick reconnects).
const GapTolerance = 2 * time.Minute

// DefaultEventTTL is kube-apiserver's default --event-ttl.
const DefaultEventTTL = time.Hour

// Reasons a coverage session ended. Empty means unknown (crash, sleep, or an
// internal restart).
const (
	EndKlewClosed       = "klew_closed"
	EndMonitoringPaused = "monitoring_paused"
	EndUnreachable      = "cluster_unreachable"
	EndRecordingOff     = "recording_off"
	EndRBACDenied       = "rbac_denied"
	EndContextInactive  = "context_inactive"
	// EndSystemSleep: a late heartbeat and the OS confirms it woke since the
	// last touch (macOS kern.waketime).
	EndSystemSleep = "system_sleep"
)

// Session is one recording period for a cluster as stored.
type Session struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
	Scope string    `json:"scope,omitempty"`
	// Backfilled means the session began with a successful list or a watch
	// resumed from a saved resourceVersion, so events still held by the API
	// server (up to the event TTL) were captured.
	Backfilled bool   `json:"backfilled,omitempty"`
	EndReason  string `json:"endReason,omitempty"`
}

// Gap is a period inside the history window with no recording.
type Gap struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
	// Reason is why recording stopped before the gap ("" when unknown).
	Reason string `json:"reason,omitempty"`
}

// Coverage summarises what part of a time range was recorded.
type Coverage struct {
	ClusterKey string `json:"clusterKey"`
	// Since is the start of the earliest effective coverage in range (zero when
	// nothing was recorded).
	Since time.Time `json:"since"`
	Until time.Time `json:"until"`
	// Sessions are the effective, merged covered ranges.
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
func (s *Store) BeginSession(ctx context.Context, clusterKey, scope string, start time.Time, backfilled bool) (int64, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.ExecContext(ctx, `INSERT INTO coverage (cluster_key, scope, started_at, ended_at, backfilled, end_reason)
		VALUES (?, ?, ?, ?, ?, '')`, clusterKey, scope, ms(start), ms(start), boolInt(backfilled))
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// ErrNoSession means the coverage row is gone (history was cleared, possibly
// from another window); the caller should begin a new session.
var ErrNoSession = errors.New("coverage session not found")

// TouchSession extends a coverage row to t and clears any provisional end
// reason (recording is demonstrably still running).
func (s *Store) TouchSession(ctx context.Context, id int64, t time.Time) error {
	return s.updateSession(ctx, id, `UPDATE coverage SET ended_at = MAX(ended_at, ?), end_reason = '' WHERE id = ?`, ms(t), id)
}

// MarkBackfilled records that the session captured the API server's retained events.
func (s *Store) MarkBackfilled(ctx context.Context, id int64) error {
	return s.updateSession(ctx, id, `UPDATE coverage SET backfilled = 1 WHERE id = ?`, id)
}

// SetEndReason records why the session stopped (or why it is currently stalled).
func (s *Store) SetEndReason(ctx context.Context, id int64, reason string) error {
	return s.updateSession(ctx, id, `UPDATE coverage SET end_reason = ? WHERE id = ?`, reason, id)
}

// EndSession extends the session to t and records why it ended.
func (s *Store) EndSession(ctx context.Context, id int64, t time.Time, reason string) error {
	return s.updateSession(ctx, id, `UPDATE coverage SET ended_at = MAX(ended_at, ?), end_reason = ? WHERE id = ?`, ms(t), reason, id)
}

func (s *Store) updateSession(ctx context.Context, id int64, query string, args ...any) error {
	if id == 0 {
		return ErrNoSession
	}
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNoSession
	}
	return nil
}

// Coverage returns effective coverage and gaps for clusterKey in [since, until]
// (until zero means now). A backfilled session also covers the eventTTL before
// it started — the API server still held those events when it listed — but
// never earlier than the previous session's end.
func (s *Store) Coverage(ctx context.Context, clusterKey string, since, until time.Time, eventTTL time.Duration) (Coverage, error) {
	if until.IsZero() {
		until = time.Now().UTC()
	}
	if eventTTL < 0 {
		eventTTL = 0
	}
	cov := Coverage{ClusterKey: clusterKey, Until: until}
	args := []any{clusterKey, ms(until.Add(eventTTL))}
	where := `cluster_key = ? AND started_at <= ?`
	if !since.IsZero() {
		// Sessions ending shortly before `since` still bound the next session's backfill.
		where += ` AND ended_at >= ?`
		args = append(args, ms(since.Add(-eventTTL-GapTolerance)))
	}
	rows, err := s.db.QueryContext(ctx, `SELECT scope, started_at, ended_at, backfilled, end_reason
		FROM coverage WHERE `+where+` ORDER BY started_at`, args...)
	if err != nil {
		return cov, err
	}
	defer rows.Close()
	var sessions []Session
	for rows.Next() {
		var sess Session
		var st, en, bf int64
		if err := rows.Scan(&sess.Scope, &st, &en, &bf, &sess.EndReason); err != nil {
			return cov, err
		}
		sess.Start, sess.End, sess.Backfilled = fromMS(st), fromMS(en), bf != 0
		sessions = append(sessions, sess)
	}
	if err := rows.Err(); err != nil {
		return cov, err
	}
	effective := clipSessions(mergeSessions(effectiveSessions(sessions, eventTTL)), since, until)
	cov.Sessions = effective
	cov.Gaps = gapsBetween(effective, until)
	if len(effective) > 0 {
		cov.Since = effective[0].Start
	}
	return cov, nil
}

// effectiveSessions moves each backfilled session's start back by eventTTL,
// bounded by the end of everything recorded before it.
func effectiveSessions(in []Session, eventTTL time.Duration) []Session {
	sort.SliceStable(in, func(i, j int) bool { return in[i].Start.Before(in[j].Start) })
	out := make([]Session, 0, len(in))
	var prevEnd time.Time
	for _, s := range in {
		eff := s
		if s.Backfilled && eventTTL > 0 {
			start := s.Start.Add(-eventTTL)
			if !prevEnd.IsZero() && start.Before(prevEnd) {
				start = prevEnd
			}
			if start.Before(eff.Start) {
				eff.Start = start
			}
		}
		out = append(out, eff)
		if s.End.After(prevEnd) {
			prevEnd = s.End
		}
	}
	return out
}

// clipSessions keeps the part of each range inside [since, until].
func clipSessions(in []Session, since, until time.Time) []Session {
	var out []Session
	for _, s := range in {
		if !since.IsZero() && s.End.Before(since) {
			continue
		}
		if s.Start.After(until) {
			continue
		}
		if !since.IsZero() && s.Start.Before(since) {
			s.Start = since
		}
		if s.End.After(until) {
			s.End = until
		}
		out = append(out, s)
	}
	return out
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

// mergeSessions sorts and joins ranges that overlap or sit within GapTolerance.
// A merged range keeps the end reason of whichever part ends last.
func mergeSessions(in []Session) []Session {
	if len(in) == 0 {
		return nil
	}
	sort.SliceStable(in, func(i, j int) bool { return in[i].Start.Before(in[j].Start) })
	out := []Session{in[0]}
	for _, s := range in[1:] {
		last := &out[len(out)-1]
		if !s.Start.After(last.End.Add(GapTolerance)) {
			if s.End.After(last.End) {
				last.End = s.End
				last.EndReason = s.EndReason
			}
			last.Backfilled = last.Backfilled || s.Backfilled
			continue
		}
		out = append(out, s)
	}
	return out
}

func gapsBetween(sessions []Session, until time.Time) []Gap {
	var gaps []Gap
	for i := 1; i < len(sessions); i++ {
		gaps = append(gaps, Gap{From: sessions[i-1].End, To: sessions[i].Start, Reason: sessions[i-1].EndReason})
	}
	if n := len(sessions); n > 0 && until.Sub(sessions[n-1].End) > GapTolerance {
		gaps = append(gaps, Gap{From: sessions[n-1].End, To: until, Reason: sessions[n-1].EndReason})
	}
	return gaps
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
