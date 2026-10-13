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

// The client's reservation endpoints serialise the RAW model, not the DTO, and
// both queries preload `Client`. `Client.Password` carried `json:"password"`, so
// the bcrypt hash travelled to the client in the response body - verified
// empirically in a probe before the fix (the body contained
// `"password":"hashed..."`).
//
// BeautyCenter and Personnel already used `json:"-"`; Client was the only one
// leaking. The tag is now `-`, and this guard fails if it ever comes back.
func TestClientReservationEndpoints_NeverLeakThePasswordHash(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	r, db := multiProfessionalRouter(t)

	seedDashboardCenter(t, db, 1, "Salon X", "salonx@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedDashboardClient(t, db, 1, "Leak", "Client")
	seedServiceRow(t, db, 10, 1, 30)

	// A value distinctive enough that finding it in the body is unambiguous.
	const marker = "DISTINCTIVE_HASH_MARKER_9f3a"
	require.NoError(t, db.Model(&models.Client{}).Where("id = ?", 1).
		Update("password", marker).Error)

	seedMultiProfessionalReservation(t, db, 1, 1, mpDay, "10:00", "10:30", []mpLine{
		{serviceID: 10, personnelID: 1, duration: 30},
	})

	// The client's own token: this is the surface that renders their bookings.
	token := roleToken(t, 1, 0, "client")

	for _, path := range []string{
		"/api/reservation/client/1",
		"/api/reservation/client/1/upcoming",
	} {
		w := do(r, http.MethodGet, path, token, "")
		require.Less(t, w.Code, 500, "%s must not fail: %s", path, w.Body.String())

		body := w.Body.String()
		assert.NotContains(t, body, marker,
			"%s leaked the password hash value", path)
		assert.NotContains(t, body, `"password"`,
			"%s exposed a password field; the client model must keep json:\"-\"", path)
	}
}
