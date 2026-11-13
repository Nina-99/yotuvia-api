package tests

import (
	"encoding/json"
	"net/http"
	"regexp"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/router"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// otpRouter mounts the auth routes with a recording mailer so tests can read the
// code that would be emailed.
func otpRouter(t *testing.T, db *gorm.DB) (*gin.Engine, *utils.FakeMailer) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	mailer := utils.NewFakeMailer()
	r := gin.New()
	rg := r.Group("/api")
	router.AuthAccountRoutes(rg, db, validator.New(), mailer)
	return r, mailer
}

var sixDigits = regexp.MustCompile(`\b\d{6}\b`)

// lastCode returns the most recent 6-digit code sent to an address.
func lastCode(t *testing.T, mailer *utils.FakeMailer, to string) string {
	t.Helper()
	msgs := mailer.Messages()
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].To != to {
			continue
		}
		if code := sixDigits.FindString(msgs[i].Text); code != "" {
			return code
		}
	}
	t.Fatalf("no code email sent to %s", to)
	return ""
}

func doRequestCode(t *testing.T, r *gin.Engine, email string) *httptestResponse {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email})
	w := do(r, http.MethodPost, "/api/auth-accounts/otp/request", "", string(body))
	return &httptestResponse{code: w.Code, body: w.Body.String()}
}

type httptestResponse struct {
	code int
	body string
}

func verifyCode(t *testing.T, r *gin.Engine, email, code string) *httptestResponse {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"email": email, "code": code})
	w := do(r, http.MethodPost, "/api/auth-accounts/otp/verify", "", string(body))
	return &httptestResponse{code: w.Code, body: w.Body.String()}
}

func setHighLimits(t *testing.T) {
	t.Helper()
	t.Setenv("RATE_LIMIT_IP_PER_MIN", "10000")
	t.Setenv("RATE_LIMIT_IP_BURST", "10000")
	t.Setenv("RATE_LIMIT_IDENTIFIER_PER_MIN", "10000")
	t.Setenv("RATE_LIMIT_IDENTIFIER_BURST", "10000")
}

// TestOTP_RequestIsUniform — a known and an unknown address get the exact same
// response, and both receive a code.
func TestOTP_RequestIsUniform(t *testing.T) {
	setHighLimits(t)
	db := setupAuthRefreshDB(t)
	r, mailer := otpRouter(t, db)

	require.NoError(t, db.Create(&models.Client{
		FirstName: "Ana", LastName: "Lopez", Username: "ana", Email: "known@test.com",
		Phone: "5550100", IsActive: true,
	}).Error)

	known := doRequestCode(t, r, "known@test.com")
	unknown := doRequestCode(t, r, "unknown@test.com")

	assert.Equal(t, http.StatusAccepted, known.code)
	assert.Equal(t, known.code, unknown.code, "status must not differ")
	assert.Equal(t, known.body, unknown.body, "body must not differ")
	assert.Equal(t, 2, mailer.CallCount(), "both requests must send an email")

	var challenges int64
	require.NoError(t, db.Model(&models.OtpChallenge{}).Count(&challenges).Error)
	assert.Equal(t, int64(2), challenges)
}

// TestOTP_ExistingAccountGetsSession — a valid code signs an active account in.
func TestOTP_ExistingAccountGetsSession(t *testing.T) {
	setHighLimits(t)
	db := setupAuthRefreshDB(t)
	r, mailer := otpRouter(t, db)

	require.NoError(t, db.Create(&models.Client{
		FirstName: "Ana", LastName: "Lopez", Username: "ana", Email: "known@test.com",
		Phone: "5550100", IsActive: true,
	}).Error)

	doRequestCode(t, r, "known@test.com")
	res := verifyCode(t, r, "known@test.com", lastCode(t, mailer, "known@test.com"))
	require.Equal(t, http.StatusOK, res.code, res.body)

	var resp authResponseEnvelope
	require.NoError(t, json.Unmarshal([]byte(res.body), &resp))
	assert.NotEmpty(t, resp.Token)
	assert.NotEmpty(t, resp.RefreshToken)
}

