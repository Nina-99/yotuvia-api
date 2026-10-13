package tests

import (
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Reassigning a reservation must move the DERIVED LEAD and every service line
// together. Phase 1 exposed the line's professional in the response, so a
// reassignment that only rewrote the lead would leave the lines naming the
// PREVIOUS professional - a divergence visible to any client that reads
// services[].personnel_id.
//
// The invariant is enforced in one place (the repository update) so no caller
// can reassign half a reservation.
func TestUpdatePersonnelID_ReassignsLeadAndLines(t *testing.T) {
	db := setupRouterAuthzDB(t)

	seedDashboardCenter(t, db, 1, "Center", "c@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardClient(t, db, 1, "Client", "One")

	const oldPersonnel = uint(1)
	const newPersonnel = uint(2)

	res := &models.Reservation{
		BeautyCenterID: 1,
		ClientID:       1,
		PersonnelID:    oldPersonnel,
		Date:           time.Now().Add(24 * time.Hour),
		StartTime:      "10:00",
		EndTime:        "11:00",
		TotalPrice:     100,
		Status:         "CONFIRMED",
	}
	require.NoError(t, db.Create(res).Error)

	lines := []models.ReservationService{
		{ReservationID: res.ID, ServiceID: 1, ServiceName: "Corte", Price: 50, Duration: 30, PersonnelID: ptrUint(oldPersonnel)},
		{ReservationID: res.ID, ServiceID: 2, ServiceName: "Color", Price: 50, Duration: 30, PersonnelID: ptrUint(oldPersonnel)},
	}
	require.NoError(t, db.Create(&lines).Error)

	repo := repository.NewReservationRepositoryImpl(db)
	require.NoError(t, repo.UpdatePersonnelID(res.ID, newPersonnel))

	var stored models.Reservation
	require.NoError(t, db.First(&stored, res.ID).Error)
	assert.Equal(t, newPersonnel, stored.PersonnelID, "the derived lead must move")

	var storedLines []models.ReservationService
	require.NoError(t, db.Where("reservation_id = ?", res.ID).Order("id ASC").Find(&storedLines).Error)
	require.Len(t, storedLines, 2)
	for i, line := range storedLines {
		require.NotNilf(t, line.PersonnelID, "line %d must keep a professional", i)
		assert.Equalf(t, newPersonnel, *line.PersonnelID,
			"line %d must follow the reassignment, not keep the previous professional", i)
	}
}
