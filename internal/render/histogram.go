package render

import (
	"fmt"
	"sort"
)

// Histogram renders reason -> count bars.
func Histogram(counts map[string]int, width int) []string {
	type kv struct {
		k string
		v int
	}
	var items []kv
	max := 0
	for k, v := range counts {
		items = append(items, kv{k, v})
		if v > max {
			max = v
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].v > items[j].v })
	var lines []string
	for _, it := range items {
		lines = append(lines, HorizontalBar(it.k, it.v, max, width))
	}
	if len(lines) == 0 {
		return []string{"  (no data)"}
	}
	return lines
}

// MinuteBuckets groups timestamps HH:MM -> count (strings pre-formatted).
func MinuteBuckets(buckets map[string]int, width int) []string {
	type kv struct {
		k string
		v int
	}
	var items []kv
	max := 0
	for k, v := range buckets {
		items = append(items, kv{k, v})
		if v > max {
			max = v
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].k < items[j].k })
	var lines []string
	for _, it := range items {
		lines = append(lines, fmt.Sprintf("  %s  %s", it.k, sparkBlocks(it.v, max, width)))
	}
	return lines
}

func sparkBlocks(v, max, width int) string {
	if max <= 0 {
		max = 1
	}
	n := int(float64(v) / float64(max) * float64(width))
	if v > 0 && n == 0 {
		n = 1
	}
	return BarFill.Render(stringsRepeat("█", n)) + BarEmpty.Render(stringsRepeat("░", width-n))
}

func stringsRepeat(s string, n int) string {
	if n <= 0 {
		return ""
	}
	out := make([]byte, 0, len(s)*n)
	for i := 0; i < n; i++ {
		out = append(out, s...)
	}
	return string(out)
}
