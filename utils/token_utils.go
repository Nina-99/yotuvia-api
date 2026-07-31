package utils

// CalculateTier returns the base tokens and bonus tokens for a given amount in Bs.
func CalculateTier(amountBs float64) (tokens, bonus float64) {
	switch {
	case amountBs >= 1000:
		return amountBs, amountBs * 0.25
	case amountBs >= 500:
		return amountBs, amountBs * 0.15
	case amountBs >= 275:
		return amountBs, amountBs * 0.09
	case amountBs >= 100:
		return amountBs, amountBs * 0.05
	default:
		return amountBs, 0
	}
}
