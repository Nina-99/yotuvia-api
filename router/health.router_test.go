package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func healthTestRouter(t *testing.T, db *gorm.DB) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	HealthRoutes(r, db)
	return r
}

func healthTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}
	return db
}

// TestHealthz_Healthy — a reachable database answers 200 and reports both
// dimensions, so a deploy script can gate on the status code alone.
func TestHealthz_Healthy(t *testing.T) {
	r := healthTestRouter(t, healthTestDB(t))

	req, _ := http.NewRequest(http.MethodGet, "/api/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (body %s)", w.Code, w.Body.String())
	}
	if body := w.Body.String(); body != `{"db":"ok","status":"ok"}` {
		t.Fatalf("unexpected body: %s", body)
	}
}

// TestHealthz_Unconfigured — a nil DB is not healthy: failing closed is the
// whole point of the probe.
func TestHealthz_Unconfigured(t *testing.T) {
	r := healthTestRouter(t, nil)

	req, _ := http.NewRequest(http.MethodGet, "/api/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

// TestHealthz_DatabaseDown — a closed connection must surface as 503, not 200.
func TestHealthz_DatabaseDown(t *testing.T) {
	db := healthTestDB(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql.DB: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("failed to close test database: %v", err)
	}

	r := healthTestRouter(t, db)

	req, _ := http.NewRequest(http.MethodGet, "/api/healthz", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d (body %s)", w.Code, w.Body.String())
	}
}
