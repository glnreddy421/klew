package kube

import (
	"math"
	"testing"
)

func TestClampInt32(t *testing.T) {
	cases := []struct {
		in   int64
		want int32
	}{
		{0, 0},
		{42, 42},
		{-7, -7},
		{math.MaxInt32, math.MaxInt32},
		{math.MaxInt32 + 1, math.MaxInt32},
		{math.MinInt32 - 1, math.MinInt32},
	}
	for _, c := range cases {
		if got := clampInt32(c.in); got != c.want {
			t.Errorf("clampInt32(%d) = %d, want %d", c.in, got, c.want)
		}
	}
	if got := coerceInt32(uint64(math.MaxUint64)); got == nil || *got != math.MaxInt32 {
		t.Errorf("coerceInt32(MaxUint64) = %v, want MaxInt32", got)
	}
}
