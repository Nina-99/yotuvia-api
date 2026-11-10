package utils

import "testing"

// TestToMilli covers the spec scenarios: exact two-decimal conversion,
// half-away-from-zero rounding of sub-milli values, negatives, and the
// repeated-small-additions exactness requirement (0.10 x 10 == 1000).
func TestToMilli(t *testing.T) {
	tests := []struct {
		name string
		in   float64
		want int64
	}{
		{"zero", 0, 0},
		{"exact two-decimal Ytv", 25.50, 25500},
		{"half-up tie (25.5555 -> 25556)", 25.5555, 25556},
		{"single milli (0.001)", 0.001, 1},
		{"sub-milli rounds away from zero (0.0004 -> 0)", 0.0004, 0},
		{"sub-milli rounds away from zero (0.0005 -> 1)", 0.0005, 1},
		{"one tenth is exactly 100 milli", 0.10, 100},
		{"whole Ytv", 5, 5000},
		{"large value", 1000, 1_000_000},
		{"negative half-up tie", -25.5555, -25556},
		{"negative sub-milli rounds away from zero", -0.0005, -1},
		{"negative exact", -3.25, -3250},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ToMilli(tt.in); got != tt.want {
				t.Errorf("ToMilli(%v) = %d, want %d", tt.in, got, tt.want)
			}
		})
	}
}

// TestRepeatedSmallAdditionsStayExact — 0.10 Ytv credited ten times must
// sum to exactly 1000 milli with zero accumulated float error.
func TestRepeatedSmallAdditionsStayExact(t *testing.T) {
	var total int64
	for i := 0; i < 10; i++ {
		total += ToMilli(0.10)
	}
	if total != 1000 {
		t.Errorf("10 x ToMilli(0.10) = %d, want 1000", total)
	}
}

func TestFromMilli(t *testing.T) {
	tests := []struct {
		name string
		in   int64
		want float64
	}{
		{"exact milli", 25500, 25.50},
		{"full stored precision preserved", 25555, 25.555},
		{"zero", 0, 0},
		{"sub-yottu", 500, 0.5},
		{"negative", -25556, -25.556},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FromMilli(tt.in); got != tt.want {
				t.Errorf("FromMilli(%d) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// TestMilliRoundTrip — FromMilli(ToMilli(v)) recovers v for values that are
// exactly representable in milli (the wire format must round-trip cleanly).
func TestMilliRoundTrip(t *testing.T) {
	for _, v := range []float64{25.50, 3.25, 0.10, 1000, 0.001} {
		if got := FromMilli(ToMilli(v)); got != v {
			t.Errorf("round trip %v: got %v", v, got)
		}
	}
}
