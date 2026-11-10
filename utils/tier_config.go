package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

type Tier struct {
	Min      float64 `json:"min"`
	BonusPct float64 `json:"bonusPct"`
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
// An unknown currency returns (nil, nil): callers must NOT inherit another
// currency's ladder, because that silently pays the wrong bonus.
func GetTierConfig(currency string) ([]Tier, error) {
	loadTierConfig()
	if loadErr != nil {
		return nil, loadErr
	}

	return tierConfig[currency], nil
}

// CalculateTier calculates tokens and bonus based on amount and currency.
// It uses the tier configuration loaded from configs/tiers.json. A currency
// without a configured ladder receives NO bonus (never another currency's).
func CalculateTier(amount float64, currency string) (tokens, bonus float64) {
	tiers, err := GetTierConfig(currency)
	if err != nil {
		Log.Errorf("[TIERS] config load failed: %v — no bonus applied", err)
		return amount, 0
	}
	if tiers == nil {
		Log.Warnf("[TIERS] no tier ladder for currency %s — no bonus applied", currency)
		return amount, 0
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
