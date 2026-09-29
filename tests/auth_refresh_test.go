package tests

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/data/request"
	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/repository"
	"github.com/Nina-99/yottu-api/router"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// authRefreshModels mirrors configs/db.config.go's AutoMigrate list plus the
// new UserSession table, so HTTP tests exercise the same schema production has.
func authRefreshModels() []interface{} {
	return []interface{}{
		&models.AuthAccount{}, &models.BeautyCenter{}, &models.BeautyCenterImage{},
		&models.Category{}, &models.TokenAccount{}, &models.TokenTopUp{},
		&models.TokenTransaction{}, &models.TokenMission{}, &models.TokenMissionProgress{},
		&models.Referral{}, &models.Client{}, &models.CustomerPunchCard{},
		&models.Favorite{}, &models.FeedPost{}, &models.LoyaltyProgram{},
		&models.Personnel{}, &models.PersonnelImage{}, &models.PersonnelService{},
		&models.PostLike{}, &models.PunchHistory{}, &models.Reservation{},
		&models.ReservationService{}, &models.Review{}, &models.Service{},
		&models.SocialMedia{}, &models.WorkingHour{}, &models.PersonnelInvitation{}, &models.Notification{}, &models.PushDevice{},
		&models.UserSession{}, &models.AuthToken{},
	}
}

// setupAuthRefreshDB opens a fresh in-memory SQLite DB with the production
// model set (plus UserSession) and a JWT secret for token issuance.
func setupAuthRefreshDB(t *testing.T) *gorm.DB {
	t.Helper()
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	for _, m := range authRefreshModels() {
		if err := db.AutoMigrate(m); err != nil {
			t.Fatalf("failed to migrate %T: %v", m, err)
		}
	}
	return db
}

// authRefreshRouter mounts the auth-account routes exactly as router.go does.
func authRefreshRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	validate := validator.New()
	r := gin.New()
	rg := r.Group("/api")
	router.AuthAccountRoutes(rg, db, validate, utils.NewFakeMailer())
	return r
}

// authResponseEnvelope is the additive AuthResponse shape: {token,
// refresh_token, user}.
type authResponseEnvelope struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	User         struct {
		ID uint `json:"id"`
	} `json:"user"`
}

// registerClient registers a client through the real endpoint and asserts the
// additive refresh_token field is present (D4).
func registerClient(t *testing.T, r *gin.Engine) authResponseEnvelope {
	t.Helper()
	body := `{"first_name":"Ana","last_name":"Lopez","email":"ana.lopez@test.com","phone":"5550100","password":"password123","username":"ana","user_type":"client","terms_accepted":true}`
	w := do(r, http.MethodPost, "/api/auth-accounts/register", "", body)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var resp authResponseEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse register response: %v", err)
	}
	assert.NotEmpty(t, resp.Token, "register must return an access token")
	assert.NotEmpty(t, resp.RefreshToken, "register must return a refresh token (D4)")
	return resp
}

// doRefresh calls POST /auth-accounts/refresh with the given raw token.
func doRefresh(t *testing.T, r *gin.Engine, rawToken string) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(request.RefreshRequest{RefreshToken: rawToken})
	w := do(r, http.MethodPost, "/api/auth-accounts/refresh", "", string(body))
	return w
}

// parseTokenPair extracts {token, refresh_token} from a refresh response.
func parseTokenPair(t *testing.T, w *httptest.ResponseRecorder) (access, refresh string) {
	t.Helper()
	var resp struct {
		Token        string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse token pair: %v", err)
	}
	return resp.Token, resp.RefreshToken
}

// sessionRowsByFamily returns every generation row of one session family.
func sessionRowsByFamily(t *testing.T, db *gorm.DB, familyID string) []models.UserSession {
	t.Helper()
	var rows []models.UserSession
	if err := db.Where("family_id = ?", familyID).Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("failed to load session family: %v", err)
	}
	return rows
}

// sessionFamilyByHash returns the family of the session presenting the hash.
func sessionFamilyByHash(t *testing.T, db *gorm.DB, hash string) string {
	t.Helper()
	var s models.UserSession
	if err := db.Where("refresh_token_hash = ?", hash).First(&s).Error; err != nil {
		t.Fatalf("failed to load session by hash: %v", err)
	}
	return s.FamilyID
}

// TestAuthRefresh_RotatesActiveToken — spec "Successful token rotation": a
// valid active refresh token rotates to a new generation (new hash + reset
// 30-day expiry) and returns a fresh access + refresh pair.
func TestAuthRefresh_RotatesActiveToken(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	resp := registerClient(t, r)
	family := sessionFamilyByHash(t, db, utils.HashRefreshToken(resp.RefreshToken))

	// First rotation.
	w := doRefresh(t, r, resp.RefreshToken)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	_, refresh2 := parseTokenPair(t, w)
	assert.NotEqual(t, resp.RefreshToken, refresh2, "rotation must mint a new refresh token")

	// Second rotation keeps rotating within the same family.
	w = doRefresh(t, r, refresh2)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	access3, refresh3 := parseTokenPair(t, w)
	assert.NotEmpty(t, access3)
	assert.NotEqual(t, refresh2, refresh3)

	rows := sessionRowsByFamily(t, db, family)
	assert.Len(t, rows, 3, "two rotations must leave three generations")
	assert.True(t, rows[0].IsRevoked)
	assert.True(t, rows[1].IsRevoked)
	assert.False(t, rows[2].IsRevoked, "only the newest generation is active")
	assert.Equal(t, utils.HashRefreshToken(refresh3), rows[2].RefreshTokenHash)
	// 30-day refresh validity (spec Token Expiry Policies).
	assert.True(t, rows[2].ExpiresAt.After(time.Now().Add(29*24*time.Hour)),
		"active generation must have ~30d expiry")
}

