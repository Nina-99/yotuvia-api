package service

import (
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
)

// EconomyConfig is the resolved token-economy parameter set for a currency.
type EconomyConfig struct {
	Currency          string
	YotuviaToFiatRate float64
	CommissionPct     float64
	MinChargeMilli    int64
}

// Canonical defaults applied when a currency has no market_config row — the
// Bolivia launch values (feat-token-economy-config D2).
var defaultEconomy = EconomyConfig{
	Currency:          "BOB",
	YotuviaToFiatRate: 1.0,
	CommissionPct:     3.5,
	MinChargeMilli:    350,
}

type MarketConfigService interface {
	// GetEconomyForCurrency resolves the economy parameters for a currency,
	// falling back to canonical defaults (with a warning) when no row exists.
	GetEconomyForCurrency(currency string) EconomyConfig
}

type MarketConfigServiceImpl struct {
	repo repository.MarketConfigRepository
}

func NewMarketConfigServiceImpl(repo repository.MarketConfigRepository) MarketConfigService {
	return &MarketConfigServiceImpl{repo: repo}
}

func (s *MarketConfigServiceImpl) GetEconomyForCurrency(currency string) EconomyConfig {
	row, err := s.repo.FindByCurrency(currency)
	if err != nil {
		utils.Log.Errorf("[MARKET] config lookup failed for %s: %v", currency, err)
		return defaultEconomy
	}
	if row == nil {
		utils.Log.Warnf("[MARKET] no market_config for currency %s — applying canonical defaults", currency)
		return defaultEconomy
	}
	return EconomyConfig{
		Currency:          row.Currency,
		YotuviaToFiatRate: row.YotuviaToFiatRate,
		CommissionPct:     row.CommissionPct,
		MinChargeMilli:    row.MinChargeMilli,
	}
}
