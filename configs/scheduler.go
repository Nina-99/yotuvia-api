package configs

import (
	"os"
	"strconv"
	"time"

	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/service"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"
)

// noShowGraceMinutes is how long after slot end the ticker waits before
// striking a no-show (client may be paying at the desk).
const noShowGraceMinutes = 30

// StartBackgroundScheduler runs the Phase 3 maintenance loop: due reminders,
// no-show sweep, deposit-hold sweep and waitlist offer expiry. Single
// instance only — when scaling past one API replica, guard the tick with a
// Postgres advisory lock (TODO) so sweeps never double-fire.
//
// The loop is best-effort by design: every sweep logs and continues, and a
// failed tick never kills the scheduler. Disabled with SCHEDULER_ENABLED=false;
// cadence via SCHEDULER_INTERVAL_SEC (default 60).
func StartBackgroundScheduler(db *gorm.DB) {
	if os.Getenv("SCHEDULER_ENABLED") == "false" {
		utils.Log.Info("[SCHEDULER] Disabled via SCHEDULER_ENABLED=false")
		return
	}
	intervalSec := 60
	if raw := os.Getenv("SCHEDULER_INTERVAL_SEC"); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			intervalSec = n
		}
	}

	notifSvc := service.NewNotificationServiceImpl(
		repository.NewNotificationRepositoryImpl(db),
		repository.NewPushDeviceRepositoryImpl(db),
	)
	validate := validator.New()
	reservationRepo := repository.NewReservationRepositoryImpl(db)
	clientRepo := repository.NewClientRepositoryImpl(db)
	reservationSvc := service.NewReservationServiceImpl(
		reservationRepo,
		repository.NewServiceRepositoryImpl(db),
		repository.NewPersonnelRepositoryImpl(db),
		clientRepo,
		nil, // TokenAccountRepository: sweeps never charge
		notifSvc,
		nil, // LoyaltyService: sweeps never redeem
		nil, // PromotionService: sweeps never discount
		nil, // TokenAccountService: sweeps never charge
		nil, // TokenMissionService: sweeps never trigger
		nil, // MarketConfigService: sweeps never price
		validate,
		repository.NewBeautyCenterRepositoryImpl(db),
	)
	waitlistSvc := service.NewWaitlistServiceImpl(
		repository.NewWaitlistRepositoryImpl(db),
		reservationRepo,
		reservationSvc,
		repository.NewServiceRepositoryImpl(db),
		clientRepo,
		notifSvc,
		validate,
	)

	go func() {
		ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
		defer ticker.Stop()
		for now := range ticker.C {
			if n, err := reservationSvc.SendDueReminders(now); err != nil {
				utils.Log.Errorf("[SCHEDULER] reminders failed: %v", err)
			} else if n > 0 {
				utils.Log.Infof("[SCHEDULER] sent %d reminders", n)
			}
			if n, err := reservationSvc.SweepNoShows(now, noShowGraceMinutes); err != nil {
				utils.Log.Errorf("[SCHEDULER] no-show sweep failed: %v", err)
			} else if n > 0 {
				utils.Log.Infof("[SCHEDULER] struck %d no-shows", n)
			}
			if n, err := reservationSvc.ExpireHolds(now); err != nil {
				utils.Log.Errorf("[SCHEDULER] hold sweep failed: %v", err)
			} else if n > 0 {
				utils.Log.Infof("[SCHEDULER] swept %d expired holds", n)
			}
			if n, err := waitlistSvc.SweepExpiredOffers(now); err != nil {
				utils.Log.Errorf("[SCHEDULER] offer sweep failed: %v", err)
			} else if n > 0 {
				utils.Log.Infof("[SCHEDULER] passed on %d expired offers", n)
			}
		}
	}()
	utils.Log.Infof("[SCHEDULER] Started (every %ds)", intervalSec)
}
