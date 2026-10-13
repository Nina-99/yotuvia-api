package tests

import (
	"testing"

	"github.com/Nina-99/yotuvia-api/configs"
	"github.com/Nina-99/yotuvia-api/data/request"
	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/repository"
	"github.com/Nina-99/yotuvia-api/service"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// newTestAuthService wires the production auth service over the shared SQLite
// schema, including the platform-admin repository the super_admin login branch
// resolves through.
func newTestAuthService(t *testing.T, db *gorm.DB) service.AuthAccountService {
	t.Helper()
	return service.NewAuthAccountServiceImpl(
		repository.NewAuthAccountRepositoryImpl(db),
		repository.NewClientRepositoryImpl(db),
		repository.NewPersonnelRepositoryImpl(db),
		repository.NewBeautyCenterRepositoryImpl(db),
		repository.NewPlatformAdminRepositoryImpl(db),
		repository.NewUserSessionRepositoryImpl(db),
		repository.NewAuthTokenRepositoryImpl(db),
		utils.NewFakeMailer(),
		nil,
		validator.New(),
	)
}

// seedPlatformAdminRow inserts one active platform admin with a bcrypt hash,
// the way the env-driven seed does, so login can be exercised end to end.
func seedPlatformAdminRow(t *testing.T, db *gorm.DB, email, plainPassword string) *models.PlatformAdmin {
	t.Helper()
	hashed, err := utils.HashPassword(plainPassword)
	require.NoError(t, err)
	admin := &models.PlatformAdmin{
		Email:    email,
		Password: hashed,
		Name:     "Platform Root",
		IsActive: true,
	}
	require.NoError(t, db.Create(admin).Error)
	return admin
}

// TestSuperAdminLogin_MintsTokenWithRoleAndNoCenterClaim — signing in without
// `user_type` must resolve the platform admin, stamp the role `super_admin` on
// the token and leave the centre claim ABSENT (a platform admin belongs to no
// centre), while still minting a refresh session.
func TestSuperAdminLogin_MintsTokenWithRoleAndNoCenterClaim(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	admin := seedPlatformAdminRow(t, db, "root@yotuvia.app", "S3cret-Passw0rd!")
	svc := newTestAuthService(t, db)

	res, err := svc.Login(request.LoginRequest{
		Identifier: "root@yotuvia.app",
		Password:   "S3cret-Passw0rd!",
	})
	require.NoError(t, err)

	assert.Equal(t, "super_admin", res.Role)
	assert.Equal(t, admin.ID, res.UserID)
	assert.Equal(t, "root@yotuvia.app", res.Email)
	assert.NotEmpty(t, res.Token)
	assert.NotEmpty(t, res.RefreshToken, "a platform-admin login mints a session too")

	claims, err := utils.ValidateToken(res.Token)
	require.NoError(t, err)
	assert.Equal(t, "super_admin", claims.Role, "the role claim drives RequireRole")
	assert.Equal(t, admin.ID, claims.UserID)
	assert.Zero(t, claims.CenterID, "a platform admin has no centre claim")

	var sessions int64
	require.NoError(t, db.Model(&models.UserSession{}).
		Where("user_id = ? AND role = ?", admin.ID, "super_admin").
		Count(&sessions).Error)
	assert.Equal(t, int64(1), sessions, "the refresh session is keyed by (user_id, role)")
}

// TestSuperAdminLogin_WrongPasswordAndUnknownIdentifierFail — a wrong password
// and an unknown identifier both answer the SAME generic credential error the
// other roles use (no enumeration, no behaviour change).
func TestSuperAdminLogin_WrongPasswordAndUnknownIdentifierFail(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedPlatformAdminRow(t, db, "root@yotuvia.app", "S3cret-Passw0rd!")
	svc := newTestAuthService(t, db)

	_, err := svc.Login(request.LoginRequest{Identifier: "root@yotuvia.app", Password: "wrong-password"})
	require.Error(t, err)
	assert.Equal(t, "invalid credentials", err.Error())

	_, err = svc.Login(request.LoginRequest{Identifier: "nobody@yotuvia.app", Password: "S3cret-Passw0rd!"})
	require.Error(t, err)
	assert.Equal(t, "invalid credentials", err.Error())
}

// TestSuperAdminLogin_ExplicitUserTypeIsNotAccepted — the platform admin signs
// in through the no-type path only: `user_type=super_admin` is still rejected by
// LoginRequest's existing oneof, so the public login contract is not widened.
func TestSuperAdminLogin_ExplicitUserTypeIsNotAccepted(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	seedPlatformAdminRow(t, db, "root@yotuvia.app", "S3cret-Passw0rd!")
	svc := newTestAuthService(t, db)

	_, err := svc.Login(request.LoginRequest{
		Identifier: "root@yotuvia.app",
		Password:   "S3cret-Passw0rd!",
		UserType:   "super_admin",
	})
	require.Error(t, err, "the existing oneof must not be loosened")
}

// TestSuperAdminLogin_DoesNotChangeOtherRoles — the super_admin entry in the
// no-type ladder must not alter how client/beauty_center/personnel resolve.
func TestSuperAdminLogin_DoesNotChangeOtherRoles(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)

	clientHash, err := utils.HashPassword("client-pass-123")
	require.NoError(t, err)
	require.NoError(t, db.Create(&models.Client{
		FirstName: "Ana",
		LastName:  "Diaz",
		Username:  "ana",
		Email:     "ana@yotuvia.app",
		Phone:     "555",
		Password:  clientHash,
		IsActive:  true,
	}).Error)

	svc := newTestAuthService(t, db)

	res, err := svc.Login(request.LoginRequest{Identifier: "ana@yotuvia.app", Password: "client-pass-123"})
	require.NoError(t, err)
	assert.Equal(t, "client", res.Role, "the no-type ladder still finds clients first")

	_, err = svc.Login(request.LoginRequest{Identifier: "ana@yotuvia.app", Password: "nope"})
	require.Error(t, err)
	assert.Equal(t, "invalid credentials", err.Error(), "the generic error message is unchanged")
}

