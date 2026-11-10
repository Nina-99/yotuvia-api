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

	// YtvToFiatRate is the display/reference rate (1 Ytv = X fiat).
	// Accounting never uses it — balances live in milli-Ytv.
	YtvToFiatRate float64 `gorm:"column:ytv_to_fiat_rate;default:1.0" json:"ytv_to_fiat_rate"`

	// CommissionPct percent of the reservation total charged as tokens
	// (3.5 means 3.5%).
	CommissionPct float64 `gorm:"default:3.5" json:"commission_pct"`

	// MinChargeMilli is the floor charge in milli-Ytv (350 = 0.35 Ytv,
	// equal to the commission of a 10 BOB reference ticket).
	MinChargeMilli int64 `gorm:"default:350" json:"min_charge_milli"`

	// Boost advertising parameters, also per currency.
	// BoostReserveCapPct caps how much of a centre's balance one campaign may
	// hold (0.5 = 50%).
	BoostReserveCapPct float64 `gorm:"default:0.5" json:"boost_reserve_cap_pct"`
	// BoostReserveCPMMilli is the auction floor: a bid below it cannot win.
	BoostReserveCPMMilli int64 `gorm:"default:1000" json:"boost_reserve_cpm_milli"`
	// BoostQualificationPct is the watch threshold that makes an impression
	// billable (50 = the recommender's signal).
	BoostQualificationPct float64 `gorm:"default:50" json:"boost_qualification_pct"`

	// Referral rewards, in milli-Ytv, snapshot on each referral row at creation.
	ReferralRewardReferrerMilli int64 `gorm:"default:20000" json:"referral_reward_referrer_milli"`
	ReferralRewardReferredMilli int64 `gorm:"default:10000" json:"referral_reward_referred_milli"`
}
