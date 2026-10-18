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
)

// Visit notes (Phase C3): a professional records a short internal note about a
// visit THEY served. Authorization is by membership (the same rule C1 uses), the
// note is author-scoped (a colleague's note is never visible), and an empty body
// is invalid input — clearing is the DELETE endpoint's job.

func noteBody(text string) string {
	return fmt.Sprintf(`{"body":%q}`, text)
}

func TestVisitNote_UpsertListAndDelete(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 7, 1, true)
	seedDashboardClient(t, db, 1, "Ana", "Lopez")
	seedServiceRow(t, db, 10, 1, 30)

	target := seedMultiProfessionalReservation(t, db, 1, 7, mpDay, "09:00", "09:30",
		[]mpLine{{serviceID: 10, personnelID: 7, duration: 30}})

	token := roleToken(t, 7, 1, "personnel")

	// Create.
	w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/visit-notes/%d", target.ID),
		token, noteBody("Client is allergic to ammonia dye"))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// List over the visit's day returns it.
	w = do(r, http.MethodGet,
		"/api/reservation/visit-notes?from="+bookingTestDateKey+"&to="+bookingTestDateKey, token, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var notes []models.VisitNote
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &notes))
	require.Len(t, notes, 1)
	assert.Equal(t, "Client is allergic to ammonia dye", notes[0].Body)
	assert.Equal(t, target.ID, notes[0].ReservationID)

	// Upsert replaces the body instead of inserting a second row.
	w = do(r, http.MethodPut, fmt.Sprintf("/api/reservation/visit-notes/%d", target.ID),
		token, noteBody("Updated: prefers unscented products"))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var count int64
	require.NoError(t, db.Model(&models.VisitNote{}).Count(&count).Error)
	assert.Equal(t, int64(1), count, "one note per (visit, author)")

	// Delete removes it.
	w = do(r, http.MethodDelete, fmt.Sprintf("/api/reservation/visit-notes/%d", target.ID), token, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, db.Model(&models.VisitNote{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}

func TestVisitNote_ForeignVisitForbidden(t *testing.T) {
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

	w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/visit-notes/%d", target.ID),
		roleToken(t, 9, 1, "personnel"), noteBody("not my visit"))
	assert.Equal(t, http.StatusForbidden, w.Code, "a colleague's visit is off limits")

	var count int64
	require.NoError(t, db.Model(&models.VisitNote{}).Count(&count).Error)
	assert.Equal(t, int64(0), count, "a forbidden write persists nothing")
}

func TestVisitNote_EmptyBodyRejected(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 7, 1, true)
	seedDashboardClient(t, db, 1, "Ana", "Lopez")
	seedServiceRow(t, db, 10, 1, 30)

	target := seedMultiProfessionalReservation(t, db, 1, 7, mpDay, "09:00", "09:30",
		[]mpLine{{serviceID: 10, personnelID: 7, duration: 30}})

	for _, body := range []string{`{"body":""}`, `{"body":"   "}`} {
		w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/visit-notes/%d", target.ID),
			roleToken(t, 7, 1, "personnel"), body)
		assert.Equal(t, http.StatusBadRequest, w.Code, "empty body: %s", body)
	}
}

// TestVisitNote_AuthorScoped proves a colleague's note on the SAME visit is
// invisible: the list only ever returns the caller's own notes.
func TestVisitNote_AuthorScoped(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 7, 1, true)
	seedDashboardPersonnel(t, db, 9, 1, true)
	seedDashboardClient(t, db, 1, "Ana", "Lopez")
	seedServiceRow(t, db, 10, 1, 30)
	seedServiceRow(t, db, 11, 1, 30)

	// One visit served by TWO professionals (a two-line plan).
	target := seedMultiProfessionalReservation(t, db, 1, 7, mpDay, "09:00", "10:00",
		[]mpLine{
			{serviceID: 10, personnelID: 7, duration: 30},
			{serviceID: 11, personnelID: 9, duration: 30},
		})

	// Both pros note the same visit.
	require.Equal(t, http.StatusOK, do(r, http.MethodPut,
		fmt.Sprintf("/api/reservation/visit-notes/%d", target.ID),
		roleToken(t, 7, 1, "personnel"), noteBody("pro 7 note")).Code)
	require.Equal(t, http.StatusOK, do(r, http.MethodPut,
		fmt.Sprintf("/api/reservation/visit-notes/%d", target.ID),
		roleToken(t, 9, 1, "personnel"), noteBody("pro 9 note")).Code)

	var all int64
	require.NoError(t, db.Model(&models.VisitNote{}).Count(&all).Error)
	assert.Equal(t, int64(2), all, "each author keeps their own note")

	// Pro 7 only ever sees their own.
	w := do(r, http.MethodGet,
		"/api/reservation/visit-notes?from="+bookingTestDateKey+"&to="+bookingTestDateKey,
		roleToken(t, 7, 1, "personnel"), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var notes []models.VisitNote
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &notes))
	require.Len(t, notes, 1)
	assert.Equal(t, "pro 7 note", notes[0].Body)
}

