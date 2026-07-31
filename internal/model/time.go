package model

import "time"

// Timestamp is an RFC3339 UTC string for JSON and Wails bindings (time.Time is unsupported).
type Timestamp string

// TimestampFrom converts t to Timestamp.
func TimestampFrom(t time.Time) Timestamp {
	if t.IsZero() {
		return ""
	}
	return Timestamp(t.UTC().Format(time.RFC3339))
}

// Time parses the timestamp.
func (t Timestamp) Time() time.Time {
	if t == "" {
		return time.Time{}
	}
	tt, err := time.Parse(time.RFC3339, string(t))
	if err != nil {
		return time.Time{}
	}
	return tt
}

// TimestampPtrFrom converts an optional time.
func TimestampPtrFrom(t *time.Time) *Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	ts := TimestampFrom(*t)
	return &ts
}

// TimePtr parses an optional timestamp.
func (t *Timestamp) TimePtr() *time.Time {
	if t == nil {
		return nil
	}
	tt := t.Time()
	if tt.IsZero() {
		return nil
	}
	return &tt
}

// Before reports whether t is before u.
func (t Timestamp) Before(u Timestamp) bool {
	return t.Time().Before(u.Time())
}

// After reports whether t is after u.
func (t Timestamp) After(u Timestamp) bool {
	return t.Time().After(u.Time())
}

// Equal reports whether t equals u.
func (t Timestamp) Equal(u Timestamp) bool {
	return t.Time().Equal(u.Time())
}

// IsZero reports whether t is unset.
func (t Timestamp) IsZero() bool {
	return t == "" || t.Time().IsZero()
}

// DurationMS converts a duration to milliseconds for JSON/Wails bindings.
func DurationMS(d time.Duration) int64 {
	return d.Milliseconds()
}

// DurationFromMS parses milliseconds into a duration.
func DurationFromMS(ms int64) time.Duration {
	return time.Duration(ms) * time.Millisecond
}
