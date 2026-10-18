package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Staff reschedule (Phase C1): a professional may edit THEIR OWN appointment
// through `PUT /reservation/me/:id`, and ONLY their own. The route reuses the
// full update, so two guards matter:
//
//  1. ownership is centre + MEMBERSHIP (lead, or a line of a multi-professional
//     plan), never centre-wide — otherwise any staff member could move a
//     colleague's booking;
//  2. the LEAD cannot move: a professional may change the slot, never reassign
//     the appointment to another professional.

func TestStaffReschedule_OwnAppointment(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 7, 1, true)
	seedDashboardClient(t, db, 1, "Ana", "Lopez")
	seedServiceRow(t, db, 10, 1, 30)

	target := seedMultiProfessionalReservation(t, db, 1, 7, mpDay, "09:00", "09:30",
		[]mpLine{{serviceID: 10, personnelID: 7, duration: 30}})

	w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/me/%d", target.ID),
		roleToken(t, 7, 1, "personnel"), updateResBody(7, "10"))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var stored models.Reservation
	require.NoError(t, db.First(&stored, target.ID).Error)
	assert.Equal(t, "10:00", stored.StartTime, "the professional's own slot moved")
	assert.Equal(t, uint(7), stored.PersonnelID, "the lead is unchanged")
}

func TestStaffReschedule_ForeignAppointmentForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 7, 1, true)
	seedDashboardPersonnel(t, db, 9, 1, true) // same centre, NOT on the booking
	seedDashboardClient(t, db, 1, "Ana", "Lopez")
	seedServiceRow(t, db, 10, 1, 30)

	target := seedMultiProfessionalReservation(t, db, 1, 7, mpDay, "09:00", "09:30",
		[]mpLine{{serviceID: 10, personnelID: 7, duration: 30}})

	w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/me/%d", target.ID),
		roleToken(t, 9, 1, "personnel"), updateResBody(9, "10"))
	assert.Equal(t, http.StatusForbidden, w.Code, "a colleague's booking is off limits")

	var stored models.Reservation
	require.NoError(t, db.First(&stored, target.ID).Error)
	assert.Equal(t, "09:00", stored.StartTime, "forbidden edit changes nothing")
}

func TestStaffReschedule_CannotReassignLead(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 7, 1, true)
	seedDashboardPersonnel(t, db, 9, 1, true)
	seedDashboardClient(t, db, 1, "Ana", "Lopez")
	seedServiceRow(t, db, 10, 1, 30)

	target := seedMultiProfessionalReservation(t, db, 1, 7, mpDay, "09:00", "09:30",
		[]mpLine{{serviceID: 10, personnelID: 7, duration: 30}})

	// The lead's owner tries to hand the appointment to pro 9.
	w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/me/%d", target.ID),
		roleToken(t, 7, 1, "personnel"), updateResBody(9, "10"))
	assert.Equal(t, http.StatusForbidden, w.Code, "a professional cannot reassign the appointment")

	var stored models.Reservation
	require.NoError(t, db.First(&stored, target.ID).Error)
	assert.Equal(t, uint(7), stored.PersonnelID, "the lead must not move")
}
