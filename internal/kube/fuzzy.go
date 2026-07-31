package kube

import (
	"sort"
	"strings"
)

// FuzzyContains reports whether needle matches haystack using substring or
// ordered subsequence matching (incremental typing friendly).
func FuzzyContains(haystack, needle string) bool {
	return FuzzyScore(haystack, needle) > 0
}

// FuzzyScore returns a match strength in [0,1]. Zero means no match.
func FuzzyScore(haystack, needle string) float64 {
	if needle == "" {
		return 1
	}
	h := strings.ToLower(haystack)
	n := strings.ToLower(needle)
	if strings.Contains(h, n) {
		if h == n {
			return 1
		}
		return 0.85
	}
	hi, ni := 0, 0
	for hi < len(h) && ni < len(n) {
		if h[hi] == n[ni] {
			ni++
		}
		hi++
	}
	if ni != len(n) {
		return 0
	}
	return 0.55
}

// SortByFuzzy ranks strings by relevance to needle, then alphabetically.
func SortByFuzzy(items []string, needle string) []string {
	if len(items) == 0 {
		return nil
	}
	out := append([]string(nil), items...)
	sort.SliceStable(out, func(i, j int) bool {
		si, sj := FuzzyScore(out[i], needle), FuzzyScore(out[j], needle)
		if si != sj {
			return si > sj
		}
		return out[i] < out[j]
	})
	return out
}
