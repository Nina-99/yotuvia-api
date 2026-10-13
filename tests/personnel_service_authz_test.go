package tests

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/data/response"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────────────────────────────────────────────
//  /api/personnel-services authorization
// ──────────────────────────────────────────────
//
// The capability table was previously mounted with NO middleware at all, so any
// anonymous caller could grant or revoke a professional's capability for any
// centre. These tests pin the fixed model: authenticated everywhere, writes
// center-only, and writes ownership-checked against the acting centre's claim.

// TestPersonnelServiceAuthz_NoToken401 proves all three routes reject an
// anonymous caller before any handler runs.
func TestPersonnelServiceAuthz_NoToken401(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, _ := authzTestRouter(t)

	cases := []struct {
		name, method, path string
	}{
		{"create", http.MethodPost, "/api/personnel-services"},
		{"delete", http.MethodDelete, "/api/personnel-services/1"},
		{"list", http.MethodGet, "/api/personnel-services?personnel_id=1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := do(r, tc.method, tc.path, "", "")
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Equal(t, "Authorization header missing", errBody(t, w.Body.String()))
		})
	}
}

// TestPersonnelServiceAuthz_ClientReadAllowedWriteForbidden proves the GET stays
// usable by the client booking picker (authenticated, not center-only) while the
// writes are gated to a center actor.
func TestPersonnelServiceAuthz_ClientReadAllowedWriteForbidden(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := authzTestRouter(t)
	seedDashboardCenter(t, db, 1, "Cap Center", "cap@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedServiceRow(t, db, 1, 1, 30)

	clientToken := roleToken(t, 9, 0, "client")

	// GET is reachable (400 "provide either..." rather than 401/403).
	w := do(r, http.MethodGet, "/api/personnel-services?personnel_id=1", clientToken, "")
	assert.Equal(t, http.StatusOK, w.Code)

	// Writes are 403 for a client.
	w = do(r, http.MethodPost, "/api/personnel-services", clientToken,
		`{"personnel_id":1,"service_id":1}`)
	assert.Equal(t, http.StatusForbidden, w.Code)

	w = do(r, http.MethodDelete, "/api/personnel-services/1", clientToken, "")
	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestPersonnelServiceAuthz_OwnershipEnforced is the core regression guard: a
// centre can only assign/remove capabilities that belong to it. A foreign
// centre's tokens are rejected with 403, and the owning centre succeeds.
func TestPersonnelServiceAuthz_OwnershipEnforced(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := authzTestRouter(t)

	seedDashboardCenter(t, db, 1, "Own Center", "own@test.com", true)
	seedDashboardPersonnel(t, db, 1, 1, true)
	seedServiceRow(t, db, 1, 1, 30)

	seedDashboardCenter(t, db, 2, "Foreign Center", "foreign@test.com", true)
	seedDashboardPersonnel(t, db, 2, 2, true)
	seedServiceRow(t, db, 2, 2, 30)

	// A capability row that belongs to centre 2.
	foreignRow := &models.PersonnelService{PersonnelID: 2, ServiceID: 2}
	require.NoError(t, db.Create(foreignRow).Error)

	ownToken := roleToken(t, 1, 1, "beauty_center")

	// ── Foreign personnel in the body → 403 ──
	w := do(r, http.MethodPost, "/api/personnel-services", ownToken,
		`{"personnel_id":2,"service_id":1}`)
	assert.Equal(t, http.StatusForbidden, w.Code,
		"assigning a foreign professional must be forbidden: %s", w.Body.String())

	// ── Foreign service in the body → 403 ──
	w = do(r, http.MethodPost, "/api/personnel-services", ownToken,
		`{"personnel_id":1,"service_id":2}`)
	assert.Equal(t, http.StatusForbidden, w.Code,
		"assigning a foreign service must be forbidden: %s", w.Body.String())

	// ── Deleting a foreign centre's row → 403 and the row survives ──
	w = do(r, http.MethodDelete, fmt.Sprintf("/api/personnel-services/%d", foreignRow.ID), ownToken, "")
	assert.Equal(t, http.StatusForbidden, w.Code)

	var stillThere models.PersonnelService
	require.NoError(t, db.First(&stillThere, foreignRow.ID).Error)

	// ── The owner succeeds ──
	w = do(r, http.MethodPost, "/api/personnel-services", ownToken,
		`{"personnel_id":1,"service_id":1}`)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var created response.PersonnelServiceResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &created))
	assert.Equal(t, uint(1), created.PersonnelID)
	assert.Equal(t, uint(1), created.ServiceID)

	w = do(r, http.MethodDelete, fmt.Sprintf("/api/personnel-services/%d", created.ID), ownToken, "")
	assert.Equal(t, http.StatusNoContent, w.Code)

	// A missing row is a 404, not a 403, for the owning centre.
	w = do(r, http.MethodDelete, "/api/personnel-services/9999", ownToken, "")
	assert.Equal(t, http.StatusNotFound, w.Code)
}
