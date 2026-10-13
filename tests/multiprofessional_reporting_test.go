package tests

import (
	"fmt"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ──────────────────────────────────────────────
//  Phase 2b: reporting + notifications per line
// ──────────────────────────────────────────────

// reservedLine is one hand-built service line. No user flow produces multi-
// professional data yet, so these fixtures build the shape directly — the same
// approach the Phase 2a availability tests take.
type reservedLine struct {
	serviceID   uint
	personnelID uint
	price       float64
	duration    int
}

// seedReservationWithPricedLines writes a reservation plus its ordered lines,
// each with an explicit professional, price and duration, and an explicit
// stored total. The lead is passed separately so a fixture can model either a
// consistent booking (total == SUM(price)) or a split one.
func seedReservationWithPricedLines(
	t *testing.T,
	db *gorm.DB,
	centerID, clientID, leadID uint,
	date time.Time,
	start, end, status string,
	total float64,
	lines []reservedLine,
) *models.Reservation {
	t.Helper()
	res := &models.Reservation{
		BeautyCenterID: centerID,
		ClientID:       clientID,
		PersonnelID:    leadID,
		Date:           date,
		StartTime:      start,
		EndTime:        end,
		TotalPrice:     total,
		Status:         status,
	}
	require.NoError(t, db.Create(res).Error)
	for _, l := range lines {
		pid := l.personnelID
		require.NoError(t, db.Create(&models.ReservationService{
			ReservationID: res.ID,
			ServiceID:     l.serviceID,
			ServiceName:   fmt.Sprintf("svc-%d", l.serviceID),
			Price:         l.price,
			Duration:      l.duration,
			PersonnelID:   &pid,
		}).Error)
	}
	return res
}

// TestDashboardRepo_PersonnelRevenue_PerLineAttribution proves revenue is
// credited to the professional who performs each line, not wholesale to the
// lead: a 300 booking split 200/100 across pro 1 / pro 2 ranks them 200 / 100,
// each with ONE appointment (it is a single booking).
func TestDashboardRepo_PersonnelRevenue_PerLineAttribution(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)

	seedDashboardCenter(t, db, 1, "Line Center", "line@test.com", true)
	seedClient(t, db, 1, "c1", "c1@example.com")
	seedPersonnel(t, db, 1, 1)
	seedPersonnel(t, db, 2, 1)
	seedServiceRow(t, db, 1, 1, 30)
	seedServiceRow(t, db, 2, 1, 30)

	seedReservationWithPricedLines(t, db, 1, 1, 1, mpDay, "10:00", "11:00", "COMPLETED", 300, []reservedLine{
		{serviceID: 1, personnelID: 1, price: 200, duration: 30},
		{serviceID: 2, personnelID: 2, price: 100, duration: 30},
	})

	rows, err := repo.PersonnelRevenue(1, mpDay, mpDay.AddDate(0, 0, 1), 10)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, uint(1), rows[0].PersonnelID)
	assert.Equal(t, 200.0, rows[0].Revenue, "line A's price goes to pro 1")
	assert.Equal(t, int64(1), rows[0].AppointmentCount)
	assert.Equal(t, uint(2), rows[1].PersonnelID)
	assert.Equal(t, 100.0, rows[1].Revenue, "line B's price goes to pro 2")
	assert.Equal(t, int64(1), rows[1].AppointmentCount)
}

// TestDashboardRepo_PersonnelRevenue_MultiServiceSingleProfessionalCountsOnce is
// the COUNT(*) TRAP guard: a two-service booking performed by ONE professional
// must keep counting as ONE appointment. Grouping by line without
// COUNT(DISTINCT reservations.id) would report 2 and inflate today's numbers.
func TestDashboardRepo_PersonnelRevenue_MultiServiceSingleProfessionalCountsOnce(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)

	seedDashboardCenter(t, db, 1, "Trap Center", "trap@test.com", true)
	seedClient(t, db, 1, "c1", "c1@example.com")
	seedPersonnel(t, db, 1, 1)
	seedServiceRow(t, db, 1, 1, 30)
	seedServiceRow(t, db, 2, 1, 30)

	// One booking, two services, the SAME professional, total == 50 + 50.
	seedReservationWithPricedLines(t, db, 1, 1, 1, mpDay, "10:00", "11:00", "COMPLETED", 100, []reservedLine{
		{serviceID: 1, personnelID: 1, price: 50, duration: 30},
		{serviceID: 2, personnelID: 1, price: 50, duration: 30},
	})

	rows, err := repo.PersonnelRevenue(1, mpDay, mpDay.AddDate(0, 0, 1), 10)
	require.NoError(t, err)
	require.Len(t, rows, 1, "one professional, one booking")
	assert.Equal(t, uint(1), rows[0].PersonnelID)
	assert.Equal(t, 100.0, rows[0].Revenue, "both lines' prices are credited")
	assert.Equal(t, int64(1), rows[0].AppointmentCount, "COUNT(DISTINCT) keeps this at one booking")
}

