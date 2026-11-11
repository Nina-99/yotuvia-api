package tests

import (
	"net/http"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// TestAuthRoleGates_OwnerOnly — staff creation and invitations are owner-only;
// a client or a staff token must be rejected before the handler runs.
func TestAuthRoleGates_OwnerOnly(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	t.Setenv("JWT_SECRET", testJWTSecret)
	t.Setenv("RATE_LIMIT_IP_PER_MIN", "10000")
	t.Setenv("RATE_LIMIT_IP_BURST", "10000")

	db := setupRouterAuthzDB(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api")
	router.AuthAccountRoutes(rg, db, validator.New())
	router.PersonnelInvitationRoutes(rg, db, validator.New())

	cases := []struct {
		name       string
		method     string
		path       string
		disallowed string
	}{
		{"register personnel", http.MethodPost, "/api/auth/register-personnel", "client"},
		{"personnel invitations", http.MethodPost, "/api/personnel/invitations", "personnel"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := do(r, tc.method, tc.path, "", "")
			assert.Equal(t, http.StatusUnauthorized, w.Code, "no token must be 401")

			disallowed := roleToken(t, 1, 1, tc.disallowed)
			w = do(r, tc.method, tc.path, disallowed, "{}")
			assert.Equal(t, http.StatusForbidden, w.Code, "non-owner must be 403")

			owner := roleToken(t, 1, 1, "beauty_center")
			w = do(r, tc.method, tc.path, owner, "{}")
			require.NotEqual(t, http.StatusUnauthorized, w.Code)
			require.NotEqual(t, http.StatusForbidden, w.Code)
		})
	}
}

// TestAuthRoleGates_RegisterPersonnelUsesClaimCenter — the body's
// beauty_center_id is ignored; the personnel is created under the owner's claim.
func TestAuthRoleGates_RegisterPersonnelUsesClaimCenter(t *testing.T) {
	utils.Log = zap.NewNop().Sugar()
	t.Setenv("JWT_SECRET", testJWTSecret)
	t.Setenv("RATE_LIMIT_IP_PER_MIN", "10000")
	t.Setenv("RATE_LIMIT_IP_BURST", "10000")

	db := setupRouterAuthzDB(t)
	seedDashboardCenter(t, db, 1, "Owner Center", "owner@test.com", true)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	rg := r.Group("/api")
	router.AuthAccountRoutes(rg, db, validator.New())

	owner := roleToken(t, 1, 1, "beauty_center")
	body := `{"beauty_center_id":999,"first_name":"Ana","last_name":"Staff","email":"staff-claim@test.com","phone":"5550101","password":"password123"}`
	w := do(r, http.MethodPost, "/api/auth/register-personnel", owner, body)
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var staff models.Personnel
	require.NoError(t, db.Where("email = ?", "staff-claim@test.com").First(&staff).Error)
	assert.Equal(t, uint(1), staff.BeautyCenterID, "the claim must win over the body")
}
