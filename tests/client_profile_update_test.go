package tests

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestClientProfileUpdate_PhonePersists — the client profile edit MUST persist
// the phone. The DTO always validated it (`required`) but the service never
// assigned it, so an edited phone was silently dropped. The value carries the
// country-code prefix produced by the mobile PhoneInput ("+591 71234567").
func TestClientProfileUpdate_PhonePersists(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	r, db := authzTestRouter(t)
	seedClient(t, db, 1, "phone-client", "phone-client@test.com")

	body := `{"first_name":"Ana","last_name":"Lopez","username":"phone-client",` +
		`"email":"phone-client@test.com","phone":"+591 71234567"}`

	w := do(r, http.MethodPut, "/api/client/1", roleToken(t, 1, 0, "client"), body)
	assert.Equal(t, http.StatusOK, w.Code)

	var stored models.Client
	assert.NoError(t, db.First(&stored, 1).Error)
	assert.Equal(t, "+591 71234567", stored.Phone, "edited phone must reach storage")
}

// TestClientProfileUpdate_PhoneLengthCap — the phone column is varchar(20); the
// DTO now enforces a matching max, so an over-long value answers a clean 400
// instead of a database error surfaced as 500. Storage stays untouched.
func TestClientProfileUpdate_PhoneLengthCap(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	r, db := authzTestRouter(t)
	seedClient(t, db, 1, "cap-client", "cap-client@test.com")

	tooLong := "+591 " + strings.Repeat("9", 21) // 26 chars > the 20-char column
	body := `{"first_name":"Ana","last_name":"Lopez","username":"cap-client",` +
		`"email":"cap-client@test.com","phone":"` + tooLong + `"}`

	w := do(r, http.MethodPut, "/api/client/1", roleToken(t, 1, 0, "client"), body)
	assert.Equal(t, http.StatusBadRequest, w.Code)

	var stored models.Client
	assert.NoError(t, db.First(&stored, 1).Error)
	assert.Equal(t, "5550001", stored.Phone, "rejected update must not mutate storage")
}

// TestClientProfileUpdate_IsActiveNotClobbered — is_active is a pointer, so an
// edit that omits it leaves the stored flag untouched. The mobile client edit
// never sends it; with a bare bool the omitted field defaulted to false and
// silently deactivated the account on every save.
func TestClientProfileUpdate_IsActiveNotClobbered(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	t.Run("omitted is_active keeps the stored value", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedClient(t, db, 1, "active-client", "active-client@test.com")

		body := `{"first_name":"Ana","last_name":"Lopez","username":"active-client",` +
			`"email":"active-client@test.com","phone":"+591 71234567"}`

		w := do(r, http.MethodPut, "/api/client/1", roleToken(t, 1, 0, "client"), body)
		assert.Equal(t, http.StatusOK, w.Code)

		var stored models.Client
		assert.NoError(t, db.First(&stored, 1).Error)
		assert.True(t, stored.IsActive, "omitted is_active must not deactivate the client")
	})

	t.Run("explicit is_active is honoured", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedClient(t, db, 1, "toggle-client", "toggle-client@test.com")

		body := `{"first_name":"Ana","last_name":"Lopez","username":"toggle-client",` +
			`"email":"toggle-client@test.com","phone":"+591 71234567","is_active":false}`

		w := do(r, http.MethodPut, "/api/client/1", roleToken(t, 1, 0, "client"), body)
		assert.Equal(t, http.StatusOK, w.Code)

		var stored models.Client
		assert.NoError(t, db.First(&stored, 1).Error)
		assert.False(t, stored.IsActive, "explicit is_active must be applied")
	})
}
