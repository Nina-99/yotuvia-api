package tests

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nina-99/yotuvia-api/models"
	"github.com/Nina-99/yotuvia-api/utils"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// Minimal valid image payloads for DetectContentType sniffing. The extension
// served MUST come from the sniffed MIME type, never from the client filename.
var jpegBytes = []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00, 0x01}
var pngBytes = []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}

// over5MBBytes carries JPEG magic but exceeds the 5MB cap by one byte, so the
// sniff would pass while the size gate must reject it with 413.
var over5MBBytes = func() []byte {
	b := make([]byte, 5*1024*1024+1)
	copy(b, jpegBytes)
	return b
}()

var textBytes = []byte("hello world this is definitely not an image")

// uploadAvatar POSTs a multipart form with one file part to /api/client/avatar
// and returns the recorder. Mirrors the production client flow (field "image",
// bearer token) and CreateFormFile usage.
func uploadAvatar(t *testing.T, r *gin.Engine, token, filename string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, err := mw.CreateFormFile("image", filename)
	if err != nil {
		t.Fatalf("failed to create form file: %v", err)
	}
	if _, err := part.Write(content); err != nil {
		t.Fatalf("failed to write part content: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("failed to close multipart writer: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, "/api/client/avatar", &buf)
	if err != nil {
		t.Fatalf("failed to build request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// avatarURLFrom extracts the avatar_url field from an upload response.
func avatarURLFrom(t *testing.T, body string) string {
	t.Helper()
	var parsed struct {
		AvatarURL string `json:"avatar_url"`
	}
	_ = json.Unmarshal([]byte(body), &parsed)
	return parsed.AvatarURL
}

// TestClientAvatar_UploadValidation — spec "Avatar Upload Endpoint": valid
// image → 200 + URL under uploads/clients/{id}/; non-image → 400; >5MB → 413;
// missing file → 400. No rejected upload may leave a file on disk.
func TestClientAvatar_UploadValidation(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	t.Cleanup(func() { os.RemoveAll("uploads/clients") })

	t.Run("jpeg upload returns 200 with avatar_url under own client dir", func(t *testing.T) {
		r, _ := authzTestRouter(t)
		token := roleToken(t, 5, 0, "client")

		w := uploadAvatar(t, r, token, "photo.jpg", jpegBytes)

		assert.Equal(t, http.StatusOK, w.Code)
		url := avatarURLFrom(t, w.Body.String())
		assert.True(t, strings.HasPrefix(url, "/uploads/clients/5/"), "avatar_url %q must live under own client dir", url)
		assert.True(t, strings.HasSuffix(url, ".jpg"), "jpeg upload must yield a .jpg url, got %q", url)
		assert.FileExists(t, filepath.Join("uploads", "clients", "5", filepath.Base(url)), "uploaded file must exist on disk")
	})

	t.Run("jpeg with spoofed extension is stored with sniffed .jpg ext", func(t *testing.T) {
		r, _ := authzTestRouter(t)
		token := roleToken(t, 5, 0, "client")

		w := uploadAvatar(t, r, token, "avatar.txt", jpegBytes)

		assert.Equal(t, http.StatusOK, w.Code)
		url := avatarURLFrom(t, w.Body.String())
		assert.True(t, strings.HasSuffix(url, ".jpg"), "ext must come from sniffed MIME, not client filename, got %q", url)
	})

	t.Run("png upload returns 200 and stores .png", func(t *testing.T) {
		r, _ := authzTestRouter(t)
		token := roleToken(t, 5, 0, "client")

		w := uploadAvatar(t, r, token, "photo.notpng", pngBytes)

		assert.Equal(t, http.StatusOK, w.Code)
		url := avatarURLFrom(t, w.Body.String())
		assert.True(t, strings.HasSuffix(url, ".png"), "png upload must yield a .png url, got %q", url)
		assert.FileExists(t, filepath.Join("uploads", "clients", "5", filepath.Base(url)))
	})

	t.Run("text file is rejected 400 and nothing stored", func(t *testing.T) {
		r, _ := authzTestRouter(t)
		token := roleToken(t, 6, 0, "client")

		w := uploadAvatar(t, r, token, "notes.txt", textBytes)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.NoDirExists(t, filepath.Join("uploads", "clients", "6"), "rejected upload must not be stored")
	})

	t.Run("file over 5MB is rejected 413 and nothing stored", func(t *testing.T) {
		r, _ := authzTestRouter(t)
		token := roleToken(t, 7, 0, "client")

		w := uploadAvatar(t, r, token, "big.jpg", over5MBBytes)

		assert.Equal(t, http.StatusRequestEntityTooLarge, w.Code)
		assert.NoDirExists(t, filepath.Join("uploads", "clients", "7"), "oversized upload must not be stored")
	})

	t.Run("multipart without image field is rejected 400", func(t *testing.T) {
		r, _ := authzTestRouter(t)
		token := roleToken(t, 8, 0, "client")

		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		if err := mw.WriteField("other", "value"); err != nil {
			t.Fatalf("failed to write field: %v", err)
		}
		_ = mw.Close()
		req, _ := http.NewRequest(http.MethodPost, "/api/client/avatar", &buf)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.NoDirExists(t, filepath.Join("uploads", "clients", "8"))
	})
}

// TestClientAvatar_Authz — spec api-authz "Client Avatar Upload Authorization":
// role client + clientID claim only; personnel/beauty_center → 403; missing or
// invalid token → 401. Denied requests must not store anything.
func TestClientAvatar_Authz(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()
	t.Cleanup(func() { os.RemoveAll("uploads/clients") })

	t.Run("client uploads own avatar stored under claim only", func(t *testing.T) {
		r, _ := authzTestRouter(t)
		token := roleToken(t, 5, 0, "client")

		w := uploadAvatar(t, r, token, "me.jpg", jpegBytes)

		assert.Equal(t, http.StatusOK, w.Code)
		url := avatarURLFrom(t, w.Body.String())
		assert.True(t, strings.HasPrefix(url, "/uploads/clients/5/"), "avatar must bind to claim 5, got %q", url)
	})

	t.Run("personnel token is denied 403 and nothing stored", func(t *testing.T) {
		r, _ := authzTestRouter(t)
		token := roleToken(t, 9, 1, "personnel")

		w := uploadAvatar(t, r, token, "x.jpg", jpegBytes)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
		assert.NoDirExists(t, filepath.Join("uploads", "clients", "9"))
	})

	t.Run("beauty_center token is denied 403 and nothing stored", func(t *testing.T) {
		r, _ := authzTestRouter(t)
		token := roleToken(t, 9, 1, "beauty_center")

		w := uploadAvatar(t, r, token, "x.jpg", jpegBytes)

		assert.Equal(t, http.StatusForbidden, w.Code)
		assert.Equal(t, "forbidden", errBody(t, w.Body.String()))
		assert.NoDirExists(t, filepath.Join("uploads", "clients", "9"))
	})

	t.Run("no token is 401 and nothing stored", func(t *testing.T) {
		r, _ := authzTestRouter(t)
		w := uploadAvatar(t, r, "", "x.jpg", jpegBytes)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.NoDirExists(t, filepath.Join("uploads", "clients", "1"))
	})

	t.Run("invalid token is 401 and nothing stored", func(t *testing.T) {
		r, _ := authzTestRouter(t)
		w := uploadAvatar(t, r, "not-a-real-token", "x.jpg", jpegBytes)

		assert.Equal(t, http.StatusUnauthorized, w.Code)
		assert.NoDirExists(t, filepath.Join("uploads", "clients", "1"))
	})
}

// fullUpdateBody builds a complete UpdateClientRequest, optionally including
// avatar_url and password so each persistence scenario controls exactly one
// variable (all other required fields stay present).
func fullUpdateBody(avatarURL, password string) string {
	return `{"first_name":"Nina","last_name":"Test","username":"nina","email":"nina@test.com","password":"` +
		password + `","phone":"5550001","is_active":true,"avatar_url":"` + avatarURL + `"}`
}

// TestClientAvatar_Persistence — spec "Avatar URL Persistence": PUT persists
// avatar_url; omitting it preserves the existing value; empty password keeps the
// stored hash; all other required fields remain enforced (PUT without them 400).
func TestClientAvatar_Persistence(t *testing.T) {
	t.Setenv("JWT_SECRET", testJWTSecret)
	utils.Log = zap.NewNop().Sugar()

	t.Run("PUT persists avatar_url and GET /me returns it", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedClient(t, db, 1, "nina", "nina@test.com")
		token := roleToken(t, 1, 0, "client")

		w := do(r, http.MethodPut, "/api/client/1", token, fullUpdateBody("/uploads/clients/1/a1.jpg", "newpass"))

		assert.Equal(t, http.StatusOK, w.Code)

		me := do(r, http.MethodGet, "/api/client/1", token, "")
		assert.Equal(t, http.StatusOK, me.Code)
		assert.Contains(t, me.Body.String(), `"avatar_url":"/uploads/clients/1/a1.jpg"`)
	})

	t.Run("PUT without avatar_url preserves existing value", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedClient(t, db, 1, "nina", "nina@test.com")
		token := roleToken(t, 1, 0, "client")

		first := do(r, http.MethodPut, "/api/client/1", token, fullUpdateBody("/uploads/clients/1/a2.jpg", "newpass"))
		assert.Equal(t, http.StatusOK, first.Code)

		second := do(r, http.MethodPut, "/api/client/1", token, fullUpdateBody("", "newpass"))
		assert.Equal(t, http.StatusOK, second.Code)

		me := do(r, http.MethodGet, "/api/client/1", token, "")
		assert.Equal(t, http.StatusOK, me.Code)
		assert.Contains(t, me.Body.String(), `"avatar_url":"/uploads/clients/1/a2.jpg"`, "omitted avatar_url must be preserved")
	})

	t.Run("PUT with empty password keeps existing hash", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedClient(t, db, 1, "nina", "nina@test.com")
		hash, err := utils.HashPassword("chosen-pass")
		if err != nil {
			t.Fatalf("failed to hash: %v", err)
		}
		if err := db.Model(&models.Client{}).Where("id = ?", 1).Update("password", hash).Error; err != nil {
			t.Fatalf("failed to set hash: %v", err)
		}
		token := roleToken(t, 1, 0, "client")

		w := do(r, http.MethodPut, "/api/client/1", token, fullUpdateBody("", ""))

		assert.Equal(t, http.StatusOK, w.Code)

		var stored models.Client
		if err := db.First(&stored, 1).Error; err != nil {
			t.Fatalf("failed to reload client: %v", err)
		}
		assert.Equal(t, hash, stored.Password, "empty password must not re-hash or replace the stored hash")
	})

	t.Run("PUT missing required first_name is 400", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedClient(t, db, 1, "nina", "nina@test.com")
		token := roleToken(t, 1, 0, "client")

		body := `{"last_name":"Test","username":"nina","email":"nina@test.com","phone":"5550001","is_active":true}`
		w := do(r, http.MethodPut, "/api/client/1", token, body)

		assert.Equal(t, http.StatusBadRequest, w.Code)
		assert.Equal(t, "validation failed", errBody(t, w.Body.String()))
	})

	// Cross-client PUT remains owner-only (approval: existing authz intact).
	t.Run("cross-client PUT with avatar_url is still 403", func(t *testing.T) {
		r, db := authzTestRouter(t)
		seedClient(t, db, 1, "nina", "nina@test.com")
		other := roleToken(t, 9, 0, "client")

		w := do(r, http.MethodPut, "/api/client/1", other, fullUpdateBody("/uploads/clients/1/evil.jpg", "x"))

		assert.Equal(t, http.StatusForbidden, w.Code)

		var stored models.Client
		if err := db.First(&stored, 1).Error; err != nil {
			t.Fatalf("failed to reload client: %v", err)
		}
		assert.Empty(t, stored.AvatarURL, "cross-client PUT must not mutate avatar_url")
	})
}