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

// schedulerSweeps is the per-tick maintenance work, expressed as functions so
// the tick is testable without a live scheduler or a full service graph.
type schedulerSweeps struct {
	sendDueReminders func(now time.Time) (int, error)
	sweepNoShows     func(now time.Time, graceMinutes int) (int, error)
	expireHolds      func(now time.Time) (int, error)
	sweepOffers      func(now time.Time) (int, error)
}

// runSchedulerTick executes all four sweeps for one tick. Every sweep stays
// best-effort: a failure is logged and the remaining sweeps still run, so one
// broken query never starves the others. Extracted from the loop so the tick
// can be exercised on SQLite.
func runSchedulerTick(s schedulerSweeps, now time.Time) {
	if n, err := s.sendDueReminders(now); err != nil {
		utils.Log.Errorf("[SCHEDULER] reminders failed: %v", err)
	} else if n > 0 {
		utils.Log.Infof("[SCHEDULER] sent %d reminders", n)
	}
	if n, err := s.sweepNoShows(now, noShowGraceMinutes); err != nil {
		utils.Log.Errorf("[SCHEDULER] no-show sweep failed: %v", err)
	} else if n > 0 {
		utils.Log.Infof("[SCHEDULER] struck %d no-shows", n)
	}
	if n, err := s.expireHolds(now); err != nil {
		utils.Log.Errorf("[SCHEDULER] hold sweep failed: %v", err)
	} else if n > 0 {
		utils.Log.Infof("[SCHEDULER] swept %d expired holds", n)
	}
	if n, err := s.sweepOffers(now); err != nil {
		utils.Log.Errorf("[SCHEDULER] offer sweep failed: %v", err)
	} else if n > 0 {
		utils.Log.Infof("[SCHEDULER] passed on %d expired offers", n)
	}
}

// StartBackgroundScheduler runs the Phase 3 maintenance loop: due reminders,
// no-show sweep, deposit-hold sweep and waitlist offer expiry.
//
// Multi-replica safety: each tick first takes a Postgres advisory lock
// (schedulerAdvisoryLockKey) so exactly one replica fires a given tick — the
// others skip it quietly. On non-Postgres dialects the lock is a documented
// no-op.
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
		nil, // FeedPostRepository: sweeps never attribute bookings
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

	sweeps := schedulerSweeps{
		sendDueReminders: reservationSvc.SendDueReminders,
		sweepNoShows:     reservationSvc.SweepNoShows,
		expireHolds:      reservationSvc.ExpireHolds,
		sweepOffers:      waitlistSvc.SweepExpiredOffers,
	}

	go func() {
		ticker := time.NewTicker(time.Duration(intervalSec) * time.Second)
		defer ticker.Stop()
		for now := range ticker.C {
			// Exactly one replica runs a given tick. The deferred release runs
			// even if a sweep panics, so the lock can never be stranded.
			release, acquired := acquireTickLock(db)
			if !acquired {
				utils.Log.Infof("[SCHEDULER] tick skipped: another replica holds the advisory lock")
				continue
			}
			func() {
				defer release()
				runSchedulerTick(sweeps, now)
			}()
		}
	}()
	utils.Log.Infof("[SCHEDULER] Started (every %ds)", intervalSec)
}