// TestOTP_UnknownEmailGetsTicket — a verified unknown address gets a ticket and
// no tokens.
func TestOTP_UnknownEmailGetsTicket(t *testing.T) {
	setHighLimits(t)
	db := setupAuthRefreshDB(t)
	r, mailer := otpRouter(t, db)

	doRequestCode(t, r, "new@test.com")
	res := verifyCode(t, r, "new@test.com", lastCode(t, mailer, "new@test.com"))
	require.Equal(t, http.StatusOK, res.code, res.body)

	var payload struct {
		RegistrationTicket string `json:"registration_ticket"`
		Email              string `json:"email"`
		Token              string `json:"token"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.body), &payload))
	assert.NotEmpty(t, payload.RegistrationTicket)
	assert.Equal(t, "new@test.com", payload.Email)
	assert.Empty(t, payload.Token, "an unregistered email must not receive tokens")
}

// TestOTP_ResendInvalidatesPrevious — only the newest code verifies.
func TestOTP_ResendInvalidatesPrevious(t *testing.T) {
	setHighLimits(t)
	db := setupAuthRefreshDB(t)
	r, mailer := otpRouter(t, db)

	require.NoError(t, db.Create(&models.Client{
		FirstName: "Ana", LastName: "Lopez", Username: "ana", Email: "known@test.com",
		Phone: "5550100", IsActive: true,
	}).Error)

	doRequestCode(t, r, "known@test.com")
	first := lastCode(t, mailer, "known@test.com")
	doRequestCode(t, r, "known@test.com")
	second := lastCode(t, mailer, "known@test.com")
	require.NotEqual(t, first, second)

	old := verifyCode(t, r, "known@test.com", first)
	assert.Equal(t, http.StatusUnauthorized, old.code, "superseded code must fail")

	fresh := verifyCode(t, r, "known@test.com", second)
	assert.Equal(t, http.StatusOK, fresh.code, fresh.body)
}

// TestOTP_WrongCodeAndAttemptCap — five wrong attempts burn the challenge.
func TestOTP_WrongCodeAndAttemptCap(t *testing.T) {
	setHighLimits(t)
	db := setupAuthRefreshDB(t)
	r, mailer := otpRouter(t, db)

	require.NoError(t, db.Create(&models.Client{
		FirstName: "Ana", LastName: "Lopez", Username: "ana", Email: "known@test.com",
		Phone: "5550100", IsActive: true,
	}).Error)

	doRequestCode(t, r, "known@test.com")
	good := lastCode(t, mailer, "known@test.com")
	wrong := "000000"
	if good == wrong {
		wrong = "111111"
	}

	for i := 0; i < 5; i++ {
		res := verifyCode(t, r, "known@test.com", wrong)
		assert.Equal(t, http.StatusUnauthorized, res.code)
	}

	res := verifyCode(t, r, "known@test.com", good)
	assert.Equal(t, http.StatusUnauthorized, res.code, "a burned challenge must reject the correct code too")
}

// TestOTP_InactiveAccountRejected — a deactivated client cannot sign in by code.
func TestOTP_InactiveAccountRejected(t *testing.T) {
	setHighLimits(t)
	db := setupAuthRefreshDB(t)
	r, mailer := otpRouter(t, db)

	client := models.Client{
		FirstName: "Ana", LastName: "Lopez", Username: "ana", Email: "off@test.com",
		Phone: "5550100", IsActive: false,
	}
	require.NoError(t, db.Create(&client).Error)
	// gorm:"default:true" coerces the zero value back to true on insert.
	require.NoError(t, db.Model(&client).Update("is_active", false).Error)

	doRequestCode(t, r, "off@test.com")
	res := verifyCode(t, r, "off@test.com", lastCode(t, mailer, "off@test.com"))
	assert.Equal(t, http.StatusUnauthorized, res.code, res.body)
}

// TestOTP_RequestThrottledPerEmail — the fourth request for one address is 429.
func TestOTP_RequestThrottledPerEmail(t *testing.T) {
	setHighLimits(t)
	db := setupAuthRefreshDB(t)
	r, _ := otpRouter(t, db)

	for i := 0; i < 3; i++ {
		res := doRequestCode(t, r, "flood@test.com")
		require.Equal(t, http.StatusAccepted, res.code)
	}
	res := doRequestCode(t, r, "flood@test.com")
	assert.Equal(t, http.StatusTooManyRequests, res.code)
}

// TestRegistrationTicket_CannotAuthenticate — a ticket is not a bearer token.
func TestRegistrationTicket_CannotAuthenticate(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	ticket, err := utils.GenerateRegistrationTicket("new@test.com")
	require.NoError(t, err)

	_, err = utils.ValidateToken(ticket)
	assert.Error(t, err, "a registration ticket must never pass access-token validation")
}

// TestRegistrationTicket_TamperRejected — a modified ticket is refused.
func TestRegistrationTicket_TamperRejected(t *testing.T) {
	setHighLimits(t)
	db := setupAuthRefreshDB(t)
	r, mailer := otpRouter(t, db)

	doRequestCode(t, r, "new@test.com")
	res := verifyCode(t, r, "new@test.com", lastCode(t, mailer, "new@test.com"))
	var payload struct {
		RegistrationTicket string `json:"registration_ticket"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.body), &payload))

	body, _ := json.Marshal(map[string]any{
		"registration_ticket": payload.RegistrationTicket + "tampered",
		"user_type":           "client",
		"first_name":          "Ana",
		"last_name":           "Lopez",
		"phone":               "5550100",
		"terms_accepted":      true,
	})
	w := do(r, http.MethodPost, "/api/auth-accounts/register/complete", "", string(body))
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
}