// TestDashboardService_Occupancy_MultiProfessionalSplit proves occupancy is
// computed per professional from the shared segment kernel: a 10:00-11:00
// booking whose two 30-minute lines belong to two professionals gives EACH 30
// booked minutes, not the lead's whole 60.
func TestDashboardService_Occupancy_MultiProfessionalSplit(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db := setupRouterAuthzDB(t)
	repo := repository.NewDashboardRepositoryImpl(db)
	svc := service.NewDashboardServiceImpl(repo, nil)

	seedDashboardCenter(t, db, 1, "Occ Multi", "occm@test.com", true)
	seedClient(t, db, 1, "c1", "c1@example.com")
	seedPersonnel(t, db, 1, 1)
	seedPersonnel(t, db, 2, 1)
	seedServiceRow(t, db, 1, 1, 30)
	seedServiceRow(t, db, 2, 1, 30)
	// Monday 08:00-12:00 → 240 available minutes for the single range day.
	seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "12:00")

	seedReservationWithPricedLines(t, db, 1, 1, 1, mpDay, "10:00", "11:00", "CONFIRMED", 100, []reservedLine{
		{serviceID: 1, personnelID: 1, price: 50, duration: 30},
		{serviceID: 2, personnelID: 2, price: 50, duration: 30},
	})

	resp, err := svc.GetStatsWithRange(1, mpDay, mpDay, 30)
	require.NoError(t, err)
	require.Len(t, resp.PersonnelOccupancy, 2, "both professionals are booked")

	byID := make(map[uint]response.PersonnelOccupancyDTO, len(resp.PersonnelOccupancy))
	for _, o := range resp.PersonnelOccupancy {
		byID[o.PersonnelID] = o
	}
	assert.Equal(t, 30, byID[1].BookedMinutes, "pro 1 works only their own segment")
	assert.Equal(t, 30, byID[2].BookedMinutes, "pro 2 works only their own segment")
	assert.Equal(t, 240, byID[1].AvailableMinutes)
	assert.Equal(t, 240, byID[2].AvailableMinutes)
}

// TestReservationPersonnelNotifications_CancelNotifiesEveryLineProfessional
// proves a cancellation reaches EVERY professional with a line — one
// notification row each — and no professional without a line.
func TestReservationPersonnelNotifications_CancelNotifiesEveryLineProfessional(t *testing.T) {
	db := setupRouterAuthzDB(t)
	_ = seedClient(t, db, 5, "client5", "c5@example.com")
	seedPersonnel(t, db, 1, 1)
	seedPersonnel(t, db, 2, 1)
	seedPersonnel(t, db, 3, 1)
	seedDashboardCenter(t, db, 1, "Notif Center", "nc@test.com", true)

	res := seedReservationWithPricedLines(t, db, 1, 5, 1, mpDay, "10:00", "11:00", "CONFIRMED", 100, []reservedLine{
		{serviceID: 1, personnelID: 1, price: 50, duration: 30},
		{serviceID: 2, personnelID: 2, price: 50, duration: 30},
	})

	svc := newReservationTriggerTestService(t, db)
	require.NoError(t, svc.Cancel(res.ID, 5, ""))

	for _, pid := range []uint{1, 2} {
		var n int64
		require.NoError(t, db.Model(&models.Notification{}).
			Where("personnel_id = ? AND type = ?", pid, "reservation_cancelled").
			Count(&n).Error)
		assert.Equal(t, int64(1), n, "pro %d gets exactly one cancellation row", pid)
	}

	var noLine int64
	require.NoError(t, db.Model(&models.Notification{}).
		Where("personnel_id = ?", 3).Count(&noLine).Error)
	assert.Zero(t, noLine, "a professional with no line is not notified")
}

// TestReservationPersonnelNotifications_ReassignNotifiesPreviousLineProfessionals
// proves a reassignment notifies every professional on the PREVIOUS lines (the
// losing parties) and the newly assigned professional (the gaining party),
// each exactly once.
func TestReservationPersonnelNotifications_ReassignNotifiesPreviousLineProfessionals(t *testing.T) {
	db := setupRouterAuthzDB(t)
	_ = seedClient(t, db, 5, "client5", "c5@example.com")
	seedPersonnel(t, db, 1, 1)
	seedPersonnel(t, db, 2, 1)
	seedPersonnel(t, db, 3, 1)
	seedDashboardCenter(t, db, 1, "Notif Center", "nc@test.com", true)

	res := seedReservationWithPricedLines(t, db, 1, 5, 1, mpDay, "10:00", "11:00", "PENDING", 100, []reservedLine{
		{serviceID: 1, personnelID: 1, price: 50, duration: 30},
		{serviceID: 2, personnelID: 2, price: 50, duration: 30},
	})

	svc := newReservationTriggerTestService(t, db)
	require.NoError(t, svc.AssignPersonnel(res.ID, 3, 1))

	for _, pid := range []uint{1, 2, 3} {
		var n int64
		require.NoError(t, db.Model(&models.Notification{}).
			Where("personnel_id = ? AND type = ?", pid, "personnel_changed").
			Count(&n).Error)
		assert.Equal(t, int64(1), n, "pro %d gets exactly one personnel_changed row", pid)
	}
}
