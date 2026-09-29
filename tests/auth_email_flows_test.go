package tests

// End-to-end coverage for the email-verification and password-recovery flows:
// registration emails, single-use/expiry semantics, no-enumeration responses,
// the mailer abstraction (dev fallback + provider config) and the best-effort
// guarantee that a mail failure never fails registration.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Nina-99/yottu-api/models"
	"github.com/Nina-99/yottu-api/router"
	"github.com/Nina-99/yottu-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

// authEmailRouter mounts the auth routes with an injected recording mailer, so
// tests can read the exact link/token that was "sent".
func authEmailRouter(t *testing.T, db *gorm.DB, mailer utils.Mailer) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	validate := validator.New()
	r := gin.New()
	rg := r.Group("/api")
	router.AuthAccountRoutes(rg, db, validate, mailer)
	return r
}

// relaxAuthRateLimit widens the per-IP/per-identifier budgets so a test that
// makes several same-email calls is not throttled by the shared auth limiter.
func relaxAuthRateLimit(t *testing.T) {
	t.Helper()
	t.Setenv("RATE_LIMIT_IP_PER_MIN", "1000")
	t.Setenv("RATE_LIMIT_IP_BURST", "1000")
	t.Setenv("RATE_LIMIT_IDENTIFIER_PER_MIN", "1000")
	t.Setenv("RATE_LIMIT_IDENTIFIER_BURST", "1000")
}

// registerClientAs registers a client with the given unique email/username.
func registerClientAs(t *testing.T, r *gin.Engine, email, username string) authResponseEnvelope {
	t.Helper()
	body := fmt.Sprintf(
		`{"first_name":"Ana","last_name":"Lopez","email":%q,"phone":"5550100","password":"password123","username":%q,"user_type":"client","terms_accepted":true}`,
		email, username)
	w := do(r, http.MethodPost, "/api/auth-accounts/register", "", body)
	assert.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var resp authResponseEnvelope
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse register response: %v", err)
	}
	assert.NotEmpty(t, resp.RefreshToken)
	return resp
}

// tokenFromMessage pulls the raw token out of the email's plain-text body.
func tokenFromMessage(t *testing.T, msg utils.Message) string {
	t.Helper()
	i := strings.Index(msg.Text, "token=")
	if i < 0 {
		t.Fatalf("no token in email body:\n%s", msg.Text)
	}
	raw := msg.Text[i+len("token="):]
	if j := strings.IndexAny(raw, "\r\n \t"); j >= 0 {
		raw = raw[:j]
	}
	dec, err := url.QueryUnescape(raw)
	if err != nil {
		t.Fatalf("token is not URL-encoded: %v", err)
	}
	if dec == "" {
		t.Fatal("empty token in email")
	}
	return dec
}

// mustLastMessage returns the most recent email or fails.
func mustLastMessage(t *testing.T, m *utils.FakeMailer) utils.Message {
	t.Helper()
	msg, ok := m.Last()
	if !ok {
		t.Fatal("expected an email to have been sent, but none was")
	}
	return msg
}

// TestRegister_SendsVerificationEmail — registration mints a verification token
// and emails a deep link containing it (HTML + text, Spanish, gold accent).
func TestRegister_SendsVerificationEmail(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	mailer := utils.NewFakeMailer()
	r := authEmailRouter(t, db, mailer)

	resp := registerClientAs(t, r, "verify.me@test.com", "verifyme")

	msgs := mailer.Messages()
	assert.Len(t, msgs, 1, "registration must send exactly one email")
	msg := msgs[0]
	assert.Equal(t, "verify.me@test.com", msg.To)
	assert.Contains(t, msg.Subject, "Verifica")
	assert.Contains(t, msg.Text, "yottu://verify-email", "the link must be the app deep link")
	assert.Contains(t, msg.HTML, "#CFB53B", "HTML must carry the Yottu gold accent")
	assert.NotEmpty(t, msg.HTML, "an HTML alternative must be present")

	token := tokenFromMessage(t, msg)
	assert.NotEmpty(t, token)

	var stored models.AuthToken
	err := db.Where("user_type = ? AND user_id = ? AND purpose = ?", "client", resp.User.ID, models.TokenPurposeEmailVerification).
		First(&stored).Error
	assert.NoError(t, err, "the token row must exist")
	assert.Equal(t, utils.HashAuthToken(token), stored.TokenHash, "only the hash is stored")
	assert.NotEqual(t, token, stored.TokenHash, "the raw token must never be persisted")
	assert.Nil(t, stored.UsedAt)
	assert.True(t, stored.ExpiresAt.After(time.Now().Add(23*time.Hour)), "verification TTL ~24h")

	var account models.AuthAccount
	assert.NoError(t, db.Where("provider = ? AND user_type = ? AND client_id = ?", "local", "client", resp.User.ID).First(&account).Error)
	assert.Nil(t, account.EmailVerifiedAt, "a fresh account starts unverified")
}

