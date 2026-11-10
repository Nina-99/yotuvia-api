package utils

import "math"

// ToMilli converts decimal Ytv to int64 milli-Ytv (1 Ytv = 1,000 milli).
// It first quantizes to 6 decimal places so exact ties such as 25.5555 round
// half-away-from-zero to 25556 rather than 25555, matching PostgreSQL's ROUND
// used by the data migration backfill.
func ToMilli(v float64) int64 {
	six := int64(math.Round(v * 1_000_000))
	if six >= 0 {
		return (six + 500) / 1_000
	}
	return -((-six + 500) / 1_000)
}

// FromMilli converts int64 milli-Ytv back to decimal Ytv for the JSON wire.
func FromMilli(m int64) float64 { return float64(m) / 1_000 }