// TestVisitNote_OwnerNotesAnyVisitOfItsCentre: the owner may annotate any visit
// of its centre (not just ones they served), and that note is kept SEPARATE from
// the professional's own note on the same visit.
func TestVisitNote_OwnerNotesAnyVisitOfItsCentre(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 7, 1, true)
	seedDashboardClient(t, db, 1, "Ana", "Lopez")
	seedServiceRow(t, db, 10, 1, 30)

	target := seedMultiProfessionalReservation(t, db, 1, 7, mpDay, "09:00", "09:30",
		[]mpLine{{serviceID: 10, personnelID: 7, duration: 30}})

	day := bookingTestDateKey
	ownerToken := roleToken(t, 1, 1, "beauty_center")

	// The owner notes a visit they did not serve.
	w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/visit-notes/%d", target.ID),
		ownerToken, noteBody("Centre follow-up: client asked for a colour quote"))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// The professional notes the same visit too.
	require.Equal(t, http.StatusOK, do(r, http.MethodPut,
		fmt.Sprintf("/api/reservation/visit-notes/%d", target.ID),
		roleToken(t, 7, 1, "personnel"), noteBody("Used formula 7.3")).Code)

	var all int64
	require.NoError(t, db.Model(&models.VisitNote{}).Count(&all).Error)
	assert.Equal(t, int64(2), all, "an owner note and a staff note coexist")

	// The owner's list is scoped to the owner.
	w = do(r, http.MethodGet, "/api/reservation/visit-notes?from="+day+"&to="+day, ownerToken, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var ownerNotes []models.VisitNote
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &ownerNotes))
	require.Len(t, ownerNotes, 1)
	assert.Contains(t, ownerNotes[0].Body, "colour quote")

	// ...and the professional's list still shows only their own.
	w = do(r, http.MethodGet, "/api/reservation/visit-notes?from="+day+"&to="+day,
		roleToken(t, 7, 1, "personnel"), "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var staffNotes []models.VisitNote
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &staffNotes))
	require.Len(t, staffNotes, 1)
	assert.Equal(t, "Used formula 7.3", staffNotes[0].Body)
}

// TestVisitNote_OwnerCannotNoteForeignVisit guards the centre boundary: owning
// centre 2 says nothing about centre 1's visits.
func TestVisitNote_OwnerCannotNoteForeignVisit(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardCenter(t, db, 2, "Salon Y", "salony@test.com", true)
	seedDashboardPersonnel(t, db, 7, 1, true)
	seedDashboardClient(t, db, 1, "Ana", "Lopez")
	seedServiceRow(t, db, 10, 1, 30)

	target := seedMultiProfessionalReservation(t, db, 1, 7, mpDay, "09:00", "09:30",
		[]mpLine{{serviceID: 10, personnelID: 7, duration: 30}})

	w := do(r, http.MethodPut, fmt.Sprintf("/api/reservation/visit-notes/%d", target.ID),
		roleToken(t, 2, 2, "beauty_center"), noteBody("not my centre"))
	assert.Equal(t, http.StatusForbidden, w.Code)

	var count int64
	require.NoError(t, db.Model(&models.VisitNote{}).Count(&count).Error)
	assert.Equal(t, int64(0), count)
}
