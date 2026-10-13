package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// lineAttributionLite is the subset of ReservationResponse needed to assert the
// additive line-level professional exposure.
type lineAttributionLite struct {
	ID          uint `json:"id"`
	PersonnelID uint `json:"personnel_id"`
	Services    []struct {
		ServiceID   uint  `json:"service_id"`
		PersonnelID *uint `json:"personnel_id"`
	} `json:"services"`
}

// loadReservationLines reloads the persisted service rows for a reservation.
func loadReservationLines(t *testing.T, db *gorm.DB, reservationID uint) []models.ReservationService {
	t.Helper()
	var lines []models.ReservationService
	require.NoError(t, db.Where("reservation_id = ?", reservationID).Order("id ASC").Find(&lines).Error)
	return lines
}

// assertLinesCarryLead pins the Phase 1 invariant: every line carries a
// professional AND the reservation's derived lead equals the first line's.
func assertLinesCarryLead(t *testing.T, db *gorm.DB, reservationID uint, wantPersonnel uint) {
	t.Helper()
	lines := loadReservationLines(t, db, reservationID)
	require.NotEmpty(t, lines, "a booking must persist at least one service line")
	for i, line := range lines {
		require.NotNilf(t, line.PersonnelID, "line %d must carry the professional", i)
		assert.Equalf(t, wantPersonnel, *line.PersonnelID, "line %d professional", i)
	}

	var stored models.Reservation
	require.NoError(t, db.First(&stored, reservationID).Error)
	assert.Equal(t, wantPersonnel, stored.PersonnelID, "reservation lead")
	assert.Equal(t, stored.PersonnelID, *lines[0].PersonnelID,
		"the reservation's lead equals the first line's professional")
}

// TestReservationLines_CarryLeadOnWalkinBooking covers the admin creation path:
// a two-service walk-in must attribute the professional to every line.
func TestReservationLines_CarryLeadOnWalkinBooking(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reservationWalkinTestRouter(t)
	seedWalkinData(t, db)

	body := `{"beauty_center_id":1,"client_id":1,"personnel_id":1,"service_ids":[1,3],"date":"2026-08-16","start_time":"10:30"}`
	w := postWalkin(r, roleToken(t, 7, 1, "beauty_center"), body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var resp lineAttributionLite
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assertLinesCarryLead(t, db, resp.ID, 1)
}

// TestReservationLines_CarryLeadOnClientBooking covers the normal client path.
func TestReservationLines_CarryLeadOnClientBooking(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reservationWalkinTestRouter(t)
	seedWalkinData(t, db)

	body := `{"beauty_center_id":1,"personnel_id":1,"service_ids":[1,3],"date":"2026-08-16","start_time":"13:00"}`
	w := do(r, http.MethodPost, "/api/reservation/", roleToken(t, 1, 0, "client"), body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	// The client create endpoint answers with a message, not the DTO; reload
	// the freshly created row by its unique slot.
	var created models.Reservation
	require.NoError(t, db.Where("client_id = ? AND start_time = ?", 1, "13:00").First(&created).Error)
	assertLinesCarryLead(t, db, created.ID, 1)
}

// TestReservationLines_CarryLeadOnUpdate covers the second write site: replacing
// a reservation's services must re-attribute every new line to the lead.
func TestReservationLines_CarryLeadOnUpdate(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reservationWalkinTestRouter(t)
	seed := seedWalkinData(t, db)

	body := fmt.Sprintf(
		`{"beauty_center_id":1,"client_id":%d,"personnel_id":1,"service_ids":[1,3],"date":"2026-08-16","start_time":"15:00"}`,
		seed.overlapSlot.ClientID)
	w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/%d", seed.overlapSlot.ID),
		roleToken(t, 7, 1, "beauty_center"), body)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assertLinesCarryLead(t, db, seed.overlapSlot.ID, 1)
}

// TestReservationResponse_ExposesLinePersonnelID pins the additive contract:
// services[] now carries the line's personnel_id, with no personnel name.
func TestReservationResponse_ExposesLinePersonnelID(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := reservationWalkinTestRouter(t)
	seedWalkinData(t, db)

	body := `{"beauty_center_id":1,"client_id":1,"personnel_id":1,"service_ids":[1,3],"date":"2026-08-16","start_time":"10:30"}`
	w := postWalkin(r, roleToken(t, 7, 1, "beauty_center"), body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var resp lineAttributionLite
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Services, 2)
	for _, svc := range resp.Services {
		require.NotNil(t, svc.PersonnelID, "each service line exposes personnel_id")
		assert.Equal(t, uint(1), *svc.PersonnelID)
	}

	// A line's professional is exposed by id only — never a nested name preload.
	assert.NotContains(t, w.Body.String(), "personnel_name")
}