// TestCompleteRegistration_Client — the verified email becomes an active client.
func TestCompleteRegistration_Client(t *testing.T) {
	setHighLimits(t)
	db := setupAuthRefreshDB(t)
	r, mailer := otpRouter(t, db)

	doRequestCode(t, r, "new@test.com")
	res := verifyCode(t, r, "new@test.com", lastCode(t, mailer, "new@test.com"))
	var payload struct {
		RegistrationTicket string `json:"registration_ticket"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.body), &payload))

	body, _ := json.Marshal(map[string]any{
		"registration_ticket": payload.RegistrationTicket,
		"user_type":           "client",
		"first_name":          "Ana",
		"last_name":           "Lopez",
		"phone":               "5550100",
		"terms_accepted":      true,
	})
	w := do(r, http.MethodPost, "/api/auth-accounts/register/complete", "", string(body))
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var client models.Client
	require.NoError(t, db.Where("email = ?", "new@test.com").First(&client).Error)
	assert.True(t, client.IsActive)

	var account models.AuthAccount
	require.NoError(t, db.Where("provider = ? AND user_type = ? AND client_id = ?",
		"local", "client", client.ID).First(&account).Error)
	assert.NotNil(t, account.TermsAcceptedAt)
}

// TestCompleteRegistration_RejectsPersonnel — staff is invitation-only.
func TestCompleteRegistration_RejectsPersonnel(t *testing.T) {
	setHighLimits(t)
	db := setupAuthRefreshDB(t)
	r, mailer := otpRouter(t, db)

	doRequestCode(t, r, "new@test.com")
	res := verifyCode(t, r, "new@test.com", lastCode(t, mailer, "new@test.com"))
	var payload struct {
		RegistrationTicket string `json:"registration_ticket"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.body), &payload))

	body, _ := json.Marshal(map[string]any{
		"registration_ticket": payload.RegistrationTicket,
		"user_type":           "personnel",
		"first_name":          "Ana",
		"last_name":           "Lopez",
		"phone":               "5550100",
		"terms_accepted":      true,
	})
	w := do(r, http.MethodPost, "/api/auth-accounts/register/complete", "", string(body))
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestCompleteRegistration_EmailTaken — an email registered between verify and
// complete answers 409.
func TestCompleteRegistration_EmailTaken(t *testing.T) {
	setHighLimits(t)
	db := setupAuthRefreshDB(t)
	r, mailer := otpRouter(t, db)

	doRequestCode(t, r, "race@test.com")
	res := verifyCode(t, r, "race@test.com", lastCode(t, mailer, "race@test.com"))
	var payload struct {
		RegistrationTicket string `json:"registration_ticket"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.body), &payload))

	require.NoError(t, db.Create(&models.Client{
		FirstName: "Race", LastName: "Winner", Username: "race", Email: "race@test.com",
		Phone: "5550100", IsActive: true,
	}).Error)

	body, _ := json.Marshal(map[string]any{
		"registration_ticket": payload.RegistrationTicket,
		"user_type":           "client",
		"first_name":          "Ana",
		"last_name":           "Lopez",
		"phone":               "5550100",
		"terms_accepted":      true,
	})
	w := do(r, http.MethodPost, "/api/auth-accounts/register/complete", "", string(body))
	assert.Equal(t, http.StatusConflict, w.Code, w.Body.String())
}

// TestOTP_PlatformAdminGetsSession — a platform admin email must sign in with a
// code instead of receiving a registration ticket that can never complete.
func TestOTP_PlatformAdminGetsSession(t *testing.T) {
	setHighLimits(t)
	db := setupAuthRefreshDB(t)
	r, mailer := otpRouter(t, db)

	seedPlatformAdminRow(t, db, "root-otp@test.com", "S3cret-Passw0rd!")

	doRequestCode(t, r, "root-otp@test.com")
	res := verifyCode(t, r, "root-otp@test.com", lastCode(t, mailer, "root-otp@test.com"))
	require.Equal(t, http.StatusOK, res.code, res.body)

	var payload struct {
		Token string `json:"token"`
		User  struct {
			Role  string `json:"role"`
			Email string `json:"email"`
		} `json:"user"`
	}
	require.NoError(t, json.Unmarshal([]byte(res.body), &payload))
	assert.NotEmpty(t, payload.Token)
	assert.Equal(t, "super_admin", payload.User.Role)
	assert.Equal(t, "root-otp@test.com", payload.User.Email)
}