// TestVerifyEmail_SuccessThenReuseFails — verification succeeds once, stamps
// EmailVerifiedAt, and the same token is rejected on reuse.
func TestVerifyEmail_SuccessThenReuseFails(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	mailer := utils.NewFakeMailer()
	r := authEmailRouter(t, db, mailer)

	resp := registerClientAs(t, r, "verify.once@test.com", "verifyonce")
	token := tokenFromMessage(t, mustLastMessage(t, mailer))

	body := fmt.Sprintf(`{"token":%q}`, token)
	w := do(r, http.MethodPost, "/api/auth-accounts/verify-email", "", body)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var account models.AuthAccount
	assert.NoError(t, db.Where("provider = ? AND client_id = ?", "local", resp.User.ID).First(&account).Error)
	assert.NotNil(t, account.EmailVerifiedAt, "EmailVerifiedAt must be stamped")

	var stored models.AuthToken
	assert.NoError(t, db.Where("token_hash = ?", utils.HashAuthToken(token)).First(&stored).Error)
	assert.NotNil(t, stored.UsedAt, "the token must be marked used")

	// Reuse → 400 with the uniform message.
	w = do(r, http.MethodPost, "/api/auth-accounts/verify-email", "", body)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "invalid or expired token", errBody(t, w.Body.String()))
}

// TestVerifyEmail_ExpiredRejected — an aged-out token is 400.
func TestVerifyEmail_ExpiredRejected(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	mailer := utils.NewFakeMailer()
	r := authEmailRouter(t, db, mailer)

	registerClientAs(t, r, "verify.expired@test.com", "verifyexpired")
	token := tokenFromMessage(t, mustLastMessage(t, mailer))

	db.Model(&models.AuthToken{}).
		Where("token_hash = ?", utils.HashAuthToken(token)).
		Update("expires_at", time.Now().Add(-time.Minute))

	w := do(r, http.MethodPost, "/api/auth-accounts/verify-email", "", fmt.Sprintf(`{"token":%q}`, token))
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "invalid or expired token", errBody(t, w.Body.String()))
}

