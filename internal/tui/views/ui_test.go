package views

import "testing"

func TestClampScroll(t *testing.T) {
	t.Parallel()
	if got := ClampScroll(10, 12, 5); got != 7 {
		t.Fatalf("got %d want 7", got)
	}
	if got := ClampScroll(-1, 12, 5); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
	if got := ClampScroll(0, 3, 5); got != 0 {
		t.Fatalf("got %d want 0", got)
	}
}
