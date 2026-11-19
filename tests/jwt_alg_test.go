package tests

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Nina-99/yotuvia-api/middleware"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
)

// testJWTSecret is deliberately longer than any HMAC output size so the test
// focuses on the signing-METHOD guard, not key length.
const testJWTSecret = "yotuvia-test-secret-0123456789abcdef0123456789abcdef"

// validTestClaims mirrors what utils.GenerateJWT encodes.
func validTestClaims() *utils.JWTClaim {
	return &utils.JWTClaim{
		UserID:   7,
		CenterID: 1,
		Role:     "beauty_center",
		Email:    "owner@test.com",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(72 * time.Hour)),
		},
	}
}

// signToken signs claims with the given method/key, bypassing utils.GenerateJWT
// so forged algorithms (none, RS256, HS384, wrong-secret HS256) can be built.
func signToken(claims *utils.JWTClaim, method jwt.SigningMethod, key interface{}) string {
	signed, err := jwt.NewWithClaims(method, claims).SignedString(key)
	if err != nil {
		panic(err)
	}
	return signed
}

// GenerateTestJWT issues an HS256 token through the production code path.
func GenerateTestJWT(userID, centerID uint, role string) (string, error) {
	return utils.GenerateJWT(userID, centerID, role, "test@test.com", "", "test-session")
}

// jwtProbeRouter mounts JWTAuthMiddleware in front of a probe handler so tests
// assert both the status code and that the handler never ran on 401.
func jwtProbeRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(middleware.JWTAuthMiddleware())
	r.GET("/probe", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"ok": true, "userID": ctx.GetUint("clientID")})
	})
	return r
}

func TestJWTAlgorithmGuard(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r := jwtProbeRouter()

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate RSA key: %v", err)
	}

	validHS256, err := utils.GenerateJWT(7, 1, "beauty_center", "owner@test.com", "", "test-session")
	if err != nil {
		t.Fatalf("failed to generate valid token: %v", err)
	}

	table := []struct {
		name       string
		token      string
		wantStatus int
	}{
		{"valid HS256 token passes", validHS256, http.StatusOK},
		{"alg=none rejected", signToken(validTestClaims(), jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType), http.StatusUnauthorized},
		{"RS256 signature rejected", signToken(validTestClaims(), jwt.SigningMethodRS256, rsaKey), http.StatusUnauthorized},
		{"HS384 with same secret rejected", signToken(validTestClaims(), jwt.SigningMethodHS384, []byte(testJWTSecret)), http.StatusUnauthorized},
		{"HS256 with wrong secret rejected", signToken(validTestClaims(), jwt.SigningMethodHS256, []byte("some-other-secret-that-is-also-quite-long")), http.StatusUnauthorized},
	}

	for _, tt := range table {
		t.Run(tt.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", "/probe", nil)
			req.Header.Set("Authorization", "Bearer "+tt.token)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if tt.wantStatus == http.StatusOK {
				// Handler ran and the middleware extracted the identity.
				var body struct {
					Ok     bool `json:"ok"`
					UserID uint `json:"userID"`
				}
				assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
				assert.True(t, body.Ok)
				assert.Equal(t, uint(7), body.UserID)
			} else {
				// Handler must NOT have run — probe body must be absent.
				assert.NotContains(t, w.Body.String(), `"ok":true`)
			}
		})
	}
}
