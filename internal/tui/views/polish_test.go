package views

import "testing"

func TestKVLinesWrapsLongValue(t *testing.T) {
	t.Parallel()
	lines := kvLines("Memory", "192Mi req · 384Mi limit", 28)
	if len(lines) != 2 {
		t.Fatalf("lines=%d want stacked value", len(lines))
	}
	if lines[1] == "" || !containsPlain(lines[1], "384Mi limit") {
		t.Fatalf("missing full value: %q", lines[1])
	}
}

func TestWrapPanelLineNoEllipsis(t *testing.T) {
	t.Parallel()
	lines := wrapPanelLine("Memory                192Mi req · 384Mi limit", 30)
	if len(lines) < 2 {
		t.Fatalf("expected wrap, got %v", lines)
	}
	joined := stringsJoin(lines)
	if containsPlain(joined, "…") {
		t.Fatalf("wrapped line truncated: %v", lines)
	}
}

func containsPlain(s, sub string) bool {
	return stringsContains(stripANSI(s), sub)
}

func stringsJoin(ss []string) string {
	out := ""
	for _, s := range ss {
		out += s
	}
	return out
}

func stringsContains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexString(s, sub) >= 0)
}

func indexString(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}
