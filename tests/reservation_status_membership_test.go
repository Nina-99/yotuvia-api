package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// ──────────────────────────────────────────────
//  "You can operate on what you can see" — staff status transitions
// ──────────────────────────────────────────────

// TestReservationStatusByPersonnel_NonLeadLineProfessionalCanTransition proves
// a NON-lead professional who merely owns a service line can advance a shared
// booking: the agenda already shows it (membership = lead OR line), so the
// transition must not answer 403.
func TestReservationStatusByPersonnel_NonLeadLineProfessionalCanTransition(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)

	// Pro 1 is the DERIVED LEAD; pro 2 is only on line 2.
	res := seedReservationWithPricedLines(t, db, 1, 1, 1, mpDay, "10:00", "11:00", "PENDING", 100, []reservedLine{
		{serviceID: 1, personnelID: 1, price: 50, duration: 30},
		{serviceID: 2, personnelID: 2, price: 50, duration: 30},
	})

	w := do(r, http.MethodPatch, fmt.Sprintf("/api/reservation/me/%d/status", res.ID),
		roleToken(t, 2, 1, "personnel"), `{"status":"CONFIRMED"}`)
	require.Equal(t, http.StatusOK, w.Code, "a line professional must be able to accept: %s", w.Body.String())

	var stored models.Reservation
	require.NoError(t, db.First(&stored, res.ID).Error)
	assert.Equal(t, "CONFIRMED", stored.Status)
}

// TestReservationStatusByPersonnel_UnrelatedProfessionalForbidden proves a
// professional with NO relation still gets 403 and cannot mutate the booking,
// and that the centre check is unchanged.
func TestReservationStatusByPersonnel_UnrelatedProfessionalForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardPersonnel(t, db, 2, 1, true)
	seedDashboardPersonnel(t, db, 3, 1, true)

	res := seedReservationWithPricedLines(t, db, 1, 1, 1, mpDay, "10:00", "11:00", "PENDING", 100, []reservedLine{
		{serviceID: 1, personnelID: 1, price: 50, duration: 30},
		{serviceID: 2, personnelID: 2, price: 50, duration: 30},
	})

	// Pro 3 has no lead and no line → 403.
	w := do(r, http.MethodPatch, fmt.Sprintf("/api/reservation/me/%d/status", res.ID),
		roleToken(t, 3, 1, "personnel"), `{"status":"CONFIRMED"}`)
	assert.Equal(t, http.StatusForbidden, w.Code)

	var stored models.Reservation
	require.NoError(t, db.First(&stored, res.ID).Error)
	assert.Equal(t, "PENDING", stored.Status, "a forbidden actor must not mutate the booking")

	// The centre check is unchanged: the right professional at the wrong centre
	// is still refused.
	w = do(r, http.MethodPatch, fmt.Sprintf("/api/reservation/me/%d/status", res.ID),
		roleToken(t, 2, 2, "personnel"), `{"status":"CONFIRMED"}`)
	assert.Equal(t, http.StatusForbidden, w.Code, "a different centre claim is still forbidden")
}

// TestReservationRepo_MembershipMatchesAgenda proves the shared predicate: the
// agenda and the membership check agree for a line professional and for an
// unrelated one — they can never drift.
func TestReservationRepo_MembershipMatchesAgenda(t *testing.T) {
	db := setupRouterAuthzDB(t)
	repo := repository.NewReservationRepositoryImpl(db)

	res := seedReservationWithPricedLines(t, db, 1, 1, 1, mpDay, "10:00", "11:00", "CONFIRMED", 100, []reservedLine{
		{serviceID: 1, personnelID: 1, price: 50, duration: 30},
		{serviceID: 2, personnelID: 2, price: 50, duration: 30},
	})

	for _, tc := range []struct {
		personnelID uint
		want        bool
	}{
		{1, true},  // lead
		{2, true},  // line only
		{3, false}, // unrelated
	} {
		agenda, err := repo.FindByPersonnelId(tc.personnelID, nil, nil)
		require.NoError(t, err)
		member, err := repo.IsPersonnelMember(res.ID, tc.personnelID)
		require.NoError(t, err)
		assert.Equalf(t, tc.want, member, "membership for pro %d", tc.personnelID)
		assert.Equalf(t, tc.want, len(agenda) == 1, "agenda membership for pro %d", tc.personnelID)
	}
}
