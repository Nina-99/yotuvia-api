package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// An assignment-free edit of a SINGLE-professional booking must move the DERIVED
// LEAD together with the lines. It did not: `repository.Update` is `db.Save`,
// and GORM derives the foreign key from a PRELOADED belongs-to association, so
// the stale `Personnel` loaded by FindById silently won and the reservation
// kept the OLD professional at the row level while every line moved to the new
// one.
//
// That divergence is exactly what the per-line model exists to prevent, and the
// sibling test only asserted the lines - which is why it survived unnoticed. A
// client who changes the professional on their booking would still see the old
// one in every surface that reads the reservation's lead.
func TestUpdateReservation_SingleProfessionalMovesTheLead(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Lead", "Client")
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)

	// Every line belongs to pro 1, so this is an ordinary single-professional
	// booking - the case the edit form produces every day.
	target := seedMultiProfessionalReservation(t, db, 1, 1, mpDay, "10:00", "11:00", []mpLine{
		{serviceID: 10, personnelID: 1, duration: 30},
		{serviceID: 11, personnelID: 1, duration: 30},
	})

	w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/%d", target.ID),
		roleToken(t, 7, 1, "beauty_center"), updateResBody(2, "10,11"))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var stored models.Reservation
	require.NoError(t, db.First(&stored, target.ID).Error)
	assert.Equal(t, uint(2), stored.PersonnelID,
		"the derived lead must move with the lines, not stay on the previous professional")

	lines := loadReservationLines(t, db, target.ID)
	require.Len(t, lines, 2)
	for i, line := range lines {
		require.NotNilf(t, line.PersonnelID, "line %d must carry a professional", i)
		assert.Equalf(t, uint(2), *line.PersonnelID, "line %d", i)
	}
	assert.Equal(t, stored.PersonnelID, *lines[0].PersonnelID,
		"the lead equals the first line's professional")
}
