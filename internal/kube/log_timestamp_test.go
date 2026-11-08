package kube

import (
	"testing"
	"time"
)

func TestSplitLogTimestamp(t *testing.T) {
	fallback := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		line     string
		wantTime time.Time
		wantText string
	}{
		{"2026-11-06T13:04:05.123456789Z GET /healthz 200", time.Date(2026, 11, 6, 13, 4, 5, 123456789, time.UTC), "GET /healthz 200"},
		{"2026-11-06T08:04:05-05:00 started", time.Date(2026, 11, 6, 13, 4, 5, 0, time.UTC), "started"},
		{"2026-11-06T13:04:05Z", time.Date(2026, 11, 6, 13, 4, 5, 0, time.UTC), ""},
		{"no timestamp here", fallback, "no timestamp here"},
		{"", fallback, ""},
	}
	for _, c := range cases {
		gotTime, gotText := splitLogTimestamp(c.line, fallback)
		if !gotTime.Equal(c.wantTime) || gotText != c.wantText {
			t.Errorf("splitLogTimestamp(%q) = %v, %q; want %v, %q", c.line, gotTime, gotText, c.wantTime, c.wantText)
		}
	}
}