// TestSeedPlatformAdmin_IsIdempotentAndNeverOverwritesPassword — the env-driven
// seed creates the account once; a re-run with a different SUPER_ADMIN_PASSWORD
// must neither duplicate the row nor rotate the stored hash.
func TestSeedPlatformAdmin_IsIdempotentAndNeverOverwritesPassword(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	configs.DB = db
	defer func() { configs.DB = nil }()

	t.Setenv("SUPER_ADMIN_EMAIL", "root@yotuvia.app")
	t.Setenv("SUPER_ADMIN_PASSWORD", "first-strong-local-pw")
	configs.SeedPlatformAdmin()

	var admins []models.PlatformAdmin
	require.NoError(t, db.Find(&admins).Error)
	require.Len(t, admins, 1, "the seed must create exactly one row")
	assert.Equal(t, "root@yotuvia.app", admins[0].Email)
	assert.True(t, admins[0].IsActive)
	require.True(t, utils.VerifyPassword("first-strong-local-pw", admins[0].Password),
		"the seeded hash must verify the env password")
	firstHash := admins[0].Password

	// Second run, different password: idempotent.
	t.Setenv("SUPER_ADMIN_PASSWORD", "second-strong-local-pw")
	configs.SeedPlatformAdmin()

	admins = nil
	require.NoError(t, db.Find(&admins).Error)
	require.Len(t, admins, 1, "a re-run must not duplicate the account")
	assert.Equal(t, firstHash, admins[0].Password, "a re-run must not overwrite the stored hash")
	assert.True(t, utils.VerifyPassword("first-strong-local-pw", admins[0].Password))
	assert.False(t, utils.VerifyPassword("second-strong-local-pw", admins[0].Password))
}

// TestSeedPlatformAdmin_SkipsWhenEnvUnset — with no env configuration the seed
// is a logged no-op, never a row with an empty password.
func TestSeedPlatformAdmin_SkipsWhenEnvUnset(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	db := setupRouterAuthzDB(t)
	configs.DB = db
	defer func() { configs.DB = nil }()

	t.Setenv("SUPER_ADMIN_EMAIL", "")
	t.Setenv("SUPER_ADMIN_PASSWORD", "")
	configs.SeedPlatformAdmin()

	var count int64
	require.NoError(t, db.Model(&models.PlatformAdmin{}).Count(&count).Error)
	assert.Zero(t, count, "unset env must not create an account")

	// Only the email set is still a no-op: a half-configured seed must not
	// create a passwordless principal.
	t.Setenv("SUPER_ADMIN_EMAIL", "root@yotuvia.app")
	configs.SeedPlatformAdmin()
	count = 0
	require.NoError(t, db.Model(&models.PlatformAdmin{}).Count(&count).Error)
	assert.Zero(t, count, "an email without a password must not create an account")
}
