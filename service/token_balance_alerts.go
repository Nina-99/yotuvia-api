package service

// Balance health: the two alerts the owner must receive, and the suspension
// that follows a deficit nobody fixed.
//
// The rule for the whole file is "one notification per episode". Every alert is
// stamped on the account and the stamp is cleared when the state recovers, so a
// centre that completes ten visits while low gets ONE warning, and a centre that
// dips, recovers and dips again gets a fresh one. No notification table scan, no
// time-window heuristics.

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/Nina-99/yotuvia-api/utils"
)

const (
	// defaultLowBalanceYtv is the "you are running out" threshold, in Ytv. It is
	// compared against AVAILABLE funds (balance - holds + overdraft), because
	// that is what the owner can actually spend.
	defaultLowBalanceYtv = 10.0
	// defaultSuspendAfterHours is how long a centre may stay in deficit before
	// the platform suspends it. Default 30 days: long enough to notice and pay,
	// short enough that an abandoned centre stops being offered.
	defaultSuspendAfterHours = 720
)

// lowBalanceThresholdMilli resolves TOKEN_LOW_BALANCE_YTV.
func lowBalanceThresholdMilli() int64 {
	raw := os.Getenv("TOKEN_LOW_BALANCE_YTV")
	if raw == "" {
		return utils.ToMilli(defaultLowBalanceYtv)
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 {
		return utils.ToMilli(defaultLowBalanceYtv)
	}
	return utils.ToMilli(v)
}

// suspendAfter resolves TOKEN_SUSPEND_AFTER_HOURS. 0 disables suspension.
func suspendAfter() time.Duration {
	raw := os.Getenv("TOKEN_SUSPEND_AFTER_HOURS")
	if raw == "" {
		return time.Duration(defaultSuspendAfterHours) * time.Hour
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return time.Duration(defaultSuspendAfterHours) * time.Hour
	}
	return time.Duration(n) * time.Hour
}

// HealthSnapshot derives the owner-facing state from the same numbers the
// booking gate uses, so the dashboard can never tell a different story than the
// gate: "low" still accepts bookings, "deficit" and "suspended" do not.
func (s *TokenAccountServiceImpl) HealthSnapshot(beautyCenterID uint) (TokenHealth, error) {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil {
		return TokenHealth{}, err
	}
	if account == nil {
		// Legacy centre without a wallet: the gate auto-creates one with zero
		// balance, so report the same "new centre" reality the gate will see.
		return TokenHealth{State: "ok", CanAcceptBookings: true}, nil
	}

	available := account.Balance - account.ReservedMilli + utils.ToMilli(s.computeOverdraft(beautyCenterID))

	health := TokenHealth{State: "ok", CanAcceptBookings: true}
	switch {
	case account.SuspendedAt != nil:
		health.State = "suspended"
		health.CanAcceptBookings = false
	case account.DeficitSince != nil:
		health.State = "deficit"
		health.CanAcceptBookings = false
	case available <= lowBalanceThresholdMilli():
		health.State = "low"
	}

	if account.DeficitSince != nil {
		since := *account.DeficitSince
		health.DeficitSince = &since // copy: never hand out an aliased pointer
		health.DeficitDays = int(time.Since(since).Hours() / 24)
	}
	return health, nil
}

// IsSuspended reports whether the centre is currently suspended for non-payment.
func (s *TokenAccountServiceImpl) IsSuspended(beautyCenterID uint) (bool, error) {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil || account == nil {
		return false, err
	}
	return account.SuspendedAt != nil, nil
}

// EvaluateBalanceAlerts inspects one wallet and emits at most one notification
// per episode. Safe to call after every debit and every top-up.
func (s *TokenAccountServiceImpl) EvaluateBalanceAlerts(beautyCenterID uint) error {
	account, err := s.accountRepo.FindByBeautyCenterID(beautyCenterID)
	if err != nil || account == nil {
		return err
	}

	now := time.Now()
	overdraftMilli := utils.ToMilli(s.computeOverdraft(beautyCenterID))
	available := account.Balance - account.ReservedMilli + overdraftMilli

	lowAt, deficitAt, suspendedAt := account.LowBalanceNotifiedAt, account.DeficitNotifiedAt, account.SuspendedAt
	changed := false

	set := func(dst **time.Time, v *time.Time) {
		if *dst == nil && v == nil {
			return
		}
		if *dst != nil && v != nil && (*dst).Equal(*v) {
			return
		}
		*dst = v
		changed = true
	}

	// ── Low balance ──────────────────────────────────────────────────────────
	threshold := lowBalanceThresholdMilli()
	inDeficit := account.DeficitSince != nil
	if available <= threshold && !inDeficit {
		if account.LowBalanceNotifiedAt == nil {
			if err := s.notifyCenter(account.BeautyCenterID, "token_low_balance",
				"Te estás quedando sin Ytv",
				fmt.Sprintf("Te quedan %.2f Ytv disponibles. Recargá para seguir recibiendo reservas.",
					utils.FromMilli(available)),
				fmt.Sprintf(`{"available_milli":%d}`, available),
			); err != nil {
				return err
			}
			set(&lowAt, &now)
		}
	} else {
		// Healthy, or already in deficit (that case gets its own, stronger
		// message): the low-balance episode is over.
		set(&lowAt, nil)
	}

	// ── Deficit (cannot fund a new visit) ────────────────────────────────────
	if inDeficit {
		if account.DeficitNotifiedAt == nil {
			if err := s.notifyCenter(account.BeautyCenterID, "token_deficit",
				"Tu centro no puede recibir reservas nuevas",
				"El saldo de Ytv no alcanza para cubrir la comisión de un turno. "+
					"Recargá para volver a recibir reservas.",
				`{}`,
			); err != nil {
				return err
			}
			set(&deficitAt, &now)
		}

		// ── Suspension ───────────────────────────────────────────────────────
		if window := suspendAfter(); window > 0 && account.SuspendedAt == nil &&
			now.Sub(*account.DeficitSince) >= window {
			if err := s.notifyCenter(account.BeautyCenterID, "token_suspended",
				"Tu cuenta fue suspendida",
				"Pasaron más de "+humanHours(window)+" sin saldo suficiente. "+
					"Tu centro dejó de aparecer en las búsquedas. Recargá y se reactiva automáticamente.",
				`{}`,
			); err != nil {
				return err
			}
			set(&suspendedAt, &now)
		}
	} else {
		// Funds are back (or were never gone): clear both, so the next episode
		// starts clean.
		set(&deficitAt, nil)
		set(&suspendedAt, nil)
	}

	if !changed {
		return nil
	}
	// A map update, not a struct Save: clearing a stamp must write NULL.
	return s.accountRepo.SetHealthStamps(account.ID, lowAt, deficitAt, suspendedAt)
}

// notifyCenter routes through the notification service when it is wired.
func (s *TokenAccountServiceImpl) notifyCenter(centerID uint, notifType, title, message, data string) error {
	if s.notificationService == nil {
		return nil
	}
	return s.notificationService.CreateForCenter(centerID, notifType, title, message, data)
}

// humanHours renders a suspension window for the copy ("30 días", "48 horas").
func humanHours(d time.Duration) string {
	hours := int(d.Hours())
	if hours%24 == 0 {
		days := hours / 24
		if days == 1 {
			return "1 día"
		}
		return fmt.Sprintf("%d días", days)
	}
	return fmt.Sprintf("%d horas", hours)
}

// SweepBalanceAlerts evaluates every account that could need attention. It is
// the safety net for the event-driven call sites: a centre that goes quiet (no
// visits, no top-ups) still gets its deficit alert and its suspension on time.
func (s *TokenAccountServiceImpl) SweepBalanceAlerts(now time.Time) (int, error) {
	// Ceiling = the tightest possible "low" + the largest overdraft (Premium,
	// 30 Ytv), so no centre that could be low is missed by the SQL prefilter.
	ceiling := lowBalanceThresholdMilli() + utils.ToMilli(30)
	accounts, err := s.accountRepo.FindNeedingHealthCheck(ceiling)
	if err != nil {
		return 0, err
	}

	evaluated := 0
	for _, account := range accounts {
		if err := s.EvaluateBalanceAlerts(account.BeautyCenterID); err != nil {
			// Best-effort: one broken account must not stop the sweep.
			if utils.Log != nil {
				utils.Log.Errorf("[TOKENS] alert evaluation failed for centre %d: %v",
					account.BeautyCenterID, err)
			}
			continue
		}
		evaluated++
	}
	return evaluated, nil
}
