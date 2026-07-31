package kube

import "testing"

func TestFuzzyContains(t *testing.T) {
	t.Parallel()
	if !FuzzyContains("payments", "pay") {
		t.Fatal("expected substring match")
	}
	if !FuzzyContains("payments", "pymt") {
		t.Fatal("expected subsequence match")
	}
	if FuzzyContains("payments", "xyz") {
		t.Fatal("expected no match")
	}
}

func TestSortByFuzzy(t *testing.T) {
	t.Parallel()
	got := SortByFuzzy([]string{"default", "payments", "monitoring"}, "payment")
	if len(got) != 3 || got[0] != "payments" {
		t.Fatalf("got %v, want payments first", got)
	}
}
