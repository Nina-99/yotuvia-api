package models

import (
	"gorm.io/gorm"
)

// MarketConfig holds the per-currency token economy parameters (feat-token-
// economy-config). Keyed by the beauty center's currency code so the
// verification flow resolves values without a country concept. Values are
// platform-ops editable data — a future superuser panel will CRUD these rows.
type MarketConfig struct {
	gorm.Model

	// Currency ISO code, e.g. "BOB", "CLP". Unique: one economy per currency.
	Currency string `gorm:"size:3;not null;uniqueIndex" json:"currency"`

	// YotuviaToFiatRate is the display/reference rate (1 YOTUVIA = X fiat).
	// Accounting never uses it — balances live in milli-YOTTU.
	YotuviaToFiatRate float64 `gorm:"default:1.0" json:"yotuvia_to_fiat_rate"`

	// CommissionPct percent of the reservation total charged as tokens
	// (3.5 means 3.5%).
	CommissionPct float64 `gorm:"default:3.5" json:"commission_pct"`

	// MinChargeMilli is the floor charge in milli-YOTTU (350 = 0.35 YOTTU,
	// equal to the commission of a 10 BOB reference ticket).
	MinChargeMilli int64 `gorm:"default:350" json:"min_charge_milli"`
}
