package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type Tier struct {
	Min       float64 `json:"min"`
	BonusPct  float64 `json:"bonusPct"`
}

var (
	tierConfig map[string][]Tier
	tierOnce   sync.Once
	loadErr    error
)

func loadTierConfig() {
	tierOnce.Do(func() {
		path := os.Getenv("TIERS_CONFIG_PATH")
		if path == "" {
			// Try multiple paths: from project root, from binary location, etc.
			_, filename, _, _ := runtime.Caller(0)
			// tier_config.go is in utils/, configs is in parent of utils
			utilsDir := filepath.Dir(filename)
			projectRoot := filepath.Dir(utilsDir)
			path = filepath.Join(projectRoot, "configs", "tiers.json")
		}

		data, err := os.ReadFile(path)
		if err != nil {
			loadErr = err
			return
		}

		var config map[string][]Tier
		if err := json.Unmarshal(data, &config); err != nil {
			loadErr = err
			return
		}

		tierConfig = config
	})
}

// GetTierConfig returns the tier configuration for a given currency.
// If the currency is not found, it returns the first available config (fallback).
func GetTierConfig(currency string) ([]Tier, error) {
	loadTierConfig()
	if loadErr != nil {
		return nil, loadErr
	}

	if tiers, ok := tierConfig[currency]; ok {
		return tiers, nil
	}

	// Fallback: return first available currency config
	for _, v := range tierConfig {
		return v, nil
	}

	return nil, nil
}

// CalculateTier calculates tokens and bonus based on amount and currency.
// It uses the tier configuration loaded from configs/tiers.json.
func CalculateTier(amount float64, currency string) (tokens, bonus float64) {
	tiers, err := GetTierConfig(currency)
	if err != nil || tiers == nil {
		// Fallback to hardcoded VES-like tiers if config fails
		return calculateTierFallback(amount)
	}

	var bestBonusPct float64
	for _, tier := range tiers {
		if amount >= tier.Min && tier.BonusPct > bestBonusPct {
			bestBonusPct = tier.BonusPct
		}
	}

	tokens = amount
	bonus = amount * bestBonusPct
	return
}

func calculateTierFallback(amountBs float64) (tokens, bonus float64) {
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