// TestVerifyEmail_UnknownTokenRejected — a token that was never issued is 400.
func TestVerifyEmail_UnknownTokenRejected(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	r := authEmailRouter(t, db, utils.NewFakeMailer())

	w := do(r, http.MethodPost, "/api/auth-accounts/verify-email", "", `{"token":"garbage-never-issued"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "invalid or expired token", errBody(t, w.Body.String()))
}

// TestResendVerification_Always200 — unknown emails, unverified accounts and
// already-verified accounts ALL answer 200; only a genuinely unverified account
// triggers a new email.
func TestResendVerification_Always200(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	mailer := utils.NewFakeMailer()
	r := authEmailRouter(t, db, mailer)

	// Unknown email → 200, nothing sent.
	w := do(r, http.MethodPost, "/api/auth-accounts/resend-verification", "", `{"email":"nobody@test.com"}`)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 0, mailer.CallCount(), "unknown email must not send")

	// Unverified account → 200, a fresh token/email.
	registerClientAs(t, r, "resend.me@test.com", "resendme")
	first := tokenFromMessage(t, mustLastMessage(t, mailer))

	w = do(r, http.MethodPost, "/api/auth-accounts/resend-verification", "", `{"email":"resend.me@test.com"}`)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 2, mailer.CallCount(), "resend must send for an unverified account")
	second := tokenFromMessage(t, mustLastMessage(t, mailer))
	assert.NotEqual(t, first, second, "resend must mint a new token")

	// The prior token is retired when a new one is issued.
	var retired models.AuthToken
	assert.NoError(t, db.Where("token_hash = ?", utils.HashAuthToken(first)).First(&retired).Error)
	assert.NotNil(t, retired.UsedAt, "issuing a new token must invalidate the previous one")

	// Verify, then resend → still 200, no new email.
	verifyBody := fmt.Sprintf(`{"token":%q}`, second)
	assert.Equal(t, http.StatusOK, do(r, http.MethodPost, "/api/auth-accounts/verify-email", "", verifyBody).Code)
	before := mailer.CallCount()
	w = do(r, http.MethodPost, "/api/auth-accounts/resend-verification", "", `{"email":"resend.me@test.com"}`)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, before, mailer.CallCount(), "already-verified must not send")
}

// TestForgotPassword_Always200SendsOnlyWhenExists — 200 for unknown emails with
// no send; 200 + reset email for a real account.
func TestForgotPassword_Always200SendsOnlyWhenExists(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	mailer := utils.NewFakeMailer()
	r := authEmailRouter(t, db, mailer)

	w := do(r, http.MethodPost, "/api/auth-accounts/forgot-password", "", `{"email":"ghost@test.com"}`)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, 0, mailer.CallCount(), "unknown email must not send")

	registerClientAs(t, r, "reset.me@test.com", "resetme")
	baseline := mailer.CallCount() // registration verification email

	w = do(r, http.MethodPost, "/api/auth-accounts/forgot-password", "", `{"email":"reset.me@test.com"}`)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, baseline+1, mailer.CallCount(), "a real account must receive a reset email")

	last := mustLastMessage(t, mailer)
	assert.Equal(t, "reset.me@test.com", last.To)
	assert.Contains(t, last.Subject, "Restablece")
	assert.Contains(t, last.Text, "yottu://reset-password")

	token := tokenFromMessage(t, last)
	var stored models.AuthToken
	assert.NoError(t, db.Where("token_hash = ?", utils.HashAuthToken(token)).First(&stored).Error)
	assert.Equal(t, models.TokenPurposePasswordReset, stored.Purpose)
	assert.True(t, stored.ExpiresAt.Before(time.Now().Add(2*time.Hour)), "reset TTL ~1h")
}

// TestResetPassword_ChangesPasswordSingleUseAndRevokesSessions — the full
// recovery flow: new password works, the old one stops working, the token is
// single-use, and EVERY pre-reset session is revoked.
func TestResetPassword_ChangesPasswordSingleUseAndRevokesSessions(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	mailer := utils.NewFakeMailer()
	r := authEmailRouter(t, db, mailer)

	reg := registerClientAs(t, r, "change.me@test.com", "changeme")

	// A second session (another device).
	loginBody := `{"identifier":"change.me@test.com","password":"password123","user_type":"client"}`
	w := do(r, http.MethodPost, "/api/auth-accounts/login", "", loginBody)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())
	_, secondRefresh := parseTokenPair(t, w)

	// Request the reset link.
	assert.Equal(t, http.StatusOK,
		do(r, http.MethodPost, "/api/auth-accounts/forgot-password", "", `{"email":"change.me@test.com"}`).Code)
	resetToken := tokenFromMessage(t, mustLastMessage(t, mailer))

	// Reset.
	resetBody := fmt.Sprintf(`{"token":%q,"password":"newpassword456"}`, resetToken)
	w = do(r, http.MethodPost, "/api/auth-accounts/reset-password", "", resetBody)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Old password no longer works; the new one does.
	w = do(r, http.MethodPost, "/api/auth-accounts/login", "", loginBody)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "the old password must stop working")
	newLoginBody := `{"identifier":"change.me@test.com","password":"newpassword456","user_type":"client"}`
	w = do(r, http.MethodPost, "/api/auth-accounts/login", "", newLoginBody)
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	// Both pre-reset sessions are revoked.
	assert.Equal(t, http.StatusUnauthorized, doRefresh(t, r, reg.RefreshToken).Code, "register session must be revoked")
	assert.Equal(t, http.StatusUnauthorized, doRefresh(t, r, secondRefresh).Code, "second session must be revoked")

	// The token is single-use.
	w = do(r, http.MethodPost, "/api/auth-accounts/reset-password", "", resetBody)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "invalid or expired token", errBody(t, w.Body.String()))
}

// TestResetPassword_InvalidTokenRejected — a bogus token is 400 and changes
// nothing.
func TestResetPassword_InvalidTokenRejected(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	r := authEmailRouter(t, db, utils.NewFakeMailer())

	w := do(r, http.MethodPost, "/api/auth-accounts/reset-password", "", `{"token":"never-issued","password":"newpassword456"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
	assert.Equal(t, "invalid or expired token", errBody(t, w.Body.String()))
}

// TestResetPassword_RejectsWeakPassword — the new password obeys the SAME
// min-length rule as registration.
func TestResetPassword_RejectsWeakPassword(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	r := authEmailRouter(t, db, utils.NewFakeMailer())

	w := do(r, http.MethodPost, "/api/auth-accounts/reset-password", "", `{"token":"x","password":"123"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
}

// TestRegistrationSucceedsWhenMailerErrors — the verification email is
// best-effort: a provider failure must NOT fail registration.
func TestRegistrationSucceedsWhenMailerErrors(t *testing.T) {
	relaxAuthRateLimit(t)
	db := setupAuthRefreshDB(t)
	mailer := utils.NewFakeMailer()
	mailer.SendErr = errors.New("smtp offline")
	r := authEmailRouter(t, db, mailer)

	resp := registerClientAs(t, r, "mail.fails@test.com", "mailfails")
	assert.Equal(t, 1, mailer.CallCount(), "the send must have been attempted")

	var count int64
	db.Model(&models.Client{}).Where("email = ?", "mail.fails@test.com").Count(&count)
	assert.Equal(t, int64(1), count, "registration must succeed despite the mail failure")

	// The token still exists, so the user can resend / retry.
	var tokenCount int64
	db.Model(&models.AuthToken{}).Where("user_type = ? AND user_id = ?", "client", resp.User.ID).Count(&tokenCount)
	assert.Equal(t, int64(1), tokenCount, "the token is minted even if delivery fails")
}

// TestMailerFactory_DevFallbackAndConfig — no SMTP_HOST selects the log-only dev
// mailer (never an error); a configured host selects SMTP; bad config errors.
func TestMailerFactory_DevFallbackAndConfig(t *testing.T) {
	t.Run("unconfigured → dev (log-only), Send never fails", func(t *testing.T) {
		t.Setenv("SMTP_HOST", "")
		m, err := utils.MailerFromEnv()
		assert.NoError(t, err)
		assert.Equal(t, "dev (log-only)", m.Mode())
		assert.NoError(t, m.Send(context.Background(), utils.Message{
			To: "anyone@test.com", Subject: "Prueba", Text: "yottu://verify-email?token=abc",
		}), "the dev mailer must log, not fail")
	})

	t.Run("configured host → smtp", func(t *testing.T) {
		t.Setenv("SMTP_HOST", "smtp.example.com")
		t.Setenv("SMTP_PORT", "465")
		t.Setenv("SMTP_FROM", "no-reply@yottu.app")
		t.Setenv("SMTP_TLS_MODE", "tls")
		m, err := utils.MailerFromEnv()
		assert.NoError(t, err)
		assert.Equal(t, "smtp", m.Mode())
	})

	t.Run("invalid TLS mode errors", func(t *testing.T) {
		t.Setenv("SMTP_HOST", "smtp.example.com")
		t.Setenv("SMTP_FROM", "no-reply@yottu.app")
		t.Setenv("SMTP_TLS_MODE", "bogus")
		_, err := utils.MailerFromEnv()
		assert.Error(t, err)
	})

	t.Run("invalid port errors", func(t *testing.T) {
		t.Setenv("SMTP_HOST", "smtp.example.com")
		t.Setenv("SMTP_FROM", "no-reply@yottu.app")
		t.Setenv("SMTP_PORT", "not-a-port")
		_, err := utils.MailerFromEnv()
		assert.Error(t, err)
	})
}

// TestEmailRoutes_BehindRateLimiter — the new routes share the auth limiter, so
// an attacker cannot hammer them unbounded.
func TestEmailRoutes_BehindRateLimiter(t *testing.T) {
	t.Setenv("RATE_LIMIT_IP_PER_MIN", "1")
	t.Setenv("RATE_LIMIT_IP_BURST", "2")
	t.Setenv("RATE_LIMIT_IDENTIFIER_PER_MIN", "1000")
	t.Setenv("RATE_LIMIT_IDENTIFIER_BURST", "1000")

	db := setupAuthRefreshDB(t)
	r := authEmailRouter(t, db, utils.NewFakeMailer())

	for i := 0; i < 2; i++ {
		w := do(r, http.MethodPost, "/api/auth-accounts/forgot-password", "", `{"email":"rate@test.com"}`)
		assert.Equal(t, http.StatusOK, w.Code)
	}
	assert429(t, do(r, http.MethodPost, "/api/auth-accounts/forgot-password", "", `{"email":"rate@test.com"}`))
}
