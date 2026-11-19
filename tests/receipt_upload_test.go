package tests

// Regression coverage for the stored-XSS chain that lets a low-privilege client
// take over a centre admin's session:
//
//  1. POST /api/upload/receipt accepted `receipt.html` (declaring
//     `Content-Type: image/jpeg`) and stored it with the CLIENT-controlled
//     extension, so the reverse proxy served it as text/html from the app
//     origin.
//  2. POST /api/reservation/:id/proof accepted ANY string as the receipt
//     reference, so the attacker could hand the admin a clickable link to that
//     stored HTML without ever leaving the product's own UI.
//
// Both halves must reject independently; either one alone breaks the chain.

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Nina-99/yotuvia-api/router"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// receiptTestRouter mounts the upload + reservation surfaces on their own
// engine. UPLOAD_DIR points at a temp dir so a rejected upload can be asserted
// against the real storage location.
func receiptTestRouter(t *testing.T) (*gin.Engine, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := setupRouterAuthzDB(t)
	validate := validator.New()

	dir := t.TempDir()
	t.Setenv("UPLOAD_DIR", dir)

	r := gin.New()
	rg := r.Group("/api")
	router.UploadRoutes(rg)
	router.ReservationRoutes(rg, db, validate)
	return r, dir
}

// uploadReceipt POSTs a multipart file part named "file" with an explicit
// Content-Type header, mirroring what an attacker controls.
func uploadReceipt(t *testing.T, r *gin.Engine, token, filename, contentType string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	h := make(map[string][]string)
	h["Content-Disposition"] = []string{`form-data; name="file"; filename="` + filename + `"`}
	h["Content-Type"] = []string{contentType}
	part, err := mw.CreatePart(h)
	if err != nil {
		t.Fatalf("failed to create part: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write content: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("failed to close writer: %v", err)
	}

	req, _ := http.NewRequest(http.MethodPost, "/api/upload/receipt", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func storedFiles(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("cannot read upload dir: %v", err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

// TestReceiptUpload_ExtensionComesFromSniffedBytes — the stored filename must
// never carry an extension the client asked for.
func TestReceiptUpload_ExtensionComesFromSniffedBytes(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	cases := []struct {
		name        string
		filename    string
		contentType string
		content     []byte
		status      int
		wantExt     string
	}{
		{
			name:        "html declared as jpeg is rejected and nothing is stored",
			filename:    "receipt.html",
			contentType: "image/jpeg",
			content:     []byte("<html><script>fetch('//evil')</script></html>"),
			status:      http.StatusBadRequest,
		},
		{
			name:        "svg declared as image is rejected",
			filename:    "receipt.svg",
			contentType: "image/png",
			content:     []byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`),
			status:      http.StatusBadRequest,
		},
		{
			name:        "real jpeg named .html is stored as .jpg",
			filename:    "receipt.html",
			contentType: "image/jpeg",
			content:     jpegBytes,
			status:      http.StatusOK,
			wantExt:     ".jpg",
		},
		{
			name:        "real png named .js is stored as .png",
			filename:    "receipt.js",
			contentType: "application/javascript",
			content:     pngBytes,
			status:      http.StatusOK,
			wantExt:     ".png",
		},
		{
			name:        "plain text is rejected",
			filename:    "note.jpg",
			contentType: "image/jpeg",
			content:     textBytes,
			status:      http.StatusBadRequest,
		},
		{
			name:        "oversized upload is rejected",
			filename:    "big.jpg",
			contentType: "image/jpeg",
			content:     over5MBBytes,
			status:      http.StatusBadRequest,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, dir := receiptTestRouter(t)
			token := roleToken(t, 5, 0, "client")

			w := uploadReceipt(t, r, token, tc.filename, tc.contentType, tc.content)

			assert.Equal(t, tc.status, w.Code, w.Body.String())

			files := storedFiles(t, dir)
			if tc.status != http.StatusOK {
				assert.Empty(t, files, "a rejected upload must not leave a file on disk")
				return
			}

			assert.Len(t, files, 1)
			assert.True(t, strings.HasSuffix(files[0], tc.wantExt),
				"stored file %q must end in %q (extension from sniffed bytes)", files[0], tc.wantExt)
			assert.NotContains(t, files[0], ".html")
			assert.NotContains(t, files[0], ".svg")

			var body struct {
				URL string `json:"url"`
			}
			_ = json.Unmarshal(w.Body.Bytes(), &body)
			assert.True(t, utils.IsStoredReceiptPath(body.URL),
				"returned url %q must be a path this API re-accepts", body.URL)
		})
	}
}

// TestReceiptUpload_Authz — anonymous callers cannot write to disk.
func TestReceiptUpload_Authz(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	r, dir := receiptTestRouter(t)

	w := uploadReceipt(t, r, "", "receipt.jpg", "image/jpeg", jpegBytes)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	assert.Empty(t, storedFiles(t, dir), "an unauthenticated upload must not touch the disk")
}

// TestUploadProof_RejectsForeignImageURL — the proof reference is validated
// before any reservation is loaded, so a bad URL is a 400 even for a
// nonexistent reservation, while a well-formed one falls through to the 404.
func TestUploadProof_RejectsForeignImageURL(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	validPath := "/uploads/receipts/46768122-5b32-48a4-8ccb-1bdc19350669.png"

	cases := []struct {
		name   string
		url    string
		status int
	}{
		{"absolute https url is rejected", "https://evil.example/pixel.png", http.StatusBadRequest},
		{"protocol-relative url is rejected", "//evil.example/pixel.png", http.StatusBadRequest},
		{"javascript scheme is rejected", "javascript:alert(1)", http.StatusBadRequest},
		{"data uri is rejected", "data:image/png;base64,AAAA", http.StatusBadRequest},
		{"stored html path is rejected", "/uploads/receipts/46768122-5b32-48a4-8ccb-1bdc19350669.html", http.StatusBadRequest},
		{"svg path is rejected", "/uploads/receipts/46768122-5b32-48a4-8ccb-1bdc19350669.svg", http.StatusBadRequest},
		{"path traversal is rejected", "/uploads/receipts/../secrets.png", http.StatusBadRequest},
		{"foreign upload dir is rejected", "/uploads/clients/5/avatar.png", http.StatusBadRequest},
		{"empty url is rejected", "", http.StatusBadRequest},
		{"well-formed own path passes validation and hits the missing row", validPath, http.StatusNotFound},
	}

	r, _ := receiptTestRouter(t)
	token := roleToken(t, 5, 0, "client")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := `{"image_url":` + quote(tc.url) + `}`
			w := do(r, http.MethodPost, "/api/reservation/424242/proof", token, body)

			assert.Equal(t, tc.status, w.Code, w.Body.String())
		})
	}
}

// quote wraps a string in JSON double quotes with the escapes needed for the
// path-shaped inputs in the table above.
func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
