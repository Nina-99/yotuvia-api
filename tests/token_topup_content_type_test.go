package tests

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/stretchr/testify/assert"
)

// POST /api/tokens/topup is how a centre buys Yotuvia tokens — there is no
// payment gateway, so this endpoint IS the purchase flow. It used to branch on
//
//	if len(contentType) > 0 && contentType[:30] == "multipart/form-data" {
//
// which has two defects at once:
//   - "application/json" is 16 bytes, so the fixed [:30] slice PANICS and the
//     caller gets a 500 (recovered by Gin in production, a crash in tests);
//   - a real multipart header is "multipart/form-data; boundary=...", whose
//     first 30 bytes are "multipart/form-data; boundary=", so the comparison is
//     always false and the multipart branch was unreachable dead code.
func TestTokenTopUp_JsonContentTypeDoesNotPanicAndCreatesTopUp(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := authzTestRouter(t)
	assert.NoError(t, db.Create(&models.BeautyCenter{Name: "Centre One", Email: "one@test.com", Currency: "BOB"}).Error)

	token := roleToken(t, 1, 1, "beauty_center")
	res := do(r, http.MethodPost, "/api/tokens/topup", token, `{"amount_bs":100,"receipt_url":"http://x/y.png"}`)

	if res.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201 (body: %s)", res.Code, res.Body.String())
	}

	var count int64
	db.Model(&models.TokenTopUp{}).Count(&count)
	if count != 1 {
		t.Fatalf("topup rows = %d, want 1", count)
	}
}

// A multipart request must actually reach the multipart branch. Without a file
// that branch answers "file is required"; routing it into ShouldBindJSON
// instead produced a JSON parsing error, which is what the old comparison did.
func TestTokenTopUp_MultipartReachesTheMultipartBranch(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	r, db := authzTestRouter(t)
	assert.NoError(t, db.Create(&models.BeautyCenter{Name: "Centre One", Email: "one@test.com", Currency: "BOB"}).Error)

	token := roleToken(t, 1, 1, "beauty_center")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	assert.NoError(t, writer.WriteField("amount_bs", "100"))
	assert.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/tokens/topup", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	r.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (body: %s)", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "file is required") {
		t.Fatalf("body = %s, want the multipart branch's \"file is required\"", res.Body.String())
	}
}