// TestAuthRefresh_ReuseRevokesEntireFamily — spec "Refresh token reuse
// compromise": replaying an already-rotated token revokes every session in its
// family (including the newest generation) and returns 401.
func TestAuthRefresh_ReuseRevokesEntireFamily(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	resp := registerClient(t, r)
	family := sessionFamilyByHash(t, db, utils.HashRefreshToken(resp.RefreshToken))

	// Rotate once so the original token is stale.
	w := doRefresh(t, r, resp.RefreshToken)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	_, rotated := parseTokenPair(t, w)

	// Replay the already-rotated token → 401.
	w = doRefresh(t, r, resp.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Equal(t, "unauthorized", errBody(t, w.Body.String()))

	// The whole family is revoked: old AND newest generation.
	rows := sessionRowsByFamily(t, db, family)
	assert.Len(t, rows, 2)
	for _, row := range rows {
		assert.True(t, row.IsRevoked, "family generation %d must be revoked after reuse", row.ID)
	}

	// The rotated token is dead too — the family no longer has an active row.
	w = doRefresh(t, r, rotated)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// TestAuthRefresh_ExpiredToken401 — tasks 1.7/1.1: an expired refresh token is
// rejected with 401. Expiry is a lifecycle event, NOT theft: the family is
// left untouched.
func TestAuthRefresh_ExpiredToken401(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	resp := registerClient(t, r)
	family := sessionFamilyByHash(t, db, utils.HashRefreshToken(resp.RefreshToken))

	// Age the active session past its 30-day validity.
	db.Model(&models.UserSession{}).
		Where("refresh_token_hash = ?", utils.HashRefreshToken(resp.RefreshToken)).
		Update("expires_at", time.Now().Add(-time.Minute))

	w := doRefresh(t, r, resp.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	rows := sessionRowsByFamily(t, db, family)
	assert.Len(t, rows, 1)
	assert.False(t, rows[0].IsRevoked, "expiry must not revoke the family")
}

// TestAuthRefresh_UnknownToken401 — triangulation: a token that never existed
// is rejected without touching any family.
func TestAuthRefresh_UnknownToken401(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	w := doRefresh(t, r, "garbage-token-that-was-never-issued")
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	var count int64
	db.Model(&models.UserSession{}).Count(&count)
	assert.Equal(t, int64(0), count, "unknown token must not create or revoke rows")
}

// TestAuthLogout_RevokesSessionIdempotently — spec "Session Logout": logout
// revokes the presented session (204) and is idempotent for repeated calls.
func TestAuthLogout_RevokesSessionIdempotently(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	resp := registerClient(t, r)
	body, _ := json.Marshal(request.LogoutRequest{RefreshToken: resp.RefreshToken})

	w := do(r, http.MethodPost, "/api/auth-accounts/logout", resp.Token, string(body))
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Repeated logout with the same (now revoked) token is still 204.
	w = do(r, http.MethodPost, "/api/auth-accounts/logout", resp.Token, string(body))
	assert.Equal(t, http.StatusNoContent, w.Code)

	// The session row is revoked in the DB.
	var s models.UserSession
	assert.NoError(t, db.Where("refresh_token_hash = ?", utils.HashRefreshToken(resp.RefreshToken)).First(&s).Error)
	assert.True(t, s.IsRevoked, "logout must revoke the session row")

	// A revoked session cannot refresh afterwards.
	w = doRefresh(t, r, resp.RefreshToken)
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Logout is behind JWTAuthMiddleware: no bearer → 401.
	w = do(r, http.MethodPost, "/api/auth-accounts/logout", "", string(body))
	assert.Equal(t, http.StatusUnauthorized, w.Code)

	// Missing refresh_token in body → 400.
	w = do(r, http.MethodPost, "/api/auth-accounts/logout", resp.Token, `{}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// TestLoginAndRegisterReturnRefreshToken — D4: login AND register issue a
// refresh token additively; each login mints its own session row.
func TestLoginAndRegisterReturnRefreshToken(t *testing.T) {
	db := setupAuthRefreshDB(t)
	r := authRefreshRouter(t, db)

	reg := registerClient(t, r)

	loginBody := `{"identifier":"ana.lopez@test.com","password":"password123","user_type":"client"}`
	w := do(r, http.MethodPost, "/api/auth-accounts/login", "", loginBody)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var loginResp authResponseEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("failed to parse login response: %v", err)
	}
	assert.NotEmpty(t, loginResp.Token)
	assert.NotEmpty(t, loginResp.RefreshToken, "login must return a refresh token (D4)")
	assert.NotEqual(t, reg.RefreshToken, loginResp.RefreshToken, "each login must mint a new session")
	assert.Equal(t, reg.User.ID, loginResp.User.ID)

	var count int64
	db.Model(&models.UserSession{}).Where("user_id = ?", loginResp.User.ID).Count(&count)
	assert.Equal(t, int64(2), count, "register + login must create two session rows")
}

// TestAccessTokenExpiry15MinutesAndClaimsIntact — spec "Access token
// expiration": access tokens are valid 15 minutes with the identity claims
// intact (tasks 1.4/1.10 gate).
func TestAccessTokenExpiry15MinutesAndClaimsIntact(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	token, err := utils.GenerateJWT(42, 7, "beauty_center", "owner@test.com", "the-owner")
	assert.NoError(t, err)

	claims, err := utils.ValidateToken(token)
	assert.NoError(t, err)
	assert.Equal(t, uint(42), claims.UserID)
	assert.Equal(t, uint(7), claims.CenterID)
	assert.Equal(t, "beauty_center", claims.Role)
	assert.Equal(t, "owner@test.com", claims.Email)
	assert.Equal(t, "the-owner", claims.Username)

	exp := claims.ExpiresAt.Time
	assert.True(t, exp.After(time.Now().Add(10*time.Minute)),
		"access token must be valid for ~15m (got exp %v)", exp)
	assert.True(t, exp.Before(time.Now().Add(16*time.Minute)),
		"access token must not outlive 16m (got exp %v)", exp)
}

// TestUserSessionRepository_RotateCAS — task 1.5 primitive: the CAS claim on a
// session generation succeeds exactly once; a concurrent claim on the same
// generation loses; a wrong presented hash never claims.
func TestUserSessionRepository_RotateCAS(t *testing.T) {
	db := setupAuthRefreshDB(t)
	repo := repository.NewUserSessionRepositoryImpl(db)
	ctx := context.Background()

	s := &models.UserSession{
		UserID:           1,
		FamilyID:         "fam-cas-1",
		RefreshTokenHash: "hash-a",
		ExpiresAt:        time.Now().Add(30 * 24 * time.Hour),
		Role:             "client",
	}
	assert.NoError(t, repo.Create(ctx, s))

	claimed, err := repo.RotateCAS(ctx, s.ID, "hash-a")
	assert.NoError(t, err)
	assert.True(t, claimed, "first claim on the active generation must win")

	claimed, err = repo.RotateCAS(ctx, s.ID, "hash-a")
	assert.NoError(t, err)
	assert.False(t, claimed, "second claim on the same generation must lose")

	s2 := &models.UserSession{
		UserID:           2,
		FamilyID:         "fam-cas-2",
		RefreshTokenHash: "hash-b",
		ExpiresAt:        time.Now().Add(time.Hour),
		Role:             "client",
	}
	assert.NoError(t, repo.Create(ctx, s2))
	claimed, err = repo.RotateCAS(ctx, s2.ID, "wrong-hash")
	assert.NoError(t, err)
	assert.False(t, claimed, "a mismatched presented hash must never claim")

	var stored models.UserSession
	assert.NoError(t, db.First(&stored, s.ID).Error)
	assert.True(t, stored.IsRevoked, "a won CAS claim must persist as revoked")
}

// TestUserSessionRepository_FindByHashAndRevokeFamily — task 1.5: hash lookup
// returns the matching generation (nil for unknown) and family revocation hits
// every generation in the family and nothing outside it.
func TestUserSessionRepository_FindByHashAndRevokeFamily(t *testing.T) {
	db := setupAuthRefreshDB(t)
	repo := repository.NewUserSessionRepositoryImpl(db)
	ctx := context.Background()

	for _, s := range []*models.UserSession{
		{UserID: 1, FamilyID: "fam-x", RefreshTokenHash: "h1", ExpiresAt: time.Now().Add(time.Hour), Role: "client"},
		{UserID: 1, FamilyID: "fam-x", RefreshTokenHash: "h2", ExpiresAt: time.Now().Add(time.Hour), Role: "client"},
		{UserID: 1, FamilyID: "fam-y", RefreshTokenHash: "h3", ExpiresAt: time.Now().Add(time.Hour), Role: "client"},
	} {
		assert.NoError(t, repo.Create(ctx, s))
	}

	got, err := repo.FindByHash(ctx, "h2")
	assert.NoError(t, err)
	assert.NotNil(t, got)
	assert.Equal(t, "fam-x", got.FamilyID)

	missing, err := repo.FindByHash(ctx, "never-issued")
	assert.NoError(t, err)
	assert.Nil(t, missing)

	assert.NoError(t, repo.RevokeFamily(ctx, "fam-x"))
	for _, row := range sessionRowsByFamily(t, db, "fam-x") {
		assert.True(t, row.IsRevoked)
	}
	outside := sessionRowsByFamily(t, db, "fam-y")
	assert.Len(t, outside, 1)
	assert.False(t, outside[0].IsRevoked, "revocation must not leak to other families")
}
