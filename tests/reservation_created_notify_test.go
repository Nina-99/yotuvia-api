package tests

import (
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// A NEW appointment must never surprise the people who work it: the client
// booking path notifies BOTH the centre and every professional on the plan,
// with an in-app row each (and, through CreateFor*, a push to their devices).

func TestCreateReservation_NotifiesCentreAndEveryProfessional(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Chain", "Client")
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)
	seedWorkingHour(t, db, 1, int(mpDay.Weekday()), "08:00", "13:00")

	w := do(r, http.MethodPost, "/api/reservation/", roleToken(t, 1, 0, "client"),
		assignmentBody([2]uint{10, 11}, [2]uint{1, 2}, "09:00"))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// The centre is told once.
	var centreRows []models.Notification
	require.NoError(t, db.Where("beauty_center_id = ? AND type = ?", 1, "reservation_created").
		Find(&centreRows).Error)
	assert.Len(t, centreRows, 1, "the centre gets one new-reservation notification")

	// EVERY professional on the plan is told, not just the lead.
	var staffRows []models.Notification
	require.NoError(t, db.Where("type = ? AND personnel_id IS NOT NULL", "reservation_created").Find(&staffRows).Error)
	require.Len(t, staffRows, 2, "both professionals get a new-appointment notification")

	recipients := map[uint]bool{}
	for _, row := range staffRows {
		require.NotNil(t, row.PersonnelID, "a staff row carries its personnel id")
		recipients[*row.PersonnelID] = true
	}
	assert.True(t, recipients[1], "professional 1 was told")
	assert.True(t, recipients[2], "professional 2 was told")
}

// TestCreateReservation_NotifiesSingleProfessional covers the ordinary
// single-professional booking: exactly one staff notification, no duplicates.
func TestCreateReservation_NotifiesSingleProfessional(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db, date := bookingFixture(t)
	seedWorkingHour(t, db, 1, int(date.Weekday()), "09:00", "18:00")

	w := postClientBooking(r, roleToken(t, 1, 0, "client"), date.Format("2006-01-02"), "10:00")
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var staffRows []models.Notification
	require.NoError(t, db.Where("type = ? AND personnel_id IS NOT NULL", "reservation_created").Find(&staffRows).Error)
	require.Len(t, staffRows, 1)
	require.NotNil(t, staffRows[0].PersonnelID)
	assert.Equal(t, uint(1), *staffRows[0].PersonnelID)

	var centreRows []models.Notification
	require.NoError(t, db.Where("beauty_center_id = ? AND type = ?", 1, "reservation_created").
		Find(&centreRows).Error)
	assert.Len(t, centreRows, 1)
}
